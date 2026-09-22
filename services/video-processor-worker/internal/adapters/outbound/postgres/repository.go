package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
)

type ExecutionRepository struct {
	pool *Pool
}

func NewExecutionRepository(pool *Pool) *ExecutionRepository {
	return &ExecutionRepository{pool: pool}
}

type executionRow struct {
	RequestID  string
	Attempt    int
	Outcome    string
	ZipKey     *string
	FrameCount *int
	Reason     *string
	StartedAt  time.Time
	FinishedAt *time.Time
}

func (r executionRow) toDomain() video.Execution {
	e := video.Execution{RequestID: r.RequestID, Attempt: r.Attempt, Outcome: video.Outcome(r.Outcome), StartedAt: r.StartedAt.UTC()}
	if r.ZipKey != nil {
		e.ZipKey = *r.ZipKey
	}
	if r.FrameCount != nil {
		e.FrameCount = *r.FrameCount
	}
	if r.Reason != nil {
		e.Reason = *r.Reason
	}
	if r.FinishedAt != nil {
		e.FinishedAt = r.FinishedAt.UTC()
	}
	return e
}

func (r *ExecutionRepository) FindExecution(ctx context.Context, requestID string) (video.Execution, error) {
	rows, _ := r.pool.Query(ctx, `
		SELECT request_id, attempt, outcome, zip_key, frame_count, reason, started_at, finished_at
		FROM processing_execution WHERE request_id = $1`, requestID)
	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[executionRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return video.Execution{}, video.ErrNotFound
	}
	if err != nil {
		return video.Execution{}, fmt.Errorf("postgres: find execution: %w", err)
	}
	return row.toDomain(), nil
}

func (r *ExecutionRepository) SaveExecution(ctx context.Context, e video.Execution) error {
	var finished *time.Time
	if !e.FinishedAt.IsZero() {
		finished = &e.FinishedAt
	}
	var frames *int
	if e.Outcome == video.OutcomeCompleted {
		frames = &e.FrameCount
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO processing_execution (request_id, attempt, outcome, zip_key, frame_count, reason, started_at, finished_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, NULLIF($6, ''), $7, $8)
		ON CONFLICT (request_id) DO UPDATE SET
			attempt = EXCLUDED.attempt, outcome = EXCLUDED.outcome, zip_key = EXCLUDED.zip_key,
			frame_count = EXCLUDED.frame_count, reason = EXCLUDED.reason,
			started_at = EXCLUDED.started_at, finished_at = EXCLUDED.finished_at`,
		e.RequestID, e.Attempt, string(e.Outcome), e.ZipKey, frames, e.Reason, e.StartedAt, finished)
	if err != nil {
		return fmt.Errorf("postgres: save execution: %w", err)
	}
	return nil
}

func (r *ExecutionRepository) WasNotified(ctx context.Context, requestID string, outcome video.Outcome) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM notification_log WHERE request_id = $1 AND outcome = $2)`, requestID, string(outcome)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres: was notified: %w", err)
	}
	return exists, nil
}

func (r *ExecutionRepository) MarkNotified(ctx context.Context, requestID string, outcome video.Outcome) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notification_log (request_id, outcome, sent_at) VALUES ($1, $2, now())
		ON CONFLICT DO NOTHING`, requestID, string(outcome))
	if err != nil {
		return fmt.Errorf("postgres: mark notified: %w", err)
	}
	return nil
}
