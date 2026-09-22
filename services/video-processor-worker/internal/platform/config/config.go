package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL        string
	AWSEndpoint        string
	AWSRegion          string
	S3Bucket           string
	SNSTopicARN        string
	SQSQueueURL        string
	Concurrency        int
	VisibilityTimeout  time.Duration
	Heartbeat          time.Duration
	RetryDelay         time.Duration
	FrameRate          int
	MaxAttempts        int
	FFmpegBinary       string
	FFmpegThreads      int
	WorkDir            string
	MailFrom           string
	SMTPAddr           string
	MailerSendEndpoint string
	MailerSendToken    string
	MetricsPort        string
}

func Load() (Config, error) {
	var errs []error
	cfg := Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		AWSEndpoint:        os.Getenv("AWS_ENDPOINT_URL"),
		AWSRegion:          env("AWS_REGION", "us-east-1"),
		S3Bucket:           os.Getenv("S3_BUCKET"),
		SNSTopicARN:        os.Getenv("SNS_TOPIC_ARN"),
		SQSQueueURL:        os.Getenv("SQS_QUEUE_URL"),
		Concurrency:        envInt("WORKER_CONCURRENCY", 2, &errs),
		VisibilityTimeout:  envDuration("SQS_VISIBILITY_TIMEOUT", 5*time.Minute, &errs),
		Heartbeat:          envDuration("SQS_HEARTBEAT", time.Minute, &errs),
		RetryDelay:         envDuration("SQS_RETRY_DELAY", 30*time.Second, &errs),
		FrameRate:          envInt("FRAME_RATE", 1, &errs),
		MaxAttempts:        envInt("MAX_ATTEMPTS", 3, &errs),
		FFmpegBinary:       env("FFMPEG_BINARY", "ffmpeg"),
		FFmpegThreads:      envInt("FFMPEG_THREADS", 1, &errs),
		WorkDir:            env("WORK_DIR", os.TempDir()),
		MailFrom:           env("MAIL_FROM", "noreply@fiapx.example"),
		SMTPAddr:           os.Getenv("SMTP_ADDR"),
		MailerSendEndpoint: env("MAILERSEND_ENDPOINT", "https://api.mailersend.com"),
		MailerSendToken:    os.Getenv("MAILERSEND_TOKEN"),
		MetricsPort:        env("METRICS_PORT", "9090"),
	}
	for name, value := range map[string]string{
		"DATABASE_URL": cfg.DatabaseURL, "S3_BUCKET": cfg.S3Bucket, "SNS_TOPIC_ARN": cfg.SNSTopicARN, "SQS_QUEUE_URL": cfg.SQSQueueURL,
	} {
		if value == "" {
			errs = append(errs, fmt.Errorf("config: %s is required", name))
		}
	}
	if cfg.SMTPAddr == "" && cfg.MailerSendToken == "" {
		errs = append(errs, errors.New("config: either SMTP_ADDR or MAILERSEND_TOKEN is required"))
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

func envInt(name string, fallback int, errs *[]error) int {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
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
