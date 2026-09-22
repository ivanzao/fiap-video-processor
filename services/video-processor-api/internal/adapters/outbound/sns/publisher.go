package sns

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/events"
)

type Publisher struct {
	client   *awssns.Client
	topicARN string
}

func NewPublisher(client *awssns.Client, topicARN string) *Publisher {
	return &Publisher{client: client, topicARN: topicARN}
}

func (p *Publisher) Publish(ctx context.Context, env events.Envelope) error {
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
	_, err = p.client.Publish(ctx, &awssns.PublishInput{
		TopicArn:          aws.String(p.topicARN),
		Message:           aws.String(string(body)),
		MessageAttributes: attrs,
	})
	if err != nil {
		return fmt.Errorf("sns: publish %s: %w", env.EventType, err)
	}
	return nil
}
