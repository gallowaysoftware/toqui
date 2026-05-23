package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler serves health check endpoints for load balancer probes.
type HealthHandler struct {
	pool      *pgxpool.Pool
	startTime time.Time
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(pool *pgxpool.Pool, startTime time.Time) *HealthHandler {
	return &HealthHandler{pool: pool, startTime: startTime}
}

// HandleHealth handles GET /health — detailed health check with component statuses.
func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check database
	dbStart := time.Now()
	dbErr := h.pool.Ping(ctx)
	dbLatency := time.Since(dbStart).Milliseconds()

	dbStatus := "healthy"
	if dbErr != nil {
		dbStatus = "unhealthy"
	}

	overallStatus := "healthy"
	statusCode := http.StatusOK
	if dbStatus != "healthy" {
		overallStatus = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]any{
		"status": overallStatus,
		"components": map[string]any{
			"database": map[string]any{
				"status":     dbStatus,
				"latency_ms": dbLatency,
			},
			"server": map[string]any{
				"status":         "healthy",
				"uptime_seconds": int(time.Since(h.startTime).Seconds()),
			},
		},
	})
}

// HandleReadiness handles GET /ready — lightweight readiness probe.
func (h *HealthHandler) HandleReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ready": true})
}
