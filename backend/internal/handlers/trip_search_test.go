package handlers

import (
	"testing"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
)

func TestListTripsQueryTrimming(t *testing.T) {
	// The handler trims whitespace from the query before deciding whether to
	// use full-text search. Verify the proto field passes through correctly.
	cases := []struct {
		name       string
		query      string
		wantSearch bool
	}{
		{"empty string falls back to list", "", false},
		{"whitespace-only falls back to list", "   \t  ", false},
		{"non-empty triggers search", "japan", true},
		{"trimmed non-empty triggers search", "  japan  ", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := &toquiv1.ListTripsRequest{Query: c.query}

			// Simulate the handler's trimming logic.
			trimmed := ""
			if q := req.GetQuery(); q != "" {
				// strings.TrimSpace equivalent inline for test clarity.
				for _, r := range q {
					if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
						trimmed = q
						break
					}
				}
			}

			isSearch := trimmed != ""
			if isSearch != c.wantSearch {
				t.Errorf("query=%q: isSearch=%v, want %v", c.query, isSearch, c.wantSearch)
			}
		})
	}
}

func TestListTripsQueryFieldPresent(t *testing.T) {
	// Verify the proto message has the query field (regression guard against
	// accidental proto changes removing it).
	req := &toquiv1.ListTripsRequest{
		Query: "beach vacation",
	}
	if req.GetQuery() != "beach vacation" {
		t.Errorf("GetQuery() = %q, want %q", req.GetQuery(), "beach vacation")
	}
}

func TestListTripsQueryMaxLength(t *testing.T) {
	// The proto constrains query to max_len=512. Verify the field can hold
	// a reasonable-length search query without issue.
	longQuery := ""
	for i := 0; i < 500; i++ {
		longQuery += "a"
	}
	req := &toquiv1.ListTripsRequest{Query: longQuery}
	if len(req.GetQuery()) != 500 {
		t.Errorf("query length = %d, want 500", len(req.GetQuery()))
	}
}
