//go:build e2e

package e2e_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func freshUser() string {
	return fmt.Sprintf("e2e-%d@example.com", time.Now().UnixNano())
}

func TestVideoIsProcessedIntoFramesAndTheUserIsNotified(t *testing.T) {
	c := newClient(t)
	email := freshUser()
	c.signUpAndLogin(email)
	sample, err := os.ReadFile("testdata/sample.mp4")
	require.NoError(t, err)

	videoID := c.upload("sample.mp4", "video/mp4", sample)
	item := c.awaitTerminal(videoID, 2*time.Minute)

	require.Equal(t, "COMPLETED", item["status"], "item: %v", item)
	result, _ := item["result"].(map[string]any)
	assert.EqualValues(t, 3, result["frameCount"])

	status, body := c.getJSON("/v1/videos/" + videoID + "/download")
	require.Equal(t, http.StatusOK, status)
	res, raw := c.do(http.MethodGet, body["downloadUrl"].(string), nil, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	require.NoError(t, err)
	assert.Len(t, reader.File, 3)

	assert.Eventually(t, func() bool { return len(c.mailSubjects(email)) == 1 }, 20*time.Second, time.Second)
	assert.Equal(t, []string{"Seu vídeo sample.mp4 foi processado"}, c.mailSubjects(email))
}

func TestUnprocessableVideoFailsWithoutRetriesAndCannotBeDownloaded(t *testing.T) {
	c := newClient(t)
	email := freshUser()
	c.signUpAndLogin(email)

	videoID := c.upload("garbage.mp4", "video/mp4", []byte("definitely not a video"))
	item := c.awaitTerminal(videoID, 2*time.Minute)

	require.Equal(t, "FAILED", item["status"], "item: %v", item)
	assert.EqualValues(t, 1, item["attempts"])
	result, _ := item["result"].(map[string]any)
	assert.Contains(t, result["failureReason"], "unprocessable video")

	status, _ := c.getJSON("/v1/videos/" + videoID + "/download")
	assert.Equal(t, http.StatusConflict, status)

	assert.Eventually(t, func() bool { return len(c.mailSubjects(email)) == 1 }, 20*time.Second, time.Second)
	assert.Equal(t, []string{"Falha ao processar o vídeo garbage.mp4"}, c.mailSubjects(email))
}

func TestIdentityIsRequiredAndForeignVideosAreHidden(t *testing.T) {
	c := newClient(t)
	res, _ := c.do(http.MethodGet, c.base+"/v1/videos", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)

	owner := newClient(t)
	owner.signUpAndLogin(freshUser())
	videoID := owner.upload("garbage.mp4", "video/mp4", []byte("x"))

	other := newClient(t)
	other.signUpAndLogin(freshUser())
	status, _ := other.getJSON("/v1/videos/" + videoID + "/download")
	assert.Equal(t, http.StatusForbidden, status)
}
