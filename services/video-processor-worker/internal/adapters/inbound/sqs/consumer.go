package sqs

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/events"
)

type Metrics interface {
	MessageHandled(result string, duration time.Duration)
}

type nopMetrics struct{}

func (nopMetrics) MessageHandled(string, time.Duration) {}

type Executor interface {
	ExecuteProcessRequest(ctx context.Context, req video.ProcessRequest) error
}

type Config struct {
	Region            string
	Endpoint          string
	QueueURL          string
	Concurrency       int
	VisibilityTimeout time.Duration
	Heartbeat         time.Duration
	RetryDelay        time.Duration
	Metrics           Metrics
}

type Consumer struct {
	client *awssqs.Client
	svc    Executor
	log    *slog.Logger
	cfg    Config
}

func NewConsumer(client *awssqs.Client, svc Executor, log *slog.Logger, cfg Config) *Consumer {
	if cfg.Metrics == nil {
		cfg.Metrics = nopMetrics{}
	}
	return &Consumer{client: client, svc: svc, log: log, cfg: cfg}
}

func (c *Consumer) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < c.cfg.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.loop(ctx)
		}()
	}
	wg.Wait()
}

func (c *Consumer) loop(ctx context.Context) {
	for ctx.Err() == nil {
		out, err := c.client.ReceiveMessage(ctx, &awssqs.ReceiveMessageInput{
			QueueUrl:                    aws.String(c.cfg.QueueURL),
			MaxNumberOfMessages:         1,
			WaitTimeSeconds:             20,
			VisibilityTimeout:           int32(c.cfg.VisibilityTimeout.Seconds()),
			MessageSystemAttributeNames: []types.MessageSystemAttributeName{types.MessageSystemAttributeNameApproximateReceiveCount},
			MessageAttributeNames:       []string{"All"},
		})
		if err != nil {
			if ctx.Err() == nil {
				c.log.Error("sqs receive failed", "err", err)
				time.Sleep(time.Second)
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
	ctx, span := otel.Tracer("video-processor-worker/sqs").Start(ctx, "process video")
	defer span.End()
	start := time.Now()

	attempt, _ := strconv.Atoi(msg.Attributes[string(types.MessageSystemAttributeNameApproximateReceiveCount)])
	attempt = max(attempt, 1)
	log := c.log.With("messageId", aws.ToString(msg.MessageId), "attempt", attempt)

	req, err := c.decode(msg, attempt)
	if err != nil {
		log.Warn("discarding malformed message", "err", err)
		c.delete(ctx, msg, log)
		c.cfg.Metrics.MessageHandled("malformed", time.Since(start))
		return
	}
	log = log.With("requestId", req.RequestID)
	span.SetAttributes(attribute.String("video.request_id", req.RequestID), attribute.Int("video.attempt", attempt))

	stopHeartbeat := c.heartbeat(ctx, msg, log)
	err = c.svc.ExecuteProcessRequest(ctx, req)
	stopHeartbeat()

	switch {
	case err == nil:
		log.Info("process request settled")
		c.delete(ctx, msg, log)
		c.cfg.Metrics.MessageHandled("settled", time.Since(start))
	case errors.Is(err, video.ErrRetryLater):
		span.RecordError(err)
		log.Warn("process request will be retried", "err", err)
		c.changeVisibility(ctx, msg, c.cfg.RetryDelay, log)
		c.cfg.Metrics.MessageHandled("retry", time.Since(start))
	default:
		span.RecordError(err)
		log.Error("process request failed unexpectedly, leaving message for redelivery", "err", err)
		c.changeVisibility(ctx, msg, c.cfg.RetryDelay, log)
		c.cfg.Metrics.MessageHandled("error", time.Since(start))
	}
}

func (c *Consumer) decode(msg types.Message, attempt int) (video.ProcessRequest, error) {
	env, err := events.Decode([]byte(aws.ToString(msg.Body)))
	if err != nil {
		return video.ProcessRequest{}, err
	}
	p, err := events.PayloadAs[events.VideoProcessingRequested](env)
	if err != nil {
		return video.ProcessRequest{}, err
	}
	return video.ProcessRequest{
		RequestID: p.RequestID, VideoID: p.VideoID, UserID: p.UserID, UserEmail: p.UserEmail,
		ObjectKey: p.ObjectKey, Filename: p.Filename, Attempt: attempt,
	}, nil
}

func (c *Consumer) heartbeat(ctx context.Context, msg types.Message, log *slog.Logger) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(c.cfg.Heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.changeVisibility(ctx, msg, c.cfg.VisibilityTimeout, log)
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func (c *Consumer) changeVisibility(ctx context.Context, msg types.Message, timeout time.Duration, log *slog.Logger) {
	_, err := c.client.ChangeMessageVisibility(context.WithoutCancel(ctx), &awssqs.ChangeMessageVisibilityInput{
		QueueUrl: aws.String(c.cfg.QueueURL), ReceiptHandle: msg.ReceiptHandle, VisibilityTimeout: int32(timeout.Seconds()),
	})
	if err != nil {
		log.Error("sqs change visibility failed", "err", err)
	}
}

func (c *Consumer) delete(ctx context.Context, msg types.Message, log *slog.Logger) {
	_, err := c.client.DeleteMessage(context.WithoutCancel(ctx), &awssqs.DeleteMessageInput{
		QueueUrl: aws.String(c.cfg.QueueURL), ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		log.Error("sqs delete failed", "err", err)
	}
}
