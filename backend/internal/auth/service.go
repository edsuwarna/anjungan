package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/edsuwarna/anjungan/internal/common/model"
	"github.com/edsuwarna/anjungan/internal/config"
	ratelimit "github.com/edsuwarna/anjungan/internal/ratelimit"
)

// ─── Repository interface (implemented by common/db) ──────────────────────

type UserRepository interface {
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	CreateUser(ctx context.Context, user *model.User) error
}

// ─── Service ───────────────────────────────────────────────────────────────

type Service struct {
	users       UserRepository
	cfg         config.JWTConfig
	rdb         *redis.Client
	rateLimiter *ratelimit.RateLimiter
	securityCfg config.SecurityConfig
}

func NewService(users UserRepository, cfg config.JWTConfig, rdb *redis.Client, rl *ratelimit.RateLimiter, secCfg config.SecurityConfig) *Service {
	return &Service{users: users, cfg: cfg, rdb: rdb, rateLimiter: rl, securityCfg: secCfg}
}

func (s *Service) Login(ctx context.Context, email, password, totpCode, ip string) (*TokenResponse, error) {
	// 1. Check account-level lockout
	locked, _, err := s.rateLimiter.IsLocked(ctx, email)
	if err != nil {
		return nil, err
	}
	if locked {
		return nil, ErrAccountLocked
	}

	// 2. Check IP+email rate limit
	status, err := s.rateLimiter.CheckLogin(ctx, ip, email)
	if err != nil {
		return nil, err
	}
	if !status.Allowed {
		return nil, ErrAccountLocked
	}

	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// Record the failed attempt — may trigger lockout
		s.rateLimiter.RecordFailed(ctx, ip, email)
		// Return vague error to avoid revealing account existence or lock state
		return nil, ErrInvalidCredentials
	}

	// 3. Clear rate limit counters on successful login
	s.rateLimiter.RecordSuccess(ctx, ip, email)

	if user.TOTPEnabled && totpCode == "" {
		return nil, ErrTOTPRequired
	}

	return s.generateTokenPair(user)
}

func (s *Service) Register(ctx context.Context, email, name, password string) (*model.User, error) {
	if len(password) < s.securityCfg.MinPasswordLength {
		return nil, ErrPasswordTooShort
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		ID:           uuid.New().String(),
		Email:        email,
		Name:         name,
		PasswordHash: string(hash),
		Role:         "developer",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.users.CreateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.Secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (s *Service) generateTokenPair(user *model.User) (*TokenResponse, error) {
	now := time.Now()
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTTL)),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessStr, err := accessToken.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return nil, err
	}

	refreshClaims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.RefreshTTL)),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshStr, err := refreshToken.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  accessStr,
		RefreshToken: refreshStr,
		ExpiresIn:    int64(s.cfg.AccessTTL.Seconds()),
		User:         user,
	}, nil
}

// ─── Models ────────────────────────────────────────────────────────────────

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code,omitempty"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type TokenResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int64       `json:"expires_in"`
	User         *model.User `json:"user"`
}

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// ─── Error sentinels ────────────────────────────────────────────────────────

var (
	ErrTOTPRequired       = errors.New("totp code required")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account locked due to too many failed attempts")
	ErrPasswordTooShort   = errors.New("password does not meet minimum length requirement")
)
