package tier

import "testing"

func TestUserTier_Valid(t *testing.T) {
	tests := []struct {
		tier UserTier
		want bool
	}{
		{Free, true},
		{Pro, true},
		{UserTier(""), false},
		{UserTier("enterprise"), false},
		{UserTier("FREE"), false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := tt.tier.Valid(); got != tt.want {
				t.Errorf("UserTier(%q).Valid() = %v, want %v", tt.tier, got, tt.want)
			}
		})
	}
}

func TestUserTier_IsPro(t *testing.T) {
	tests := []struct {
		tier UserTier
		want bool
	}{
		{Free, false},
		{Pro, true},
		{UserTier(""), false},
		{UserTier("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := tt.tier.IsPro(); got != tt.want {
				t.Errorf("UserTier(%q).IsPro() = %v, want %v", tt.tier, got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		input string
		want  UserTier
	}{
		{"free", Free},
		{"pro", Pro},
		{"", Free},        // empty defaults to free
		{"premium", Free}, // unknown defaults to free
		{"FREE", Free},    // wrong case defaults to free
		{"Pro", Free},     // wrong case defaults to free
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := Parse(tt.input); got != tt.want {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
