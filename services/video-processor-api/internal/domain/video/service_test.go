package video_test

import (
	"testing"
	"time"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	now = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	ana = video.Identity{UserID: "usr-ana", Email: "ana@example.com"}
	bia = video.Identity{UserID: "usr-bia", Email: "bia@example.com"}
)

type harness struct {
	svc   *video.Service
	repo  *fakeRepo
	store *fakeStore
}

func newHarness(t *testing.T) harness {
	t.Helper()
	repo, store := newFakeRepo(), newFakeStore()
	svc := video.NewService(repo, store, fakeClock{now}, &fakeIDs{}, video.Config{
		MaxUploadBytes: 500 * 1024 * 1024,
		UploadTTL:      15 * time.Minute,
		DownloadTTL:    time.Hour,
	})
	return harness{svc: svc, repo: repo, store: store}
}

func TestCreateVideoCreatesVideoAndPresignedURL(t *testing.T) {
	h := newHarness(t)

	ticket, err := h.svc.CreateVideo(t.Context(), ana, video.UploadCommand{
		Filename: "ferias.MP4", ContentType: "video/mp4", SizeBytes: 10_000_000,
	})

	require.NoError(t, err)
	assert.Equal(t, "id-1", ticket.VideoID)
	assert.Equal(t, "https://store.local/upload/uploads/usr-ana/id-1.mp4", ticket.UploadURL)
	assert.Equal(t, now.Add(15*time.Minute), ticket.ExpiresAt)
	saved := h.repo.videos["id-1"]
	assert.Equal(t, "usr-ana", saved.UserID)
	assert.Equal(t, "ferias.MP4", saved.Filename)
	assert.Equal(t, "uploads/usr-ana/id-1.mp4", saved.ObjectKey)
	require.Len(t, h.store.presigned, 1)
	assert.Equal(t, int64(10_000_000), h.store.presigned[0].sizeBytes)
	assert.Equal(t, "video/mp4", h.store.presigned[0].contentType)
}

func TestCreateVideoRejectsUnsupportedFormat(t *testing.T) {
	h := newHarness(t)

	_, err := h.svc.CreateVideo(t.Context(), ana, video.UploadCommand{Filename: "slides.pdf", ContentType: "application/pdf", SizeBytes: 100})

	assert.ErrorIs(t, err, video.ErrUnsupportedFormat)
	assert.Empty(t, h.repo.videos)
}

func TestCreateVideoRejectsVideoOverSizeLimit(t *testing.T) {
	h := newHarness(t)

	_, err := h.svc.CreateVideo(t.Context(), ana, video.UploadCommand{Filename: "longo.mkv", ContentType: "video/x-matroska", SizeBytes: 500*1024*1024 + 1})

	assert.ErrorIs(t, err, video.ErrVideoTooLarge)
	assert.Empty(t, h.store.presigned)
}

func uploaded(t *testing.T, h harness, identity video.Identity, filename string) video.UploadTicket {
	t.Helper()
	ticket, err := h.svc.CreateVideo(t.Context(), identity, video.UploadCommand{Filename: filename, ContentType: "video/mp4", SizeBytes: 1000})
	require.NoError(t, err)
	h.store.putAs(h.repo.videos[ticket.VideoID].ObjectKey, video.ObjectOwner{UserID: identity.UserID, VideoID: ticket.VideoID})
	return ticket
}

func TestProcessVideoCreatesPendingRequestAndQueuesProcessingEvent(t *testing.T) {
	h := newHarness(t)
	ticket := uploaded(t, h, ana, "ferias.mp4")

	summary, err := h.svc.ProcessVideo(t.Context(), ana, ticket.VideoID)

	require.NoError(t, err)
	req := summary.Request
	assert.Equal(t, video.StatusPending, req.Status)
	assert.Equal(t, ticket.VideoID, req.VideoID)
	assert.Equal(t, "usr-ana", req.UserID)
	assert.Equal(t, 0, req.Attempts)
	assert.Equal(t, "ferias.mp4", summary.Video.Filename)
	require.Len(t, h.repo.outbox, 1)
	assert.Equal(t, video.ProcessingRequested{
		RequestID: req.ID, VideoID: ticket.VideoID, UserID: "usr-ana", UserEmail: "ana@example.com",
		ObjectKey: "uploads/usr-ana/id-1.mp4", Filename: "ferias.mp4",
	}, h.repo.outbox[0])
}

func TestProcessVideoIsIdempotentForTheSameVideo(t *testing.T) {
	h := newHarness(t)
	ticket := uploaded(t, h, ana, "ferias.mp4")
	first, err := h.svc.ProcessVideo(t.Context(), ana, ticket.VideoID)
	require.NoError(t, err)

	second, err := h.svc.ProcessVideo(t.Context(), ana, ticket.VideoID)

	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Len(t, h.repo.outbox, 1)
}

