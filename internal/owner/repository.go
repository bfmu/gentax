package owner

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pgUniqueViolation = "23505"

// pgxRepository implements Repository using a *pgxpool.Pool.
type pgxRepository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository backed by a pgxpool.Pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgxRepository{pool: pool}
}

// Create inserts a new owner and returns it.
// Maps PostgreSQL unique violation (23505) to ErrDuplicateEmail.
func (r *pgxRepository) Create(ctx context.Context, name, email, passwordHash string) (*Owner, error) {
	const q = `
		INSERT INTO owners (name, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, name, email, password_hash, created_at`

	var o Owner
	err := r.pool.QueryRow(ctx, q, name, email, passwordHash).
		Scan(&o.ID, &o.Name, &o.Email, &o.PasswordHash, &o.CreatedAt)
	if err != nil {
		return nil, mapOwnerPgError(err)
	}
	return &o, nil
}

// GetByEmail retrieves an owner by email.
// Returns ErrNotFound if no matching record exists.
func (r *pgxRepository) GetByEmail(ctx context.Context, email string) (*Owner, error) {
	const q = `
		SELECT id, name, email, password_hash, created_at
		FROM owners
		WHERE email = $1`

	var o Owner
	err := r.pool.QueryRow(ctx, q, email).
		Scan(&o.ID, &o.Name, &o.Email, &o.PasswordHash, &o.CreatedAt)
	if err != nil {
		return nil, mapOwnerPgError(err)
	}
	return &o, nil
}

// Count returns the total number of owners.
func (r *pgxRepository) Count(ctx context.Context) (int, error) {
	const q = `SELECT COUNT(*) FROM owners`
	var n int
	if err := r.pool.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func mapOwnerPgError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return ErrDuplicateEmail
	}
	return err
}
