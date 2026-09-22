package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidToken = errors.New("user: invalid token")

type Identity struct {
	UserID string
	Email  string
}

type TokenVerifier interface {
	Verify(ctx context.Context, token string) (Identity, error)
}

type Authorizer struct {
	tokens TokenVerifier
}

func NewAuthorizer(tokens TokenVerifier) *Authorizer {
	return &Authorizer{tokens: tokens}
}

func (a *Authorizer) Authorize(ctx context.Context, token string) (Identity, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Identity{}, ErrInvalidToken
	}
	identity, err := a.tokens.Verify(ctx, token)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if identity.UserID == "" {
		return Identity{}, ErrInvalidToken
	}
	return identity, nil
}
