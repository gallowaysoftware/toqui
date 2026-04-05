package ai

import (
	"context"
	"errors"
	"testing"
)

// mockProvider is a minimal Provider for testing.
type mockProvider struct {
	name   string
	err    error
	events []Event
}

func (m *mockProvider) ChatStream(_ context.Context, _ *ChatRequest) (<-chan Event, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan Event, len(m.events))
	for _, e := range m.events {
		ch <- e
	}
	close(ch)
	return ch, nil
}

func (m *mockProvider) Name() string {
	return m.name
}

func TestFallbackProvider_PrimarySucceeds(t *testing.T) {
	primary := &mockProvider{name: "primary", events: []Event{{Type: EventDone}}}
	fallback := &mockProvider{name: "fallback", events: []Event{{Type: EventDone}}}

	fp := NewFallbackProvider(primary, fallback)

	ch, err := fp.ChatStream(context.Background(), &ChatRequest{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Drain channel
	var count int
	for range ch {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 event from primary, got %d", count)
	}
}

func TestFallbackProvider_PrimaryFails_FallbackSucceeds(t *testing.T) {
	primary := &mockProvider{name: "primary", err: errors.New("primary down")}
	fallback := &mockProvider{name: "fallback", events: []Event{{Type: EventDone}}}

	fp := NewFallbackProvider(primary, fallback)

	ch, err := fp.ChatStream(context.Background(), &ChatRequest{})
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}

	var count int
	for range ch {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 event from fallback, got %d", count)
	}
}

func TestFallbackProvider_BothFail(t *testing.T) {
	primary := &mockProvider{name: "primary", err: errors.New("primary down")}
	fallback := &mockProvider{name: "fallback", err: errors.New("fallback down")}

	fp := NewFallbackProvider(primary, fallback)

	_, err := fp.ChatStream(context.Background(), &ChatRequest{})
	if err == nil {
		t.Fatal("expected error when both providers fail")
	}

	// Error message should mention both providers
	msg := err.Error()
	if !errors.Is(err, primary.err) {
		t.Errorf("expected wrapped primary error, got: %s", msg)
	}
}

func TestFallbackProvider_NoFallback(t *testing.T) {
	primary := &mockProvider{name: "primary", err: errors.New("primary down")}

	fp := NewFallbackProvider(primary, nil)

	_, err := fp.ChatStream(context.Background(), &ChatRequest{})
	if err == nil {
		t.Fatal("expected error when no fallback")
	}
}

func TestFallbackProvider_Name(t *testing.T) {
	fp := NewFallbackProvider(
		&mockProvider{name: "gemini"},
		&mockProvider{name: "claude"},
	)
	if fp.Name() != "gemini" {
		t.Errorf("expected name 'gemini', got %q", fp.Name())
	}
}
