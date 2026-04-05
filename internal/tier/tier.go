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

	// Pro is the paid tier (per-trip unlock). Pro users receive unbiased
	// recommendations from any source with no affiliate requirement.
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

// IsPro returns true when the user has at least Pro-level access.
func (t UserTier) IsPro() bool {
	return t == Pro || t == Explorer || t == Voyager
}

// IsUnlimited returns true when the user has unlimited daily messages.
func (t UserTier) IsUnlimited() bool {
	return t == Explorer || t == Voyager
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
