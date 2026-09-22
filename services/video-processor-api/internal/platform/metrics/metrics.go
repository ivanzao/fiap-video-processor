package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
)

type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.HistogramVec
	video    *VideoMetrics
	events   *EventMetrics
}

type VideoMetrics struct {
	uploads  prometheus.Counter
	statuses *prometheus.CounterVec
}

type EventMetrics struct {
	consumed *prometheus.CounterVec
}

func New() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	m := &Metrics{
		registry: reg,
		requests: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_server_request_duration_seconds",
			Help:    "Duration of HTTP requests served by the API, by route and status code",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route", "status"}),
		video: &VideoMetrics{
			uploads: prometheus.NewCounter(prometheus.CounterOpts{Name: "video_uploads_total", Help: "Videos for which an upload URL was issued"}),
			statuses: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "video_process_requests_total", Help: "Process Request transitions into each Status",
			}, []string{"status"}),
		},
		events: &EventMetrics{
			consumed: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "video_inbound_events_total", Help: "Worker events consumed by the API, by type and result",
			}, []string{"event_type", "result"}),
		},
	}
	reg.MustRegister(m.requests, m.video.uploads, m.video.statuses, m.events.consumed)
	for _, status := range []video.Status{video.StatusPending, video.StatusProcessing, video.StatusCompleted, video.StatusFailed} {
		m.video.statuses.WithLabelValues(string(status))
	}
	return m
}

func (m *Metrics) Video() *VideoMetrics  { return m.video }
func (m *Metrics) Events() *EventMetrics { return m.events }
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (v *VideoMetrics) UploadRequested() { v.uploads.Inc() }
func (v *VideoMetrics) StatusChanged(status video.Status) {
	v.statuses.WithLabelValues(string(status)).Inc()
}

func (e *EventMetrics) EventConsumed(eventType, result string) {
	e.consumed.WithLabelValues(eventType, result).Inc()
}

func (m *Metrics) HTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		m.requests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Observe(time.Since(start).Seconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
