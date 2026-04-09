package driver

import (
	"context"
	"errors"
	"time"

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
type pgxRepository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository backed by a pgxpool.Pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgxRepository{pool: pool}
}

// Create inserts a new driver record and returns it.
// Maps PostgreSQL unique violation (23505) on telegram_id to ErrDuplicateTelegram.
func (r *pgxRepository) Create(ctx context.Context, input CreateInput) (*Driver, error) {
	const q = `
		INSERT INTO drivers (owner_id, full_name, phone)
		VALUES ($1, $2, $3)
		RETURNING id, owner_id, telegram_id, full_name, phone, active,
		          link_token, link_token_expires_at, link_token_used, created_at`

	var d Driver
	err := r.pool.QueryRow(ctx, q, input.OwnerID, input.FullName, input.Phone).
		Scan(
			&d.ID, &d.OwnerID, &d.TelegramID, &d.FullName, &d.Phone, &d.Active,
			&d.LinkToken, &d.LinkTokenExpiresAt, &d.LinkTokenUsed, &d.CreatedAt,
		)
	if err != nil {
		return nil, mapPgError(err)
	}
	return &d, nil
}

// GetByID retrieves a driver by its ID and ownerID.
// Returns ErrNotFound if no matching record exists.
func (r *pgxRepository) GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Driver, error) {
	const q = `
		SELECT id, owner_id, telegram_id, full_name, phone, active,
		       link_token, link_token_expires_at, link_token_used, created_at
		FROM drivers
		WHERE id = $1 AND owner_id = $2`

	var d Driver
	err := r.pool.QueryRow(ctx, q, id, ownerID).
		Scan(
			&d.ID, &d.OwnerID, &d.TelegramID, &d.FullName, &d.Phone, &d.Active,
			&d.LinkToken, &d.LinkTokenExpiresAt, &d.LinkTokenUsed, &d.CreatedAt,
		)
	if err != nil {
		return nil, mapPgError(err)
	}
	return &d, nil
}

// GetByTelegramID retrieves a driver by telegram_id (globally unique across all owners).
func (r *pgxRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*Driver, error) {
	const q = `
		SELECT id, owner_id, telegram_id, full_name, phone, active,
		       link_token, link_token_expires_at, link_token_used, created_at
		FROM drivers
		WHERE telegram_id = $1`

	var d Driver
	err := r.pool.QueryRow(ctx, q, telegramID).
		Scan(
			&d.ID, &d.OwnerID, &d.TelegramID, &d.FullName, &d.Phone, &d.Active,
			&d.LinkToken, &d.LinkTokenExpiresAt, &d.LinkTokenUsed, &d.CreatedAt,
		)
	if err != nil {
		return nil, mapPgError(err)
	}
	return &d, nil
}

// GetByLinkToken retrieves a driver by its link_token value.
func (r *pgxRepository) GetByLinkToken(ctx context.Context, token string) (*Driver, error) {
	const q = `
		SELECT id, owner_id, telegram_id, full_name, phone, active,
		       link_token, link_token_expires_at, link_token_used, created_at
		FROM drivers
		WHERE link_token = $1`

	var d Driver
	err := r.pool.QueryRow(ctx, q, token).
		Scan(
			&d.ID, &d.OwnerID, &d.TelegramID, &d.FullName, &d.Phone, &d.Active,
			&d.LinkToken, &d.LinkTokenExpiresAt, &d.LinkTokenUsed, &d.CreatedAt,
		)
	if err != nil {
		return nil, mapPgError(err)
	}
	return &d, nil
}

