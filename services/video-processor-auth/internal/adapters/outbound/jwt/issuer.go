package jwt

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/domain/user"
)

const (
	Issuer   = "video-processor-auth"
	Audience = "video-processor-api"
)

var ErrInvalidToken = errors.New("jwt: invalid token")

type claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type HS256 struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewHS256(secret string, ttl time.Duration, now func() time.Time) *HS256 {
	return &HS256{secret: []byte(secret), ttl: ttl, now: now}
}

func (h *HS256) Issue(u user.User) (string, error) {
	now := h.now()
	claims := claims{
		Email: u.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Audience:  jwt.ClaimStrings{Audience},
			Subject:   u.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(h.ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(h.secret)
	if err != nil {
		return "", fmt.Errorf("jwt: sign: %w", err)
	}
	return signed, nil
}

func (h *HS256) Verify(_ context.Context, token string) (user.Identity, error) {
	var claims claims
	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return h.secret, nil
	}, jwt.WithIssuer(Issuer), jwt.WithAudience(Audience), jwt.WithTimeFunc(h.now), jwt.WithExpirationRequired())
	if err != nil {
		return user.Identity{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if !parsed.Valid || claims.Subject == "" {
		return user.Identity{}, ErrInvalidToken
	}
	return user.Identity{UserID: claims.Subject, Email: claims.Email}, nil
}
