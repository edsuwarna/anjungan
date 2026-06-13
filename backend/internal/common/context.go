package common

import "context"

// ─── Context keys for user claims ─────────────────────────────────────────────

type userCtxKey string

const userKey userCtxKey = "current_user"

// UserClaims holds basic user identity extracted from JWT for cross-package use.
type UserClaims struct {
	UserID string
	Email  string
	Role   string
}

// SetUserClaims stores user claims in context for downstream handlers.
// Call this from a middleware after JWT validation.
func SetUserClaims(ctx context.Context, claims *UserClaims) context.Context {
	return context.WithValue(ctx, userKey, claims)
}

// GetUserClaims reads user claims from context.
// Returns nil if no claims are set.
func GetUserClaims(ctx context.Context) *UserClaims {
	c, _ := ctx.Value(userKey).(*UserClaims)
	return c
}
