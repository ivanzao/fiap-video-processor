package httpapi

import (
	"context"
	"errors"
	"net/http"
)

const (
	HeaderUserID    = "X-User-Id"
	HeaderUserEmail = "X-User-Email"
)

var ErrUnauthenticated = errors.New("auth: request carries no authenticated identity")

type Identity struct {
	UserID string
	Email  string
}

type contextKey struct{}

func IdentityFromRequest(r *http.Request) (Identity, error) {
	id := Identity{UserID: r.Header.Get(HeaderUserID), Email: r.Header.Get(HeaderUserEmail)}
	if id.UserID == "" {
		return Identity{}, ErrUnauthenticated
	}
	return id, nil
}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(contextKey{}).(Identity)
	return id, ok
}

func RequireIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := IdentityFromRequest(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "request carries no authenticated identity")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
	})
}
