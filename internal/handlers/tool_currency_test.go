package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCurrencyTool_Definition pins the tool name + parameter schema. The
// AI layer dispatches by name, so any rename would silently break every
// "convert this in USD" prompt. The required-fields list pins the
// JSON-Schema validation expected by the tool-loop runtime.
func TestCurrencyTool_Definition(t *testing.T) {
	def := NewCurrencyTool().Definition()

	if def.Name != "currency_convert" {
		t.Errorf("Name = %q, want currency_convert", def.Name)
	}
	if def.Description == "" {
		t.Error("Description must be non-empty (used by the AI to decide when to call this tool)")
	}

	var schema struct {
		Type       string         `json:"type"`
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
	}
	if err := json.Unmarshal(def.Parameters, &schema); err != nil {
		t.Fatalf("Parameters must be a valid JSON Schema: %v", err)
	}
	for _, want := range []string{"amount", "from", "to"} {
		if _, ok := schema.Properties[want]; !ok {
			t.Errorf("schema missing required property %q", want)
		}
	}
	if len(schema.Required) != 3 {
		t.Errorf("Required = %v, want exactly [amount, from, to]", schema.Required)
	}
}

// TestCurrencyTool_HappyPath_ConvertsAmountAndShape pins the success
// path: the tool issues a single GET to /latest with normalized
// (uppercased, trimmed) currency codes, parses the upstream response,
// and returns a structured payload with both raw numbers and pre-formatted
// strings. The formatted strings are what the AI surfaces to the user, so
// they must stay stable.
func TestCurrencyTool_HappyPath_ConvertsAmountAndShape(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"amount":500.00,"base":"THB","date":"2026-04-29","rates":{"USD":14.75}}`))
	}))
	defer srv.Close()

	tool := NewCurrencyTool().WithBaseURL(srv.URL)

	args, _ := json.Marshal(map[string]any{"amount": 500.0, "from": "  thb  ", "to": "usd"})
	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute errored on a happy-path call: %v", err)
	}

	// Currency codes uppercased + trimmed before being sent upstream — pins
	// the normalization since lowercase codes are a real user pattern in chat.
	if !strings.Contains(capturedQuery, "from=THB") || !strings.Contains(capturedQuery, "to=USD") {
		t.Errorf("upstream query = %q, expected normalized THB→USD codes", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "amount=500.00") {
		t.Errorf("upstream query = %q, expected amount=500.00", capturedQuery)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("Execute output is not valid JSON: %v", err)
	}
	if got["from"] != "THB" || got["to"] != "USD" {
		t.Errorf("from/to = %v/%v, want THB/USD", got["from"], got["to"])
	}
	if got["converted"] != 14.75 {
		t.Errorf("converted = %v, want 14.75", got["converted"])
	}
	// formatted strings are user-facing — must include both currency codes.
	if !strings.Contains(got["formatted"].(string), "THB") || !strings.Contains(got["formatted"].(string), "USD") {
		t.Errorf("formatted = %q, expected to include both currency codes", got["formatted"])
	}
	if !strings.Contains(got["rate_formatted"].(string), "1 THB =") {
		t.Errorf("rate_formatted = %q, expected '1 THB = ...' shape", got["rate_formatted"])
	}
}

// TestCurrencyTool_MissingCurrency_ReturnsStructuredError covers the
// pre-flight validation that the AI's malformed args don't reach the
// upstream API. The structured-error shape (NOT a Go error) lets the AI
// see the failure and adjust without retrying — see the chat-loop
// handler for why error JSON beats `error` returns.
func TestCurrencyTool_MissingCurrency_ReturnsStructuredError(t *testing.T) {
	tool := NewCurrencyTool().WithBaseURL("https://example.invalid")

	args, _ := json.Marshal(map[string]any{"amount": 100.0, "from": "", "to": "USD"})
	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute should not return an error for missing currency, got: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if got["error"] != "missing_currency" {
		t.Errorf("error = %q, want missing_currency", got["error"])
	}
}

// TestCurrencyTool_NegativeOrZeroAmount_DefaultsToOne pins the
// "ask the rate" use case. When the AI calls the tool to ASK what 1 unit
// is worth (rather than convert a specific amount), it may pass 0 or a
// negative value. The tool defaults to 1 so the rate lookup still works
// rather than returning a confusing "0 THB = 0 USD".
func TestCurrencyTool_NegativeOrZeroAmount_DefaultsToOne(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"amount":1.00,"base":"EUR","date":"2026-04-29","rates":{"GBP":0.85}}`))
	}))
	defer srv.Close()

	tool := NewCurrencyTool().WithBaseURL(srv.URL)

	for _, amt := range []float64{0, -50} {
		args, _ := json.Marshal(map[string]any{"amount": amt, "from": "EUR", "to": "GBP"})
		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("Execute(amount=%v) errored: %v", amt, err)
		}
		if !strings.Contains(capturedQuery, "amount=1.00") {
			t.Errorf("amount=%v should default to 1.00, query was %q", amt, capturedQuery)
		}
	}
}

// TestCurrencyTool_UpstreamHTTPError_ReturnsStructuredError covers the
// case where Frankfurter responds with non-200 (e.g. 400 on an unknown
// currency code, 500 on their side). The tool MUST NOT propagate a Go
// error here — instead it returns a structured-error JSON so the AI can
// recover gracefully ("I couldn't fetch the rate, can you double-check
// the currency code?").
func TestCurrencyTool_UpstreamHTTPError_ReturnsStructuredError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	tool := NewCurrencyTool().WithBaseURL(srv.URL)
	args, _ := json.Marshal(map[string]any{"amount": 100.0, "from": "USD", "to": "ZZZ"})

	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute should not return Go error on upstream HTTP failure: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if got["error"] != "exchange_rate_error" {
		t.Errorf("error = %q, want exchange_rate_error", got["error"])
	}
	if !strings.Contains(got["message"], "400") {
		t.Errorf("message = %q, expected to mention status 400", got["message"])
	}
}

// TestCurrencyTool_TargetCurrencyNotInResponse covers the API-contract
// edge case where Frankfurter accepts the request but returns a rates
// map that doesn't contain the target currency (e.g. legitimate
// happens for unsupported pairs). Pins the structured-error shape so
// the AI can say "I couldn't find a rate for that pair."
func TestCurrencyTool_TargetCurrencyNotInResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"amount":100.00,"base":"USD","date":"2026-04-29","rates":{"EUR":92}}`))
	}))
	defer srv.Close()

	tool := NewCurrencyTool().WithBaseURL(srv.URL)
	args, _ := json.Marshal(map[string]any{"amount": 100.0, "from": "USD", "to": "GBP"})

	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute errored: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if got["error"] != "currency_not_found" {
		t.Errorf("error = %q, want currency_not_found", got["error"])
	}
}

// TestCurrencyTool_MalformedArgsReturnsGoError covers the contract
// boundary: when the AI sends args that aren't even valid JSON for the
// schema (a programming error, not a user error), the tool returns a
// Go error rather than a structured-error JSON. The chat handler logs
// these as warnings and surfaces a generic message to the user.
func TestCurrencyTool_MalformedArgsReturnsGoError(t *testing.T) {
	tool := NewCurrencyTool()

	out, err := tool.Execute(context.Background(), json.RawMessage(`{not json}`))
	if err == nil {
		t.Errorf("expected Go error for malformed args, got output %s", string(out))
	}
}
