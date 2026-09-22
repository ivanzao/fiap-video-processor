package authorizer

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/domain/user"
)

type Request struct {
	Headers map[string]string `json:"headers"`
}

type Response struct {
	IsAuthorized bool           `json:"isAuthorized"`
	Context      map[string]any `json:"context,omitempty"`
}

type Authorizer interface {
	Authorize(ctx context.Context, token string) (user.Identity, error)
}

type Handler func(ctx context.Context, req Request) (Response, error)

func New(auth Authorizer, log *slog.Logger) Handler {
	return func(ctx context.Context, req Request) (Response, error) {
		token, ok := bearerToken(req.Headers)
		if !ok {
			return Response{IsAuthorized: false}, nil
		}
		identity, err := auth.Authorize(ctx, token)
		if err != nil {
			log.Info("token rejected", "err", err.Error())
			return Response{IsAuthorized: false}, nil
		}
		return Response{IsAuthorized: true, Context: map[string]any{"userId": identity.UserID, "email": identity.Email}}, nil
	}
}

func bearerToken(headers map[string]string) (string, bool) {
	for name, value := range headers {
		if !strings.EqualFold(name, "authorization") {
			continue
		}
		scheme, token, found := strings.Cut(value, " ")
		if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
			return "", false
		}
		return strings.TrimSpace(token), true
	}
	return "", false
}