func TestProcessVideoFailsWhenObjectIsNotInStorage(t *testing.T) {
	h := newHarness(t)
	ticket, err := h.svc.CreateVideo(t.Context(), ana, video.UploadCommand{Filename: "ferias.mp4", ContentType: "video/mp4", SizeBytes: 1000})
	require.NoError(t, err)

	_, err = h.svc.ProcessVideo(t.Context(), ana, ticket.VideoID)

	assert.ErrorIs(t, err, video.ErrUploadNotFound)
	assert.Empty(t, h.repo.requests)
}

func TestProcessVideoRejectsAnObjectThatDoesNotCarryTheVideoOwner(t *testing.T) {
	h := newHarness(t)
	ticket, err := h.svc.CreateVideo(t.Context(), ana, video.UploadCommand{Filename: "ferias.mp4", ContentType: "video/mp4", SizeBytes: 1000})
	require.NoError(t, err)
	h.store.putAs(h.repo.videos[ticket.VideoID].ObjectKey, video.ObjectOwner{UserID: "usr-bia", VideoID: ticket.VideoID})

	_, err = h.svc.ProcessVideo(t.Context(), ana, ticket.VideoID)

	assert.ErrorIs(t, err, video.ErrUploadMismatch)
	assert.Empty(t, h.repo.requests)
}

func TestCreateVideoSignsTheOwnerIntoTheUploadAndReturnsItsHeaders(t *testing.T) {
	h := newHarness(t)

	ticket, err := h.svc.CreateVideo(t.Context(), ana, video.UploadCommand{Filename: "ferias.mp4", ContentType: "video/mp4", SizeBytes: 1000})

	require.NoError(t, err)
	assert.Equal(t, video.ObjectOwner{UserID: ana.UserID, VideoID: ticket.VideoID}, h.store.presigned[0].owner)
	assert.Equal(t, map[string]string{"x-owner": ana.UserID + "/" + ticket.VideoID}, ticket.UploadHeaders)
}

func TestProcessVideoRejectsAnotherUsersVideo(t *testing.T) {
	h := newHarness(t)
	ticket := uploaded(t, h, ana, "ferias.mp4")

	_, err := h.svc.ProcessVideo(t.Context(), bia, ticket.VideoID)

	assert.ErrorIs(t, err, video.ErrForbidden)
}

func TestProcessVideoOfUnknownVideoIsNotFound(t *testing.T) {
	h := newHarness(t)

	_, err := h.svc.ProcessVideo(t.Context(), ana, "id-999")

	assert.ErrorIs(t, err, video.ErrNotFound)
}

func TestListVideosShowsOnlyTheCallersRequestsNewestFirst(t *testing.T) {
	h := newHarness(t)
	clock := &mutableClock{now: now}
	h.svc = video.NewService(h.repo, h.store, clock, &fakeIDs{}, video.Config{MaxUploadBytes: 1 << 30, UploadTTL: time.Minute, DownloadTTL: time.Minute})
	older := uploaded(t, h, ana, "a.mp4")
	_, err := h.svc.ProcessVideo(t.Context(), ana, older.VideoID)
	require.NoError(t, err)
	clock.now = now.Add(time.Minute)
	newer := uploaded(t, h, ana, "b.mp4")
	_, err = h.svc.ProcessVideo(t.Context(), ana, newer.VideoID)
	require.NoError(t, err)
	other := uploaded(t, h, bia, "c.mp4")
	_, err = h.svc.ProcessVideo(t.Context(), bia, other.VideoID)
	require.NoError(t, err)

	list, err := h.svc.ListVideos(t.Context(), ana)

	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "b.mp4", list[0].Video.Filename)
	assert.Equal(t, "a.mp4", list[1].Video.Filename)
	assert.Equal(t, video.StatusPending, list[0].Request.Status)
	assert.Nil(t, list[0].Result)
}

func TestDownloadResultReturnsPresignedURLForCompletedRequest(t *testing.T) {
	h := newHarness(t)
	ticket := uploaded(t, h, ana, "ferias.mp4")
	summary, err := h.svc.ProcessVideo(t.Context(), ana, ticket.VideoID)
	require.NoError(t, err)
	req := summary.Request
	require.NoError(t, h.svc.StartProcessing(t.Context(), "evt-1", req.ID, 1))
	require.NoError(t, h.svc.CompleteProcessing(t.Context(), "evt-2", req.ID, "results/"+req.ID+".zip", 42))

	url, err := h.svc.DownloadResult(t.Context(), ana, ticket.VideoID)

	require.NoError(t, err)
	assert.Equal(t, "https://store.local/download/results/"+req.ID+".zip", url)
}

