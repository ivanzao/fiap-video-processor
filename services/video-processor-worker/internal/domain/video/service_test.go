package video_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
)

var now = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

type harness struct {
	svc       *video.Service
	store     *fakeStore
	extractor *fakeExtractor
	packager  *fakePackager
	repo      *fakeRepo
	publisher *fakePublisher
	notifier  *fakeNotifier
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		store: newFakeStore(), extractor: &fakeExtractor{frames: 3}, packager: &fakePackager{},
		repo: newFakeRepo(), publisher: &fakePublisher{}, notifier: &fakeNotifier{},
	}
	h.svc = video.NewService(video.Dependencies{
		Store: h.store, Extractor: h.extractor, Packager: h.packager, Workspace: fakeWorkspace{t},
		Repo: h.repo, Publisher: h.publisher, Notifier: h.notifier, Clock: fakeClock{now}, IDs: &fakeIDs{},
	}, video.Config{FrameRate: 1, MaxAttempts: 3})
	return h
}

func request(attempt int) video.ProcessRequest {
	return video.ProcessRequest{
		RequestID: "req-1", VideoID: "vid-1", UserID: "usr-ana", UserEmail: "ana@example.com",
		ObjectKey: "uploads/usr-ana/vid-1.mp4", Filename: "ferias.mp4", Attempt: attempt,
	}
}

func TestProcessExtractsPackagesUploadsPublishesAndNotifies(t *testing.T) {
	h := newHarness(t)
	h.store.objects["uploads/usr-ana/vid-1.mp4"] = []byte("video-bytes")

	err := h.svc.ExecuteProcessRequest(t.Context(), request(1))

	require.NoError(t, err)
	assert.Equal(t, []extractCall{{frameRate: 1}}, h.extractor.seen)
	assert.Equal(t, "zip:3", string(h.store.objects["results/req-1.zip"]))
	assert.Equal(t, "application/zip", h.store.uploaded["results/req-1.zip"])
	assert.Equal(t, []video.ProcessingStarted{{RequestID: "req-1", Attempt: 1}}, h.publisher.started)
	require.Len(t, h.publisher.completed, 1)
	assert.Equal(t, video.ProcessingCompleted{RequestID: "req-1", ZipKey: "results/req-1.zip", FrameCount: 3}, h.publisher.completed[0])
	assert.Empty(t, h.publisher.failed)
	exec := h.repo.executions["req-1"]
	assert.Equal(t, video.OutcomeCompleted, exec.Outcome)
	assert.Equal(t, 1, exec.Attempt)
	assert.Equal(t, 3, exec.FrameCount)
	require.Len(t, h.notifier.sent, 1)
	assert.Equal(t, video.Notification{To: "ana@example.com", RequestID: "req-1", Filename: "ferias.mp4", Outcome: video.OutcomeCompleted, FrameCount: 3}, h.notifier.sent[0])
}

func TestUnprocessableVideoFailsDefinitivelyOnFirstAttempt(t *testing.T) {
	h := newHarness(t)
	h.store.objects["uploads/usr-ana/vid-1.mp4"] = []byte("garbage")
	h.extractor.err = &video.UnprocessableError{Reason: "moov atom not found"}

	err := h.svc.ExecuteProcessRequest(t.Context(), request(1))

	require.NoError(t, err, "definitive outcomes acknowledge the message")
	require.Len(t, h.publisher.failed, 1)
	assert.Equal(t, video.ProcessingFailed{RequestID: "req-1", Reason: "unprocessable video: moov atom not found", Retryable: false, Attempt: 1}, h.publisher.failed[0])
	assert.Empty(t, h.publisher.completed)
	exec := h.repo.executions["req-1"]
	assert.Equal(t, video.OutcomeFailed, exec.Outcome)
	assert.Equal(t, "unprocessable video: moov atom not found", exec.Reason)
	require.Len(t, h.notifier.sent, 1)
	assert.Equal(t, video.OutcomeFailed, h.notifier.sent[0].Outcome)
	assert.Equal(t, "unprocessable video: moov atom not found", h.notifier.sent[0].Reason)
}

