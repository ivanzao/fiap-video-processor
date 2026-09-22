package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/app"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/platform/config"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/platform/observability"
)

func main() {
	log := observability.NewLogger("video-processor-auth")
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
	return app.RunLocalServer(ctx, cfg, log)
}
