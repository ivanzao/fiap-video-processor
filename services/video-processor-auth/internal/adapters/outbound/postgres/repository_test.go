//go:build integration

package postgres_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/adapters/outbound/postgres"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/domain/user"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/testinfra"
)

func TestRepositoryRoundTripsUsersAndEnforcesUniqueEmail(t *testing.T) {
	repo := postgres.NewUserRepository(testinfra.StartPostgres(t))
	ctx := t.Context()
	ana := user.User{ID: uuid.NewString(), Email: "ana@example.com", PasswordHash: "hash", CreatedAt: time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)}

	require.NoError(t, repo.Create(ctx, ana))
	got, err := repo.FindByEmail(ctx, "ana@example.com")
	require.NoError(t, err)
	assert.Equal(t, ana, got)

	err = repo.Create(ctx, user.User{ID: uuid.NewString(), Email: "ana@example.com", PasswordHash: "other", CreatedAt: ana.CreatedAt})
	assert.ErrorIs(t, err, user.ErrEmailTaken)

	_, err = repo.FindByEmail(ctx, "nobody@example.com")
	assert.ErrorIs(t, err, user.ErrNotFound)

	assert.Error(t, repo.Create(ctx, user.User{ID: "not-a-uuid", Email: "x@example.com"}))
}
