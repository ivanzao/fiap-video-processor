package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type Config struct {
	Region         string
	Endpoint       string
	PublicEndpoint string
	Bucket         string
}

func Connect(ctx context.Context, cfg Config) (*Store, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("s3: load aws config: %w", err)
	}
	client := newClient(awsCfg, cfg.Endpoint)
	presignClient := client
	if cfg.PublicEndpoint != "" {
		presignClient = newClient(awsCfg, cfg.PublicEndpoint)
	}
	return NewStore(client, presignClient, cfg.Bucket), nil
}

func newClient(awsCfg aws.Config, endpoint string) *awss3.Client {
	return awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = &endpoint
			o.UsePathStyle = true
		}
	})
}
