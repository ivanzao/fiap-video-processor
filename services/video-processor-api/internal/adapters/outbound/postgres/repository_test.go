//go:build integration

package postgres_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/outbound/postgres"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/events"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/testinfra"
)

var fixedNow = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

func newVideo() video.Video {
	const user = "usr-ana"
	id := uuid.Must(uuid.NewV7()).String()
	return video.Video{ID: id, UserID: user, Filename: "ferias.mp4", ContentType: "video/mp4", SizeBytes: 1000, ObjectKey: "uploads/" + user + "/" + id + ".mp4", CreatedAt: fixedNow}
}

func TestRepositoryStoresVideoAndRequestWithOutboxEventInOneTransaction(t *testing.T) {
	pool := testinfra.StartPostgres(t)
	repo := postgres.NewVideoRepository(pool)
	ctx := t.Context()
	v := newVideo()
	require.NoError(t, repo.SaveVideo(ctx, v))
	req := video.ProcessRequest{ID: uuid.Must(uuid.NewV7()).String(), VideoID: v.ID, UserID: v.UserID, Status: video.StatusPending, CreatedAt: fixedNow, UpdatedAt: fixedNow}

	err := repo.CreateRequest(ctx, req, video.ProcessingRequested{RequestID: req.ID, VideoID: v.ID, UserID: v.UserID, UserEmail: "ana@example.com", ObjectKey: v.ObjectKey, Filename: v.Filename})

	require.NoError(t, err)
	found, err := repo.FindVideo(ctx, v.ID)
	require.NoError(t, err)
	assert.Equal(t, v, found)
	byVideo, err := repo.FindRequestByVideo(ctx, v.ID)
	require.NoError(t, err)
	assert.Equal(t, req, byVideo)
	pending, err := postgres.NewOutbox(pool).PendingEvents(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, events.TypeVideoProcessingRequested, pending[0].Envelope.EventType)
	payload, err := events.PayloadAs[events.VideoProcessingRequested](pending[0].Envelope)
	require.NoError(t, err)
	assert.Equal(t, "ana@example.com", payload.UserEmail)
	assert.Equal(t, req.ID, payload.RequestID)
}

func TestRepositoryMarksOutboxEventsPublished(t *testing.T) {
	pool := testinfra.StartPostgres(t)
	repo := postgres.NewVideoRepository(pool)
	ctx := t.Context()
	v := newVideo()
	require.NoError(t, repo.SaveVideo(ctx, v))
	req := video.ProcessRequest{ID: uuid.Must(uuid.NewV7()).String(), VideoID: v.ID, UserID: v.UserID, Status: video.StatusPending, CreatedAt: fixedNow, UpdatedAt: fixedNow}
	require.NoError(t, repo.CreateRequest(ctx, req, video.ProcessingRequested{RequestID: req.ID, VideoID: v.ID, UserID: v.UserID}))
	pending, err := postgres.NewOutbox(pool).PendingEvents(ctx, 10)
	require.NoError(t, err)

	require.NoError(t, postgres.NewOutbox(pool).MarkPublished(ctx, []int64{pending[0].ID}, fixedNow))

	after, err := postgres.NewOutbox(pool).PendingEvents(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, after)
}

func TestRepositoryAppliesEventsIdempotentlyAndListsNewestFirst(t *testing.T) {
	pool := testinfra.StartPostgres(t)
	repo := postgres.NewVideoRepository(pool)
	ctx := t.Context()
	older, newer := newVideo(), newVideo()
	newer.CreatedAt = fixedNow.Add(time.Minute)
	require.NoError(t, repo.SaveVideo(ctx, older))
	require.NoError(t, repo.SaveVideo(ctx, newer))
	olderReq := video.ProcessRequest{ID: uuid.Must(uuid.NewV7()).String(), VideoID: older.ID, UserID: "usr-ana", Status: video.StatusPending, CreatedAt: fixedNow, UpdatedAt: fixedNow}
	newerReq := video.ProcessRequest{ID: uuid.Must(uuid.NewV7()).String(), VideoID: newer.ID, UserID: "usr-ana", Status: video.StatusPending, CreatedAt: fixedNow.Add(time.Minute), UpdatedAt: fixedNow.Add(time.Minute)}
	require.NoError(t, repo.CreateRequest(ctx, olderReq, video.ProcessingRequested{RequestID: olderReq.ID}))
	require.NoError(t, repo.CreateRequest(ctx, newerReq, video.ProcessingRequested{RequestID: newerReq.ID}))
	eventID := uuid.Must(uuid.NewV7()).String()
	olderReq.Status, olderReq.Attempts = video.StatusCompleted, 1
	result := video.ProcessResult{RequestID: olderReq.ID, ZipKey: "results/x.zip", FrameCount: 7, RecordedAt: fixedNow}

	require.NoError(t, repo.ApplyEvent(ctx, eventID, olderReq, &result))

	processed, err := repo.WasEventProcessed(ctx, eventID)
	require.NoError(t, err)
	assert.True(t, processed)
	assert.Error(t, repo.ApplyEvent(ctx, eventID, olderReq, &result), "same event id must violate the processed_event primary key")
	list, err := repo.ListByUser(ctx, "usr-ana")
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, newer.ID, list[0].Video.ID)
	assert.Nil(t, list[0].Result)
	assert.Equal(t, older.ID, list[1].Video.ID)
	require.NotNil(t, list[1].Result)
	assert.Equal(t, 7, list[1].Result.FrameCount)
	assert.Equal(t, video.StatusCompleted, list[1].Request.Status)
	other, err := repo.ListByUser(ctx, "usr-bia")
	require.NoError(t, err)
	assert.Empty(t, other)
}

func TestRepositoryRejectsSecondRequestForSameVideoAsExisting(t *testing.T) {
	pool := testinfra.StartPostgres(t)
	repo := postgres.NewVideoRepository(pool)
	ctx := t.Context()
	v := newVideo()
	require.NoError(t, repo.SaveVideo(ctx, v))
	first := video.ProcessRequest{ID: uuid.Must(uuid.NewV7()).String(), VideoID: v.ID, UserID: v.UserID, Status: video.StatusPending, CreatedAt: fixedNow, UpdatedAt: fixedNow}
	second := first
	second.ID = uuid.Must(uuid.NewV7()).String()
	require.NoError(t, repo.CreateRequest(ctx, first, video.ProcessingRequested{RequestID: first.ID}))

	err := repo.CreateRequest(ctx, second, video.ProcessingRequested{RequestID: second.ID})

	assert.ErrorIs(t, err, video.ErrRequestExists)
	pending, err := postgres.NewOutbox(pool).PendingEvents(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, pending, 1)
}

func TestRepositoryReportsNotFound(t *testing.T) {
	pool := testinfra.StartPostgres(t)
	repo := postgres.NewVideoRepository(pool)

	_, err := repo.FindVideo(t.Context(), uuid.Must(uuid.NewV7()).String())
	assert.ErrorIs(t, err, video.ErrNotFound)
	_, err = repo.FindRequest(t.Context(), "not-a-uuid")
	assert.ErrorIs(t, err, video.ErrNotFound)
	_, err = repo.FindResult(t.Context(), uuid.Must(uuid.NewV7()).String())
	assert.ErrorIs(t, err, video.ErrNotFound)
}
