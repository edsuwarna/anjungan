package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	"github.com/edsuwarna/anjungan/internal/admin"
	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/authactivity"
	"github.com/edsuwarna/anjungan/internal/bookmark"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/compliance"
	"github.com/edsuwarna/anjungan/internal/config"
	"github.com/edsuwarna/anjungan/internal/container"
	"github.com/edsuwarna/anjungan/internal/dashboard"
	"github.com/edsuwarna/anjungan/internal/infra"
	"github.com/edsuwarna/anjungan/internal/notification"
	"github.com/edsuwarna/anjungan/internal/ratelimit"
	"github.com/edsuwarna/anjungan/internal/registry"
	"github.com/edsuwarna/anjungan/internal/self"
	"github.com/edsuwarna/anjungan/internal/settings"
	"github.com/edsuwarna/anjungan/internal/sslmonitor"
	"github.com/edsuwarna/anjungan/internal/uptime"
)

type Server struct {
	cfg  *config.Config
	db   *db.DB
	mux  *chi.Mux
}

func New(cfg *config.Config) (*Server, error) {
	zerolog.SetGlobalLevel(parseLogLevel(cfg.Log.Level))
	zlog.Logger = zlog.With().Caller().Logger()

	database, err := db.Connect(cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("database connection: %w", err)
	}
	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("database ping: %w", err)
	}

	zlog.Info().Msgf("connected to database: %s", cfg.Postgres.DBName)

	rdb := db.NewRedis(cfg.Redis)
	zlog.Info().Msg("redis connected")

	// ─── Auto-run pending migrations ─────────────────────────────────────
	zlog.Info().Str("dir", cfg.MigrationsPath).Msg("running database migrations")
	if n, err := db.RunMigrations(context.Background(), database.Pool, cfg.MigrationsPath); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	} else if n > 0 {
		zlog.Info().Int("applied", n).Msg("database migrations applied")
	} else {
		zlog.Info().Msg("no pending migrations")
	}

	repo := db.NewRepository(database)

	// ─── Build handlers ────────────────────────────────────────────────────
	rl := ratelimit.New(rdb, cfg.Security.RateLimitMaxAttempts, cfg.Security.RateLimitWindow, cfg.Security.RateLimitLockout)
	authSvc := auth.NewService(repo, cfg.JWT, rdb, rl, cfg.Security)
	authH := auth.NewHandler(authSvc, repo, repo)

	// Auth Activity — login monitoring
	authActivityH := authactivity.NewHandler(repo)
	authActivityH.SetRedis(rdb)

	srv := &Server{cfg: cfg, db: database}
	srv.setupRouter(authH, authSvc, repo, rl, authActivityH)

	// ─── Brute force detection scheduler ─────────────────────────────────
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		ctx := context.Background()
		for range ticker.C {
			alerts, err := repo.DetectBruteForce(ctx)
			if err != nil {
				zlog.Error().Err(err).Msg("brute force detection error")
				continue
			}
			if len(alerts) > 0 {
				zlog.Warn().Int("alerts", len(alerts)).Msg("brute force alerts detected")
				for _, a := range alerts {
				zlog.Warn().
					Str("ip", a.IPAddress).
					Int("failures", a.Failures).
					Int("users", a.UserCount).
					Int("window_min", a.WindowMinutes).
					Msg("brute force alert")
				// Store alert in audit log for persistence
				meta, _ := json.Marshal(map[string]interface{}{
						"ip_address":     a.IPAddress,
						"failures":       a.Failures,
						"user_count":     a.UserCount,
						"window_minutes": a.WindowMinutes,
						"first_attempt":  a.FirstAttempt,
						"last_attempt":   a.LastAttempt,
					})
					_ = meta // reserved — future: write to security_events table
				}
			}
		}
	}()
	zlog.Info().Msg("brute force detection scheduler started (every 60s)")

	// ─── Self-server auto-registration ────────────────────────────────────
	if cfg.SelfServer.Enabled {
		detector := self.NewDetector(repo, &cfg.SelfServer)
		go detector.DetectAndRegister(context.Background())
	}

	return srv, nil
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) setupRouter(authH *auth.Handler, authSvc *auth.Service, repo *db.Repository, rl *ratelimit.RateLimiter, authActivityH *authactivity.Handler) {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Public SSE endpoint for uptime (no auth middleware — EventSource can't set headers)
	// Auth via token query param is handled inside the handler
	uptimeH := uptime.NewHandler(repo)
	uptimeH.SetJWTSecret(s.cfg.JWT.Secret)
	uptimeH.InitSSE()
	r.Get("/api/uptime/events", uptimeH.SSEEvents)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/auth", authRoutes(authH))

		// Public: registration status check (login page needs this without auth)
		settingsH := settings.NewHandler(repo)
		r.Get("/settings/registration", settingsH.GetRegistration)

		r.Route("/", func(r chi.Router) {
			r.Use(authSvc.Middleware)
			r.Mount("/servers", infra.NewHandler(repo, s.cfg.SelfServer.DockerSocketPath).Routes())
			r.Mount("/ssh-keys", infra.NewSSHKeyHandler(repo).Routes())
			r.Mount("/containers", container.NewHandler(repo, s.cfg.SelfServer.DockerSocketPath).Routes())
		regHandler := registry.NewHandler(s.cfg.Registry, repo)
		regHandler.Start(context.Background())
		r.Mount("/registry", regHandler.Routes())
			r.Mount("/compliance", compliance.NewHandler(repo, s.cfg.SelfServer.DockerSocketPath).Routes())
			sslMonH := sslmonitor.NewHandler(repo)
			r.Mount("/ssl-monitors", sslMonH.Routes())

			// Start SSL monitor scheduler
			sslSched := sslmonitor.NewScheduler(repo, sslMonH)
			sslSched.Start(context.Background())

			// Uptime Monitoring
			r.Mount("/uptime-monitors", uptimeH.Routes())

			// Notification Targets
			notifH := notification.NewHandler(repo)
			r.Mount("/notification-targets", notifH.Routes())

			// Bookmarks
			bookmarkH := bookmark.NewHandler(repo)
			r.Mount("/bookmarks", bookmarkH.Routes())

			// Start uptime scheduler
			uptimeSched := uptime.NewScheduler(repo, uptimeH)
			uptimeSched.Start(context.Background())

			r.Mount("/admin", admin.NewHandler(repo, rl).Routes())

		// Auth Activity — login monitoring (admin only)
		r.Group(func(r chi.Router) {
		r.Use(auth.RequireAdmin)
		r.Use(bridgeClaims) // copy auth claims to common context
		r.Mount("/auth-activity", authActivityH.Routes())
	})

			r.Mount("/settings", settingsH.Routes())
			r.Get("/dashboard", dashboard.NewHandler(repo).Summary)
		})
	})

	s.mux = r
}

func authRoutes(h *auth.Handler) chi.Router {
	r := chi.NewRouter()
	r.Post("/login", h.Login)
	r.Post("/register", h.Register)
	r.Post("/refresh", h.Refresh)
	r.Post("/verify-2fa", h.Verify2FA)
	r.Post("/verify-totp", h.Verify2FA)
	r.Post("/setup-totp", h.SetupTOTP)
	r.Post("/verify-totp-setup", h.VerifyTOTPSetup)
	r.Post("/disable-totp", h.DisableTOTP)
	r.Get("/me", h.Me)
	r.Post("/logout", h.Logout)
	r.Put("/password", h.ChangePassword)
	r.Put("/profile", h.UpdateProfile)
	r.Get("/login-history", h.LoginHistory)
	return r
}

// bridgeClaims copies auth package's JWT claims to the common context
// so downstream handlers (e.g. authactivity) can read user identity
// without importing the auth package (avoids import cycles).
func bridgeClaims(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetClaims(r.Context())
		if claims != nil {
			ctx := common.SetUserClaims(r.Context(), &common.UserClaims{
				UserID: claims.UserID,
				Email:  claims.Email,
				Role:   claims.Role,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func parseLogLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
