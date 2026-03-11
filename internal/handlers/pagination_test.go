package handlers

import "testing"

func TestClampPageSize(t *testing.T) {
	tests := []struct {
		name      string
		requested int32
		dflt      int32
		max       int32
		want      int32
	}{
		{"zero defaults", 0, 20, 100, 20},
		{"negative defaults", -5, 20, 100, 20},
		{"within range", 50, 20, 100, 50},
		{"at max", 100, 20, 100, 100},
		{"exceeds max", 500, 20, 100, 100},
		{"one is valid", 1, 20, 100, 1},
		{"equals default", 20, 20, 100, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampPageSize(tt.requested, tt.dflt, tt.max)
			if got != tt.want {
				t.Errorf("clampPageSize(%d, %d, %d) = %d, want %d",
					tt.requested, tt.dflt, tt.max, got, tt.want)
			}
		})
	}
}
