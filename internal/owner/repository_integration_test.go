//go:build integration

package owner_test

import (
	"context"
	"testing"

	"github.com/bmunoz/gentax/internal/owner"
	"github.com/bmunoz/gentax/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOwnerRepository_Create verifies that Create inserts a new owner and returns it
// with all fields populated.
func TestOwnerRepository_Create(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := owner.NewRepository(pool)
	ctx := context.Background()

	name := "Alice Taxi"
	email := "alice@taxi.com"
	hash := "$2a$10$fakehashvalue"

	got, err := repo.Create(ctx, name, email, hash)

	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.NotEmpty(t, got.ID)
	assert.Equal(t, name, got.Name)
	assert.Equal(t, email, got.Email)
	assert.Equal(t, hash, got.PasswordHash)
	assert.False(t, got.CreatedAt.IsZero())
}

// TestOwnerRepository_Create_DuplicateEmail verifies that creating an owner with a
// duplicate email returns ErrDuplicateEmail.
func TestOwnerRepository_Create_DuplicateEmail(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := owner.NewRepository(pool)
	ctx := context.Background()

	_, err := repo.Create(ctx, "First Owner", "dup@taxi.com", "hash1")
	require.NoError(t, err)

	_, err = repo.Create(ctx, "Second Owner", "dup@taxi.com", "hash2")
	require.ErrorIs(t, err, owner.ErrDuplicateEmail)
}

// TestOwnerRepository_GetByEmail_Found verifies that GetByEmail returns the owner
// when a matching record exists.
func TestOwnerRepository_GetByEmail_Found(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := owner.NewRepository(pool)
	ctx := context.Background()

	name := "Bob Taxi"
	email := "bob@taxi.com"
	hash := "$2a$10$anotherfakehash"

	created, err := repo.Create(ctx, name, email, hash)
	require.NoError(t, err)

	got, err := repo.GetByEmail(ctx, email)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, name, got.Name)
	assert.Equal(t, email, got.Email)
	assert.Equal(t, hash, got.PasswordHash)
}

// TestOwnerRepository_GetByEmail_NotFound verifies that GetByEmail returns ErrNotFound
// when no owner matches the given email.
func TestOwnerRepository_GetByEmail_NotFound(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := owner.NewRepository(pool)
	ctx := context.Background()

	_, err := repo.GetByEmail(ctx, "nonexistent@taxi.com")

	require.ErrorIs(t, err, owner.ErrNotFound)
}
