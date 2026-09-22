package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/platform/config"
)

func TestLoadAppliesDefaultsAndRequiresSecrets(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "s")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.Config{Port: "8081", DatabaseURL: "postgres://x", JWTSecret: "s", TokenTTL: time.Hour, BcryptCost: 10}, cfg)
}

func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("TOKEN_TTL", "soon")
	t.Setenv("BCRYPT_COST", "many")

	_, err := config.Load()

	require.Error(t, err)
	for _, want := range []string{"DATABASE_URL", "JWT_SECRET", "TOKEN_TTL", "BCRYPT_COST"} {
		assert.Contains(t, err.Error(), want)
	}
}

func TestLoadVerifierOnlyNeedsTheSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	_, err := config.LoadVerifier()
	assert.Error(t, err)

	t.Setenv("JWT_SECRET", "s")
	cfg, err := config.LoadVerifier()
	require.NoError(t, err)
	assert.Equal(t, "s", cfg.JWTSecret)
}
