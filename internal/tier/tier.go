// Package tier defines user subscription tiers and their capabilities.
//
// Currently all users are on the free tier. The Pro tier infrastructure
// exists so gating logic can be enabled when monetisation launches.
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
)

// Valid returns true if t is a recognised tier value.
func (t UserTier) Valid() bool {
	switch t {
	case Free, Pro:
		return true
	default:
		return false
	}
}

// IsPro returns true when the user has an active Pro subscription.
func (t UserTier) IsPro() bool {
	return t == Pro
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
