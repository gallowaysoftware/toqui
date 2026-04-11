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
