package sns

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/events"
)

type Publisher struct {
	client   *awssns.Client
	topicARN string
	ids      video.IDGenerator
	clock    video.Clock
}

func NewPublisher(client *awssns.Client, topicARN string, ids video.IDGenerator, clock video.Clock) *Publisher {
	return &Publisher{client: client, topicARN: topicARN, ids: ids, clock: clock}
}

func (p *Publisher) PublishStarted(ctx context.Context, e video.ProcessingStarted) error {
	return p.publish(ctx, events.VideoProcessingStarted{RequestID: e.RequestID, Attempt: e.Attempt})
}

func (p *Publisher) PublishCompleted(ctx context.Context, e video.ProcessingCompleted) error {
	return p.publish(ctx, events.VideoProcessingCompleted{
		RequestID: e.RequestID, ZipKey: e.ZipKey, FrameCount: e.FrameCount, DurationMs: e.Duration.Milliseconds(),
	})
}

func (p *Publisher) PublishFailed(ctx context.Context, e video.ProcessingFailed) error {
	return p.publish(ctx, events.VideoProcessingFailed{RequestID: e.RequestID, Reason: e.Reason, Retryable: e.Retryable, Attempt: e.Attempt})
}

func (p *Publisher) publish(ctx context.Context, payload events.Payload) error {
	env := events.Wrap(p.ids.NewID(), p.clock.Now(), payload)
	body, err := events.Encode(env)
	if err != nil {
		return fmt.Errorf("sns: encode: %w", err)
	}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	attrs := map[string]types.MessageAttributeValue{}
	for k, v := range events.MessageAttributes(env, carrier.Get(events.AttributeTraceParent)) {
		attrs[k] = types.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(v)}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := p.client.Publish(ctx, &awssns.PublishInput{
		TopicArn: aws.String(p.topicARN), Message: aws.String(string(body)), MessageAttributes: attrs,
	}); err != nil {
		return fmt.Errorf("sns: publish %s: %w", env.EventType, err)
	}
	return nil
}
