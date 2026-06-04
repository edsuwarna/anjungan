package metrics

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
	sshtool "github.com/edsuwarna/anjungan/internal/infra/ssh"
)

// ─── Collector ──────────────────────────────────────────────────────────────

type Collector struct {
	repo     *db.Repository
	vmClient *VMClient
	interval time.Duration
}

func NewCollector(repo *db.Repository, vmClient *VMClient, interval time.Duration) *Collector {
	if interval < time.Minute {
		interval = 5 * time.Minute
	}
	return &Collector{
		repo:     repo,
		vmClient: vmClient,
		interval: interval,
	}
}

// Start launches the collector in a background goroutine.
// It runs immediately, then repeats on the given interval.
func (c *Collector) Start(ctx context.Context) {
	log.Printf("[metrics-collector] starting, interval=%v", c.interval)

	// Run immediately on startup, then tick
	c.collectAll(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.collectAll(ctx)
		case <-ctx.Done():
			log.Printf("[metrics-collector] stopped: %v", ctx.Err())
			return
		}
	}
}

func (c *Collector) collectAll(ctx context.Context) {
	start := time.Now()
	log.Printf("[metrics-collector] collecting metrics from all servers...")

	servers, err := c.repo.ListServers(ctx)
	if err != nil {
		log.Printf("[metrics-collector] list servers error: %v", err)
		return
	}

	var collected, failed int
	for _, srv := range servers {
		select {
		case <-ctx.Done():
			return
		default:
		}

		snapshot, err := c.collectOne(ctx, srv)
		if err != nil {
			log.Printf("[metrics-collector] server %s (%s): %v", srv.Name, srv.Host, err)
			failed++
			continue
		}

		if err := c.vmClient.InsertMetrics(ctx, srv.ID, snapshot); err != nil {
			log.Printf("[metrics-collector] insert vm %s: %v", srv.Name, err)
			failed++
			continue
		}
		collected++
	}

	log.Printf("[metrics-collector] done: %d collected, %d failed, took %v",
		collected, failed, time.Since(start).Round(time.Millisecond))
}

func (c *Collector) collectOne(ctx context.Context, srv *model.Server) (*MetricsSnapshot, error) {
	sshCfg := sshtool.Config{
		Host:     srv.Host,
		Port:     srv.Port,
		User:     srv.SSHUser,
		AuthType: srv.SSHAuthType,
		Key:      srv.SSHKey,
		Password: srv.SSHPassword,
		Timeout:  15 * time.Second,
	}

	snap := &MetricsSnapshot{
		CPULoad1:  math.NaN(),
		CPULoad5:  math.NaN(),
		CPULoad15: math.NaN(),
		DiskUsedPct: math.NaN(),
	}

	// CPU
	cpuOut, err := sshtool.RunCommand(ctx, sshCfg, "cat /proc/loadavg")
	if err == nil {
		var l1, l5, l15 float64
		if _, e := fmt.Sscanf(cpuOut, "%f %f %f", &l1, &l5, &l15); e == nil {
			snap.CPULoad1 = l1
			snap.CPULoad5 = l5
			snap.CPULoad15 = l15
		}
	}

	// Memory (bytes)
	memOut, err := sshtool.RunCommand(ctx, sshCfg, "free -b | awk 'NR==2{print $2,$3}'")
	if err == nil {
		fmt.Sscanf(memOut, "%d %d", &snap.MemTotalBytes, &snap.MemUsedBytes)
	}

	// Disk (bytes)
	diskOut, err := sshtool.RunCommand(ctx, sshCfg, `df -B1 / | awk 'NR==2{print $2,$3,$5}'`)
	if err == nil {
		var total, used int64
		var pctStr string
		if _, e := fmt.Sscanf(diskOut, "%d %d %s", &total, &used, &pctStr); e == nil {
			snap.DiskTotalBytes = total
			snap.DiskUsedBytes = used
			var pct float64
			if _, e := fmt.Sscanf(pctStr, "%f%%", &pct); e == nil {
				snap.DiskUsedPct = pct
			}
		}
	}

	// Network (cumulative bytes since boot)
	netOut, err := sshtool.RunCommand(ctx, sshCfg,
		`awk '{rx=$1; tx=$2} END{printf "%d %d", rx, tx}' /proc/net/dev | tail -1`)
	if err == nil {
		fmt.Sscanf(netOut, "%d %d", &snap.NetRXBytes, &snap.NetTXBytes)
	}

	return snap, nil
}
