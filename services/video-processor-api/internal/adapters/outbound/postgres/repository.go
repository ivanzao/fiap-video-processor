package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/events"
)

type VideoRepository struct {
	pool *Pool
}

func NewVideoRepository(pool *Pool) *VideoRepository {
	return &VideoRepository{pool: pool}
}

type videoRow struct {
	ID          uuid.UUID
	UserID      string
	Filename    string
	ContentType string
	SizeBytes   int64
	ObjectKey   string
	CreatedAt   time.Time
}

func (r videoRow) toDomain() video.Video {
	return video.Video{
		ID: r.ID.String(), UserID: r.UserID, Filename: r.Filename, ContentType: r.ContentType,
		SizeBytes: r.SizeBytes, ObjectKey: r.ObjectKey, CreatedAt: r.CreatedAt.UTC(),
	}
}

type processRequestRow struct {
	ID        uuid.UUID
	VideoID   uuid.UUID
	UserID    string
	Status    string
	Attempts  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (r processRequestRow) toDomain() video.ProcessRequest {
	return video.ProcessRequest{
		ID: r.ID.String(), VideoID: r.VideoID.String(), UserID: r.UserID, Status: video.Status(r.Status),
		Attempts: r.Attempts, CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
}

type processResultRow struct {
	RequestID     uuid.UUID
	ZipKey        *string
	FrameCount    *int
	FailureReason *string
	RecordedAt    time.Time
}

func (r processResultRow) toDomain() video.ProcessResult {
	res := video.ProcessResult{RequestID: r.RequestID.String(), RecordedAt: r.RecordedAt.UTC()}
	if r.ZipKey != nil {
		res.ZipKey = *r.ZipKey
	}
	if r.FrameCount != nil {
		res.FrameCount = *r.FrameCount
	}
	if r.FailureReason != nil {
		res.FailureReason = *r.FailureReason
	}
	return res
}

func (r *VideoRepository) SaveVideo(ctx context.Context, v video.Video) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO video (id, user_id, filename, content_type, size_bytes, object_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		v.ID, v.UserID, v.Filename, v.ContentType, v.SizeBytes, v.ObjectKey, v.CreatedAt)
	return wrap("save video", err)
}

func (r *VideoRepository) FindVideo(ctx context.Context, id string) (video.Video, error) {
	row, err := queryOne[videoRow](ctx, r.pool, "find video", `
		SELECT id, user_id, filename, content_type, size_bytes, object_key, created_at
		FROM video WHERE id = $1`, asUUID(id))
	if err != nil {
		return video.Video{}, err
	}
	return row.toDomain(), nil
}

func (r *VideoRepository) FindRequest(ctx context.Context, id string) (video.ProcessRequest, error) {
	return r.findRequestWhere(ctx, "id = $1", asUUID(id))
}

func (r *VideoRepository) FindRequestByVideo(ctx context.Context, videoID string) (video.ProcessRequest, error) {
	return r.findRequestWhere(ctx, "video_id = $1", asUUID(videoID))
}

func (r *VideoRepository) findRequestWhere(ctx context.Context, where string, arg any) (video.ProcessRequest, error) {
	row, err := queryOne[processRequestRow](ctx, r.pool, "find process request", `
		SELECT id, video_id, user_id, status, attempts, created_at, updated_at
		FROM video_process_request WHERE `+where, arg)
	if err != nil {
		return video.ProcessRequest{}, err
	}
	return row.toDomain(), nil
}

func (r *VideoRepository) CreateRequest(ctx context.Context, req video.ProcessRequest, evt video.ProcessingRequested) error {
	env := events.Wrap(uuid.Must(uuid.NewV7()).String(), req.CreatedAt, events.VideoProcessingRequested{
		RequestID: evt.RequestID, VideoID: evt.VideoID, UserID: evt.UserID,
		UserEmail: evt.UserEmail, ObjectKey: evt.ObjectKey, Filename: evt.Filename,
	})
	return r.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO video_process_request (id, video_id, user_id, status, attempts, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			req.ID, req.VideoID, req.UserID, string(req.Status), req.Attempts, req.CreatedAt, req.UpdatedAt); err != nil {
			if isUniqueViolation(err) {
				return video.ErrRequestExists
			}
			return wrap("insert process request", err)
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO outbox (event_id, event_type, event_version, payload, occurred_at) VALUES ($1, $2, $3, $4, $5)`,
			env.EventID, env.EventType, env.EventVersion, env.Payload, env.OccurredAt)
		return wrap("insert outbox", err)
	})
}

func (r *VideoRepository) ListByUser(ctx context.Context, userID string) ([]video.RequestSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT q.id, q.video_id, q.user_id, q.status, q.attempts, q.created_at, q.updated_at,
		       v.id, v.user_id, v.filename, v.content_type, v.size_bytes, v.object_key, v.created_at,
		       s.request_id, s.zip_key, s.frame_count, s.failure_reason, s.recorded_at
		FROM video_process_request q
		JOIN video v ON v.id = q.video_id
		LEFT JOIN video_process_result s ON s.request_id = q.id
		WHERE q.user_id = $1
		ORDER BY q.created_at DESC`, userID)
	if err != nil {
		return nil, wrap("list by user", err)
	}
	defer rows.Close()
	var out []video.RequestSummary
	for rows.Next() {
		var q processRequestRow
		var v videoRow
		var resID *uuid.UUID
		var res processResultRow
		var recordedAt *time.Time
		if err := rows.Scan(
			&q.ID, &q.VideoID, &q.UserID, &q.Status, &q.Attempts, &q.CreatedAt, &q.UpdatedAt,
			&v.ID, &v.UserID, &v.Filename, &v.ContentType, &v.SizeBytes, &v.ObjectKey, &v.CreatedAt,
			&resID, &res.ZipKey, &res.FrameCount, &res.FailureReason, &recordedAt,
		); err != nil {
			return nil, wrap("scan list by user", err)
		}
		summary := video.RequestSummary{Request: q.toDomain(), Video: v.toDomain()}
		if resID != nil {
			res.RequestID, res.RecordedAt = *resID, *recordedAt
			result := res.toDomain()
			summary.Result = &result
		}
		out = append(out, summary)
	}
	return out, wrap("iterate list by user", rows.Err())
}

func (r *VideoRepository) FindResult(ctx context.Context, requestID string) (video.ProcessResult, error) {
	row, err := queryOne[processResultRow](ctx, r.pool, "find process result", `
		SELECT request_id, zip_key, frame_count, failure_reason, recorded_at
		FROM video_process_result WHERE request_id = $1`, asUUID(requestID))
	if err != nil {
		return video.ProcessResult{}, err
	}
	return row.toDomain(), nil
}

func (r *VideoRepository) WasEventProcessed(ctx context.Context, eventID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM processed_event WHERE event_id = $1)`, eventID).Scan(&exists)
	return exists, wrap("check processed event", err)
}

func (r *VideoRepository) ApplyEvent(ctx context.Context, eventID string, req video.ProcessRequest, res *video.ProcessResult) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE video_process_request SET status = $2, attempts = $3, updated_at = $4 WHERE id = $1`,
			req.ID, string(req.Status), req.Attempts, req.UpdatedAt); err != nil {
			return wrap("update process request", err)
		}
		if res != nil {
			if _, err := tx.Exec(ctx, `
				INSERT INTO video_process_result (request_id, zip_key, frame_count, failure_reason, recorded_at)
				VALUES ($1, NULLIF($2, ''), $3, NULLIF($4, ''), $5)
				ON CONFLICT (request_id) DO UPDATE
				SET zip_key = EXCLUDED.zip_key, frame_count = EXCLUDED.frame_count,
				    failure_reason = EXCLUDED.failure_reason, recorded_at = EXCLUDED.recorded_at`,
				res.RequestID, res.ZipKey, frameCountOrNull(res), res.FailureReason, res.RecordedAt); err != nil {
				return wrap("upsert process result", err)
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO processed_event (event_id, processed_at) VALUES ($1, $2)`, eventID, req.UpdatedAt)
		return wrap("insert processed event", err)
	})
}

func frameCountOrNull(res *video.ProcessResult) *int {
	if res.ZipKey == "" {
		return nil
	}
	return &res.FrameCount
}

func (r *VideoRepository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return wrap("begin", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return wrap("commit", tx.Commit(ctx))
}

func asUUID(id string) uuid.UUID {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil
	}
	return parsed
}

func queryOne[T any](ctx context.Context, pool *Pool, op, sql string, args ...any) (T, error) {
	rows, _ := pool.Query(ctx, sql, args...)
	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[T])
	if errors.Is(err, pgx.ErrNoRows) {
		return row, video.ErrNotFound
	}
	return row, wrap(op, err)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("postgres: %s: %w", op, err)
}
