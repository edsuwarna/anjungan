package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"

	"github.com/edsuwarna/anjungan/internal/common/model"
	"github.com/edsuwarna/anjungan/internal/config"
	ratelimit "github.com/edsuwarna/anjungan/internal/ratelimit"
)

// ─── Repository interface (implemented by common/db) ──────────────────────

type UserRepository interface {
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id string) (*model.User, error)
	CreateUser(ctx context.Context, user *model.User) error
	UpdateUser(ctx context.Context, user *model.User) error
	UpdateUserPassword(ctx context.Context, id, passwordHash string) error
	UpdateUserTOTPSecret(ctx context.Context, id, secret string) error
	UpdateUserTOTPEnabled(ctx context.Context, id string, enabled bool) error
	GetSetting(ctx context.Context, key string) (*model.Settings, error)
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

func (s *Service) ChangePassword(ctx context.Context, email, currentPassword, newPassword string) error {
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}

	if len(newPassword) < s.securityCfg.MinPasswordLength {
		return ErrPasswordTooShort
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.users.UpdateUserPassword(ctx, user.ID, string(hash))
}

func (s *Service) UpdateProfile(ctx context.Context, email, newName, newEmail string) (*model.User, error) {
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if newName != "" && newName != user.Name {
		user.Name = strings.TrimSpace(newName)
	}
	if newEmail != "" && newEmail != user.Email {
		newEmail = strings.TrimSpace(strings.ToLower(newEmail))
		existing, _ := s.users.GetUserByEmail(ctx, newEmail)
		if existing != nil && existing.ID != user.ID {
			return nil, errors.New("email already in use")
		}
		user.Email = newEmail
	}
	if err := s.users.UpdateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) IsRegistrationEnabled(ctx context.Context) bool {
	setting, err := s.users.GetSetting(ctx, "registration.enabled")
	if err != nil {
		return false
	}
	return setting.Value == "true"
}

// ─── TOTP 2FA ──────────────────────────────────────────────────────────────

// TOTPSetupResponse contains the provisioning info for setting up TOTP.
type TOTPSetupResponse struct {
	Secret          string `json:"secret"`
	ProvisioningURI string `json:"provisioning_uri"`
	QRCodeBase64    string `json:"qr_code_base64"`
}

// SetupTOTP generates a new TOTP secret and returns provisioning URI + QR code.
// Does NOT enable 2FA yet — user must verify first.
func (s *Service) SetupTOTP(ctx context.Context, email string) (*TOTPSetupResponse, error) {
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Anjungan",
		AccountName: user.Email,
	})
	if err != nil {
		return nil, err
	}

	// Save secret immediately so it persists across page refresh
	if err := s.users.UpdateUserTOTPSecret(ctx, user.ID, key.Secret()); err != nil {
		return nil, err
	}

	// Generate QR code as base64 PNG
	var buf bytes.Buffer
	qr, err := qrcode.New(key.URL(), qrcode.Medium)
	if err != nil {
		return nil, err
	}
	if err := qr.Write(256, &buf); err != nil {
		return nil, err
	}

	return &TOTPSetupResponse{
		Secret:          key.Secret(),
		ProvisioningURI: key.URL(),
		QRCodeBase64:    base64.StdEncoding.EncodeToString(buf.Bytes()),
	}, nil
}

// VerifyTOTPSetup confirms the user set up 2FA correctly by validating a code.
// On success, enables TOTP for the user.
func (s *Service) VerifyTOTPSetup(ctx context.Context, email, token string) error {
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return ErrInvalidCredentials
	}

	if user.TOTPSecret == "" {
		return errors.New("2FA not initialized — run setup first")
	}

	if !totp.Validate(token, user.TOTPSecret) {
		return errors.New("invalid verification code")
	}

	return s.users.UpdateUserTOTPEnabled(ctx, user.ID, true)
}

// DisableTOTP disables 2FA for the user after password confirmation.
func (s *Service) DisableTOTP(ctx context.Context, email, password string) error {
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}

	if err := s.users.UpdateUserTOTPSecret(ctx, user.ID, ""); err != nil {
		return err
	}
	return s.users.UpdateUserTOTPEnabled(ctx, user.ID, false)
}

// VerifyTOTPCode validates a TOTP code during login (step 2 of 2FA login flow).
// Returns the full TokenResponse (JWT pair + user) on success.
func (s *Service) VerifyTOTPCode(ctx context.Context, email, token string) (*TokenResponse, error) {
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !user.TOTPEnabled {
		return nil, errors.New("2FA is not enabled for this account")
	}

	if !totp.Validate(token, user.TOTPSecret) {
		return nil, errors.New("invalid 2FA code")
	}

	return s.generateTokenPair(user)
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
