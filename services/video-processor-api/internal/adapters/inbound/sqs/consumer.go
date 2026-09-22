package sqs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/events"
)

const receiveBackoff = time.Second

var errMalformed = errors.New("sqs: malformed message")

func discardable(err error) bool {
	return errors.Is(err, errMalformed) || errors.Is(err, video.ErrInvalidTransition) || errors.Is(err, video.ErrNotFound) || errors.Is(err, events.ErrTypeMismatch)
}

type Metrics interface {
	EventConsumed(eventType, result string)
}

type nopMetrics struct{}

func (nopMetrics) EventConsumed(string, string) {}

type Option func(*Consumer)

func WithMetrics(m Metrics) Option {
	return func(c *Consumer) { c.metrics = m }
}

type ProcessingTracker interface {
	StartProcessing(ctx context.Context, eventID, requestID string, attempt int) error
	CompleteProcessing(ctx context.Context, eventID, requestID, zipKey string, frameCount int) error
	FailProcessing(ctx context.Context, eventID, requestID, reason string, retryable bool, attempt int) error
}

type Consumer struct {
	client   *awssqs.Client
	queueURL string
	svc      ProcessingTracker
	log      *slog.Logger
	metrics  Metrics
}

func NewConsumer(client *awssqs.Client, queueURL string, svc ProcessingTracker, log *slog.Logger, opts ...Option) *Consumer {
	c := &Consumer{client: client, queueURL: queueURL, svc: svc, log: log, metrics: nopMetrics{}}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Consumer) Run(ctx context.Context) {
	for ctx.Err() == nil {
		out, err := c.client.ReceiveMessage(ctx, &awssqs.ReceiveMessageInput{
			QueueUrl:              aws.String(c.queueURL),
			MaxNumberOfMessages:   10,
			WaitTimeSeconds:       20,
			MessageAttributeNames: []string{"All"},
		})
		if err != nil {
			if ctx.Err() == nil {
				c.log.Error("sqs receive failed", "err", err)
				time.Sleep(receiveBackoff)
			}
			continue
		}
		for _, msg := range out.Messages {
			c.consume(ctx, msg)
		}
	}
}

func (c *Consumer) consume(ctx context.Context, msg types.Message) {
	carrier := propagation.MapCarrier{}
	if tp, ok := msg.MessageAttributes[events.AttributeTraceParent]; ok {
		carrier.Set(events.AttributeTraceParent, aws.ToString(tp.StringValue))
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	ctx, span := otel.Tracer("video-processor-api/sqs").Start(ctx, "consume worker event")
	defer span.End()
	log := c.log.With("messageId", aws.ToString(msg.MessageId))
	err := c.Handle(ctx, []byte(aws.ToString(msg.Body)))
	if err != nil && !discardable(err) {
		span.RecordError(err)
		log.Error("event application failed, leaving message for redelivery", "err", err)
		return
	}
	if err != nil {
		log.Warn("event discarded", "err", err)
	}
	if _, err := c.client.DeleteMessage(ctx, &awssqs.DeleteMessageInput{QueueUrl: aws.String(c.queueURL), ReceiptHandle: msg.ReceiptHandle}); err != nil {
		c.log.Error("sqs delete failed", "err", err)
	}
}

func (c *Consumer) Handle(ctx context.Context, body []byte) error {
	eventType, err := c.handle(ctx, body)
	switch {
	case err == nil:
		c.metrics.EventConsumed(eventType, "applied")
	case discardable(err):
		c.metrics.EventConsumed(eventType, "discarded")
	default:
		c.metrics.EventConsumed(eventType, "failed")
	}
	return err
}

func (c *Consumer) handle(ctx context.Context, body []byte) (string, error) {
	env, err := events.Decode(body)
	if err != nil {
		return "unknown", fmt.Errorf("%w: %w", errMalformed, err)
	}
	return env.EventType, c.dispatch(ctx, env)
}

func (c *Consumer) dispatch(ctx context.Context, env events.Envelope) error {
	switch env.EventType {
	case events.TypeVideoProcessingStarted:
		p, err := events.PayloadAs[events.VideoProcessingStarted](env)
		if err != nil {
			return err
		}
		return c.svc.StartProcessing(ctx, env.EventID, p.RequestID, p.Attempt)
	case events.TypeVideoProcessingCompleted:
		p, err := events.PayloadAs[events.VideoProcessingCompleted](env)
		if err != nil {
			return err
		}
		return c.svc.CompleteProcessing(ctx, env.EventID, p.RequestID, p.ZipKey, p.FrameCount)
	case events.TypeVideoProcessingFailed:
		p, err := events.PayloadAs[events.VideoProcessingFailed](env)
		if err != nil {
			return err
		}
		return c.svc.FailProcessing(ctx, env.EventID, p.RequestID, p.Reason, p.Retryable, p.Attempt)
	default:
		return fmt.Errorf("%w: unhandled %s", errMalformed, env.EventType)
	}
}
