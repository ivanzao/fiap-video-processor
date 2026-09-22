package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port             string
	MetricsPort      string
	DatabaseURL      string
	AWSEndpoint      string
	S3PublicEndpoint string
	AWSRegion        string
	S3Bucket         string
	SNSTopicARN      string
	SQSQueueURL      string
	MaxUploadBytes   int64
	UploadTTL        time.Duration
	DownloadTTL      time.Duration
	OutboxInterval   time.Duration
	OutboxBatch      int
}

func Load() (Config, error) {
	var errs []error
	cfg := Config{
		Port:             env("PORT", "8080"),
		MetricsPort:      env("METRICS_PORT", "9090"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		AWSEndpoint:      os.Getenv("AWS_ENDPOINT_URL"),
		S3PublicEndpoint: os.Getenv("S3_PUBLIC_ENDPOINT"),
		AWSRegion:        env("AWS_REGION", "us-east-1"),
		S3Bucket:         os.Getenv("S3_BUCKET"),
		SNSTopicARN:      os.Getenv("SNS_TOPIC_ARN"),
		SQSQueueURL:      os.Getenv("SQS_QUEUE_URL"),
		MaxUploadBytes:   envInt64("MAX_UPLOAD_BYTES", 500*1024*1024, &errs),
		UploadTTL:        envDuration("UPLOAD_TTL", 15*time.Minute, &errs),
		DownloadTTL:      envDuration("DOWNLOAD_TTL", time.Hour, &errs),
		OutboxInterval:   envDuration("OUTBOX_INTERVAL", 2*time.Second, &errs),
		OutboxBatch:      int(envInt64("OUTBOX_BATCH", 50, &errs)),
	}
	for name, value := range map[string]string{
		"DATABASE_URL": cfg.DatabaseURL, "S3_BUCKET": cfg.S3Bucket, "SNS_TOPIC_ARN": cfg.SNSTopicARN, "SQS_QUEUE_URL": cfg.SQSQueueURL,
	} {
		if value == "" {
			errs = append(errs, fmt.Errorf("config: %s is required", name))
		}
	}
	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func envInt64(name string, fallback int64, errs *[]error) int64 {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("config: %s: %w", name, err))
	}
	return n
}

func envDuration(name string, fallback time.Duration, errs *[]error) time.Duration {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("config: %s: %w", name, err))
	}
	return d
}
