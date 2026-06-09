package registry

import (
	"net/http"
	"time"

	"github.com/edsuwarna/anjungan/internal/common"
)

// HealthCheck pings the Zot registry and returns status.
// GET /registry/health
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(h.cfg.URL + "/v2/")
	if err != nil {
		common.JSON(w, http.StatusOK, map[string]interface{}{
			"status":  "down",
			"message": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	status := "up"
	message := "Registry is reachable"
	if resp.StatusCode >= 400 {
		status = "down"
		message = "Registry returned status " + resp.Status
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"status":  status,
		"message": message,
	})
}
