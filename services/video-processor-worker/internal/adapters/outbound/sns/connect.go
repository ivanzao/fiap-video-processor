package sns

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
)

type Config struct {
	Region   string
	Endpoint string
	TopicARN string
}

func Connect(ctx context.Context, cfg Config, ids video.IDGenerator, clock video.Clock) (*Publisher, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("sns: load aws config: %w", err)
	}
	client := awssns.NewFromConfig(awsCfg, func(o *awssns.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = &cfg.Endpoint
		}
	})
	return NewPublisher(client, cfg.TopicARN, ids, clock), nil
}
