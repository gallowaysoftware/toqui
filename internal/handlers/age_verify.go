package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// AgeVerifyHandler handles the POST /auth/verify-age endpoint.
type AgeVerifyHandler struct {
	authSvc *auth.Service
	queries *dbgen.Queries
}

// NewAgeVerifyHandler creates a new AgeVerifyHandler.
func NewAgeVerifyHandler(authSvc *auth.Service, queries *dbgen.Queries) *AgeVerifyHandler {
	return &AgeVerifyHandler{authSvc: authSvc, queries: queries}
}

type verifyAgeRequest struct {
	DateOfBirth string `json:"date_of_birth"` // format: "2000-01-15"
}

// HandleVerifyAge validates the user's date of birth and records age verification.
func (h *AgeVerifyHandler) HandleVerifyAge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	var req verifyAgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.DateOfBirth == "" {
		http.Error(w, "date_of_birth is required", http.StatusBadRequest)
		return
	}

	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		http.Error(w, "invalid date_of_birth format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	// Validate age >= 18
	now := time.Now()
	age := now.Year() - dob.Year()
	if now.YearDay() < dob.YearDay() {
		age--
	}

	if age < 18 {
		slog.Info("age verification failed: under 18",
			"user_id", userID.String(),
		)
		http.Error(w, "you must be at least 18 years old to use this service", http.StatusForbidden)
		return
	}

	// Don't accept future dates or unreasonably old dates
	if dob.After(now) || age > 150 {
		http.Error(w, "invalid date of birth", http.StatusBadRequest)
		return
	}

	if err := h.queries.SetAgeVerified(r.Context(), userID); err != nil {
		slog.Error("failed to set age verification", "user_id", userID.String(), "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("age verification succeeded", "user_id", userID.String())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"verified":true}`))
}
