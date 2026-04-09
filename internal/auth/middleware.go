package auth

import (
	"context"
	"net/http"
	"strings"
)

// RequireAuth returns a Chi-compatible middleware that:
//  1. Extracts the JWT from the Authorization: Bearer <token> header.
//  2. Validates it using the provided TokenValidator.
//  3. On success, injects *Claims into the request context and calls next.
//  4. Returns 401 if the header is missing, malformed, or the token is invalid/expired.
//  5. Returns 403 if allowedRoles are specified and the token's role is not in the list.
func RequireAuth(validator TokenValidator, allowedRoles ...Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract bearer token.
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing Authorization header", http.StatusUnauthorized)
				return
			}

			const prefix = "Bearer "
			if !strings.HasPrefix(authHeader, prefix) {
				http.Error(w, "invalid Authorization header format", http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, prefix)
			if tokenStr == "" {
				http.Error(w, "missing token", http.StatusUnauthorized)
				return
			}

			// Validate JWT.
			claims, err := validator.Validate(tokenStr)
			if err != nil {
				http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			// Role check — only enforced when allowedRoles is non-empty.
			if len(allowedRoles) > 0 {
				allowed := false
				for _, role := range allowedRoles {
					if claims.Role == role {
						allowed = true
						break
					}
				}
				if !allowed {
					http.Error(w, "forbidden: insufficient role", http.StatusForbidden)
					return
				}
			}

			// Inject claims into context and call next handler.
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext extracts *Claims from the request context.
// Returns nil if claims are not present (e.g., unauthenticated route).
func ClaimsFromContext(ctx context.Context) *Claims {
	v := ctx.Value(claimsKey)
	if v == nil {
		return nil
	}
	claims, _ := v.(*Claims)
	return claims
}
