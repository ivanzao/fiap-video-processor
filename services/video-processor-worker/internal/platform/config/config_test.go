package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/config"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("S3_BUCKET", "videos")
	t.Setenv("SNS_TOPIC_ARN", "arn:aws:sns:us-east-1:000000000000:video-events")
	t.Setenv("SQS_QUEUE_URL", "http://localhost:4566/000000000000/worker-queue")
}

func clearOptional(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"AWS_ENDPOINT_URL", "AWS_REGION", "WORKER_CONCURRENCY", "SQS_VISIBILITY_TIMEOUT", "SQS_HEARTBEAT",
		"SQS_RETRY_DELAY", "FRAME_RATE", "MAX_ATTEMPTS", "FFMPEG_BINARY", "FFMPEG_THREADS", "WORK_DIR",
		"MAIL_FROM", "SMTP_ADDR", "MAILERSEND_ENDPOINT", "MAILERSEND_TOKEN", "METRICS_PORT",
	} {
		t.Setenv(name, "")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	clearOptional(t)
	setRequired(t)
	t.Setenv("SMTP_ADDR", "mailpit:1025")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.Config{
		DatabaseURL: "postgres://x", AWSRegion: "us-east-1", S3Bucket: "videos",
		SNSTopicARN: "arn:aws:sns:us-east-1:000000000000:video-events",
		SQSQueueURL: "http://localhost:4566/000000000000/worker-queue",
		Concurrency: 2, VisibilityTimeout: 5 * time.Minute, Heartbeat: time.Minute, RetryDelay: 30 * time.Second,
		FrameRate: 1, MaxAttempts: 3, FFmpegBinary: "ffmpeg", FFmpegThreads: 1, WorkDir: os.TempDir(),
		MailFrom: "noreply@fiapx.example", SMTPAddr: "mailpit:1025",
		MailerSendEndpoint: "https://api.mailersend.com", MetricsPort: "9090",
	}, cfg)
}

func TestLoadAcceptsMailerSendInsteadOfSMTP(t *testing.T) {
	clearOptional(t)
	setRequired(t)
	t.Setenv("MAILERSEND_TOKEN", "token")
	t.Setenv("MAILERSEND_ENDPOINT", "http://mailersend.test")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Empty(t, cfg.SMTPAddr)
	assert.Equal(t, "token", cfg.MailerSendToken)
	assert.Equal(t, "http://mailersend.test", cfg.MailerSendEndpoint)
}

func TestLoadReadsOverrides(t *testing.T) {
	setRequired(t)
	t.Setenv("SMTP_ADDR", "mailpit:1025")
	t.Setenv("AWS_ENDPOINT_URL", "http://localstack:4566")
	t.Setenv("AWS_REGION", "sa-east-1")
	t.Setenv("WORKER_CONCURRENCY", "4")
	t.Setenv("SQS_VISIBILITY_TIMEOUT", "10m")
	t.Setenv("SQS_HEARTBEAT", "2m")
	t.Setenv("SQS_RETRY_DELAY", "5s")
	t.Setenv("FRAME_RATE", "2")
	t.Setenv("MAX_ATTEMPTS", "5")
	t.Setenv("FFMPEG_BINARY", "/usr/bin/ffmpeg")
	t.Setenv("FFMPEG_THREADS", "3")
	t.Setenv("WORK_DIR", "/work")
	t.Setenv("MAIL_FROM", "videos@example.com")
	t.Setenv("METRICS_PORT", "9100")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "http://localstack:4566", cfg.AWSEndpoint)
	assert.Equal(t, "sa-east-1", cfg.AWSRegion)
	assert.Equal(t, 4, cfg.Concurrency)
	assert.Equal(t, 10*time.Minute, cfg.VisibilityTimeout)
	assert.Equal(t, 2*time.Minute, cfg.Heartbeat)
	assert.Equal(t, 5*time.Second, cfg.RetryDelay)
	assert.Equal(t, 2, cfg.FrameRate)
	assert.Equal(t, 5, cfg.MaxAttempts)
	assert.Equal(t, "/usr/bin/ffmpeg", cfg.FFmpegBinary)
	assert.Equal(t, 3, cfg.FFmpegThreads)
	assert.Equal(t, "/work", cfg.WorkDir)
	assert.Equal(t, "videos@example.com", cfg.MailFrom)
	assert.Equal(t, "9100", cfg.MetricsPort)
}

func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	clearOptional(t)
	for _, name := range []string{"DATABASE_URL", "S3_BUCKET", "SNS_TOPIC_ARN", "SQS_QUEUE_URL"} {
		t.Setenv(name, "")
	}
	t.Setenv("WORKER_CONCURRENCY", "many")
	t.Setenv("SQS_VISIBILITY_TIMEOUT", "long")
	t.Setenv("SQS_HEARTBEAT", "often")
	t.Setenv("SQS_RETRY_DELAY", "later")
	t.Setenv("FRAME_RATE", "fast")
	t.Setenv("MAX_ATTEMPTS", "few")
	t.Setenv("FFMPEG_THREADS", "some")

	cfg, err := config.Load()

	require.Error(t, err)
	assert.Equal(t, config.Config{}, cfg)
	for _, want := range []string{
		"DATABASE_URL", "S3_BUCKET", "SNS_TOPIC_ARN", "SQS_QUEUE_URL", "SMTP_ADDR or MAILERSEND_TOKEN",
		"WORKER_CONCURRENCY", "SQS_VISIBILITY_TIMEOUT", "SQS_HEARTBEAT", "SQS_RETRY_DELAY",
		"FRAME_RATE", "MAX_ATTEMPTS", "FFMPEG_THREADS",
	} {
		assert.Contains(t, err.Error(), want)
	}
}
