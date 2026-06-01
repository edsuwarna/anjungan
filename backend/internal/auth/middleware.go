package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/edsuwarna/anjungan/internal/common"
)

type ctxKey string

const claimsKey ctxKey = "claims"

func GetClaims(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			common.Error(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == authHeader {
			common.Error(w, http.StatusUnauthorized, "invalid authorization format")
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
