package bookmark

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/edsuwarna/anjungan/internal/audit"
	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

type Handler struct {
	repo *db.Repository
}

func NewHandler(repo *db.Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Patch("/reorder", h.Reorder)
	return r
}

// ─── List ────────────────────────────────────────────────────────────────────

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	bookmarks, err := h.repo.ListBookmarks(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("bookmark list failed")
		common.Error(w, http.StatusInternalServerError, "failed to list bookmarks")
		return
	}

	common.JSON(w, http.StatusOK, bookmarks)
}

// ─── Create ──────────────────────────────────────────────────────────────────

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req model.BookmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := req.Validate(); msg != "" {
		common.Error(w, http.StatusBadRequest, msg)
		return
	}

	b := &model.Bookmark{
		UserID:    claims.UserID,
		Title:     req.Title,
		URL:       req.URL,
		IconType:  req.IconType,
		IconValue: req.IconValue,
		Category:  req.Category,
		Description: req.Description,
		SortOrder: 0,
	}
	if req.SortOrder != nil {
		b.SortOrder = *req.SortOrder
	}
	if req.Pinned != nil {
		b.Pinned = *req.Pinned
	}

	if err := h.repo.CreateBookmark(r.Context(), b); err != nil {
		log.Error().Err(err).Msg("bookmark create failed")
		common.Error(w, http.StatusInternalServerError, "failed to create bookmark")
		return
	}

	meta, _ := json.Marshal(map[string]interface{}{
		"title":    b.Title,
		"url":      b.URL,
		"category": b.Category,
		"pinned":   b.Pinned,
	})
	audit.Log(h.repo, claims.UserID, claims.Email, audit.RemoteIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For")),
		"bookmark.create", "bookmark", b.ID, claims.Email+" created bookmark \""+b.Title+"\"", json.RawMessage(meta))

	common.JSON(w, http.StatusCreated, b)
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")

	existing, err := h.repo.GetBookmark(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "bookmark not found")
		return
	}

	var req model.BookmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Partial update — only validate title/url when they are provided
	if req.Title != "" || req.URL != "" {
		if msg := req.Validate(); msg != "" {
			common.Error(w, http.StatusBadRequest, msg)
			return
		}
	}

	if req.Title != "" {
		existing.Title = req.Title
	}
	if req.URL != "" {
		existing.URL = req.URL
	}
	if req.IconType != "" {
		existing.IconType = req.IconType
	}
	if req.IconValue != "" {
		existing.IconValue = req.IconValue
	}
	if req.Category != "" {
		existing.Category = req.Category
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.Pinned != nil {
		existing.Pinned = *req.Pinned
	}
	if req.SortOrder != nil {
		existing.SortOrder = *req.SortOrder
	}

	if err := h.repo.UpdateBookmark(r.Context(), existing); err != nil {
		log.Error().Err(err).Msg("bookmark update failed")
		common.Error(w, http.StatusInternalServerError, "failed to update bookmark")
		return
	}

	meta, _ := json.Marshal(map[string]interface{}{
		"title":    existing.Title,
		"url":      existing.URL,
		"category": existing.Category,
		"pinned":   existing.Pinned,
	})
	audit.Log(h.repo, claims.UserID, claims.Email, audit.RemoteIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For")),
		"bookmark.update", "bookmark", id, claims.Email+" updated bookmark \""+existing.Title+"\"", json.RawMessage(meta))

	common.JSON(w, http.StatusOK, existing)
}

// ─── Delete ──────────────────────────────────────────────────────────────────

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")

	existing, err := h.repo.GetBookmark(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "bookmark not found")
		return
	}

	if err := h.repo.DeleteBookmark(r.Context(), id); err != nil {
		log.Error().Err(err).Msg("bookmark delete failed")
		common.Error(w, http.StatusInternalServerError, "failed to delete bookmark")
		return
	}

	meta, _ := json.Marshal(map[string]interface{}{
		"title":    existing.Title,
		"url":      existing.URL,
		"category": existing.Category,
	})
	audit.Log(h.repo, claims.UserID, claims.Email, audit.RemoteIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For")),
		"bookmark.delete", "bookmark", id, claims.Email+" deleted bookmark \""+existing.Title+"\"", json.RawMessage(meta))

	common.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ─── Reorder ─────────────────────────────────────────────────────────────────

func (h *Handler) Reorder(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var items []model.BookmarkReorderItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.repo.ReorderBookmarks(r.Context(), items); err != nil {
		log.Error().Err(err).Msg("bookmark reorder failed")
		common.Error(w, http.StatusInternalServerError, "failed to reorder bookmarks")
		return
	}

	audit.Log(h.repo, claims.UserID, claims.Email, audit.RemoteIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For")),
		"bookmark.reorder", "bookmark", "", claims.Email+" reordered bookmarks")

	common.JSON(w, http.StatusOK, map[string]string{"status": "reordered"})
}
