package sslmonitor

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

// ─── Scheduler ──────────────────────────────────────────────────────────────

// Scheduler runs periodic SSL checks and auto-purges old history.
type Scheduler struct {
	repo       *db.Repository
	handler    *Handler
	done       chan struct{}
	wg         sync.WaitGroup
	checkTick  time.Duration // how often to check for due monitors
	purgeTick  time.Duration // how often to purge old history
	purgeAge   time.Duration // age threshold for purge
}

// NewScheduler creates a new SSL monitor scheduler.
func NewScheduler(repo *db.Repository, handler *Handler) *Scheduler {
	return &Scheduler{
		repo:      repo,
		handler:   handler,
		done:      make(chan struct{}),
		checkTick: 1 * time.Minute,       // check for due monitors every minute
		purgeTick: 24 * time.Hour,        // purge old history once daily
		purgeAge:  90 * 24 * time.Hour,   // keep 90 days of history
	}
}

// Start begins the scheduler loop in a background goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(2)

	go s.runCheckLoop(ctx)
	go s.runPurgeLoop(ctx)

	log.Println("[sslmonitor] scheduler started — checking every 1m, purge every 24h")
}

// Stop signals the scheduler to shut down.
func (s *Scheduler) Stop() {
	close(s.done)
	s.wg.Wait()
	log.Println("[sslmonitor] scheduler stopped")
}

// runCheckLoop periodically checks all due monitors.
func (s *Scheduler) runCheckLoop(ctx context.Context) {
	defer s.wg.Done()

	// Run an initial check shortly after startup
	go func() {
		time.Sleep(10 * time.Second)
		s.checkDueMonitors(ctx)
	}()

	ticker := time.NewTicker(s.checkTick)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.checkDueMonitors(ctx)
		}
	}
}

// checkDueMonitors finds and checks all monitors that are due.
func (s *Scheduler) checkDueMonitors(ctx context.Context) {
	monitors, err := s.repo.ListDueSSLMonitors(ctx)
	if err != nil {
		log.Printf("[sslmonitor] failed to list due monitors: %v", err)
		return
	}

	if len(monitors) == 0 {
		return
	}

	log.Printf("[sslmonitor] checking %d due monitor(s)", len(monitors))

	// Use a semaphore to limit concurrent checks
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	for _, m := range monitors {
		wg.Add(1)
		sem <- struct{}{}

		go func(mon *model.SSLMonitor) {
			defer wg.Done()
			defer func() { <-sem }()

			result := Check(ctx, mon.Domain, mon.Port)
			s.handler.saveResult(ctx, mon, result)
		}(m)
	}

	wg.Wait()
}

// runPurgeLoop periodically removes old check history.
func (s *Scheduler) runPurgeLoop(ctx context.Context) {
	defer s.wg.Done()

	// Run initial purge at startup
	s.purgeOldHistory(ctx)

	ticker := time.NewTicker(s.purgeTick)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.purgeOldHistory(ctx)
		}
	}
}

func (s *Scheduler) purgeOldHistory(ctx context.Context) {
	n, err := s.repo.PurgeSSLCheckHistory(ctx, s.purgeAge)
	if err != nil {
		log.Printf("[sslmonitor] history purge failed: %v", err)
		return
	}
	if n > 0 {
		log.Printf("[sslmonitor] purged %d old history entries", n)
	}
}
