package handlers

import (
	"encoding/json"
	"testing"
)

func TestValidConsentTypes(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantOK bool
	}{
		{"terms is valid", "terms", true},
		{"privacy_policy is valid", "privacy_policy", true},
		{"analytics is valid", "analytics", true},
		{"empty string is invalid", "", false},
		{"unknown type is invalid", "marketing", false},
		{"sql injection attempt is invalid", "'; DROP TABLE users;--", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validConsentTypes[tt.input]
			if got != tt.wantOK {
				t.Errorf("validConsentTypes[%q] = %v, want %v", tt.input, got, tt.wantOK)
			}
		})
	}
}

func TestConsentTypeExtraction(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantType string
		wantOK   bool
	}{
		{"terms", "/api/privacy/consents/terms", "terms", true},
		{"privacy_policy", "/api/privacy/consents/privacy_policy", "privacy_policy", true},
		{"analytics", "/api/privacy/consents/analytics", "analytics", true},
		{"empty type", "/api/privacy/consents/", "", false},
		{"no trailing path", "/api/privacy/consents", "", false},
		{"invalid type", "/api/privacy/consents/marketing", "marketing", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const prefix = "/api/privacy/consents/"
			got := ""
			if len(tt.path) > len(prefix) {
				got = tt.path[len(prefix):]
			}
			gotOK := got != "" && validConsentTypes[got]
			if got != tt.wantType || gotOK != tt.wantOK {
				t.Errorf("path=%q: got type=%q ok=%v, want type=%q ok=%v",
					tt.path, got, gotOK, tt.wantType, tt.wantOK)
			}
		})
	}
}

func TestBatchConsentRequestJSON(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantTerms     bool
		wantPrivacy   bool
		wantMarketing bool
		wantParseErr  bool
	}{
		{
			name:          "all accepted",
			input:         `{"terms_accepted":true,"privacy_accepted":true,"marketing_opt_in":true}`,
			wantTerms:     true,
			wantPrivacy:   true,
			wantMarketing: true,
		},
		{
			name:        "required only",
			input:       `{"terms_accepted":true,"privacy_accepted":true}`,
			wantTerms:   true,
			wantPrivacy: true,
		},
		{
			name:        "marketing opt out",
			input:       `{"terms_accepted":true,"privacy_accepted":true,"marketing_opt_in":false}`,
			wantTerms:   true,
			wantPrivacy: true,
		},
		{
			name:        "terms missing",
			input:       `{"privacy_accepted":true}`,
			wantPrivacy: true,
		},
		{
			name:  "empty object",
			input: `{}`,
		},
		{
			name:         "invalid JSON",
			input:        `not json`,
			wantParseErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req batchConsentRequest
			err := json.Unmarshal([]byte(tt.input), &req)
			if tt.wantParseErr {
				if err == nil {
					t.Error("expected parse error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			if req.TermsAccepted != tt.wantTerms {
				t.Errorf("TermsAccepted = %v, want %v", req.TermsAccepted, tt.wantTerms)
			}
			if req.PrivacyAccepted != tt.wantPrivacy {
				t.Errorf("PrivacyAccepted = %v, want %v", req.PrivacyAccepted, tt.wantPrivacy)
			}
			if req.MarketingOptIn != tt.wantMarketing {
				t.Errorf("MarketingOptIn = %v, want %v", req.MarketingOptIn, tt.wantMarketing)
			}
		})
	}
}

func TestBatchConsentResponseJSON(t *testing.T) {
	resp := batchConsentResponse{
		Recorded: []string{"terms", "privacy_policy", "analytics"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded batchConsentResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(decoded.Recorded) != 3 {
		t.Errorf("Recorded length = %d, want 3", len(decoded.Recorded))
	}

	expected := map[string]bool{"terms": true, "privacy_policy": true, "analytics": true}
	for _, r := range decoded.Recorded {
		if !expected[r] {
			t.Errorf("unexpected recorded consent type: %q", r)
		}
	}
}

func TestConsentPendingInOAuthResult(t *testing.T) {
	// Verify consent_pending is properly serialized in oauthResult
	result := oauthResult{
		UserID:         "test-user-id",
		Email:          "test@example.com",
		ConsentPending: true,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	pending, ok := decoded["consent_pending"]
	if !ok {
		t.Fatal("consent_pending field missing from serialized oauthResult")
	}
	if pending != true {
		t.Errorf("consent_pending = %v, want true", pending)
	}

	// Verify omitempty works when consent is not pending
	resultDone := oauthResult{
		UserID: "test-user-id",
		Email:  "test@example.com",
	}

	data2, _ := json.Marshal(resultDone)
	var decoded2 map[string]any
	json.Unmarshal(data2, &decoded2)

	if _, exists := decoded2["consent_pending"]; exists {
		t.Error("consent_pending should be omitted when false (omitempty)")
	}
}

func TestConsentPendingInExchangeResponse(t *testing.T) {
	resp := exchangeResponse{
		UserID:         "test-user-id",
		Email:          "test@example.com",
		ConsentPending: true,
		ExpiresAt:      1700000000,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	pending, ok := decoded["consent_pending"]
	if !ok {
		t.Fatal("consent_pending field missing from serialized exchangeResponse")
	}
	if pending != true {
		t.Errorf("consent_pending = %v, want true", pending)
	}
}
