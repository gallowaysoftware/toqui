package handlers

import "testing"

func TestIsEmailDomainAllowed(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		domains  []string
		expected bool
	}{
		{"empty allowlist allows all", "user@gmail.com", nil, true},
		{"allowed domain", "user@toqui.travel", []string{"toqui.travel"}, true},
		{"disallowed domain", "user@gmail.com", []string{"toqui.travel"}, false},
		{"case insensitive", "user@TOQUI.TRAVEL", []string{"toqui.travel"}, true},
		{"multiple allowed", "user@thegalloways.ca", []string{"toqui.travel", "thegalloways.ca"}, true},
		{"multiple disallowed", "user@gmail.com", []string{"toqui.travel", "thegalloways.ca"}, false},
		{"no @ sign", "invalidemail", []string{"toqui.travel"}, false},
		{"empty email", "", []string{"toqui.travel"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEmailDomainAllowed(tt.email, tt.domains)
			if got != tt.expected {
				t.Errorf("isEmailDomainAllowed(%q, %v) = %v, want %v",
					tt.email, tt.domains, got, tt.expected)
			}
		})
	}
}
