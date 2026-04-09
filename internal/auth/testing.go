package auth

import "context"

// ContextWithClaims returns a context with the given claims injected.
// This is intended for use in handler unit tests that need to simulate
// an authenticated request without going through the full JWT middleware.
func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}