func TestTransientFailureWithAttemptsLeftAsksForRedelivery(t *testing.T) {
	h := newHarness(t)
	h.store.failDownload = errTransient

	err := h.svc.ExecuteProcessRequest(t.Context(), request(1))

	assert.ErrorIs(t, err, video.ErrRetryLater)
	require.Len(t, h.publisher.failed, 1)
	assert.Equal(t, video.ProcessingFailed{RequestID: "req-1", Reason: "network hiccup", Retryable: true, Attempt: 1}, h.publisher.failed[0])
	assert.Equal(t, video.OutcomeRetrying, h.repo.executions["req-1"].Outcome)
	assert.Empty(t, h.notifier.sent, "no notification while attempts remain")
}

func TestTransientFailureOnLastAttemptFailsDefinitively(t *testing.T) {
	h := newHarness(t)
	h.store.failDownload = errTransient

	err := h.svc.ExecuteProcessRequest(t.Context(), request(3))

	require.NoError(t, err)
	require.Len(t, h.publisher.failed, 1)
	assert.Equal(t, video.ProcessingFailed{RequestID: "req-1", Reason: "network hiccup", Retryable: false, Attempt: 3}, h.publisher.failed[0])
	assert.Equal(t, video.OutcomeFailed, h.repo.executions["req-1"].Outcome)
	require.Len(t, h.notifier.sent, 1)
	assert.Equal(t, video.OutcomeFailed, h.notifier.sent[0].Outcome)
}

func TestMissingObjectIsNotRetried(t *testing.T) {
	h := newHarness(t)

	err := h.svc.ExecuteProcessRequest(t.Context(), request(1))

	require.NoError(t, err)
	require.Len(t, h.publisher.failed, 1)
	assert.False(t, h.publisher.failed[0].Retryable)
	assert.Equal(t, video.OutcomeFailed, h.repo.executions["req-1"].Outcome)
}

func TestRedeliveryAfterCompletionOnlyRepublishesTheResult(t *testing.T) {
	h := newHarness(t)
	h.store.objects["uploads/usr-ana/vid-1.mp4"] = []byte("video-bytes")
	require.NoError(t, h.svc.ExecuteProcessRequest(t.Context(), request(1)))
	h.extractor.seen = nil

	err := h.svc.ExecuteProcessRequest(t.Context(), request(2))

	require.NoError(t, err)
	assert.Empty(t, h.extractor.seen, "video is not processed twice")
	assert.Len(t, h.publisher.started, 1)
	require.Len(t, h.publisher.completed, 2)
	assert.Equal(t, h.publisher.completed[0], h.publisher.completed[1])
	assert.Len(t, h.notifier.sent, 1, "notification is sent once per terminal outcome")
}

func TestRedeliveryAfterDefinitiveFailureRepublishesTheFailure(t *testing.T) {
	h := newHarness(t)
	h.store.objects["uploads/usr-ana/vid-1.mp4"] = []byte("garbage")
	h.extractor.err = &video.UnprocessableError{Reason: "corrupt"}
	require.NoError(t, h.svc.ExecuteProcessRequest(t.Context(), request(1)))
	h.extractor.err = nil
	h.extractor.seen = nil

	err := h.svc.ExecuteProcessRequest(t.Context(), request(2))

	require.NoError(t, err)
	assert.Empty(t, h.extractor.seen)
	require.Len(t, h.publisher.failed, 2)
	assert.False(t, h.publisher.failed[1].Retryable)
	assert.Len(t, h.notifier.sent, 1)
}

