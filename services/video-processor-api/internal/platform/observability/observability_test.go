package observability_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/observability"
)

func TestLoggerWritesJSONLinesTaggedWithService(t *testing.T) {
	var buf bytes.Buffer
	logger := observability.NewLoggerTo(&buf, "video-processor-api")

	logger.Info("request accepted", "requestId", "req-1")

	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
	assert.Equal(t, "INFO", line["level"])
	assert.Equal(t, "request accepted", line["msg"])
	assert.Equal(t, "video-processor-api", line["service"])
	assert.Equal(t, "req-1", line["requestId"])
}

func TestSetupTracingWithoutEndpointIsANoOpThatStillShutsDown(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := observability.SetupTracing(t.Context(), "video-processor-api")

	require.NoError(t, err)
	assert.NoError(t, shutdown(t.Context()))
}

func TestSetupTracingWithEndpointBuildsAProviderWithTheServiceName(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")

	shutdown, err := observability.SetupTracing(t.Context(), "video-processor-api")

	require.NoError(t, err)
	assert.NoError(t, shutdown(t.Context()))
}
