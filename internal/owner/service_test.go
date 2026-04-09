package owner_test

import (
	"context"
	"testing"
	"time"

	"github.com/bmunoz/gentax/internal/owner"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func makeOwnerWithHash(t *testing.T, email, password string) *owner.Owner {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return &owner.Owner{
		ID:           uuid.New(),
		Name:         "Test Owner",
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
	}
}

// anyString matches any non-empty string argument (used for password hashes).
var anyString = mock.MatchedBy(func(s string) bool { return len(s) > 0 })

// --- Authenticate ---

func TestOwnerService_Authenticate_CorrectPassword(t *testing.T) {
	repo := new(owner.MockRepository)
	svc := owner.NewService(repo)

	email := "owner@example.com"
	password := "secret123"
	stored := makeOwnerWithHash(t, email, password)

	repo.On("GetByEmail", context.Background(), email).Return(stored, nil)

	got, err := svc.Authenticate(context.Background(), email, password)

	require.NoError(t, err)
	assert.Equal(t, stored.ID, got.ID)
	repo.AssertExpectations(t)
}

func TestOwnerService_Authenticate_WrongPassword(t *testing.T) {
	repo := new(owner.MockRepository)
	svc := owner.NewService(repo)

	email := "owner@example.com"
	stored := makeOwnerWithHash(t, email, "correctpassword")

	repo.On("GetByEmail", context.Background(), email).Return(stored, nil)

	_, err := svc.Authenticate(context.Background(), email, "wrongpassword")

	require.ErrorIs(t, err, owner.ErrInvalidCredentials)
	repo.AssertExpectations(t)
}

func TestOwnerService_Authenticate_NotFound(t *testing.T) {
	repo := new(owner.MockRepository)
	svc := owner.NewService(repo)

	email := "noone@example.com"

	repo.On("GetByEmail", context.Background(), email).Return(nil, owner.ErrNotFound)

	_, err := svc.Authenticate(context.Background(), email, "anypassword")

	require.ErrorIs(t, err, owner.ErrInvalidCredentials)
	repo.AssertExpectations(t)
}

// --- Create ---

func TestOwnerService_Create_Success(t *testing.T) {
	repo := new(owner.MockRepository)
	svc := owner.NewService(repo)

	name := "Fleet Owner"
	email := "fleet@example.com"
	password := "strongpassword"

	repo.On("Create", context.Background(), name, email, anyString).Return(
		&owner.Owner{
			ID:        uuid.New(),
			Name:      name,
			Email:     email,
			CreatedAt: time.Now(),
		}, nil,
	)

	got, err := svc.Create(context.Background(), name, email, password)

	require.NoError(t, err)
	assert.Equal(t, email, got.Email)
	repo.AssertExpectations(t)
}

func TestOwnerService_Create_DuplicateEmail(t *testing.T) {
	repo := new(owner.MockRepository)
	svc := owner.NewService(repo)

	repo.On("Create", context.Background(), "Owner", "dup@example.com", anyString).
		Return(nil, owner.ErrDuplicateEmail)

	_, err := svc.Create(context.Background(), "Owner", "dup@example.com", "password123")

	require.ErrorIs(t, err, owner.ErrDuplicateEmail)
	repo.AssertExpectations(t)
}
