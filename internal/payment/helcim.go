package payment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

const (
	helcimBaseURL    = "https://api.helcim.com/v2"
	maxResponseBytes = 1 << 20 // 1 MB
)

// Service handles Helcim payment operations.
type Service struct {
	apiToken   string
	priceCents int
	queries    *dbgen.Queries
	client     *http.Client
}

// NewService creates a new payment service.
func NewService(apiToken string, priceCents int, queries *dbgen.Queries) *Service {
	return &Service{
		apiToken:   apiToken,
		priceCents: priceCents,
		queries:    queries,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

// CheckoutResult is returned after initializing a checkout session.
type CheckoutResult struct {
	CheckoutToken string
	SecretToken   string
}

// InitializeCheckout creates a Helcim checkout session for a trip purchase.
func (s *Service) InitializeCheckout(ctx context.Context, userID, tripID uuid.UUID) (*CheckoutResult, error) {
	// Check if trip is already unlocked
	unlocked, err := s.queries.IsTripUnlocked(ctx, dbgen.IsTripUnlockedParams{
		UserID: userID,
		TripID: tripID,
	})
	if err != nil {
		return nil, fmt.Errorf("check trip unlock: %w", err)
	}
	if unlocked {
		return nil, fmt.Errorf("trip already unlocked")
	}

	// Amount in dollars (Helcim expects decimal)
	amount := float64(s.priceCents) / 100.0

	body := map[string]any{
		"paymentType": "purchase",
		"amount":      amount,
		"currency":    "CAD",
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, helcimBaseURL+"/helcim-pay/initialize", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("api-token", s.apiToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("helcim API call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("helcim checkout init failed",
			"status", resp.StatusCode,
			"body", string(respBody),
		)
		return nil, fmt.Errorf("helcim API error: status %d", resp.StatusCode)
	}

	var result struct {
		CheckoutToken string `json:"checkoutToken"`
		SecretToken   string `json:"secretToken"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if result.CheckoutToken == "" || result.SecretToken == "" {
		return nil, fmt.Errorf("helcim returned empty tokens")
	}

	// Store the session in DB
	_, err = s.queries.CreateCheckoutSession(ctx, dbgen.CreateCheckoutSessionParams{
		UserID:        userID,
		TripID:        tripID,
		CheckoutToken: result.CheckoutToken,
		SecretToken:   result.SecretToken,
		AmountCents:   int32(s.priceCents),
		Currency:      "CAD",
	})
	if err != nil {
		return nil, fmt.Errorf("store checkout session: %w", err)
	}

	slog.Info("helcim checkout session created",
		"user_id", userID,
		"trip_id", tripID,
		"amount_cents", s.priceCents,
	)

	return &CheckoutResult{
		CheckoutToken: result.CheckoutToken,
		SecretToken:   result.SecretToken,
	}, nil
}

// ValidateAndRecordPayment validates the HelcimPay.js response hash and records a successful payment.
// The userID parameter ensures the authenticated user owns the checkout session (IDOR prevention).
func (s *Service) ValidateAndRecordPayment(ctx context.Context, userID uuid.UUID, checkoutToken string, responseData json.RawMessage, responseHash string) error {
	// Look up the session
	session, err := s.queries.GetCheckoutSessionByToken(ctx, checkoutToken)
	if err != nil {
		return fmt.Errorf("checkout session not found: %w", err)
	}

	// Verify the authenticated user owns this checkout session.
	if session.UserID != userID {
		slog.Warn("payment validation IDOR attempt",
			"authenticated_user", userID,
			"session_owner", session.UserID,
			"checkout_token", checkoutToken,
		)
		return fmt.Errorf("checkout session not found: %w", err)
	}

	if session.Status != "open" {
		return fmt.Errorf("checkout session already %s", session.Status)
	}

	// Reject sessions older than 1 hour to prevent stale token abuse.
	if time.Since(session.CreatedAt) > time.Hour {
		_ = s.queries.MarkCheckoutSessionExpired(ctx, checkoutToken)
		return fmt.Errorf("checkout session expired")
	}

	// Validate hash: SHA-256(JSON(data) + secretToken)
	cleanedData, err := compactJSON(responseData)
	if err != nil {
		return fmt.Errorf("compact JSON: %w", err)
	}

	h := sha256.New()
	h.Write(cleanedData)
	h.Write([]byte(session.SecretToken))
	expectedHash := hex.EncodeToString(h.Sum(nil))

	if expectedHash != responseHash {
		slog.Warn("helcim payment hash mismatch",
			"user_id", session.UserID,
			"trip_id", session.TripID,
			"expected", expectedHash,
			"got", responseHash,
		)
		return fmt.Errorf("payment validation failed: hash mismatch")
	}

	// Parse transaction details from response data
	var txn struct {
		TransactionID string `json:"transactionId"`
		ApprovalCode  string `json:"approvalCode"`
		CardToken     string `json:"cardToken"`
		Amount        string `json:"amount"`
		Currency      string `json:"currency"`
		Status        string `json:"status"`
	}
	if err := json.Unmarshal(responseData, &txn); err != nil {
		return fmt.Errorf("parse transaction: %w", err)
	}

	if txn.Status != "APPROVED" {
		return fmt.Errorf("transaction not approved: %s", txn.Status)
	}

	// Check idempotency — don't double-record
	_, err = s.queries.GetPaymentByTransactionID(ctx, txn.TransactionID)
	if err == nil {
		// Already recorded — success (idempotent)
		return nil
	}

	// Record payment
	payment, err := s.queries.CreatePayment(ctx, dbgen.CreatePaymentParams{
		UserID:              session.UserID,
		TripID:              session.TripID,
		HelcimTransactionID: txn.TransactionID,
		ApprovalCode:        textFromStr(txn.ApprovalCode),
		CardToken:           textFromStr(txn.CardToken),
		AmountCents:         session.AmountCents,
		Currency:            session.Currency,
		Status:              "approved",
		ResponseHash:        textFromStr(responseHash),
	})
	if err != nil {
		return fmt.Errorf("record payment: %w", err)
	}

	// Mark checkout session complete
	if err := s.queries.MarkCheckoutSessionComplete(ctx, checkoutToken); err != nil {
		slog.Error("failed to mark checkout session complete", "error", err, "token", checkoutToken)
	}

	// Unlock the trip
	_, err = s.queries.CreateTripUnlock(ctx, dbgen.CreateTripUnlockParams{
		UserID:    session.UserID,
		TripID:    session.TripID,
		PaymentID: pgtype.UUID{Bytes: payment.ID, Valid: true},
		Source:    "purchase",
	})
	if err != nil {
		return fmt.Errorf("create trip unlock: %w", err)
	}

	audit.Log(audit.EventTripProPurchase,
		"user_id", session.UserID.String(),
		"trip_id", session.TripID.String(),
		"amount_cents", session.AmountCents,
		"helcim_txn", txn.TransactionID,
	)

	slog.Info("trip pro purchased",
		"user_id", session.UserID,
		"trip_id", session.TripID,
		"helcim_txn", txn.TransactionID,
		"amount_cents", session.AmountCents,
	)

	return nil
}

// IsTripUnlocked checks if a user has access to Trip Pro features for a given trip.
func (s *Service) IsTripUnlocked(ctx context.Context, userID, tripID uuid.UUID) (bool, error) {
	return s.queries.IsTripUnlocked(ctx, dbgen.IsTripUnlockedParams{
		UserID: userID,
		TripID: tripID,
	})
}

// PriceCents returns the configured Trip Pro price in cents.
func (s *Service) PriceCents() int {
	return s.priceCents
}

func compactJSON(data json.RawMessage) ([]byte, error) {
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	return json.Marshal(parsed)
}

func textFromStr(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
