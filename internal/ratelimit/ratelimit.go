package ratelimit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
)

type userEntry struct {
	aiLimiter      *rate.Limiter
	generalLimiter *rate.Limiter
	lastSeen       time.Time
}

// interceptor implements connect.Interceptor with per-user rate limiting.
type interceptor struct {
	mu             sync.Mutex
	users          map[uuid.UUID]*userEntry
	aiPerMinute    int
	generalPerMin  int
	cleanupStop    chan struct{}
}

// NewInterceptor creates a rate-limiting interceptor. aiPerMinute controls the
// rate for AI chat RPCs (SendMessage); generalPerMinute controls all other RPCs.
func NewInterceptor(aiPerMinute, generalPerMinute int) *interceptor {
	i := &interceptor{
		users:         make(map[uuid.UUID]*userEntry),
		aiPerMinute:   aiPerMinute,
		generalPerMin: generalPerMinute,
		cleanupStop:   make(chan struct{}),
	}
	go i.cleanupLoop()
	return i
}

func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := i.check(ctx, req.Spec().Procedure); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // server-side: no-op
}

func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := i.check(ctx, conn.Spec().Procedure); err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

// check enforces the rate limit for the authenticated user. If the user is not
// in context (e.g. public endpoints), the request is allowed through.
func (i *interceptor) check(ctx context.Context, procedure string) error {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil // unauthenticated / public endpoint — skip
	}

	entry := i.getOrCreate(userID)

	limiter := entry.generalLimiter
	if isAIProcedure(procedure) {
		limiter = entry.aiLimiter
	}

	if !limiter.Allow() {
		return connect.NewError(
			connect.CodeResourceExhausted,
			fmt.Errorf("rate limit exceeded, please try again later"),
		)
	}
	return nil
}

func (i *interceptor) getOrCreate(userID uuid.UUID) *userEntry {
	i.mu.Lock()
	defer i.mu.Unlock()

	entry, ok := i.users[userID]
	if !ok {
		entry = &userEntry{
			aiLimiter:      rate.NewLimiter(rate.Every(time.Minute/time.Duration(i.aiPerMinute)), i.aiPerMinute),
			generalLimiter: rate.NewLimiter(rate.Every(time.Minute/time.Duration(i.generalPerMin)), i.generalPerMin),
		}
		i.users[userID] = entry
	}
	entry.lastSeen = time.Now()
	return entry
}

func isAIProcedure(procedure string) bool {
	return strings.Contains(procedure, "SendMessage")
}

// cleanupLoop removes stale entries every minute.
func (i *interceptor) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			i.evictStale()
		case <-i.cleanupStop:
			return
		}
	}
}

func (i *interceptor) evictStale() {
	i.mu.Lock()
	defer i.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for uid, entry := range i.users {
		if entry.lastSeen.Before(cutoff) {
			delete(i.users, uid)
		}
	}
}

// Stop terminates the background cleanup goroutine.
func (i *interceptor) Stop() {
	close(i.cleanupStop)
}
