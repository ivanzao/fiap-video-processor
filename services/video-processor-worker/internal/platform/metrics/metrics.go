package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
)

type Metrics struct {
	registry   *prometheus.Registry
	processing *ProcessingMetrics
	queue      *QueueMetrics
}

type ProcessingMetrics struct {
	executions *prometheus.CounterVec
	duration   *prometheus.HistogramVec
	frames     prometheus.Counter
}

type QueueMetrics struct {
	messages *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func New() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	buckets := []float64{1, 2.5, 5, 10, 20, 30, 60, 120, 180, 300, 600}
	m := &Metrics{
		registry: reg,
		processing: &ProcessingMetrics{
			executions: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "processing_executions_total", Help: "Executions finished, by outcome"}, []string{"outcome"}),
			duration:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "processing_duration_seconds", Help: "Wall time of an Execution, by outcome", Buckets: buckets}, []string{"outcome"}),
			frames:     prometheus.NewCounter(prometheus.CounterOpts{Name: "processing_frames_total", Help: "Frames extracted by completed Executions"}),
		},
		queue: &QueueMetrics{
			messages: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "sqs_messages_total", Help: "Queue messages handled, by result"}, []string{"result"}),
			duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "sqs_message_duration_seconds", Help: "Time a message was held by the consumer, by result", Buckets: buckets}, []string{"result"}),
		},
	}
	reg.MustRegister(m.processing.executions, m.processing.duration, m.processing.frames, m.queue.messages, m.queue.duration)
	for _, outcome := range []video.Outcome{video.OutcomeCompleted, video.OutcomeFailed, video.OutcomeRetrying} {
		m.processing.executions.WithLabelValues(string(outcome))
	}
	return m
}

func (m *Metrics) Processing() *ProcessingMetrics { return m.processing }
func (m *Metrics) Queue() *QueueMetrics           { return m.queue }
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (p *ProcessingMetrics) ExecutionFinished(outcome video.Outcome, duration time.Duration, frames int) {
	p.executions.WithLabelValues(string(outcome)).Inc()
	p.duration.WithLabelValues(string(outcome)).Observe(duration.Seconds())
	p.frames.Add(float64(frames))
}

func (q *QueueMetrics) MessageHandled(result string, duration time.Duration) {
	q.messages.WithLabelValues(result).Inc()
	q.duration.WithLabelValues(result).Observe(duration.Seconds())
}
