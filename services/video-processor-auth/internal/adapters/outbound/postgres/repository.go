package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/domain/user"
)

type UserRepository struct {
	pool *Pool
}

func NewUserRepository(pool *Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

type userRow struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

func (r userRow) toDomain() user.User {
	return user.User{ID: r.ID.String(), Email: r.Email, PasswordHash: r.PasswordHash, CreatedAt: r.CreatedAt.UTC()}
}

func (r *UserRepository) Create(ctx context.Context, u user.User) error {
	id, err := uuid.Parse(u.ID)
	if err != nil {
		return fmt.Errorf("postgres: user id: %w", err)
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO "user" (id, email, password_hash, created_at) VALUES ($1, $2, $3, $4)`,
		id, u.Email, u.PasswordHash, u.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		return user.ErrEmailTaken
	}
	if err != nil {
		return fmt.Errorf("postgres: insert user: %w", err)
	}
	return nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (user.User, error) {
	var row userRow
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at FROM "user" WHERE email = $1`, email,
	).Scan(&row.ID, &row.Email, &row.PasswordHash, &row.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return user.User{}, user.ErrNotFound
	}
	if err != nil {
		return user.User{}, fmt.Errorf("postgres: find user: %w", err)
	}
	return row.toDomain(), nil
}
