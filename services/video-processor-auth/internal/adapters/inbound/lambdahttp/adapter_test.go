package lambdahttp_test

import (
	"encoding/base64"
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/adapters/inbound/lambdahttp"
)

func TestWrapTranslatesGatewayRequestAndResponse(t *testing.T) {
	var seen *http.Request
	var seenBody string
	handler := lambdahttp.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		b, _ := io.ReadAll(r.Body)
		seenBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	res, err := handler(t.Context(), events.APIGatewayV2HTTPRequest{
		RawPath:         "/auth/v1/signup",
		RawQueryString:  "debug=1",
		Headers:         map[string]string{"content-type": "application/json"},
		Body:            base64.StdEncoding.EncodeToString([]byte(`{"email":"a@b.co"}`)),
		IsBase64Encoded: true,
		RequestContext:  events.APIGatewayV2HTTPRequestContext{HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: http.MethodPost, SourceIP: "10.0.0.1"}},
	})

	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, seen.Method)
	assert.Equal(t, "/auth/v1/signup", seen.URL.Path)
	assert.Equal(t, "1", seen.URL.Query().Get("debug"))
	assert.Equal(t, "application/json", seen.Header.Get("Content-Type"))
	assert.Equal(t, "10.0.0.1", seen.RemoteAddr)
	assert.Equal(t, `{"email":"a@b.co"}`, seenBody)
	assert.Equal(t, http.StatusCreated, res.StatusCode)
	assert.Equal(t, "application/json", res.Headers["Content-Type"])
	assert.Equal(t, `{"ok":true}`, res.Body)
}

func TestWrapRejectsCorruptBase64Body(t *testing.T) {
	handler := lambdahttp.Wrap(http.NotFoundHandler())
	_, err := handler(t.Context(), events.APIGatewayV2HTTPRequest{
		RawPath: "/", Body: "%%%", IsBase64Encoded: true,
		RequestContext: events.APIGatewayV2HTTPRequestContext{HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: http.MethodPost}},
	})
	assert.Error(t, err)
}
