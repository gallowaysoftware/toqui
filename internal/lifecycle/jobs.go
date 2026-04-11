package lifecycle

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// Jobs runs periodic background maintenance tasks:
//   - DeleteExpiredRefreshTokens every hour
//   - ArchiveCompletedTrips every 24 hours (with random offset)
//   - RetryFailedDeletions every hour
type Jobs struct {
	lifecycleSvc *Service
	queries      *dbgen.Queries
}

// NewJobs creates a new Jobs instance.
func NewJobs(lifecycleSvc *Service, pool *pgxpool.Pool) *Jobs {
	return &Jobs{
		lifecycleSvc: lifecycleSvc,
		queries:      dbgen.New(pool),
	}
}

// Start launches the background job goroutine. It blocks until ctx is cancelled,
// so callers should run it in a goroutine. On context cancellation it stops
// gracefully and returns.
func (j *Jobs) Start(ctx context.Context) {
	// Stagger archival by a random offset (0–60 min) to avoid thundering herd
	// across multiple instances.
	archiveOffset := time.Duration(rand.IntN(60)) * time.Minute
	slog.Info("lifecycle: background jobs starting",
		"token_cleanup_interval", "1h",
		"archival_interval", "24h",
		"archival_offset", archiveOffset.String(),
		"deletion_retry_interval", "1h",
	)

	tokenTicker := time.NewTicker(1 * time.Hour)
	defer tokenTicker.Stop()

	archiveTicker := time.NewTicker(24 * time.Hour)
	defer archiveTicker.Stop()

	deletionRetryTicker := time.NewTicker(1 * time.Hour)
	defer deletionRetryTicker.Stop()

	// Run token cleanup immediately on startup (expired tokens may have
	// accumulated while the server was down).
	if j.queries != nil {
		j.cleanupExpiredTokens(ctx)
	}

	// Delay first archival run by the random offset.
	archiveReady := time.After(archiveOffset)
	archiveStarted := false

	for {
		select {
		case <-ctx.Done():
			slog.Info("lifecycle: background jobs stopping")
			return

		case <-tokenTicker.C:
			j.cleanupExpiredTokens(ctx)

		case <-archiveReady:
			// First archival run after random offset, then use the ticker.
			if !archiveStarted {
				j.archiveTrips(ctx)
				archiveStarted = true
			}

		case <-archiveTicker.C:
			if archiveStarted {
				j.archiveTrips(ctx)
			}

		case <-deletionRetryTicker.C:
			j.retryFailedDeletions(ctx)
		}
	}
}

func (j *Jobs) cleanupExpiredTokens(ctx context.Context) {
	if err := j.queries.DeleteExpiredRefreshTokens(ctx); err != nil {
		slog.Error("lifecycle: failed to cleanup expired refresh tokens", "error", err)
		return
	}
	slog.Info("lifecycle: expired refresh tokens cleaned up")
}

func (j *Jobs) archiveTrips(ctx context.Context) {
	count, err := j.lifecycleSvc.ArchiveCompletedTrips(ctx)
	if err != nil {
		slog.Error("lifecycle: failed to archive completed trips", "error", err)
		return
	}
	slog.Info("lifecycle: archived trips", "count", count)
}

func (j *Jobs) retryFailedDeletions(ctx context.Context) {
	retried, failed, err := j.lifecycleSvc.RetryFailedDeletions(ctx)
	if err != nil {
		slog.Error("lifecycle: failed to retry deletions", "error", err)
		return
	}
	if retried > 0 || failed > 0 {
		slog.Info("lifecycle: deletion retries processed", "retried", retried, "failed", failed)
	}
}
