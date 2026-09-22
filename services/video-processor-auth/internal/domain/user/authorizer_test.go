package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/domain/user"
)

type fakeVerifier struct {
	identities map[string]user.Identity
}

func (f fakeVerifier) Verify(_ context.Context, token string) (user.Identity, error) {
	identity, ok := f.identities[token]
	if !ok {
		return user.Identity{}, errors.New("signature mismatch")
	}
	return identity, nil
}

func TestAuthorizeReturnsTheIdentityBehindAValidToken(t *testing.T) {
	auth := user.NewAuthorizer(fakeVerifier{identities: map[string]user.Identity{"good": {UserID: "usr-1", Email: "ana@example.com"}}})

	identity, err := auth.Authorize(t.Context(), " good ")

	require.NoError(t, err)
	assert.Equal(t, user.Identity{UserID: "usr-1", Email: "ana@example.com"}, identity)
}

func TestAuthorizeRejectsEmptyUnknownAndAnonymousTokens(t *testing.T) {
	auth := user.NewAuthorizer(fakeVerifier{identities: map[string]user.Identity{"anonymous": {Email: "x@example.com"}}})

	for name, token := range map[string]string{"empty": "  ", "unknown": "forged", "anonymous": "anonymous"} {
		t.Run(name, func(t *testing.T) {
			_, err := auth.Authorize(t.Context(), token)
			assert.ErrorIs(t, err, user.ErrInvalidToken)
		})
	}
}
