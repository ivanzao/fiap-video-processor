package main

import (
	"context"
	"os"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/app"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/platform/observability"
)

func main() {
	log := observability.NewLogger("video-processor-auth-migrate")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Error("fatal", "err", "DATABASE_URL is required")
		os.Exit(1)
	}
	if err := app.Migrate(context.Background(), dsn); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
	log.Info("migrations applied")
}
