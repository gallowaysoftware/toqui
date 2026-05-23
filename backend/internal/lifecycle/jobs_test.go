package lifecycle

import (
	"context"
	"testing"
	"time"
)

func TestJobsStartStops(t *testing.T) {
	// Verify that Start returns promptly when the context is cancelled.
	// We pass nil dependencies because we only test the control flow — the
	// first tick hasn't fired yet when we cancel.
	j := &Jobs{}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		j.Start(ctx)
	}()

	// Give the goroutine time to enter the select loop.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// OK — Start returned after context cancellation.
	case <-time.After(2 * time.Second):
		t.Fatal("Jobs.Start did not return after context cancellation")
	}
}

func TestMaxDeletionRetries(t *testing.T) {
	if maxDeletionRetries != 5 {
		t.Errorf("maxDeletionRetries = %d, want 5", maxDeletionRetries)
	}
}

// TestArchiveTrips_NilLifecycleSvc_NoPanic pins the defensive nil-guard
// in archiveTrips — pre-fix, when rand.IntN(60) returned 0 in
// TestJobsStartStops the `<-archiveReady` select case fired before the
// test cancelled the context, calling ArchiveCompletedTrips on a nil
// service and panicking. The flake hit ~1.7% of CI runs (toqui-backend
// #407). This test makes the regression explicit so a future refactor
// that removes the guard breaks loudly rather than re-introducing the
// flake.
func TestArchiveTrips_NilLifecycleSvc_NoPanic(t *testing.T) {
	j := &Jobs{} // both lifecycleSvc and queries are nil
	// Should NOT panic — the nil-guard makes it a no-op.
	j.archiveTrips(context.Background())
}

// TestRetryFailedDeletions_NilLifecycleSvc_NoPanic mirrors the
// archiveTrips guard for the deletion-retry branch.
func TestRetryFailedDeletions_NilLifecycleSvc_NoPanic(t *testing.T) {
	j := &Jobs{}
	j.retryFailedDeletions(context.Background())
}
