package project

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

type contextKey string

const (
	projectIDKey   contextKey = "project_id"
	projectSlugKey contextKey = "project_slug"
	projectObjKey  contextKey = "project_obj"
)

func GetProjectID(ctx context.Context) string {
	v, _ := ctx.Value(projectIDKey).(string)
	return v
}

func GetProjectSlug(ctx context.Context) string {
	v, _ := ctx.Value(projectSlugKey).(string)
	return v
}

func GetProject(ctx context.Context) *model.Project {
	v, _ := ctx.Value(projectObjKey).(*model.Project)
	return v
}

// ProjectContextMiddleware extracts project slug from URL param,
// looks up the project, and injects it into request context.
func ProjectContextMiddleware(repo *db.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slug := chi.URLParam(r, "slug")
			if slug == "" {
				http.Error(w, `{"error":"missing project slug"}`, http.StatusBadRequest)
				return
			}

			p, err := repo.GetProjectBySlug(r.Context(), slug)
			if err != nil || p == nil {
				http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
				return
			}

			ctx := context.WithValue(r.Context(), projectIDKey, p.ID)
			ctx = context.WithValue(ctx, projectSlugKey, slug)
			ctx = context.WithValue(ctx, projectObjKey, p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
