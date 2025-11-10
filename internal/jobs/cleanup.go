package jobs

import (
	"context"
	"time"

	"github.com/diagnosis/luxsuv-api-v2/internal/logger"
	"github.com/diagnosis/luxsuv-api-v2/internal/store"
)

type CleanupJob struct {
	refreshStore      store.RefreshStore
	verificationStore store.AuthVerificationStore
	interval          time.Duration
	stopCh            chan struct{}
}

func NewCleanupJob(rs store.RefreshStore, vs store.AuthVerificationStore, interval time.Duration) *CleanupJob {
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	return &CleanupJob{
		refreshStore:      rs,
		verificationStore: vs,
		interval:          interval,
		stopCh:            make(chan struct{}),
	}
}

func (j *CleanupJob) Start() {
	ticker := time.NewTicker(j.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				j.run()
			case <-j.stopCh:
				return
			}
		}
	}()
}

func (j *CleanupJob) Stop() {
	close(j.stopCh)
}

func (j *CleanupJob) run() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deleted, err := j.refreshStore.DeleteExpired(ctx)
	if err != nil {
		logger.Error(ctx, "failed to delete expired refresh tokens", "error", err)
	} else if deleted > 0 {
		logger.Info(ctx, "deleted expired refresh tokens", "count", deleted)
	}

	deletedVerif, err := j.verificationStore.DeleteExpired(ctx, time.Now())
	if err != nil {
		logger.Error(ctx, "failed to delete expired verification tokens", "error", err)
	} else if deletedVerif > 0 {
		logger.Info(ctx, "deleted expired verification tokens", "count", deletedVerif)
	}
}
