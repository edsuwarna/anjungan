package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/edsuwarna/anjungan/internal/common"
)

type ctxKey string

const claimsKey ctxKey = "claims"

// ClaimsKey is the context key for storing JWT claims.
// Exported so other packages can inject claims into context (e.g. SSE endpoints).
const ClaimsKey ctxKey = "claims"

func GetClaims(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := ""

		// Check Authorization header first
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenStr == authHeader {
				common.Error(w, http.StatusUnauthorized, "invalid authorization format")
				return
			}
		}

		// Fall back to query parameter (for WebSocket connections)
		if tokenStr == "" {
			tokenStr = r.URL.Query().Get("token")
		}

		if tokenStr == "" {
			common.Error(w, http.StatusUnauthorized, "missing authorization")
			return
		}

		claims, err := s.ValidateAccessToken(tokenStr)
		if err != nil {
			common.Error(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin is middleware that enforces admin role.
// Use on routes that should only be accessible to admin users.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaims(r.Context())
		if claims == nil || claims.Role != "admin" {
			common.Error(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
