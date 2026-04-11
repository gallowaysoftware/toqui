package handlers

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gallowaysoftware/toqui-backend/internal/analytics"
)

// AffiliateHandler handles affiliate link click tracking endpoints.
type AffiliateHandler struct {
	analyticsClient *analytics.Client
}

// NewAffiliateHandler creates a new AffiliateHandler.
func NewAffiliateHandler(analyticsClient *analytics.Client) *AffiliateHandler {
	return &AffiliateHandler{
		analyticsClient: analyticsClient,
	}
}

// HandleClick handles GET /api/affiliate/click.
// Logs an affiliate_link_clicked event to PostHog and redirects (302) to the
// decoded affiliate URL. Query parameters:
//   - url: the encoded affiliate URL to redirect to (required)
//   - partner: the affiliate partner name, e.g. "skyscanner" (optional, for tracking)
//   - category: the booking category, e.g. "flight" (optional, for tracking)
//
// The user is tracked anonymously — no auth is required and no PII is collected.
func (h *AffiliateHandler) HandleClick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		http.Error(w, "missing required 'url' parameter", http.StatusBadRequest)
		return
	}

	// Validate that the URL is well-formed to prevent open redirect attacks.
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		http.Error(w, "invalid url parameter", http.StatusBadRequest)
		return
	}

	// Only allow HTTPS redirects (or HTTP for localhost in development).
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		http.Error(w, "invalid url scheme", http.StatusBadRequest)
		return
	}

	// Restrict redirects to known affiliate partner domains to prevent
	// abuse as an open redirector.
	if !isAllowedAffiliateDomain(parsed.Host) {
		slog.Warn("affiliate click: blocked redirect to unknown domain",
			"host", parsed.Host,
			"url", rawURL,
		)
		http.Error(w, "redirect domain not allowed", http.StatusBadRequest)
		return
	}

	partner := r.URL.Query().Get("partner")
	category := r.URL.Query().Get("category")

	// Track click event (async, non-blocking). Uses "anonymous" as the
	// distinct_id since this endpoint does not require authentication.
	if h.analyticsClient != nil {
		props := map[string]any{}
		if partner != "" {
			props["partner"] = partner
		}
		if category != "" {
			props["category"] = category
		}
		h.analyticsClient.Track("anonymous", "affiliate_link_clicked", props)
	}

	slog.Debug("affiliate click redirect",
		"partner", partner,
		"category", category,
		"target", rawURL,
	)

	http.Redirect(w, r, rawURL, http.StatusFound)
}

// allowedAffiliateDomains is the set of domains we allow redirects to.
// This prevents the click endpoint from being abused as an open redirector.
var allowedAffiliateDomains = map[string]bool{
	"www.skyscanner.com":   true,
	"skyscanner.com":       true,
	"www.booking.com":      true,
	"booking.com":          true,
	"www.getyourguide.com": true,
	"getyourguide.com":     true,
	"www.viator.com":       true,
	"viator.com":           true,
	"www.discovercars.com": true,
	"discovercars.com":     true,
	"safetywing.com":       true,
	"www.safetywing.com":   true,
}

// isAllowedAffiliateDomain checks whether a host is in the allowlist.
func isAllowedAffiliateDomain(host string) bool {
	return allowedAffiliateDomains[host]
}
