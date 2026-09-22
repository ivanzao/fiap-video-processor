package main

import (
	"os"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/app"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/platform/config"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/platform/observability"
)

func main() {
	log := observability.NewLogger("video-processor-auth-authorizer")
	cfg, err := config.LoadVerifier()
	if err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
	lambda.Start(app.AuthorizerLambda(cfg, log))
}