func TestNotificationFailureAsksForRedeliveryWithoutReprocessing(t *testing.T) {
	h := newHarness(t)
	h.store.objects["uploads/usr-ana/vid-1.mp4"] = []byte("video-bytes")
	h.notifier.err = errTransient

	err := h.svc.ExecuteProcessRequest(t.Context(), request(1))

	require.ErrorIs(t, err, video.ErrRetryLater)
	assert.Equal(t, video.OutcomeCompleted, h.repo.executions["req-1"].Outcome)
	assert.Len(t, h.publisher.completed, 1)
	assert.Empty(t, h.repo.notified, "an unsent notification is not marked as sent")

	h.notifier.err = nil
	err = h.svc.ExecuteProcessRequest(t.Context(), request(2))

	require.NoError(t, err)
	assert.Len(t, h.extractor.seen, 1, "the redelivery republishes instead of extracting again")
	assert.Len(t, h.publisher.completed, 2)
	assert.Len(t, h.notifier.sent, 1)
	assert.True(t, h.repo.notified["req-1/COMPLETED"])
}

func TestFailureNotificationThatCannotBeSentAsksForRedelivery(t *testing.T) {
	h := newHarness(t)
	h.store.objects["uploads/usr-ana/vid-1.mp4"] = []byte("video-bytes")
	h.extractor.err = &video.UnprocessableError{Reason: "moov atom not found"}
	h.notifier.err = errTransient

	err := h.svc.ExecuteProcessRequest(t.Context(), request(1))

	require.ErrorIs(t, err, video.ErrRetryLater)
	assert.Equal(t, video.OutcomeFailed, h.repo.executions["req-1"].Outcome)
	assert.Empty(t, h.notifier.sent)
}

func TestPublishFailureAfterCompletionAsksForRedelivery(t *testing.T) {
	h := newHarness(t)
	h.store.objects["uploads/usr-ana/vid-1.mp4"] = []byte("video-bytes")
	h.publisher.failCompleted = errTransient

	err := h.svc.ExecuteProcessRequest(t.Context(), request(1))

	assert.ErrorIs(t, err, video.ErrRetryLater)
	assert.Equal(t, video.OutcomeCompleted, h.repo.executions["req-1"].Outcome, "the work is kept so the redelivery only republishes")
}

type finished struct {
	outcome video.Outcome
	frames  int
}

type fakeMetrics struct{ seen []finished }

func (m *fakeMetrics) ExecutionFinished(outcome video.Outcome, _ time.Duration, frames int) {
	m.seen = append(m.seen, finished{outcome, frames})
}

func TestProcessReportsEveryExecutionOutcome(t *testing.T) {
	h := newHarness(t)
	m := &fakeMetrics{}
	h.svc = video.NewService(video.Dependencies{
		Store: h.store, Extractor: h.extractor, Packager: h.packager, Workspace: fakeWorkspace{t},
		Repo: h.repo, Publisher: h.publisher, Notifier: h.notifier, Clock: fakeClock{now}, IDs: &fakeIDs{}, Metrics: m,
	}, video.Config{FrameRate: 1, MaxAttempts: 3})
	h.store.objects["uploads/usr-ana/vid-1.mp4"] = []byte("video-bytes")

	require.NoError(t, h.svc.ExecuteProcessRequest(t.Context(), request(1)))
	h.extractor.err = errTransient
	assert.ErrorIs(t, h.svc.ExecuteProcessRequest(t.Context(), video.ProcessRequest{RequestID: "req-2", ObjectKey: "uploads/usr-ana/vid-1.mp4", Filename: "b.mp4", Attempt: 1}), video.ErrRetryLater)
	h.extractor.err = &video.UnprocessableError{Reason: "moov atom not found"}
	require.NoError(t, h.svc.ExecuteProcessRequest(t.Context(), video.ProcessRequest{RequestID: "req-3", ObjectKey: "uploads/usr-ana/vid-1.mp4", Filename: "c.mp4", Attempt: 1}))

	assert.Equal(t, []finished{{video.OutcomeCompleted, 3}, {video.OutcomeRetrying, 0}, {video.OutcomeFailed, 0}}, m.seen)
}
