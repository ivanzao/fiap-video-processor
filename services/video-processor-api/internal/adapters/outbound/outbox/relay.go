package outbox

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/events"
)

type Publisher interface {
	Publish(ctx context.Context, env events.Envelope) error
}

type Source interface {
	PendingEvents(ctx context.Context, limit int) ([]events.PendingEvent, error)
	MarkPublished(ctx context.Context, ids []int64, at time.Time) error
}

type Relay struct {
	source    Source
	publisher Publisher
	log       *slog.Logger
	interval  time.Duration
	batch     int
}

func NewRelay(source Source, publisher Publisher, log *slog.Logger, interval time.Duration, batch int) *Relay {
	return &Relay{source: source, publisher: publisher, log: log, interval: interval, batch: batch}
}

func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		if err := r.Drain(ctx); err != nil {
			r.log.Error("outbox relay failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Relay) Drain(ctx context.Context) error {
	pending, err := r.source.PendingEvents(ctx, r.batch)
	if err != nil {
		return err
	}
	var published []int64
	for _, p := range pending {
		if err := r.publishOne(ctx, p.Envelope); err != nil {
			r.log.Error("outbox publish failed", "eventId", p.Envelope.EventID, "eventType", p.Envelope.EventType, "err", err)
			break
		}
		published = append(published, p.ID)
	}
	return r.source.MarkPublished(ctx, published, time.Now().UTC())
}

func (r *Relay) publishOne(ctx context.Context, env events.Envelope) error {
	ctx, span := otel.Tracer("video-processor-api/outbox").Start(ctx, "publish "+env.EventType)
	defer span.End()
	span.SetAttributes(attribute.String("messaging.message.id", env.EventID))
	return r.publisher.Publish(ctx, env)
}
