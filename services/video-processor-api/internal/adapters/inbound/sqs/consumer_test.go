package sqs_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/inbound/sqs"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/events"
)

var now = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

type requestStore struct {
	requests  map[string]video.ProcessRequest
	results   map[string]video.ProcessResult
	processed map[string]bool
}

func newRequestStore(reqs ...video.ProcessRequest) *requestStore {
	s := &requestStore{requests: map[string]video.ProcessRequest{}, results: map[string]video.ProcessResult{}, processed: map[string]bool{}}
	for _, r := range reqs {
		s.requests[r.ID] = r
	}
	return s
}

func (s *requestStore) SaveVideo(context.Context, video.Video) error { return nil }
func (s *requestStore) FindVideo(context.Context, string) (video.Video, error) {
	return video.Video{}, video.ErrNotFound
}
func (s *requestStore) FindRequestByVideo(context.Context, string) (video.ProcessRequest, error) {
	return video.ProcessRequest{}, video.ErrNotFound
}
func (s *requestStore) CreateRequest(context.Context, video.ProcessRequest, video.ProcessingRequested) error {
	return nil
}
func (s *requestStore) ListByUser(context.Context, string) ([]video.RequestSummary, error) {
	return nil, nil
}
func (s *requestStore) FindResult(context.Context, string) (video.ProcessResult, error) {
	return video.ProcessResult{}, video.ErrNotFound
}

func (s *requestStore) FindRequest(_ context.Context, id string) (video.ProcessRequest, error) {
	r, ok := s.requests[id]
	if !ok {
		return video.ProcessRequest{}, video.ErrNotFound
	}
	return r, nil
}

func (s *requestStore) WasEventProcessed(_ context.Context, eventID string) (bool, error) {
	return s.processed[eventID], nil
}

func (s *requestStore) ApplyEvent(_ context.Context, eventID string, req video.ProcessRequest, res *video.ProcessResult) error {
	s.requests[req.ID] = req
	if res != nil {
		s.results[req.ID] = *res
	}
	s.processed[eventID] = true
	return nil
}

type noStore struct{}

func (noStore) PresignUpload(context.Context, string, string, int64, video.ObjectOwner, time.Duration) (video.PresignedUpload, error) {
	return video.PresignedUpload{}, nil
}
func (noStore) Head(context.Context, string) (video.ObjectInfo, error) {
	return video.ObjectInfo{}, video.ErrUploadNotFound
}
func (noStore) PresignDownload(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

type clock struct{}

func (clock) Now() time.Time { return now }

type ids struct{}

func (ids) NewID() string { return "id" }

func consumerFor(store *requestStore) *sqs.Consumer {
	svc := video.NewService(store, noStore{}, clock{}, ids{}, video.Config{})
	return sqs.NewConsumer(nil, "", svc, slog.Default())
}

func encoded(t *testing.T, id string, p events.Payload) []byte {
	t.Helper()
	raw, err := events.Encode(events.Wrap(id, now, p))
	require.NoError(t, err)
	return raw
}

func pending(id string) video.ProcessRequest {
	return video.ProcessRequest{ID: id, VideoID: "vid-" + id, UserID: "usr-ana", Status: video.StatusPending, CreatedAt: now, UpdatedAt: now}
}

func TestHandleDrivesProcessRequestThroughWorkerEvents(t *testing.T) {
	store := newRequestStore(pending("r1"))
	c := consumerFor(store)

	require.NoError(t, c.Handle(t.Context(), encoded(t, "e1", events.VideoProcessingStarted{RequestID: "r1", Attempt: 1})))
	assert.Equal(t, video.StatusProcessing, store.requests["r1"].Status)

	require.NoError(t, c.Handle(t.Context(), encoded(t, "e2", events.VideoProcessingCompleted{RequestID: "r1", ZipKey: "results/r1.zip", FrameCount: 9})))
	assert.Equal(t, video.StatusCompleted, store.requests["r1"].Status)
	assert.Equal(t, 9, store.results["r1"].FrameCount)
}

func TestHandleRecordsDefinitiveFailure(t *testing.T) {
	store := newRequestStore(pending("r2"))
	c := consumerFor(store)
	require.NoError(t, c.Handle(t.Context(), encoded(t, "e1", events.VideoProcessingStarted{RequestID: "r2", Attempt: 1})))

	require.NoError(t, c.Handle(t.Context(), encoded(t, "e2", events.VideoProcessingFailed{RequestID: "r2", Reason: "unprocessable video", Retryable: false, Attempt: 1})))

	assert.Equal(t, video.StatusFailed, store.requests["r2"].Status)
	assert.Equal(t, "unprocessable video", store.results["r2"].FailureReason)
}

func TestHandleRejectsUnknownEventTypes(t *testing.T) {
	c := consumerFor(newRequestStore())

	err := c.Handle(t.Context(), encoded(t, "e1", events.VideoProcessingRequested{RequestID: "r1"}))

	assert.Error(t, err)
}

func TestHandleRejectsMalformedBodies(t *testing.T) {
	c := consumerFor(newRequestStore())

	assert.Error(t, c.Handle(t.Context(), []byte(`{"eventId":`)))
}

type fakeConsumerMetrics struct{ seen []string }

func (m *fakeConsumerMetrics) EventConsumed(eventType, result string) {
	m.seen = append(m.seen, eventType+":"+result)
}

func TestHandleReportsEachEventWithItsResult(t *testing.T) {
	store := newRequestStore(video.ProcessRequest{ID: "req-1", Status: video.StatusPending})
	m := &fakeConsumerMetrics{}
	svc := video.NewService(store, noStore{}, clock{}, ids{}, video.Config{})
	c := sqs.NewConsumer(nil, "", svc, slog.Default(), sqs.WithMetrics(m))

	require.NoError(t, c.Handle(t.Context(), encoded(t, "evt-1", events.VideoProcessingStarted{RequestID: "req-1", Attempt: 1})))
	assert.Error(t, c.Handle(t.Context(), encoded(t, "evt-2", events.VideoProcessingCompleted{RequestID: "req-404", ZipKey: "k", FrameCount: 1})))
	assert.Error(t, c.Handle(t.Context(), []byte("{not json")))

	assert.Equal(t, []string{"VideoProcessingStarted:applied", "VideoProcessingCompleted:discarded", "unknown:discarded"}, m.seen)
}