func TestDownloadResultRefusesRequestThatIsNotCompleted(t *testing.T) {
	h := newHarness(t)
	ticket := uploaded(t, h, ana, "ferias.mp4")
	_, err := h.svc.ProcessVideo(t.Context(), ana, ticket.VideoID)
	require.NoError(t, err)

	_, err = h.svc.DownloadResult(t.Context(), ana, ticket.VideoID)

	assert.ErrorIs(t, err, video.ErrNotCompleted)
	assert.Empty(t, h.store.downloads)
}

func TestDownloadResultRefusesAnotherUsersVideo(t *testing.T) {
	h := newHarness(t)
	ticket := uploaded(t, h, ana, "ferias.mp4")
	summary, err := h.svc.ProcessVideo(t.Context(), ana, ticket.VideoID)
	require.NoError(t, err)
	req := summary.Request
	require.NoError(t, h.svc.StartProcessing(t.Context(), "evt-1", req.ID, 1))
	require.NoError(t, h.svc.CompleteProcessing(t.Context(), "evt-2", req.ID, "results/x.zip", 1))

	_, err = h.svc.DownloadResult(t.Context(), bia, ticket.VideoID)

	assert.ErrorIs(t, err, video.ErrForbidden)
}

func pendingRequest(t *testing.T, h harness) video.ProcessRequest {
	t.Helper()
	ticket := uploaded(t, h, ana, "ferias.mp4")
	summary, err := h.svc.ProcessVideo(t.Context(), ana, ticket.VideoID)
	require.NoError(t, err)
	return summary.Request
}

func TestProcessVideoLosingARaceReturnsTheWinningRequest(t *testing.T) {
	h := newHarness(t)
	ticket := uploaded(t, h, ana, "ferias.mp4")
	h.repo.failNextCreateWithExists = true

	summary, err := h.svc.ProcessVideo(t.Context(), ana, ticket.VideoID)

	require.NoError(t, err)
	assert.Equal(t, "winner", summary.Request.ID)
	assert.Equal(t, video.StatusPending, summary.Request.Status)
}

func TestStartProcessingAfterTerminalStatusIsIgnored(t *testing.T) {
	h := newHarness(t)
	req := pendingRequest(t, h)
	require.NoError(t, h.svc.CompleteProcessing(t.Context(), "evt-1", req.ID, "results/r.zip", 3))

	require.NoError(t, h.svc.StartProcessing(t.Context(), "evt-2", req.ID, 1))

	assert.Equal(t, video.StatusCompleted, h.repo.requests[req.ID].Status)
	assert.True(t, h.repo.processed["evt-2"])
}

func TestStartProcessingMovesPendingToProcessing(t *testing.T) {
	h := newHarness(t)
	req := pendingRequest(t, h)

	require.NoError(t, h.svc.StartProcessing(t.Context(), "evt-1", req.ID, 1))

	got := h.repo.requests[req.ID]
	assert.Equal(t, video.StatusProcessing, got.Status)
	assert.Equal(t, 1, got.Attempts)
}

func TestStartProcessingForASecondAttemptStaysProcessing(t *testing.T) {
	h := newHarness(t)
	req := pendingRequest(t, h)
	require.NoError(t, h.svc.StartProcessing(t.Context(), "evt-1", req.ID, 1))

	require.NoError(t, h.svc.StartProcessing(t.Context(), "evt-2", req.ID, 2))

	got := h.repo.requests[req.ID]
	assert.Equal(t, video.StatusProcessing, got.Status)
	assert.Equal(t, 2, got.Attempts)
}

func TestCompleteProcessingRecordsResultAndCompletes(t *testing.T) {
	h := newHarness(t)
	req := pendingRequest(t, h)
	require.NoError(t, h.svc.StartProcessing(t.Context(), "evt-1", req.ID, 1))

	require.NoError(t, h.svc.CompleteProcessing(t.Context(), "evt-2", req.ID, "results/r.zip", 42))

	assert.Equal(t, video.StatusCompleted, h.repo.requests[req.ID].Status)
	assert.Equal(t, video.ProcessResult{RequestID: req.ID, ZipKey: "results/r.zip", FrameCount: 42, RecordedAt: now}, h.repo.results[req.ID])
	list, _ := h.svc.ListVideos(t.Context(), ana)
	assert.Equal(t, 42, list[0].Result.FrameCount)
}

