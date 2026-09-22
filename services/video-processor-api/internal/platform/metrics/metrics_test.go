package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/metrics"
)

func scrape(t *testing.T, m *metrics.Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func TestDomainCountersAreExposed(t *testing.T) {
	m := metrics.New()

	m.Video().UploadRequested()
	m.Video().StatusChanged(video.StatusPending)
	m.Video().StatusChanged(video.StatusPending)
	m.Video().StatusChanged(video.StatusCompleted)
	m.Events().EventConsumed("VideoProcessingCompleted", "applied")

	body := scrape(t, m)
	assert.Contains(t, body, "video_uploads_total 1")
	assert.Contains(t, body, `video_process_requests_total{status="PENDING"} 2`)
	assert.Contains(t, body, `video_process_requests_total{status="COMPLETED"} 1`)
	assert.Contains(t, body, `video_inbound_events_total{event_type="VideoProcessingCompleted",result="applied"} 1`)
	assert.Contains(t, body, "go_goroutines", "runtime collectors are registered")
}

func TestHTTPMiddlewareRecordsRouteMethodAndStatus(t *testing.T) {
	m := metrics.New()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/videos/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
	h := m.HTTP(mux)

	for _, path := range []string{"/v1/videos/1", "/v1/videos/2", "/nowhere"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	}

	body := scrape(t, m)
	assert.Contains(t, body, `http_server_request_duration_seconds_count{method="GET",route="GET /v1/videos/{id}",status="202"} 2`)
	assert.Contains(t, body, `http_server_request_duration_seconds_count{method="GET",route="unmatched",status="404"} 1`)
}
