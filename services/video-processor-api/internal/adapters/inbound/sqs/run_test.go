package sqs_test

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/inbound/sqs"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/events"
)

type message struct {
	MessageID     string `json:"MessageId"`
	ReceiptHandle string `json:"ReceiptHandle"`
	Body          string `json:"Body"`
	MD5OfBody     string `json:"MD5OfBody"`
}

// fakeQueue speaks just enough of the SQS JSON protocol to hand out queued
// messages once and record which ones the consumer deletes.
type fakeQueue struct {
	mu       sync.Mutex
	messages []message
	deleted  []string
}

func (q *fakeQueue) push(receipt string, body []byte) {
	sum := md5.Sum(body)
	q.messages = append(q.messages, message{MessageID: "msg-" + receipt, ReceiptHandle: receipt, Body: string(body), MD5OfBody: hex.EncodeToString(sum[:])})
}

func (q *fakeQueue) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var in struct{ ReceiptHandle string }
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
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (q *fakeQueue) deletedReceipts() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]string(nil), q.deleted...)
}

type tracker struct {
	mu    sync.Mutex
	fails map[string]error
	calls []string
}

func (tr *tracker) record(requestID string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.calls = append(tr.calls, requestID)
	return tr.fails[requestID]
}

func (tr *tracker) seen() []string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return append([]string(nil), tr.calls...)
}

func (tr *tracker) StartProcessing(_ context.Context, _, requestID string, _ int) error {
	return tr.record(requestID)
}

func (tr *tracker) CompleteProcessing(_ context.Context, _, requestID, _ string, _ int) error {
	return tr.record(requestID)
}

func (tr *tracker) FailProcessing(_ context.Context, _, requestID, _ string, _ bool, _ int) error {
	return tr.record(requestID)
}

func TestRunDeletesAppliedAndDiscardedMessagesButKeepsFailedOnes(t *testing.T) {
	queue := &fakeQueue{}
	queue.push("rcpt-ok", encoded(t, "e1", events.VideoProcessingStarted{RequestID: "r1", Attempt: 1}))
	queue.push("rcpt-bad", []byte(`{"eventId":`))
	queue.push("rcpt-fail", encoded(t, "e2", events.VideoProcessingCompleted{RequestID: "r2", ZipKey: "results/r2.zip", FrameCount: 3}))
	tr := &tracker{fails: map[string]error{"r2": errors.New("database gone")}}
	server := httptest.NewServer(queue)
	t.Cleanup(server.Close)
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	consumer, err := sqs.Connect(t.Context(), sqs.Config{Region: "us-east-1", Endpoint: server.URL, QueueURL: server.URL + "/000000000000/api-inbox"}, tr, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		consumer.Run(ctx)
	}()

	assert.Eventually(t, func() bool { return len(tr.seen()) == 2 }, 5*time.Second, 10*time.Millisecond)
	cancel()
	<-done

	assert.Equal(t, []string{"r1", "r2"}, tr.seen())
	assert.Equal(t, []string{"rcpt-ok", "rcpt-bad"}, queue.deletedReceipts())
}
