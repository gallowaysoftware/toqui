package ai

import (
	"context"
	"fmt"
	"log/slog"
)

// FallbackProvider wraps a primary and fallback Provider. If the primary
// returns an error on ChatStream, the fallback is tried. Once streaming
// has started (events are flowing), errors mid-stream are NOT retried
// because partial responses may have already been sent to the client.
type FallbackProvider struct {
	primary  Provider
	fallback Provider
}

// NewFallbackProvider creates a provider that tries primary first, falling
// back to fallback on initial connection errors. If fallback is nil, the
// provider behaves identically to primary.
func NewFallbackProvider(primary, fallback Provider) *FallbackProvider {
	return &FallbackProvider{
		primary:  primary,
		fallback: fallback,
	}
}

func (f *FallbackProvider) ChatStream(ctx context.Context, req *ChatRequest) (<-chan Event, error) {
	ch, err := f.primary.ChatStream(ctx, req)
	if err == nil {
		return ch, nil
	}

	if f.fallback == nil {
		return nil, err
	}

	slog.Warn("primary AI provider failed, trying fallback",
		"primary", f.primary.Name(),
		"fallback", f.fallback.Name(),
		"error", err,
	)

	ch, fallbackErr := f.fallback.ChatStream(ctx, req)
	if fallbackErr != nil {
		return nil, fmt.Errorf("primary (%s): %w; fallback (%s): %w",
			f.primary.Name(), err, f.fallback.Name(), fallbackErr)
	}
	return ch, nil
}

func (f *FallbackProvider) Name() string {
	return f.primary.Name()
}

// Primary returns the primary provider (for direct access if needed).
func (f *FallbackProvider) Primary() Provider {
	return f.primary
}

// Fallback returns the fallback provider, or nil if none is configured.
func (f *FallbackProvider) Fallback() Provider {
	return f.fallback
}
