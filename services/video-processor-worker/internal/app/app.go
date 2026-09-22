package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/inbound/sqs"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/ffmpeg"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/mail"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/postgres"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/s3"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/sns"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/workspace"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/zip"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/config"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/metrics"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/observability"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/system"
)

const serviceName = "video-processor-worker"

func Migrate(ctx context.Context, cfg config.Config) error {
	return postgres.Migrate(ctx, cfg.DatabaseURL)
}

func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	shutdownTracing, err := observability.SetupTracing(ctx, serviceName)
	if err != nil {
		return err
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	store, err := s3.Connect(ctx, s3.Config{Region: cfg.AWSRegion, Endpoint: cfg.AWSEndpoint, Bucket: cfg.S3Bucket})
	if err != nil {
		return err
	}
	publisher, err := sns.Connect(ctx, sns.Config{Region: cfg.AWSRegion, Endpoint: cfg.AWSEndpoint, TopicARN: cfg.SNSTopicARN}, system.UUIDGenerator{}, system.Clock{})
	if err != nil {
		return err
	}

	m := metrics.New()
	svc := video.NewService(video.Dependencies{
		Store:     store,
		Extractor: ffmpeg.NewExtractor(cfg.FFmpegBinary, cfg.FFmpegThreads),
		Packager:  zip.Packager{},
		Workspace: workspace.NewTemp(cfg.WorkDir),
		Repo:      postgres.NewExecutionRepository(pool),
		Publisher: publisher,
		Notifier:  newNotifier(cfg),
		Clock:     system.Clock{},
		IDs:       system.UUIDGenerator{},
		Metrics:   m.Processing(),
	}, video.Config{FrameRate: cfg.FrameRate, MaxAttempts: cfg.MaxAttempts})

	health := &http.Server{Addr: ":" + cfg.MetricsPort, Handler: healthHandler(m.Handler()), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := health.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("health server stopped", "err", err)
		}
	}()
	defer func() { _ = health.Shutdown(context.Background()) }()

	consumer, err := sqs.Connect(ctx, svc, log, sqs.Config{
		Region: cfg.AWSRegion, Endpoint: cfg.AWSEndpoint, QueueURL: cfg.SQSQueueURL, Concurrency: cfg.Concurrency, VisibilityTimeout: cfg.VisibilityTimeout,
		Heartbeat: cfg.Heartbeat, RetryDelay: cfg.RetryDelay, Metrics: m.Queue(),
	})
	if err != nil {
		return err
	}
	log.Info("consuming", "queue", cfg.SQSQueueURL, "concurrency", cfg.Concurrency)
	consumer.Run(ctx)
	return nil
}

func newNotifier(cfg config.Config) video.Notifier {
	if cfg.SMTPAddr != "" {
		return mail.NewSMTPNotifier(cfg.SMTPAddr, cfg.MailFrom)
	}
	return mail.NewMailerSendNotifier(&http.Client{Timeout: 10 * time.Second}, cfg.MailerSendEndpoint, cfg.MailerSendToken, cfg.MailFrom)
}

func healthHandler(metricsHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /metrics", metricsHandler)
	return mux
}
