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
	"github.com/rs/zerolog/log"

	"github.com/edsuwarna/anjungan/internal/admin"
	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/authactivity"
	"github.com/edsuwarna/anjungan/internal/bookmark"
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
	log.Logger = log.With().Caller().Logger()

	database, err := db.Connect(cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("database connection: %w", err)
	}
	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("database ping: %w", err)
	}

	// ─── Auto-run pending migrations ─────────────────────────────────────
	log.Info().Str("dir", cfg.MigrationsPath).Msg("running database migrations")
	if n, err := db.RunMigrations(context.Background(), database.Pool, cfg.MigrationsPath); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	} else if n > 0 {
		log.Info().Int("applied", n).Msg("database migrations applied")
	} else {
		log.Info().Msg("no pending migrations")
	}

	rdb := db.NewRedis(cfg.Redis)
	repo := db.NewRepository(database)

	// ─── Build handlers ────────────────────────────────────────────────────
	rl := ratelimit.New(rdb, cfg.Security.RateLimitMaxAttempts, cfg.Security.RateLimitWindow, cfg.Security.RateLimitLockout)
	authSvc := auth.NewService(repo, cfg.JWT, rdb, rl, cfg.Security)
	authH := auth.NewHandler(authSvc, repo, repo)

	// Auth Activity — login monitoring
	authActivityH := authactivity.NewHandler(repo)

	srv := &Server{cfg: cfg, db: database}
	srv.setupRouter(authH, authSvc, repo, rl, authActivityH)

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
