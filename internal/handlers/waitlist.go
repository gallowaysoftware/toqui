package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/email"
)

func textVal(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// maxEmailLength is the maximum length of an email address per RFC 5321.
const maxEmailLength = 254

// WaitlistHandler serves the public waitlist endpoints.
type WaitlistHandler struct {
	queries    *dbgen.Queries
	emailSvc   *email.Sender
	apiBaseURL string // e.g. "https://api.toqui.travel"
}

// NewWaitlistHandler creates a new WaitlistHandler.
// emailSvc may be nil to disable verification emails (email still validated on format).
func NewWaitlistHandler(pool *pgxpool.Pool, emailSvc *email.Sender, apiBaseURL string) *WaitlistHandler {
	return &WaitlistHandler{
		queries:    dbgen.New(pool),
		emailSvc:   emailSvc,
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
	}
}

type waitlistRequest struct {
	Email string `json:"email"`
}

type waitlistResponse struct {
	Position int64  `json:"position"`
	Message  string `json:"message,omitempty"`
}

type waitlistStatusResponse struct {
	Position int64 `json:"position"`
	Total    int64 `json:"total"`
}

// HandleJoin handles POST /waitlist — adds an email to the waitlist and sends verification.
func (h *WaitlistHandler) HandleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req waitlistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}
	if len(req.Email) > maxEmailLength {
		http.Error(w, "email is too long", http.StatusBadRequest)
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		http.Error(w, "invalid email address", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	token := generateVerifyToken()

	entry, err := h.queries.AddToWaitlist(ctx, dbgen.AddToWaitlistParams{
		Email:       req.Email,
		VerifyToken: textVal(token),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Already on waitlist — check if already verified
			existing, getErr := h.queries.GetWaitlistByEmail(ctx, req.Email)
			if getErr != nil {
				slog.Error("get waitlist by email failed", "email", maskEmail(req.Email), "error", getErr)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if existing.VerifiedAt.Valid {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(waitlistResponse{
					Message: "You're already on the waitlist!",
				})
				return
			}
			// Not verified — resend verification email
			if h.emailSvc != nil && existing.VerifyToken.Valid {
				verifyURL := h.apiBaseURL + "/waitlist/verify?token=" + existing.VerifyToken.String
				if sendErr := h.emailSvc.SendVerification(req.Email, verifyURL); sendErr != nil {
					slog.Error("resend verification email failed", "email", maskEmail(req.Email), "error", sendErr)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(waitlistResponse{
				Message: "We've resent your verification email. Please check your inbox.",
			})
			return
		}
		slog.Error("add to waitlist failed", "email", maskEmail(req.Email), "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Send verification email
	if h.emailSvc != nil {
		verifyURL := h.apiBaseURL + "/waitlist/verify?token=" + token
		if sendErr := h.emailSvc.SendVerification(req.Email, verifyURL); sendErr != nil {
			slog.Error("send verification email failed", "email", maskEmail(req.Email), "error", sendErr)
			// Don't fail the request — the entry is created, they can retry
		}
	} else {
		// No email service — auto-verify (local development)
		if _, verifyErr := h.queries.VerifyWaitlistEmail(ctx, textVal(token)); verifyErr != nil {
			slog.Error("auto-verify waitlist failed", "error", verifyErr)
		}
	}

	_ = entry // entry is created, position calculated after verification

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(waitlistResponse{
		Message: "Check your email to verify your waitlist signup!",
	})
}

// HandleVerify handles GET /waitlist/verify?token=TOKEN — verifies email and shows position.
func (h *WaitlistHandler) HandleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check if already verified
	existing, err := h.queries.GetWaitlistByVerifyToken(ctx, textVal(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "invalid or expired verification link", http.StatusNotFound)
			return
		}
		slog.Error("get waitlist by verify token failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if !existing.VerifiedAt.Valid {
		if _, verifyErr := h.queries.VerifyWaitlistEmail(ctx, textVal(token)); verifyErr != nil {
			slog.Error("verify waitlist email failed", "error", verifyErr)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	// Redirect to the app's waitlist page with their email
	redirectURL := "https://app.toqui.travel/waitlist?email=" + existing.Email + "&verified=true"
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// HandleStatus handles GET /waitlist/status?email=... — returns position and total.
func (h *WaitlistHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	emailAddr := strings.TrimSpace(r.URL.Query().Get("email"))
	if emailAddr == "" {
		http.Error(w, "email query parameter is required", http.StatusBadRequest)
		return
	}
	if len(emailAddr) > maxEmailLength {
		http.Error(w, "email is too long", http.StatusBadRequest)
		return
	}
	if _, err := mail.ParseAddress(emailAddr); err != nil {
		http.Error(w, "invalid email address", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	entry, err := h.queries.GetWaitlistByEmail(ctx, emailAddr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "email not found on waitlist", http.StatusNotFound)
			return
		}
		slog.Error("get waitlist by email failed", "email", maskEmail(emailAddr), "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	position, err := h.queries.CountWaitlistAhead(ctx, entry.SignedUpAt)
	if err != nil {
		slog.Error("count waitlist ahead failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	totalUsers, err := h.queries.CountUsers(ctx)
	if err != nil {
		slog.Error("count users failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(waitlistStatusResponse{
		Position: position + 1,
		Total:    totalUsers,
	})
}

func generateVerifyToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
