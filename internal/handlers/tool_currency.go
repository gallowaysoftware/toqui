package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
)

// CurrencyTool provides real-time exchange rate lookups.
// Uses the free exchangerate.host API (no API key required for basic usage).
type CurrencyTool struct {
	client  *http.Client
	baseURL string
}

type currencyArgs struct {
	Amount float64 `json:"amount"`
	From   string  `json:"from"`
	To     string  `json:"to"`
}

// frankfurterDefaultBaseURL is the default upstream base URL. Extracted as a
// constant (rather than inlined into Execute) so tests can swap it out via
// WithBaseURL — Frankfurter doesn't have a sandbox endpoint and we don't want
// CI to hit the live API on every run.
const frankfurterDefaultBaseURL = "https://api.frankfurter.app"

func NewCurrencyTool() *CurrencyTool {
	return &CurrencyTool{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: frankfurterDefaultBaseURL,
	}
}

// WithBaseURL overrides the upstream exchange-rate API base URL. Used by
// tests to redirect requests to an httptest.Server. Returning *CurrencyTool
// preserves chainability for any future option setters.
func (t *CurrencyTool) WithBaseURL(u string) *CurrencyTool {
	t.baseURL = u
	return t
}

func (t *CurrencyTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "currency_convert",
		Description: "Convert an amount between currencies using live exchange rates. Use when the user asks about exchange rates, currency conversion, or how much something costs in their home currency.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"amount": {
					"type": "number",
					"description": "Amount to convert (e.g., 500)"
				},
				"from": {
					"type": "string",
					"description": "Source currency code (e.g., 'THB', 'EUR', 'JPY')"
				},
				"to": {
					"type": "string",
					"description": "Target currency code (e.g., 'USD', 'CAD', 'GBP')"
				}
			},
			"required": ["amount", "from", "to"]
		}`),
	}
}

func (t *CurrencyTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params currencyArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	from := strings.ToUpper(strings.TrimSpace(params.From))
	to := strings.ToUpper(strings.TrimSpace(params.To))

	if from == "" || to == "" {
		return json.Marshal(map[string]string{
			"error":   "missing_currency",
			"message": "Both 'from' and 'to' currency codes are required.",
		})
	}
	if params.Amount <= 0 {
		params.Amount = 1
	}

	// Use Open Exchange Rates alternative — frankfurter.app (free, no key).
	u := fmt.Sprintf("%s/latest?amount=%.2f&from=%s&to=%s", t.baseURL, params.Amount, from, to)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create currency request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return json.Marshal(map[string]string{
			"error":   "exchange_rate_unavailable",
			"message": fmt.Sprintf("Exchange rate API error: %v", err),
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil {
		return nil, fmt.Errorf("read currency response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return json.Marshal(map[string]string{
			"error":   "exchange_rate_error",
			"message": fmt.Sprintf("Exchange rate API returned status %d: %s", resp.StatusCode, string(body)),
		})
	}

	var fxResp struct {
		Amount float64            `json:"amount"`
		Base   string             `json:"base"`
		Date   string             `json:"date"`
		Rates  map[string]float64 `json:"rates"`
	}
	if err := json.Unmarshal(body, &fxResp); err != nil {
		return nil, fmt.Errorf("parse currency response: %w", err)
	}

	converted, ok := fxResp.Rates[to]
	if !ok {
		return json.Marshal(map[string]string{
			"error":   "currency_not_found",
			"message": fmt.Sprintf("Currency '%s' not found in exchange rate data.", to),
		})
	}

	rate := converted / params.Amount
	if params.Amount == 0 {
		rate = 0
	}

	return json.Marshal(map[string]any{
		"amount":         params.Amount,
		"from":           from,
		"to":             to,
		"converted":      converted,
		"rate":           rate,
		"date":           fxResp.Date,
		"formatted":      fmt.Sprintf("%.2f %s = %.2f %s", params.Amount, from, converted, to),
		"rate_formatted": fmt.Sprintf("1 %s = %.4f %s", from, rate, to),
	})
}
