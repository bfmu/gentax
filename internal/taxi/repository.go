package taxi

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// pgUniqueViolation is the PostgreSQL error code for unique constraint violations.
	pgUniqueViolation = "23505"
)

// pgxRepository implements Repository using a *pgxpool.Pool.
// All queries include a WHERE owner_id = $n clause to enforce multi-tenant isolation.
type pgxRepository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository backed by a pgxpool.Pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgxRepository{pool: pool}
}

// Create inserts a new taxi record and returns it.
// Maps PostgreSQL unique violation (23505) to ErrDuplicatePlate.
func (r *pgxRepository) Create(ctx context.Context, input CreateInput) (*Taxi, error) {
	const q = `
		INSERT INTO taxis (owner_id, plate, model, year)
		VALUES ($1, $2, $3, $4)
		RETURNING id, owner_id, plate, model, year, active, created_at`

	var tx Taxi
	err := r.pool.QueryRow(ctx, q, input.OwnerID, input.Plate, input.Model, input.Year).
		Scan(&tx.ID, &tx.OwnerID, &tx.Plate, &tx.Model, &tx.Year, &tx.Active, &tx.CreatedAt)
	if err != nil {
		return nil, mapPgError(err)
	}
	return &tx, nil
}

// GetByID retrieves a taxi by its ID and ownerID.
// Returns ErrNotFound if no matching record exists.
func (r *pgxRepository) GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Taxi, error) {
	const q = `
		SELECT id, owner_id, plate, model, year, active, created_at
		FROM taxis
		WHERE id = $1 AND owner_id = $2`

	var tx Taxi
	err := r.pool.QueryRow(ctx, q, id, ownerID).
		Scan(&tx.ID, &tx.OwnerID, &tx.Plate, &tx.Model, &tx.Year, &tx.Active, &tx.CreatedAt)
	if err != nil {
		return nil, mapPgError(err)
	}
	return &tx, nil
}

// List returns all taxis for the given owner, ordered by created_at descending.
func (r *pgxRepository) List(ctx context.Context, ownerID uuid.UUID) ([]*Taxi, error) {
	const q = `
		SELECT id, owner_id, plate, model, year, active, created_at
		FROM taxis
		WHERE owner_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var taxis []*Taxi
	for rows.Next() {
		var tx Taxi
		if err := rows.Scan(&tx.ID, &tx.OwnerID, &tx.Plate, &tx.Model, &tx.Year, &tx.Active, &tx.CreatedAt); err != nil {
			return nil, err
		}
		taxis = append(taxis, &tx)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return taxis, nil
}

// SetActive updates the active flag for the given taxi, scoped to ownerID.
// Returns ErrNotFound if no matching record is updated.
func (r *pgxRepository) SetActive(ctx context.Context, id, ownerID uuid.UUID, active bool) error {
	const q = `UPDATE taxis SET active = $1 WHERE id = $2 AND owner_id = $3`

	tag, err := r.pool.Exec(ctx, q, active, id, ownerID)
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// mapPgError translates pgx-level errors to domain sentinel errors.
func mapPgError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return ErrDuplicatePlate
	}
	return err
}
