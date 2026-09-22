package authorizer_test

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/adapters/inbound/authorizer"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/adapters/outbound/jwt"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/domain/user"
)

func TestAuthorizerExposesIdentityForValidBearerToken(t *testing.T) {
	issuer := jwt.NewHS256("secret", time.Hour, time.Now)
	token, err := issuer.Issue(user.User{ID: "usr-1", Email: "ana@example.com"})
	require.NoError(t, err)
	handle := authorizer.New(user.NewAuthorizer(issuer), slog.New(slog.NewTextHandler(io.Discard, nil)))

	res, err := handle(t.Context(), authorizer.Request{Headers: map[string]string{"authorization": "Bearer " + token}})

	require.NoError(t, err)
	assert.True(t, res.IsAuthorized)
	assert.Equal(t, map[string]any{"userId": "usr-1", "email": "ana@example.com"}, res.Context)
}

func TestAuthorizerDeniesMissingOrInvalidTokens(t *testing.T) {
	issuer := jwt.NewHS256("secret", time.Hour, time.Now)
	forged, err := jwt.NewHS256("other", time.Hour, time.Now).Issue(user.User{ID: "usr-1"})
	require.NoError(t, err)
	handle := authorizer.New(user.NewAuthorizer(issuer), slog.New(slog.NewTextHandler(io.Discard, nil)))

	for name, headers := range map[string]map[string]string{
		"no header":     {},
		"basic scheme":  {"Authorization": "Basic abc"},
		"empty bearer":  {"Authorization": "Bearer "},
		"forged token":  {"Authorization": "Bearer " + forged},
		"garbage token": {"authorization": "Bearer nope"},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := handle(t.Context(), authorizer.Request{Headers: headers})
			require.NoError(t, err)
			assert.False(t, res.IsAuthorized)
			assert.Nil(t, res.Context)
		})
	}
}
