// Package tier defines user subscription tiers and their capabilities.
//
// Tier hierarchy: Free < Pro < Explorer < Voyager.
// Explorer and Voyager are subscription tiers with unlimited messages.
package tier

// UserTier represents a user's subscription level.
type UserTier string

const (
	// Free is the default tier for all users. Booking recommendations are
	// affiliate-only; the FTC disclosure is included with every link.
	Free UserTier = "free"

	// Pro is the paid tier (per-trip unlock). Pro users get unlimited
	// expert handoffs, PDF/calendar export, and extended message limits.
	// Booking recommendations prefer commission-free sources: the
	// recommend_booking tool picks an independent source (Google Flights,
	// Google Maps, Wikivoyage, Google search) over an affiliate partner
	// whenever one is available. Today every category exposes at least one
	// independent candidate, so Pro users see IndependentDisclosure on
	// every recommendation. The affiliate package's SelectForPreference
	// also handles the defensive case where a category has no independent
	// candidate by falling back to the affiliate partner with an honest
	// "partner link" disclosure.
	Pro UserTier = "pro"

	// Explorer is a subscription tier with expanded limits.
	Explorer UserTier = "explorer"

	// Voyager is the highest subscription tier with unlimited access.
	Voyager UserTier = "voyager"
)

// Valid returns true if t is a recognised tier value.
func (t UserTier) Valid() bool {
	switch t {
	case Free, Pro, Explorer, Voyager:
		return true
	default:
		return false
	}
}

// IsFree returns true when the user is on the default free tier.
func (t UserTier) IsFree() bool {
	return t == Free
}

// IsPro returns true when the user has at least Pro-level access.
func (t UserTier) IsPro() bool {
	return t == Pro || t == Explorer || t == Voyager
}

// IsUnlimited returns true when the user has unlimited daily messages.
func (t UserTier) IsUnlimited() bool {
	return t == Explorer || t == Voyager
}

// HasPriorityModel returns true when the user gets the "best" AI model tier
// instead of "smart". Currently only Voyager subscribers receive this.
func (t UserTier) HasPriorityModel() bool {
	return t == Voyager
}

// HasExportAccess returns true when the user can export trips (PDF/iCal).
// Available to Explorer and Voyager subscribers.
func (t UserTier) HasExportAccess() bool {
	return t == Explorer || t == Voyager
}

// HasPrioritySupport returns true when the user has access to priority
// customer support. Currently only Voyager subscribers receive this.
func (t UserTier) HasPrioritySupport() bool {
	return t == Voyager
}

// Features returns the list of feature names enabled for this tier.
// Used by the subscription status endpoint so the frontend can show what
// each tier includes.
func (t UserTier) Features() []string {
	var features []string
	if t.IsPro() {
		features = append(features, "unlimited_experts", "booking_parsing")
	}
	if t.IsUnlimited() {
		features = append(features, "unlimited_messages")
	}
	if t.HasExportAccess() {
		features = append(features, "export_pdf_ical")
	}
	if t.HasPriorityModel() {
		features = append(features, "priority_ai_model")
	}
	if t.HasPrioritySupport() {
		features = append(features, "priority_support")
	}
	return features
}

// Parse converts a raw string to a UserTier. Unrecognised values fall
// back to Free so callers never have to handle an error path.
func Parse(raw string) UserTier {
	t := UserTier(raw)
	if t.Valid() {
		return t
	}
	return Free
}
