//go:build integration

package postgres_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/postgres"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/testinfra"
)

var fixedNow = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

func TestRepositoryUpsertsExecutionsThroughTheirLifecycle(t *testing.T) {
	repo := postgres.NewExecutionRepository(testinfra.StartPostgres(t))
	ctx := t.Context()

	_, err := repo.FindExecution(ctx, "req-1")
	require.ErrorIs(t, err, video.ErrNotFound)

	running := video.Execution{RequestID: "req-1", Attempt: 1, Outcome: video.OutcomeRunning, StartedAt: fixedNow}
	require.NoError(t, repo.SaveExecution(ctx, running))
	found, err := repo.FindExecution(ctx, "req-1")
	require.NoError(t, err)
	assert.Equal(t, running, found)

	retrying := video.Execution{RequestID: "req-1", Attempt: 1, Outcome: video.OutcomeRetrying, Reason: "storage down", StartedAt: fixedNow, FinishedAt: fixedNow.Add(time.Second)}
	require.NoError(t, repo.SaveExecution(ctx, retrying))
	found, err = repo.FindExecution(ctx, "req-1")
	require.NoError(t, err)
	assert.Equal(t, retrying, found)

	completed := video.Execution{RequestID: "req-1", Attempt: 2, Outcome: video.OutcomeCompleted, ZipKey: "results/usr-ana/req-1.zip", FrameCount: 9, StartedAt: fixedNow.Add(time.Minute), FinishedAt: fixedNow.Add(2 * time.Minute)}
	require.NoError(t, repo.SaveExecution(ctx, completed))
	found, err = repo.FindExecution(ctx, "req-1")
	require.NoError(t, err)
	assert.Equal(t, completed, found)
}

func TestRepositoryRecordsNotificationsOncePerOutcome(t *testing.T) {
	repo := postgres.NewExecutionRepository(testinfra.StartPostgres(t))
	ctx := t.Context()
	require.NoError(t, repo.SaveExecution(ctx, video.Execution{RequestID: "req-1", Attempt: 1, Outcome: video.OutcomeFailed, Reason: "unprocessable video", StartedAt: fixedNow, FinishedAt: fixedNow}))

	notified, err := repo.WasNotified(ctx, "req-1", video.OutcomeFailed)
	require.NoError(t, err)
	assert.False(t, notified)

	require.NoError(t, repo.MarkNotified(ctx, "req-1", video.OutcomeFailed))
	require.NoError(t, repo.MarkNotified(ctx, "req-1", video.OutcomeFailed))

	notified, err = repo.WasNotified(ctx, "req-1", video.OutcomeFailed)
	require.NoError(t, err)
	assert.True(t, notified)
	notified, err = repo.WasNotified(ctx, "req-1", video.OutcomeCompleted)
	require.NoError(t, err)
	assert.False(t, notified)
}
