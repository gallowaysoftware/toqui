package tier

import "testing"

func TestUserTier_Valid(t *testing.T) {
	tests := []struct {
		tier UserTier
		want bool
	}{
		{Free, true},
		{Pro, true},
		{Explorer, true},
		{Voyager, true},
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
		{Explorer, true},
		{Voyager, true},
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

func TestUserTier_IsUnlimited(t *testing.T) {
	tests := []struct {
		tier UserTier
		want bool
	}{
		{Free, false},
		{Pro, false},
		{Explorer, true},
		{Voyager, true},
		{UserTier(""), false},
		{UserTier("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := tt.tier.IsUnlimited(); got != tt.want {
				t.Errorf("UserTier(%q).IsUnlimited() = %v, want %v", tt.tier, got, tt.want)
			}
		})
	}
}

func TestUserTier_HasPriorityModel(t *testing.T) {
	tests := []struct {
		tier UserTier
		want bool
	}{
		{Free, false},
		{Pro, false},
		{Explorer, false},
		{Voyager, true},
		{UserTier(""), false},
		{UserTier("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := tt.tier.HasPriorityModel(); got != tt.want {
				t.Errorf("UserTier(%q).HasPriorityModel() = %v, want %v", tt.tier, got, tt.want)
			}
		})
	}
}

func TestUserTier_HasExportAccess(t *testing.T) {
	tests := []struct {
		tier UserTier
		want bool
	}{
		{Free, false},
		{Pro, false},
		{Explorer, true},
		{Voyager, true},
		{UserTier(""), false},
		{UserTier("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := tt.tier.HasExportAccess(); got != tt.want {
				t.Errorf("UserTier(%q).HasExportAccess() = %v, want %v", tt.tier, got, tt.want)
			}
		})
	}
}

func TestUserTier_HasPrioritySupport(t *testing.T) {
	tests := []struct {
		tier UserTier
		want bool
	}{
		{Free, false},
		{Pro, false},
		{Explorer, false},
		{Voyager, true},
		{UserTier(""), false},
		{UserTier("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := tt.tier.HasPrioritySupport(); got != tt.want {
				t.Errorf("UserTier(%q).HasPrioritySupport() = %v, want %v", tt.tier, got, tt.want)
			}
		})
	}
}

func TestUserTier_Features(t *testing.T) {
	tests := []struct {
		tier     UserTier
		contains []string
		excludes []string
	}{
		{
			tier:     Free,
			contains: nil,
			excludes: []string{"unlimited_experts", "unlimited_messages", "export_pdf_ical", "priority_ai_model", "priority_support"},
		},
		{
			tier:     Pro,
			contains: []string{"unlimited_experts", "booking_parsing"},
			excludes: []string{"unlimited_messages", "export_pdf_ical", "priority_ai_model", "priority_support"},
		},
		{
			tier:     Explorer,
			contains: []string{"unlimited_experts", "booking_parsing", "unlimited_messages", "export_pdf_ical"},
			excludes: []string{"priority_ai_model", "priority_support"},
		},
		{
			tier:     Voyager,
			contains: []string{"unlimited_experts", "booking_parsing", "unlimited_messages", "export_pdf_ical", "priority_ai_model", "priority_support"},
			excludes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			features := tt.tier.Features()
			featureSet := make(map[string]bool, len(features))
			for _, f := range features {
				featureSet[f] = true
			}

			for _, want := range tt.contains {
				if !featureSet[want] {
					t.Errorf("UserTier(%q).Features() missing %q, got %v", tt.tier, want, features)
				}
			}
			for _, exclude := range tt.excludes {
				if featureSet[exclude] {
					t.Errorf("UserTier(%q).Features() should not contain %q, got %v", tt.tier, exclude, features)
				}
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
		{"explorer", Explorer},
		{"voyager", Voyager},
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
