//go:build integration

package taxi_test

import (
	"context"
	"testing"

	"github.com/bmunoz/gentax/internal/taxi"
	"github.com/bmunoz/gentax/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaxiRepository_Create(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := taxi.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)

	input := taxi.CreateInput{
		OwnerID: owner.ID,
		Plate:   "XYZ999",
		Model:   "Kia Picanto",
		Year:    2022,
	}

	got, err := repo.Create(ctx, input)

	require.NoError(t, err)
	assert.NotEmpty(t, got.ID)
	assert.Equal(t, owner.ID, got.OwnerID)
	assert.Equal(t, "XYZ999", got.Plate)
	assert.Equal(t, "Kia Picanto", got.Model)
	assert.Equal(t, 2022, got.Year)
	assert.True(t, got.Active)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestTaxiRepository_DuplicatePlate(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := taxi.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)

	input := taxi.CreateInput{
		OwnerID: owner.ID,
		Plate:   "DUP001",
		Model:   "First Model",
		Year:    2020,
	}

	_, err := repo.Create(ctx, input)
	require.NoError(t, err)

	// Insert the same plate for the same owner — must return ErrDuplicatePlate.
	_, err = repo.Create(ctx, input)
	require.ErrorIs(t, err, taxi.ErrDuplicatePlate)
}

func TestTaxiRepository_List_Isolation(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := taxi.NewRepository(pool)
	ctx := context.Background()

	ownerA := testutil.CreateOwner(t, pool)
	ownerB := testutil.CreateOwner(t, pool)

	testutil.CreateTaxi(t, pool, ownerA.ID)
	testutil.CreateTaxi(t, pool, ownerB.ID)

	taxisA, err := repo.List(ctx, ownerA.ID)
	require.NoError(t, err)
	require.Len(t, taxisA, 1)
	assert.Equal(t, ownerA.ID, taxisA[0].OwnerID)

	taxisB, err := repo.List(ctx, ownerB.ID)
	require.NoError(t, err)
	require.Len(t, taxisB, 1)
	assert.Equal(t, ownerB.ID, taxisB[0].OwnerID)
}

func TestTaxiRepository_Deactivate(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := taxi.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	tx := testutil.CreateTaxi(t, pool, owner.ID)

	// Taxi should start active.
	assert.True(t, tx.Active)

	err := repo.SetActive(ctx, tx.ID, owner.ID, false)
	require.NoError(t, err)

	// Verify by reading back.
	updated, err := repo.GetByID(ctx, tx.ID, owner.ID)
	require.NoError(t, err)
	assert.False(t, updated.Active)
}

func TestTaxiRepository_WrongOwner(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := taxi.NewRepository(pool)
	ctx := context.Background()

	owner := testutil.CreateOwner(t, pool)
	otherOwner := testutil.CreateOwner(t, pool)

	tx := testutil.CreateTaxi(t, pool, owner.ID)

	// Requesting with the wrong owner must return ErrNotFound.
	_, err := repo.GetByID(ctx, tx.ID, otherOwner.ID)
	require.ErrorIs(t, err, taxi.ErrNotFound)
}
