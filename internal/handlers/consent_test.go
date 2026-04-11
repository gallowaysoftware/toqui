package handlers

import (
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
