//go:build integration

package testutil_test

import (
	"context"
	"testing"

	"github.com/bmunoz/gentax/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewTestDB_StartsAndMigrates verifies that NewTestDB spins up a container,
// runs all 7 migrations, and returns a working pool.
func TestNewTestDB_StartsAndMigrates(t *testing.T) {
	pool := testutil.NewTestDB(t)
	require.NotNil(t, pool)

	ctx := context.Background()

	// Verify pool is usable.
	err := pool.Ping(ctx)
	require.NoError(t, err, "pool should be reachable")

	// Verify all expected tables exist.
	tables := []string{
		"owners",
		"taxis",
		"drivers",
		"driver_taxi_assignments",
		"expense_categories",
		"receipts",
		"expenses",
	}

	for _, table := range tables {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)`,
			table,
		).Scan(&exists)
		require.NoError(t, err, "checking table %s", table)
		assert.True(t, exists, "table %s should exist after migrations", table)
	}

	// Verify golang-migrate schema_migrations table exists and tracks version 7.
	var version int
	err = pool.QueryRow(ctx,
		`SELECT version FROM schema_migrations`,
	).Scan(&version)
	require.NoError(t, err, "schema_migrations should be populated")
	assert.Equal(t, 7, version, "migration version should be 7 after all migrations applied")
}
