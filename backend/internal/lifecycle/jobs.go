package lifecycle

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

// Jobs runs periodic background maintenance tasks:
//   - DeleteExpiredRefreshTokens every hour
//   - PurgeExpiredChatSessions every hour (chat retention — replaces the
//     Firestore TTL policy the original chatstore relied on)
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

	// Run token cleanup and chat purge immediately on startup (expired
	// rows may have accumulated while the server was down).
	if j.queries != nil {
		j.cleanupExpiredTokens(ctx)
		j.purgeExpiredChat(ctx)
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
			j.purgeExpiredChat(ctx)

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
	// Defensive nil-guard — see archiveTrips for the rationale (the test
	// harness constructs &Jobs{} with nil deps).
	if j.queries == nil {
		return
	}
	if err := j.queries.DeleteExpiredRefreshTokens(ctx); err != nil {
		slog.Error("lifecycle: failed to cleanup expired refresh tokens", "error", err)
		return
	}
	slog.Info("lifecycle: expired refresh tokens cleaned up")
}

// purgeExpiredChat deletes chat sessions whose expire_at has passed
// (messages cascade), plus selection-mode ("_lobby") sessions idle longer
// than the retention window — those never belong to a trip so they never
// get an expire_at stamp. This is what enforces the chat retention window
// now that chat lives in Postgres instead of Firestore. Skipped entirely
// when retention is disabled (CHAT_RETENTION_DAYS=0).
func (j *Jobs) purgeExpiredChat(ctx context.Context) {
	// Defensive nil-guards — see archiveTrips for the rationale.
	if j.queries == nil || j.lifecycleSvc == nil {
		return
	}
	retention := j.lifecycleSvc.ChatRetentionDays()
	if retention <= 0 {
		return
	}
	purged, err := j.queries.PurgeExpiredChatSessions(ctx)
	if err != nil {
		slog.Error("lifecycle: failed to purge expired chat sessions", "error", err)
		return
	}
	stale, err := j.queries.PurgeStaleLobbyChatSessions(ctx, int32(retention))
	if err != nil {
		slog.Error("lifecycle: failed to purge stale lobby chat sessions", "error", err)
		return
	}
	if purged+stale > 0 {
		slog.Info("lifecycle: purged chat sessions", "expired", purged, "stale_lobby", stale)
	}
}

func (j *Jobs) archiveTrips(ctx context.Context) {
	// Defensive nil-guard mirrors the one in cleanupExpiredTokens. The
	// test harness deliberately constructs &Jobs{} with nil deps, and
	// when rand.IntN(60) returns 0 the `archiveReady := time.After(0)`
	// case can fire before the test cancels the context — calling
	// `j.lifecycleSvc.ArchiveCompletedTrips` on nil panics. The guard
	// is cheap and defensive against any future partial-init code path.
	if j.lifecycleSvc == nil {
		return
	}
	count, err := j.lifecycleSvc.ArchiveCompletedTrips(ctx)
	if err != nil {
		slog.Error("lifecycle: failed to archive completed trips", "error", err)
		return
	}
	slog.Info("lifecycle: archived trips", "count", count)
}

func (j *Jobs) retryFailedDeletions(ctx context.Context) {
	// Defensive nil-guard — see archiveTrips for the same rationale.
	if j.lifecycleSvc == nil {
		return
	}
	retried, failed, err := j.lifecycleSvc.RetryFailedDeletions(ctx)
	if err != nil {
		slog.Error("lifecycle: failed to retry deletions", "error", err)
		return
	}
	if retried > 0 || failed > 0 {
		slog.Info("lifecycle: deletion retries processed", "retried", retried, "failed", failed)
	}
}
