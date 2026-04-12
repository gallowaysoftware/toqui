package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

// DestinationResult represents a single autocomplete result.
type DestinationResult struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

// DestinationSearchHandler serves GET /api/destinations/search?q=...
type DestinationSearchHandler struct {
	entries []destinationEntry
}

type destinationEntry struct {
	name string // lowercase for matching
	disp string // display name (original casing)
	code string // ISO 3166-1 alpha-2
}

// NewDestinationSearchHandler creates a handler with all supported destinations
// and common aliases preloaded for fast prefix matching.
func NewDestinationSearchHandler() *DestinationSearchHandler {
	h := &DestinationSearchHandler{}
	h.entries = buildDestinationEntries()
	return h
}

// HandleSearch handles GET /api/destinations/search?q=japan
func (h *DestinationSearchHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]DestinationResult{})
		return
	}

	lower := strings.ToLower(q)

	var results []DestinationResult
	seen := make(map[string]bool)

	for _, e := range h.entries {
		if strings.HasPrefix(e.name, lower) {
			if !seen[e.code] {
				results = append(results, DestinationResult{Name: e.disp, Code: e.code})
				seen[e.code] = true
			}
			if len(results) >= 10 {
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// buildDestinationEntries returns all supported destinations plus aliases,
// sorted alphabetically by name for consistent prefix matching order.
func buildDestinationEntries() []destinationEntry {
	// All 43 supported locations from the persona system.
	destinations := map[string]string{
		"IT": "Italy",
		"JP": "Japan",
		"FR": "France",
		"GB": "United Kingdom",
		"US": "United States",
		"ES": "Spain",
		"DE": "Germany",
		"PT": "Portugal",
		"GR": "Greece",
		"TH": "Thailand",
		"MX": "Mexico",
		"AU": "Australia",
		"BR": "Brazil",
		"IN": "India",
		"KR": "South Korea",
		"VN": "Vietnam",
		"MA": "Morocco",
		"PE": "Peru",
		"NZ": "New Zealand",
		"TR": "Turkey",
		"HR": "Croatia",
		"ZA": "South Africa",
		"CO": "Colombia",
		"EG": "Egypt",
		"ID": "Indonesia",
		"PH": "Philippines",
		"CN": "China",
		"CZ": "Czech Republic",
		"AT": "Austria",
		"CH": "Switzerland",
		"IE": "Ireland",
		"SE": "Sweden",
		"AR": "Argentina",
		"CL": "Chile",
		"JO": "Jordan",
		"TZ": "Tanzania",
		"IS": "Iceland",
		"SG": "Singapore",
		"HK": "Hong Kong",
		"KH": "Cambodia",
		"TW": "Taiwan",
		"NO": "Norway",
		"LK": "Sri Lanka",
	}

	// Common aliases that map to ISO codes.
	aliases := map[string]string{
		"UK":            "GB",
		"England":       "GB",
		"Scotland":      "GB",
		"Wales":         "GB",
		"Britain":       "GB",
		"Great Britain": "GB",
		"USA":           "US",
		"America":       "US",
		"Korea":         "KR",
		"Turkiye":       "TR",
		"Türkiye":       "TR",
		"Czechia":       "CZ",
		"Bali":          "ID",
		"Holland":       "NL",
		"Netherlands":   "NL",
		"Siam":          "TH",
		"Ceylon":        "LK",
		"Persia":        "IR",
	}

	var entries []destinationEntry

	// Add primary names.
	for code, name := range destinations {
		entries = append(entries, destinationEntry{
			name: strings.ToLower(name),
			disp: name,
			code: code,
		})
	}

	// Add aliases with display name showing the alias.
	for alias, code := range aliases {
		primary, ok := destinations[code]
		if !ok {
			// Alias points to a code not in our destination list; skip.
			continue
		}
		entries = append(entries, destinationEntry{
			name: strings.ToLower(alias),
			disp: primary,
			code: code,
		})
	}

	// Sort alphabetically by name for deterministic prefix matching.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	return entries
}
