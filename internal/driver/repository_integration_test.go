//go:build integration

package driver_test

import (
	"context"
	"testing"
	"time"

	"github.com/bmunoz/gentax/internal/driver"
	"github.com/bmunoz/gentax/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDriverRepository_Create(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := driver.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)

	input := driver.CreateInput{
		OwnerID:  owner.ID,
		FullName: "María García",
		Phone:    "3109876543",
	}

	drv, err := repo.Create(ctx, input)

	require.NoError(t, err)
	assert.NotEmpty(t, drv.ID)
	assert.Equal(t, owner.ID, drv.OwnerID)
	assert.Equal(t, "María García", drv.FullName)
	assert.Equal(t, "3109876543", drv.Phone)
	assert.True(t, drv.Active)
	assert.Nil(t, drv.TelegramID)
	assert.Nil(t, drv.LinkToken)
	assert.False(t, drv.LinkTokenUsed)
}

func TestDriverRepository_LinkToken_FullFlow(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := driver.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	d := testutil.CreateDriver(t, pool, owner.ID)

	token := "test-link-token-abc123"
	expiresAt := time.Now().Add(24 * time.Hour)

	// Set link token.
	err := repo.SetLinkToken(ctx, d.ID, token, expiresAt)
	require.NoError(t, err)

	// Retrieve by token.
	found, err := repo.GetByLinkToken(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, d.ID, found.ID)
	require.NotNil(t, found.LinkToken)
	assert.Equal(t, token, *found.LinkToken)
	assert.False(t, found.LinkTokenUsed)

	// Link telegram_id.
	telegramID := int64(777888999)
	err = repo.SetTelegramID(ctx, d.ID, telegramID)
	require.NoError(t, err)

	// Mark token used.
	err = repo.MarkLinkTokenUsed(ctx, d.ID)
	require.NoError(t, err)

	// Verify final state via GetByTelegramID.
	linked, err := repo.GetByTelegramID(ctx, telegramID)
	require.NoError(t, err)
	assert.Equal(t, d.ID, linked.ID)
	require.NotNil(t, linked.TelegramID)
	assert.Equal(t, telegramID, *linked.TelegramID)
	assert.True(t, linked.LinkTokenUsed)
}

func TestDriverRepository_DuplicateTelegram(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := driver.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	d1 := testutil.CreateDriver(t, pool, owner.ID)
	d2 := testutil.CreateDriver(t, pool, owner.ID)

	telegramID := int64(111222333)

	// Link telegram_id to first driver.
	err := repo.SetTelegramID(ctx, d1.ID, telegramID)
	require.NoError(t, err)

	// Attempting to link the same telegram_id to another driver must fail.
	err = repo.SetTelegramID(ctx, d2.ID, telegramID)
	require.ErrorIs(t, err, driver.ErrDuplicateTelegram)
}

// TestDriverRepository_Assignment_Isolation verifies that two owners' drivers
// cannot see each other's assignments.
func TestDriverRepository_Assignment_Isolation(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := driver.NewRepository(pool)
	ctx := context.Background()

	ownerA := testutil.CreateOwner(t, pool)
	ownerB := testutil.CreateOwner(t, pool)

	driverA := testutil.CreateDriver(t, pool, ownerA.ID)
	driverB := testutil.CreateDriver(t, pool, ownerB.ID)
	taxiA := testutil.CreateTaxi(t, pool, ownerA.ID)
	taxiB := testutil.CreateTaxi(t, pool, ownerB.ID)

	// Assign each driver to their own taxi.
	_, err := repo.CreateAssignment(ctx, driverA.ID, taxiA.ID)
	require.NoError(t, err)

	_, err = repo.CreateAssignment(ctx, driverB.ID, taxiB.ID)
	require.NoError(t, err)

	// Owner A's driver has an active assignment with their taxi.
	assignA, err := repo.GetActiveAssignment(ctx, driverA.ID)
	require.NoError(t, err)
	assert.Equal(t, taxiA.ID, assignA.TaxiID)

	// Owner B's driver has a separate assignment — no cross-contamination.
	assignB, err := repo.GetActiveAssignment(ctx, driverB.ID)
	require.NoError(t, err)
	assert.Equal(t, taxiB.ID, assignB.TaxiID)

	// Each assignment must reference the correct driver.
	assert.Equal(t, driverA.ID, assignA.DriverID)
	assert.Equal(t, driverB.ID, assignB.DriverID)
	assert.NotEqual(t, assignA.ID, assignB.ID)
}

