package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/app"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/config"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/observability"
)

func main() {
	log := observability.NewLogger("video-processor-worker")
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		log.Info("running migrations")
		return app.Migrate(ctx, cfg)
	}
	return app.Run(ctx, cfg, log)
}
