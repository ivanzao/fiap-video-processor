package main

import (
	"context"
	"os"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/app"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/platform/config"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/platform/observability"
)

func main() {
	log := observability.NewLogger("video-processor-auth-login")
	cfg, err := config.Load()
	if err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
	handler, err := app.LoginLambda(context.Background(), cfg)
	if err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
	lambda.Start(handler)
}
