package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/inbound/httpapi"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/inbound/sqs"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/outbound/outbox"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/outbound/postgres"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/outbound/s3"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/outbound/sns"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/config"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/metrics"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/observability"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/system"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/web"
)

const serviceName = "video-processor-api"

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

	store, err := s3.Connect(ctx, s3.Config{Region: cfg.AWSRegion, Endpoint: cfg.AWSEndpoint, PublicEndpoint: cfg.S3PublicEndpoint, Bucket: cfg.S3Bucket})
	if err != nil {
		return err
	}
	publisher, err := sns.Connect(ctx, sns.Config{Region: cfg.AWSRegion, Endpoint: cfg.AWSEndpoint, TopicARN: cfg.SNSTopicARN})
	if err != nil {
		return err
	}

	m := metrics.New()
	svc := video.NewService(postgres.NewVideoRepository(pool), store, system.Clock{}, system.UUIDGenerator{}, video.Config{
		MaxUploadBytes: cfg.MaxUploadBytes, UploadTTL: cfg.UploadTTL, DownloadTTL: cfg.DownloadTTL,
	}, video.WithMetrics(m.Video()))
	consumer, err := sqs.Connect(ctx, sqs.Config{Region: cfg.AWSRegion, Endpoint: cfg.AWSEndpoint, QueueURL: cfg.SQSQueueURL}, svc, log, sqs.WithMetrics(m.Events()))
	if err != nil {
		return err
	}

	go outbox.NewRelay(postgres.NewOutbox(pool), publisher, log, cfg.OutboxInterval, cfg.OutboxBatch).Run(ctx)
	go consumer.Run(ctx)

	metricsMux := http.NewServeMux()
	metricsMux.Handle("GET /metrics", m.Handler())
	metricsServer := &http.Server{Addr: ":" + cfg.MetricsPort, Handler: metricsMux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("metrics server stopped", "err", err)
		}
	}()
	defer func() { _ = metricsServer.Shutdown(context.Background()) }()

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           otelhttp.NewHandler(m.HTTP(httpapi.NewRouter(svc, web.Assets)), serviceName),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Info("listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
