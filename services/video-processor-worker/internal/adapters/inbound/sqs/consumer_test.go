package sqs_test

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/inbound/sqs"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/events"
)

const queueURL = "http://sqs.test/000000000000/video-processor-worker-inbox"

type message struct {
	MessageID     string            `json:"MessageId"`
	ReceiptHandle string            `json:"ReceiptHandle"`
	Body          string            `json:"Body"`
	MD5OfBody     string            `json:"MD5OfBody"`
	Attributes    map[string]string `json:"Attributes,omitempty"`
}

// fakeQueue speaks just enough of the SQS JSON protocol to hand out queued
// messages once and record what the consumer does with them.
type fakeQueue struct {
	mu         sync.Mutex
	messages   []message
	deleted    []string
	visibility []int32
}

func (q *fakeQueue) push(receipt, body string, receiveCount string) {
	sum := md5.Sum([]byte(body))
	q.mu.Lock()
	defer q.mu.Unlock()
	q.messages = append(q.messages, message{
		MessageID: "msg-" + receipt, ReceiptHandle: receipt, Body: body, MD5OfBody: hex.EncodeToString(sum[:]),
		Attributes: map[string]string{"ApproximateReceiveCount": receiveCount},
	})
}

func (q *fakeQueue) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ReceiptHandle     string
		VisibilityTimeout int32
	}
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &in)
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	q.mu.Lock()
	defer q.mu.Unlock()
	switch r.Header.Get("X-Amz-Target") {
	case "AmazonSQS.ReceiveMessage":
		out := struct {
			Messages []message `json:"Messages"`
		}{}
		if len(q.messages) > 0 {
			out.Messages, q.messages = q.messages[:1], q.messages[1:]
		} else {
			q.mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			q.mu.Lock()
		}
		_ = json.NewEncoder(w).Encode(out)
	case "AmazonSQS.DeleteMessage":
		q.deleted = append(q.deleted, in.ReceiptHandle)
		_, _ = w.Write([]byte("{}"))
	case "AmazonSQS.ChangeMessageVisibility":
		q.visibility = append(q.visibility, in.VisibilityTimeout)
		_, _ = w.Write([]byte("{}"))
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (q *fakeQueue) snapshot() (deleted []string, visibility []int32) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]string(nil), q.deleted...), append([]int32(nil), q.visibility...)
}

type executor struct {
	mu   sync.Mutex
	seen []video.ProcessRequest
	run  func(video.ProcessRequest) error
}

func (e *executor) ExecuteProcessRequest(_ context.Context, req video.ProcessRequest) error {
	e.mu.Lock()
	e.seen = append(e.seen, req)
	e.mu.Unlock()
	if e.run == nil {
		return nil
	}
	return e.run(req)
}

func (e *executor) requests() []video.ProcessRequest {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]video.ProcessRequest(nil), e.seen...)
}

type fakeMetrics struct {
	mu      sync.Mutex
	results []string
}

func (m *fakeMetrics) MessageHandled(result string, _ time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = append(m.results, result)
}

func (m *fakeMetrics) seen() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.results...)
}

func requested(t *testing.T, requestID string) string {
	t.Helper()
	raw, err := events.Encode(events.Wrap("evt-"+requestID, time.Now(), events.VideoProcessingRequested{
		RequestID: requestID, VideoID: "vid-1", UserID: "usr-ana", UserEmail: "ana@example.com",
		ObjectKey: "uploads/usr-ana/vid-1.mp4", Filename: "ferias.mp4",
	}))
	require.NoError(t, err)
	return string(raw)
}

func runConsumer(t *testing.T, queue *fakeQueue, exec *executor, cfg sqs.Config) (stop func()) {
	t.Helper()
	server := httptest.NewServer(queue)
	t.Cleanup(server.Close)
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	cfg.Region, cfg.Endpoint, cfg.QueueURL = "us-east-1", server.URL, queueURL
	consumer, err := sqs.Connect(t.Context(), exec, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		consumer.Run(ctx)
	}()
	return func() {
		cancel()
		<-done
	}
}

