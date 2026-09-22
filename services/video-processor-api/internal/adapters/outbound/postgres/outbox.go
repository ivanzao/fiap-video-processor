package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/events"
)

type Outbox struct {
	pool *Pool
}

func NewOutbox(pool *Pool) *Outbox {
	return &Outbox{pool: pool}
}

func (o *Outbox) PendingEvents(ctx context.Context, limit int) ([]events.PendingEvent, error) {
	rows, err := o.pool.Query(ctx, `
		SELECT id, event_id, event_type, event_version, payload, occurred_at
		FROM outbox WHERE published_at IS NULL ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return nil, wrap("select pending outbox", err)
	}
	defer rows.Close()
	var out []events.PendingEvent
	for rows.Next() {
		var p events.PendingEvent
		var payload []byte
		if err := rows.Scan(&p.ID, &p.Envelope.EventID, &p.Envelope.EventType, &p.Envelope.EventVersion, &payload, &p.Envelope.OccurredAt); err != nil {
			return nil, wrap("scan pending outbox", err)
		}
		p.Envelope.OccurredAt = p.Envelope.OccurredAt.UTC()
		p.Envelope.Payload = json.RawMessage(payload)
		out = append(out, p)
	}
	return out, wrap("iterate pending outbox", rows.Err())
}

func (o *Outbox) MarkPublished(ctx context.Context, ids []int64, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := o.pool.Exec(ctx, `UPDATE outbox SET published_at = $2 WHERE id = ANY($1)`, ids, at)
	return wrap("mark outbox published", err)
}
