package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/adapters/inbound/authorizer"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/adapters/inbound/httpapi"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/adapters/inbound/lambdahttp"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/adapters/outbound/jwt"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/adapters/outbound/password"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/adapters/outbound/postgres"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/domain/user"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/platform/config"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/platform/system"
)

func Migrate(ctx context.Context, databaseURL string) error {
	return postgres.Migrate(ctx, databaseURL)
}

func SignUpLambda(ctx context.Context, cfg config.Config) (lambdahttp.Handler, error) {
	svc, err := connectUserService(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return lambdahttp.Wrap(httpapi.SignUpHandler(svc)), nil
}

func LoginLambda(ctx context.Context, cfg config.Config) (lambdahttp.Handler, error) {
	svc, err := connectUserService(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return lambdahttp.Wrap(httpapi.LoginHandler(svc)), nil
}

func AuthorizerLambda(cfg config.Config, log *slog.Logger) authorizer.Handler {
	return authorizer.New(user.NewAuthorizer(jwt.NewHS256(cfg.JWTSecret, 0, time.Now)), log)
}

func RunLocalServer(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	server := &http.Server{Addr: ":" + cfg.Port, Handler: httpapi.NewRouter(buildUserService(pool, cfg)), ReadHeaderTimeout: 10 * time.Second}
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

func connectUserService(ctx context.Context, cfg config.Config) (*user.Service, error) {
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return buildUserService(pool, cfg), nil
}

func buildUserService(pool *postgres.Pool, cfg config.Config) *user.Service {
	return user.NewService(
		postgres.NewUserRepository(pool),
		password.Bcrypt{Cost: cfg.BcryptCost},
		jwt.NewHS256(cfg.JWTSecret, cfg.TokenTTL, time.Now),
		system.Clock{},
		system.UUIDGenerator{},
	)
}
