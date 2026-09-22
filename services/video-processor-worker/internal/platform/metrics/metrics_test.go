package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/metrics"
)

func TestProcessingAndQueueMetricsAreExposed(t *testing.T) {
	m := metrics.New()

	m.Processing().ExecutionFinished(video.OutcomeCompleted, 12*time.Second, 3)
	m.Processing().ExecutionFinished(video.OutcomeFailed, 2*time.Second, 0)
	m.Queue().MessageHandled("settled", 13*time.Second)
	m.Queue().MessageHandled("retry", time.Second)

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `processing_executions_total{outcome="COMPLETED"} 1`)
	assert.Contains(t, body, `processing_executions_total{outcome="FAILED"} 1`)
	assert.Contains(t, body, `processing_duration_seconds_count{outcome="COMPLETED"} 1`)
	assert.Contains(t, body, "processing_frames_total 3")
	assert.Contains(t, body, `sqs_messages_total{result="settled"} 1`)
	assert.Contains(t, body, `sqs_message_duration_seconds_count{result="retry"} 1`)
	assert.Contains(t, body, "go_goroutines")
}
