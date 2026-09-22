package jwt_test

import (
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/adapters/outbound/jwt"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/domain/user"
)

var now = time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

func at(t time.Time) func() time.Time { return func() time.Time { return t } }

var ana = user.User{ID: "usr-1", Email: "ana@example.com"}

func TestIssueAndVerifyRoundTrip(t *testing.T) {
	issuer := jwt.NewHS256("secret", time.Hour, at(now))

	token, err := issuer.Issue(ana)
	require.NoError(t, err)
	identity, err := issuer.Verify(t.Context(), token)

	require.NoError(t, err)
	assert.Equal(t, user.Identity{UserID: "usr-1", Email: "ana@example.com"}, identity)
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	token, err := jwt.NewHS256("secret", time.Hour, at(now)).Issue(ana)
	require.NoError(t, err)

	_, err = jwt.NewHS256("secret", time.Hour, at(now.Add(2*time.Hour))).Verify(t.Context(), token)

	assert.ErrorIs(t, err, jwt.ErrInvalidToken)
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	token, err := jwt.NewHS256("secret", time.Hour, at(now)).Issue(ana)
	require.NoError(t, err)

	_, err = jwt.NewHS256("other", time.Hour, at(now)).Verify(t.Context(), token)

	assert.ErrorIs(t, err, jwt.ErrInvalidToken)
}

func TestVerifyRejectsOtherAlgorithmsAndAudiences(t *testing.T) {
	issuer := jwt.NewHS256("secret", time.Hour, at(now))
	base := gojwt.RegisteredClaims{Issuer: jwt.Issuer, Audience: gojwt.ClaimStrings{jwt.Audience}, Subject: "usr-1", ExpiresAt: gojwt.NewNumericDate(now.Add(time.Hour))}

	hs512, err := gojwt.NewWithClaims(gojwt.SigningMethodHS512, base).SignedString([]byte("secret"))
	require.NoError(t, err)
	foreign := base
	foreign.Audience = gojwt.ClaimStrings{"someone-else"}
	otherAudience, err := gojwt.NewWithClaims(gojwt.SigningMethodHS256, foreign).SignedString([]byte("secret"))
	require.NoError(t, err)
	noSubject := base
	noSubject.Subject = ""
	anonymous, err := gojwt.NewWithClaims(gojwt.SigningMethodHS256, noSubject).SignedString([]byte("secret"))
	require.NoError(t, err)

	for name, token := range map[string]string{"hs512": hs512, "other audience": otherAudience, "no subject": anonymous, "garbage": "abc"} {
		t.Run(name, func(t *testing.T) {
			_, err := issuer.Verify(t.Context(), token)
			assert.ErrorIs(t, err, jwt.ErrInvalidToken)
		})
	}
}