// List returns all drivers for the given owner, ordered by created_at descending.
func (r *pgxRepository) List(ctx context.Context, ownerID uuid.UUID) ([]*Driver, error) {
	const q = `
		SELECT id, owner_id, telegram_id, full_name, phone, active,
		       link_token, link_token_expires_at, link_token_used, created_at
		FROM drivers
		WHERE owner_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var drivers []*Driver
	for rows.Next() {
		var d Driver
		if err := rows.Scan(
			&d.ID, &d.OwnerID, &d.TelegramID, &d.FullName, &d.Phone, &d.Active,
			&d.LinkToken, &d.LinkTokenExpiresAt, &d.LinkTokenUsed, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		drivers = append(drivers, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return drivers, nil
}

// SetActive updates the active flag for the given driver, scoped to ownerID.
// Returns ErrNotFound if no matching record is updated.
func (r *pgxRepository) SetActive(ctx context.Context, id, ownerID uuid.UUID, active bool) error {
	const q = `UPDATE drivers SET active = $1 WHERE id = $2 AND owner_id = $3`

	tag, err := r.pool.Exec(ctx, q, active, id, ownerID)
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetLinkToken stores a link token and its expiry for the given driver.
// Also resets link_token_used to false so the token can be used once.
func (r *pgxRepository) SetLinkToken(ctx context.Context, driverID uuid.UUID, token string, expiresAt time.Time) error {
	const q = `
		UPDATE drivers
		SET link_token = $1, link_token_expires_at = $2, link_token_used = false
		WHERE id = $3`

	tag, err := r.pool.Exec(ctx, q, token, expiresAt, driverID)
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkLinkTokenUsed sets link_token_used=true for the given driver.
func (r *pgxRepository) MarkLinkTokenUsed(ctx context.Context, driverID uuid.UUID) error {
	const q = `UPDATE drivers SET link_token_used = true WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q, driverID)
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetTelegramID links a telegram_id to the given driver.
// Maps unique violation (23505) to ErrDuplicateTelegram.
func (r *pgxRepository) SetTelegramID(ctx context.Context, driverID uuid.UUID, telegramID int64) error {
	const q = `UPDATE drivers SET telegram_id = $1 WHERE id = $2`

	_, err := r.pool.Exec(ctx, q, telegramID, driverID)
	if err != nil {
		return mapPgError(err)
	}
	return nil
}

// GetActiveAssignment returns the current active (unassigned_at IS NULL) assignment for a driver.
// Returns ErrNotFound if no active assignment exists.
func (r *pgxRepository) GetActiveAssignment(ctx context.Context, driverID uuid.UUID) (*Assignment, error) {
	const q = `
		SELECT id, driver_id, taxi_id, assigned_at, unassigned_at
		FROM driver_taxi_assignments
		WHERE driver_id = $1 AND unassigned_at IS NULL`

	var a Assignment
	err := r.pool.QueryRow(ctx, q, driverID).
		Scan(&a.ID, &a.DriverID, &a.TaxiID, &a.AssignedAt, &a.UnassignedAt)
	if err != nil {
		return nil, mapPgError(err)
	}
	return &a, nil
}

// CreateAssignment inserts a new driver-taxi assignment record.
func (r *pgxRepository) CreateAssignment(ctx context.Context, driverID, taxiID uuid.UUID) (*Assignment, error) {
	const q = `
		INSERT INTO driver_taxi_assignments (driver_id, taxi_id)
		VALUES ($1, $2)
		RETURNING id, driver_id, taxi_id, assigned_at, unassigned_at`

	var a Assignment
	err := r.pool.QueryRow(ctx, q, driverID, taxiID).
		Scan(&a.ID, &a.DriverID, &a.TaxiID, &a.AssignedAt, &a.UnassignedAt)
	if err != nil {
		return nil, mapPgError(err)
	}
	return &a, nil
}

// UnassignDriver sets unassigned_at=now() on the active assignment for the given driver.
// Returns ErrNotFound if no active assignment exists.
func (r *pgxRepository) UnassignDriver(ctx context.Context, driverID uuid.UUID) error {
	const q = `
		UPDATE driver_taxi_assignments
		SET unassigned_at = now()
		WHERE driver_id = $1 AND unassigned_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, driverID)
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
		return ErrDuplicateTelegram
	}
	return err
}
