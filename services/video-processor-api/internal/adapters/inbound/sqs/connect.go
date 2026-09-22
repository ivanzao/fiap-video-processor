package sqs

import (
	"context"
	"fmt"
	"log/slog"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
)

type Config struct {
	Region   string
	Endpoint string
	QueueURL string
}

func Connect(ctx context.Context, cfg Config, svc ProcessingTracker, log *slog.Logger, opts ...Option) (*Consumer, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("sqs: load aws config: %w", err)
	}
	client := awssqs.NewFromConfig(awsCfg, func(o *awssqs.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = &cfg.Endpoint
		}
	})
	return NewConsumer(client, cfg.QueueURL, svc, log, opts...), nil
}
