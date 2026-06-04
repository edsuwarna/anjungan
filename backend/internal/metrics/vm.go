package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ─── VictoriaMetrics Client ─────────────────────────────────────────────────

type VMClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewVMClient(baseURL string) *VMClient {
	return &VMClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ─── Write: Prometheus import format ────────────────────────────────────────

// InsertMetrics sends a single metrics snapshot to VictoriaMetrics
// using Prometheus exposition format via the vminsert HTTP API.
func (c *VMClient) InsertMetrics(ctx context.Context, serverID string, m *MetricsSnapshot) error {
	now := time.Now().UnixMilli() / 1000 // seconds

	var lines []string

	// CPU load (1, 5, 15 min)
	if !math.IsNaN(m.CPULoad1) {
		lines = append(lines, fmt.Sprintf(`anjungan_cpu_load{server_id=%q,load="1m"} %f %d`, serverID, m.CPULoad1, now))
	}
	if !math.IsNaN(m.CPULoad5) {
		lines = append(lines, fmt.Sprintf(`anjungan_cpu_load{server_id=%q,load="5m"} %f %d`, serverID, m.CPULoad5, now))
	}
	if !math.IsNaN(m.CPULoad15) {
		lines = append(lines, fmt.Sprintf(`anjungan_cpu_load{server_id=%q,load="15m"} %f %d`, serverID, m.CPULoad15, now))
	}

	// Memory
	lines = append(lines,
		fmt.Sprintf(`anjungan_mem_used_bytes{server_id=%q} %d %d`, serverID, m.MemUsedBytes, now),
		fmt.Sprintf(`anjungan_mem_total_bytes{server_id=%q} %d %d`, serverID, m.MemTotalBytes, now),
	)
	// Only write mem_pct when mem_total > 0
	if m.MemTotalBytes > 0 {
		memUsedPct := float64(m.MemUsedBytes) / float64(m.MemTotalBytes) * 100
		lines = append(lines, fmt.Sprintf(`anjungan_mem_used_pct{server_id=%q} %f %d`, serverID, memUsedPct, now))
	}

	// Disk (skip NaN — collection failure)
	if !math.IsNaN(m.DiskUsedPct) {
		lines = append(lines,
			fmt.Sprintf(`anjungan_disk_used_bytes{server_id=%q} %d %d`, serverID, m.DiskUsedBytes, now),
			fmt.Sprintf(`anjungan_disk_total_bytes{server_id=%q} %d %d`, serverID, m.DiskTotalBytes, now),
			fmt.Sprintf(`anjungan_disk_used_pct{server_id=%q} %f %d`, serverID, m.DiskUsedPct, now),
		)
	}

	// Network
	lines = append(lines,
		fmt.Sprintf(`anjungan_net_rx_bytes{server_id=%q} %d %d`, serverID, m.NetRXBytes, now),
		fmt.Sprintf(`anjungan_net_tx_bytes{server_id=%q} %d %d`, serverID, m.NetTXBytes, now),
	)

	body := strings.Join(lines, "\n") + "\n"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/v1/import/prometheus",
		strings.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("vm insert request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vm insert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vm insert status %d: %s", resp.StatusCode, string(b))
	}

	return nil
}

// ─── Read: Query range ──────────────────────────────────────────────────────

// VMQueryResult represents a time series query result
type VMQueryResult struct {
	Metric map[string]string `json:"metric"`
	Values [][2]interface{}  `json:"values"` // [[timestamp, value], ...]
}

type vmQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string          `json:"resultType"`
		Result     []VMQueryResult `json:"result"`
	} `json:"data"`
}

// QueryRange returns time series data for a PromQL query
func (c *VMClient) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]VMQueryResult, error) {
	u, _ := url.Parse(c.baseURL + "/api/v1/query_range")
	q := u.Query()
	q.Set("query", query)
	q.Set("start", strconv.FormatInt(start.Unix(), 10))
	q.Set("end", strconv.FormatInt(end.Unix(), 10))
	q.Set("step", promStep(step))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("vm query request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vm query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vm query status %d: %s", resp.StatusCode, string(b))
	}

	var result vmQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("vm query decode: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("vm query status: %s", result.Status)
	}

	return result.Data.Result, nil
}

func promStep(d time.Duration) string {
	if d < time.Minute {
		d = time.Minute
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// ─── Metrics Snapshot (reusable type) ───────────────────────────────────────

type MetricsSnapshot struct {
	CPULoad1      float64
	CPULoad5      float64
	CPULoad15     float64
	MemUsedBytes  int64
	MemTotalBytes int64
	DiskUsedBytes int64
	DiskTotalBytes int64
	DiskUsedPct   float64
	NetRXBytes    int64
	NetTXBytes    int64
}
