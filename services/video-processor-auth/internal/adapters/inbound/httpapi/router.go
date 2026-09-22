package httpapi

import (
	"context"
	"net/http"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/domain/user"
)

type UserService interface {
	SignUp(ctx context.Context, creds user.Credentials) (user.User, error)
	Login(ctx context.Context, creds user.Credentials) (user.Session, error)
}

func NewRouter(svc UserService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("POST /auth/v1/signup", SignUpHandler(svc))
	mux.Handle("POST /auth/v1/login", LoginHandler(svc))
	return mux
}
