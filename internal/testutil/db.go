// Package testutil provides shared test helpers: database fixtures, testcontainers setup, and domain assertions.
package testutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const migrationsDir = "../../migrations"

// NewTestDB spins up a real PostgreSQL 16 container, runs all migrations,
// and returns a *pgxpool.Pool. A t.Cleanup handler tears down the container.
func NewTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	// Run migrations via golang-migrate.
	if err := runMigrations(connStr); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Build pgxpool for tests.
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to create pgxpool: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

// runMigrations applies all golang-migrate migrations in the migrations directory.
func runMigrations(connStr string) error {
	// golang-migrate pgx/v5 driver registers under the "pgx5" scheme.
	// Replace the scheme from postgres:// or postgresql:// to pgx5://.
	migrateURL := connStr
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if len(connStr) >= len(prefix) && connStr[:len(prefix)] == prefix {
			migrateURL = "pgx5://" + connStr[len(prefix):]
			break
		}
	}

	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsDir),
		migrateURL,
	)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}

	return nil
}
