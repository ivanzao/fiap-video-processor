//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

type client struct {
	t       *testing.T
	base    string
	mailpit string
	http    *http.Client
	token   string
	userID  string
	email   string
}

func newClient(t *testing.T) *client {
	t.Helper()
	c := &client{
		t: t, base: envOr("E2E_BASE_URL", "http://localhost:8080"), mailpit: envOr("E2E_MAILPIT_URL", "http://localhost:8025"),
		http: &http.Client{Timeout: 30 * time.Second},
	}
	c.awaitHealthy(60 * time.Second)
	return c
}

func (c *client) awaitHealthy(timeout time.Duration) {
	c.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res, err := c.http.Get(c.base + "/health")
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(time.Second)
	}
	c.t.Fatalf("stack at %s did not become healthy within %s", c.base, timeout)
}

func (c *client) do(method, url string, body io.Reader, headers map[string]string) (*http.Response, []byte) {
	c.t.Helper()
	req, err := http.NewRequest(method, url, body)
	require.NoError(c.t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := c.http.Do(req)
	require.NoError(c.t, err)
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(res.Body)
	require.NoError(c.t, err)
	return res, raw
}

func (c *client) postJSON(path string, payload any, authenticated bool) (int, map[string]any) {
	c.t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(c.t, err)
	headers := map[string]string{"Content-Type": "application/json"}
	if authenticated {
		c.identityHeaders(headers)
	}
	res, body := c.do(http.MethodPost, c.base+path, bytes.NewReader(raw), headers)
	return res.StatusCode, decodeObject(body)
}

func (c *client) getJSON(path string) (int, map[string]any) {
	c.t.Helper()
	headers := map[string]string{}
	c.identityHeaders(headers)
	res, body := c.do(http.MethodGet, c.base+path, nil, headers)
	return res.StatusCode, decodeObject(body)
}

func (c *client) identityHeaders(h map[string]string) {
	h["Authorization"] = "Bearer " + c.token
	h["X-User-Id"] = c.userID
	h["X-User-Email"] = c.email
}

func decodeObject(body []byte) map[string]any {
	var out map[string]any
	if len(body) == 0 || json.Unmarshal(body, &out) != nil {
		return map[string]any{}
	}
	return out
}

func (c *client) signUpAndLogin(email string) {
	const password = "correct-horse-battery"
	c.t.Helper()
	status, _ := c.postJSON("/auth/v1/signup", map[string]string{"email": email, "password": password}, false)
	require.Contains(c.t, []int{http.StatusCreated, http.StatusConflict}, status, "signup")
	status, body := c.postJSON("/auth/v1/login", map[string]string{"email": email, "password": password}, false)
	require.Equal(c.t, http.StatusOK, status, "login: %v", body)
	token, _ := body["token"].(string)
	require.NotEmpty(c.t, token)
	claims := decodeJWT(c.t, token)
	c.token, c.userID, c.email = token, claims["sub"].(string), claims["email"].(string)
}

func decodeJWT(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims
}

func (c *client) upload(filename, contentType string, content []byte) string {
	c.t.Helper()
	status, body := c.postJSON("/v1/videos", map[string]any{"filename": filename, "contentType": contentType, "sizeBytes": len(content)}, true)
	require.Equal(c.t, http.StatusCreated, status, "request upload: %v", body)
	uploadURL, _ := body["uploadUrl"].(string)
	headers := map[string]string{}
	for name, value := range body["uploadHeaders"].(map[string]any) {
		headers[name], _ = value.(string)
	}
	res, raw := c.do(http.MethodPut, uploadURL, bytes.NewReader(content), headers)
	require.Equal(c.t, http.StatusOK, res.StatusCode, "s3 put: %s", raw)
	videoID, _ := body["videoId"].(string)
	status, body = c.postJSON("/v1/videos/"+videoID+"/process", nil, true)
	require.Equal(c.t, http.StatusAccepted, status, "confirm: %v", body)
	require.Equal(c.t, "PENDING", body["status"])
	return videoID
}

func (c *client) awaitTerminal(videoID string, timeout time.Duration) map[string]any {
	c.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, body := c.getJSON("/v1/videos")
		require.Equal(c.t, http.StatusOK, status)
		items, _ := body["items"].([]any)
		for _, raw := range items {
			item, _ := raw.(map[string]any)
			if item["videoId"] == videoID && (item["status"] == "COMPLETED" || item["status"] == "FAILED") {
				return item
			}
		}
		time.Sleep(2 * time.Second)
	}
	c.t.Fatalf("video %s did not reach a terminal Status within %s", videoID, timeout)
	return nil
}

func (c *client) mailSubjects(to string) []string {
	c.t.Helper()
	res, raw := c.do(http.MethodGet, fmt.Sprintf("%s/api/v1/search?query=%s", c.mailpit, "to:"+to), nil, nil)
	require.Equal(c.t, http.StatusOK, res.StatusCode)
	var out struct {
		Messages []struct {
			Subject string `json:"Subject"`
		} `json:"messages"`
	}
	require.NoError(c.t, json.Unmarshal(raw, &out))
	subjects := make([]string, 0, len(out.Messages))
	for _, m := range out.Messages {
		subjects = append(subjects, m.Subject)
	}
	return subjects
}