func TestProcessingEventRetryableFailureKeepsProcessingAndCountsAttempt(t *testing.T) {
	h := newHarness(t)
	req := pendingRequest(t, h)
	require.NoError(t, h.svc.StartProcessing(t.Context(), "evt-1", req.ID, 1))

	require.NoError(t, h.svc.FailProcessing(t.Context(), "evt-2", req.ID, "s3 timeout", true, 1))

	got := h.repo.requests[req.ID]
	assert.Equal(t, video.StatusProcessing, got.Status)
	assert.Equal(t, 1, got.Attempts)
	assert.Empty(t, h.repo.results)
}

func TestProcessingEventDefinitiveFailureRecordsReasonAndFails(t *testing.T) {
	h := newHarness(t)
	req := pendingRequest(t, h)
	require.NoError(t, h.svc.StartProcessing(t.Context(), "evt-1", req.ID, 1))

	require.NoError(t, h.svc.FailProcessing(t.Context(), "evt-2", req.ID, "unprocessable video", false, 1))

	assert.Equal(t, video.StatusFailed, h.repo.requests[req.ID].Status)
	assert.Equal(t, "unprocessable video", h.repo.results[req.ID].FailureReason)
}

func TestCompleteProcessingBeforeStartedCompletesAnyway(t *testing.T) {
	h := newHarness(t)
	req := pendingRequest(t, h)

	err := h.svc.CompleteProcessing(t.Context(), "evt-1", req.ID, "results/r.zip", 1)

	require.NoError(t, err)
	assert.Equal(t, video.StatusCompleted, h.repo.requests[req.ID].Status)
}

func TestProcessingEventAfterTerminalStatusIsRejected(t *testing.T) {
	h := newHarness(t)
	req := pendingRequest(t, h)
	require.NoError(t, h.svc.StartProcessing(t.Context(), "evt-1", req.ID, 1))
	require.NoError(t, h.svc.FailProcessing(t.Context(), "evt-2", req.ID, "boom", false, 1))

	err := h.svc.CompleteProcessing(t.Context(), "evt-3", req.ID, "results/r.zip", 1)

	assert.ErrorIs(t, err, video.ErrInvalidTransition)
	assert.Equal(t, video.StatusFailed, h.repo.requests[req.ID].Status)
}

func TestProcessingEventIgnoresAnEventAlreadyProcessed(t *testing.T) {
	h := newHarness(t)
	req := pendingRequest(t, h)
	require.NoError(t, h.svc.StartProcessing(t.Context(), "evt-1", req.ID, 1))
	require.NoError(t, h.svc.CompleteProcessing(t.Context(), "evt-2", req.ID, "results/r.zip", 42))

	err := h.svc.CompleteProcessing(t.Context(), "evt-2", req.ID, "results/other.zip", 1)

	require.NoError(t, err)
	assert.Equal(t, "results/r.zip", h.repo.results[req.ID].ZipKey)
}

func TestProcessingEventForUnknownRequestIsNotFound(t *testing.T) {
	h := newHarness(t)

	err := h.svc.StartProcessing(t.Context(), "evt-1", "req-missing", 1)

	assert.ErrorIs(t, err, video.ErrNotFound)
}

type fakeMetrics struct {
	uploads  int
	statuses []video.Status
}

func (m *fakeMetrics) UploadRequested()                  { m.uploads++ }
func (m *fakeMetrics) StatusChanged(status video.Status) { m.statuses = append(m.statuses, status) }

func TestServiceReportsUploadsAndEveryStatusTransition(t *testing.T) {
	h := newHarness(t)
	m := &fakeMetrics{}
	h.svc = video.NewService(h.repo, h.store, fakeClock{now}, &fakeIDs{}, video.Config{MaxUploadBytes: 1 << 30, UploadTTL: time.Minute, DownloadTTL: time.Minute}, video.WithMetrics(m))

	ticket := uploaded(t, h, ana, "ferias.mp4")
	summary, err := h.svc.ProcessVideo(t.Context(), ana, ticket.VideoID)
	require.NoError(t, err)
	require.NoError(t, h.svc.StartProcessing(t.Context(), "evt-1", summary.Request.ID, 1))
	require.NoError(t, h.svc.StartProcessing(t.Context(), "evt-1", summary.Request.ID, 1))
	require.NoError(t, h.svc.FailProcessing(t.Context(), "evt-2", summary.Request.ID, "timeout", true, 1))
	require.NoError(t, h.svc.CompleteProcessing(t.Context(), "evt-3", summary.Request.ID, "results/x.zip", 3))

	assert.Equal(t, 1, m.uploads)
	assert.Equal(t, []video.Status{video.StatusPending, video.StatusProcessing, video.StatusCompleted}, m.statuses, "duplicate events and same-status retries do not count as transitions")
}