func baseConfig(m sqs.Metrics) sqs.Config {
	return sqs.Config{Concurrency: 2, VisibilityTimeout: 5 * time.Minute, Heartbeat: time.Minute, RetryDelay: 30 * time.Second, Metrics: m}
}

func TestConsumerDeletesSettledMessagesAndPassesTheAttempt(t *testing.T) {
	queue := &fakeQueue{}
	queue.push("rcpt-1", requested(t, "req-1"), "2")
	exec := &executor{}
	m := &fakeMetrics{}
	stop := runConsumer(t, queue, exec, baseConfig(m))

	assert.Eventually(t, func() bool { d, _ := queue.snapshot(); return len(d) == 1 }, 5*time.Second, 10*time.Millisecond)
	stop()

	deleted, visibility := queue.snapshot()
	assert.Equal(t, []string{"rcpt-1"}, deleted)
	assert.Empty(t, visibility)
	require.Len(t, exec.requests(), 1)
	assert.Equal(t, video.ProcessRequest{
		RequestID: "req-1", VideoID: "vid-1", UserID: "usr-ana", UserEmail: "ana@example.com",
		ObjectKey: "uploads/usr-ana/vid-1.mp4", Filename: "ferias.mp4", Attempt: 2,
	}, exec.requests()[0])
	assert.Equal(t, []string{"settled"}, m.seen())
}

func TestConsumerDiscardsMalformedMessages(t *testing.T) {
	queue := &fakeQueue{}
	queue.push("rcpt-bad", `{"eventId":`, "1")
	wrongType, err := events.Encode(events.Wrap("evt-x", time.Now(), events.VideoProcessingStarted{RequestID: "req-x", Attempt: 1}))
	require.NoError(t, err)
	queue.push("rcpt-wrong", string(wrongType), "1")
	exec := &executor{}
	m := &fakeMetrics{}
	stop := runConsumer(t, queue, exec, baseConfig(m))

	assert.Eventually(t, func() bool { d, _ := queue.snapshot(); return len(d) == 2 }, 5*time.Second, 10*time.Millisecond)
	stop()

	deleted, _ := queue.snapshot()
	assert.ElementsMatch(t, []string{"rcpt-bad", "rcpt-wrong"}, deleted)
	assert.Empty(t, exec.requests())
	assert.Equal(t, []string{"malformed", "malformed"}, m.seen())
}

func TestConsumerDelaysRedeliveryWhenProcessingFails(t *testing.T) {
	for name, tc := range map[string]struct {
		err    error
		result string
	}{
		"retry later": {err: fmt.Errorf("%w: storage down", video.ErrRetryLater), result: "retry"},
		"unexpected":  {err: errors.New("database gone"), result: "error"},
	} {
		t.Run(name, func(t *testing.T) {
			queue := &fakeQueue{}
			queue.push("rcpt-1", requested(t, "req-1"), "1")
			exec := &executor{run: func(video.ProcessRequest) error { return tc.err }}
			m := &fakeMetrics{}
			stop := runConsumer(t, queue, exec, baseConfig(m))

			assert.Eventually(t, func() bool { _, v := queue.snapshot(); return len(v) == 1 }, 5*time.Second, 10*time.Millisecond)
			stop()

			deleted, visibility := queue.snapshot()
			assert.Empty(t, deleted)
			assert.Equal(t, []int32{30}, visibility)
			assert.Equal(t, []string{tc.result}, m.seen())
		})
	}
}

func TestConsumerExtendsVisibilityWhileProcessing(t *testing.T) {
	queue := &fakeQueue{}
	queue.push("rcpt-1", requested(t, "req-1"), "1")
	exec := &executor{run: func(video.ProcessRequest) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	}}
	cfg := baseConfig(nil)
	cfg.Concurrency = 1
	cfg.Heartbeat = 20 * time.Millisecond
	stop := runConsumer(t, queue, exec, cfg)

	assert.Eventually(t, func() bool { d, _ := queue.snapshot(); return len(d) == 1 }, 5*time.Second, 10*time.Millisecond)
	stop()

	_, visibility := queue.snapshot()
	require.NotEmpty(t, visibility)
	for _, v := range visibility {
		assert.Equal(t, int32(300), v)
	}
}
