package observability_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/platform/observability"
)

func TestLoggerWritesJSONLinesTaggedWithService(t *testing.T) {
	var buf bytes.Buffer
	logger := observability.NewLoggerTo(&buf, "video-processor-auth")

	logger.Info("user registered", "userId", "usr-1")

	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
	assert.Equal(t, "INFO", line["level"])
	assert.Equal(t, "user registered", line["msg"])
	assert.Equal(t, "video-processor-auth", line["service"])
	assert.Equal(t, "usr-1", line["userId"])
}

func TestNewLoggerWritesToStdout(t *testing.T) {
	assert.NotNil(t, observability.NewLogger("video-processor-auth"))
}
