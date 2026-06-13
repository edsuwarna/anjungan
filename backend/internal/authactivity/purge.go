package authactivity

import (
	"context"
	"time"

	zlog "github.com/rs/zerolog/log"
)

const DefaultPurgeAge = 90 * 24 * time.Hour // 90 days

// StartPurgeScheduler starts a goroutine that purges old auth events daily.
func StartPurgeScheduler(ctx context.Context, repo Repository) {
	go func() {
		runPurge(repo)

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				runPurge(repo)
			case <-ctx.Done():
				zlog.Info().Msg("auth events purge scheduler stopped")
				return
			}
		}
	}()
	zlog.Info().Msg("auth events purge scheduler started (90 day retention)")
}

func runPurge(repo Repository) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deleted, err := repo.PurgeAuthEvents(ctx, DefaultPurgeAge)
	if err != nil {
		zlog.Err(err).Msg("failed to purge old auth events")
		return
	}
	if deleted > 0 {
		zlog.Info().Int64("deleted", deleted).Msg("purged old auth events")
	}
}
