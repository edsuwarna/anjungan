package server

import (
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
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/config"
	"github.com/edsuwarna/anjungan/internal/container"
	"github.com/edsuwarna/anjungan/internal/dashboard"
	"github.com/edsuwarna/anjungan/internal/deployment"
	"github.com/edsuwarna/anjungan/internal/infra"
	"github.com/edsuwarna/anjungan/internal/registry"
	repoapi "github.com/edsuwarna/anjungan/internal/repository"
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

	rdb := db.NewRedis(cfg.Redis)
	repo := db.NewRepository(database)

	// ─── Build handlers ────────────────────────────────────────────────────
	authSvc := auth.NewService(repo, cfg.JWT, rdb)
	authH := auth.NewHandler(authSvc)

	srv := &Server{cfg: cfg, db: database}
	srv.setupRouter(authH, authSvc, repo)
	return srv, nil
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) setupRouter(authH *auth.Handler, authSvc *auth.Service, repo *db.Repository) {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/auth", authRoutes(authH))
		r.Route("/", func(r chi.Router) {
			r.Use(authSvc.Middleware)
			r.Mount("/servers", infra.NewHandler(repo).Routes())
			r.Mount("/containers", container.NewHandler().Routes())
			r.Mount("/registry", registry.NewHandler().Routes())
			r.Mount("/repositories", repoapi.NewHandler().Routes())
			r.Mount("/deployments", deployment.NewHandler().Routes())
			r.Mount("/admin", admin.NewHandler(repo).Routes())
			r.Get("/dashboard", dashboard.NewHandler().Summary)
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
	r.Get("/me", h.Me)
	r.Post("/logout", h.Logout)
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
