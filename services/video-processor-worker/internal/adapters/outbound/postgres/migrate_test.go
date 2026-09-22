//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/postgres"
)

func TestMigrateIsIdempotentAndCreatesSchema(t *testing.T) {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("video_processor_worker"), tcpostgres.WithUsername("app"), tcpostgres.WithPassword("app"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	require.NoError(t, postgres.Migrate(ctx, dsn))
	require.NoError(t, postgres.Migrate(ctx, dsn))

	pool, err := postgres.NewPool(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	var exists bool
	require.NoError(t, pool.QueryRow(ctx, "SELECT to_regclass('\"processing_execution\"') IS NOT NULL").Scan(&exists))
	require.True(t, exists, "table processing_execution must exist after migrate")
}
