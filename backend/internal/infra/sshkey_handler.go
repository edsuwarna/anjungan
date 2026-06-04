package infra

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/edsuwarna/anjungan/internal/audit"
	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

type SSHKeyHandler struct {
	repo *db.Repository
}

func NewSSHKeyHandler(repo *db.Repository) *SSHKeyHandler {
	return &SSHKeyHandler{repo: repo}
}

func (h *SSHKeyHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	return r
}

type createSSHKeyInput struct {
	Name       string `json:"name"`
	KeyType    string `json:"key_type"`
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key,omitempty"`
}

type updateSSHKeyInput struct {
	Name       *string `json:"name,omitempty"`
	KeyType    *string `json:"key_type,omitempty"`
	PrivateKey *string `json:"private_key,omitempty"`
	PublicKey  *string `json:"public_key,omitempty"`
}

func (h *SSHKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	keys, err := h.repo.ListSSHKeys(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list ssh keys")
		return
	}

	// Build responses with server counts
	resp := make([]model.SSHKeyResponse, 0, len(keys))
	for _, k := range keys {
		kr := k.ToResponse()
		count, _ := h.repo.CountServersUsingSSHKey(r.Context(), k.ID)
		kr.ServerCount = count
		resp = append(resp, kr)
	}

	common.JSON(w, http.StatusOK, resp)
}

func (h *SSHKeyHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	key, err := h.repo.GetSSHKeyByID(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "ssh key not found")
		return
	}

	kr := key.ToResponse()
	count, _ := h.repo.CountServersUsingSSHKey(r.Context(), id)
	kr.ServerCount = count
	common.JSON(w, http.StatusOK, kr)
}

func (h *SSHKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input createSSHKeyInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.Name == "" || input.PrivateKey == "" {
		common.Error(w, http.StatusBadRequest, "name and private_key are required")
		return
	}

	if input.KeyType == "" {
		input.KeyType = "ed25519"
	}

	// Auto-generate fingerprint from private key
	fingerprint := generateFingerprint(input.PrivateKey)

	userID := ""
	if claims := auth.GetClaims(r.Context()); claims != nil {
		userID = claims.UserID
	}

	now := time.Now()
	key := &model.SSHKey{
		ID:          uuid.New().String(),
		Name:        input.Name,
		KeyType:     input.KeyType,
		PrivateKey:  input.PrivateKey,
		PublicKey:   input.PublicKey,
		Fingerprint: fingerprint,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.repo.CreateSSHKey(r.Context(), key); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create ssh key")
		return
	}

	// Log activity
	_ = h.repo.SaveActivity(r.Context(), "ssh_key_added",
		fmt.Sprintf("Added SSH key %s (%s)", key.Name, key.Fingerprint), userID)

	// Audit log
	if claims := auth.GetClaims(r.Context()); claims != nil {
		meta, _ := json.Marshal(map[string]string{
			"key_name":    key.Name,
			"key_type":    key.KeyType,
			"fingerprint": key.Fingerprint,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"ssh-key.create", "ssh_key", key.ID,
			fmt.Sprintf("Created SSH key %s (%s)", key.Name, key.Fingerprint),
			json.RawMessage(meta))
	}

	kr := key.ToResponse()
	common.JSON(w, http.StatusCreated, kr)
}

func (h *SSHKeyHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.repo.GetSSHKeyByIDFull(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "ssh key not found")
		return
	}

	var input updateSSHKeyInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.Name != nil {
		existing.Name = *input.Name
	}
	if input.KeyType != nil {
		existing.KeyType = *input.KeyType
	}
	if input.PrivateKey != nil {
		existing.PrivateKey = *input.PrivateKey
		existing.Fingerprint = generateFingerprint(*input.PrivateKey)
	}
	if input.PublicKey != nil {
		existing.PublicKey = *input.PublicKey
	}

	if err := h.repo.UpdateSSHKey(r.Context(), existing); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to update ssh key")
		return
	}

	// Audit log
	if claims := auth.GetClaims(r.Context()); claims != nil {
		meta, _ := json.Marshal(map[string]string{
			"key_name": existing.Name,
			"key_type": existing.KeyType,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"ssh-key.update", "ssh_key", existing.ID,
			fmt.Sprintf("Updated SSH key %s", existing.Name),
			json.RawMessage(meta))
	}

	kr := existing.ToResponse()
	count, _ := h.repo.CountServersUsingSSHKey(r.Context(), id)
	kr.ServerCount = count
	common.JSON(w, http.StatusOK, kr)
}

func (h *SSHKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Check if key is in use
	count, err := h.repo.CountServersUsingSSHKey(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to check key usage")
		return
	}
	if count > 0 {
		common.Error(w, http.StatusConflict,
			fmt.Sprintf("Cannot delete key: used by %d server(s). Reassign servers first.", count))
		return
	}

	if err := h.repo.DeleteSSHKey(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete ssh key")
		return
	}

	// Audit log
	if claims := auth.GetClaims(r.Context()); claims != nil {
		meta, _ := json.Marshal(map[string]string{
			"key_id":   id,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"ssh-key.delete", "ssh_key", id,
			"Deleted SSH key",
			json.RawMessage(meta))
	}

	common.JSON(w, http.StatusOK, map[string]string{"message": "SSH key deleted"})
}

// generateFingerprint produces a short SHA256 hash of the key content for display
func generateFingerprint(keyContent string) string {
	trimmed := strings.TrimSpace(keyContent)
	h := sha256.Sum256([]byte(trimmed))
	// Format like SHA256:xxxx (similar to actual SSH fingerprint format)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(h[:])
}
