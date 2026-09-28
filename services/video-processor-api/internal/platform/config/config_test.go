package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/config"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("S3_BUCKET", "videos")
	t.Setenv("SNS_TOPIC_ARN", "arn:aws:sns:us-east-1:000000000000:video-events")
	t.Setenv("SQS_QUEUE_URL", "http://localhost:4566/000000000000/api-queue")
}

func clearOptional(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"PORT", "METRICS_PORT", "AWS_ENDPOINT_URL", "S3_PUBLIC_ENDPOINT", "AWS_REGION",
		"MAX_UPLOAD_BYTES", "UPLOAD_TTL", "DOWNLOAD_TTL", "OUTBOX_INTERVAL", "OUTBOX_BATCH",
	} {
		t.Setenv(name, "")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	clearOptional(t)
	setRequired(t)

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.Config{
		Port: "8080", MetricsPort: "9090", DatabaseURL: "postgres://x", AWSRegion: "us-east-1",
		S3Bucket: "videos", SNSTopicARN: "arn:aws:sns:us-east-1:000000000000:video-events",
		SQSQueueURL:    "http://localhost:4566/000000000000/api-queue",
		MaxUploadBytes: 500 * 1024 * 1024, UploadTTL: 15 * time.Minute, DownloadTTL: time.Hour,
		OutboxInterval: 2 * time.Second, OutboxBatch: 50,
	}, cfg)
}

func TestLoadReadsOverrides(t *testing.T) {
	setRequired(t)
	t.Setenv("PORT", "8000")
	t.Setenv("METRICS_PORT", "9000")
	t.Setenv("AWS_ENDPOINT_URL", "http://localstack:4566")
	t.Setenv("S3_PUBLIC_ENDPOINT", "http://localhost:4566")
	t.Setenv("AWS_REGION", "sa-east-1")
	t.Setenv("MAX_UPLOAD_BYTES", "1024")
	t.Setenv("UPLOAD_TTL", "1m")
	t.Setenv("DOWNLOAD_TTL", "2m")
	t.Setenv("OUTBOX_INTERVAL", "500ms")
	t.Setenv("OUTBOX_BATCH", "10")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "8000", cfg.Port)
	assert.Equal(t, "9000", cfg.MetricsPort)
	assert.Equal(t, "http://localstack:4566", cfg.AWSEndpoint)
	assert.Equal(t, "http://localhost:4566", cfg.S3PublicEndpoint)
	assert.Equal(t, "sa-east-1", cfg.AWSRegion)
	assert.Equal(t, int64(1024), cfg.MaxUploadBytes)
	assert.Equal(t, time.Minute, cfg.UploadTTL)
	assert.Equal(t, 2*time.Minute, cfg.DownloadTTL)
	assert.Equal(t, 500*time.Millisecond, cfg.OutboxInterval)
	assert.Equal(t, 10, cfg.OutboxBatch)
}

func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	clearOptional(t)
	for _, name := range []string{"DATABASE_URL", "S3_BUCKET", "SNS_TOPIC_ARN", "SQS_QUEUE_URL"} {
		t.Setenv(name, "")
	}
	t.Setenv("MAX_UPLOAD_BYTES", "big")
	t.Setenv("UPLOAD_TTL", "soon")
	t.Setenv("DOWNLOAD_TTL", "later")
	t.Setenv("OUTBOX_INTERVAL", "often")
	t.Setenv("OUTBOX_BATCH", "some")

	cfg, err := config.Load()

	require.Error(t, err)
	assert.Equal(t, config.Config{}, cfg)
	for _, want := range []string{
		"DATABASE_URL", "S3_BUCKET", "SNS_TOPIC_ARN", "SQS_QUEUE_URL",
		"MAX_UPLOAD_BYTES", "UPLOAD_TTL", "DOWNLOAD_TTL", "OUTBOX_INTERVAL", "OUTBOX_BATCH",
	} {
		assert.Contains(t, err.Error(), want)
	}
}