// TestDriverRepository_ListWithAssignment_WithActiveAssignment verifies that a driver
// with an active taxi assignment has assigned_taxi populated with the correct plate.
func TestDriverRepository_ListWithAssignment_WithActiveAssignment(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := driver.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	d := testutil.CreateDriver(t, pool, owner.ID)
	taxi := testutil.CreateTaxi(t, pool, owner.ID)

	_, err := repo.CreateAssignment(ctx, d.ID, taxi.ID)
	require.NoError(t, err)

	results, err := repo.ListWithAssignment(ctx, owner.ID)

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].AssignedTaxi, "expected AssignedTaxi to be non-nil")
	assert.Equal(t, taxi.ID, results[0].AssignedTaxi.ID)
	assert.Equal(t, taxi.Plate, results[0].AssignedTaxi.Plate)
	assert.Equal(t, d.ID, results[0].ID)
}

// TestDriverRepository_ListWithAssignment_NoAssignment verifies that a driver
// without an active assignment has assigned_taxi as nil.
func TestDriverRepository_ListWithAssignment_NoAssignment(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := driver.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	d := testutil.CreateDriver(t, pool, owner.ID)

	results, err := repo.ListWithAssignment(ctx, owner.ID)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Nil(t, results[0].AssignedTaxi, "expected AssignedTaxi to be nil for unassigned driver")
	assert.Equal(t, d.ID, results[0].ID)
}

// TestDriverRepository_ListWithAssignment_UnassignedAfterClose verifies that after
// a taxi is unassigned, assigned_taxi returns nil (uses unassigned_at IS NULL filter).
func TestDriverRepository_ListWithAssignment_UnassignedAfterClose(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := driver.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	d := testutil.CreateDriver(t, pool, owner.ID)
	taxi := testutil.CreateTaxi(t, pool, owner.ID)

	_, err := repo.CreateAssignment(ctx, d.ID, taxi.ID)
	require.NoError(t, err)

	// Unassign the taxi.
	err = repo.UnassignDriver(ctx, d.ID)
	require.NoError(t, err)

	results, err := repo.ListWithAssignment(ctx, owner.ID)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Nil(t, results[0].AssignedTaxi, "expected AssignedTaxi nil after unassignment")
}

// TestDriverRepository_Deactivate_UnassignsCurrent verifies that UnassignDriver sets
// unassigned_at on the active assignment and GetActiveAssignment returns ErrNotFound after.
func TestDriverRepository_Deactivate_UnassignsCurrent(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := driver.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	d := testutil.CreateDriver(t, pool, owner.ID)
	taxi := testutil.CreateTaxi(t, pool, owner.ID)

	// Create assignment.
	_, err := repo.CreateAssignment(ctx, d.ID, taxi.ID)
	require.NoError(t, err)

	// Verify active assignment exists.
	a, err := repo.GetActiveAssignment(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, taxi.ID, a.TaxiID)

	// Unassign.
	err = repo.UnassignDriver(ctx, d.ID)
	require.NoError(t, err)

	// Now there is no active assignment.
	_, err = repo.GetActiveAssignment(ctx, d.ID)
	require.ErrorIs(t, err, driver.ErrNotFound)

	// Set driver inactive.
	err = repo.SetActive(ctx, d.ID, owner.ID, false)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, d.ID, owner.ID)
	require.NoError(t, err)
	assert.False(t, got.Active)
}
