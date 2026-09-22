package postgres

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/db"
)

var dsnScheme = regexp.MustCompile(`^postgres(ql)?://`)

func Migrate(_ context.Context, dsn string) error {
	src, err := iofs.New(db.Migrations, "migrations")
	if err != nil {
		return fmt.Errorf("postgres: load migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsnScheme.ReplaceAllString(dsn, "pgx5://"))
	if err != nil {
		return fmt.Errorf("postgres: open migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("postgres: migrate up: %w", err)
	}
	return nil
}
