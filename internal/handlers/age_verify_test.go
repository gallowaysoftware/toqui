package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// --- computeAge ---
//
// The integer-age calculation feeds the under-18 branch in HandleVerifyAge —
// a wrong answer there is a destructive bug (it would either let a 17-year-old
// through, or worse, hard-delete an 18-year-old who entered a DOB the night
// before their birthday). These tests pin the boundary cases I'd otherwise
// keep getting nervous about.

func TestComputeAge_ExactlyEighteenOnTheBirthday(t *testing.T) {
	// Birthday is "today" 18 years ago — counts as 18, NOT 17.
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	dob := time.Date(2008, 5, 4, 0, 0, 0, 0, time.UTC)
	if got := computeAge(dob, now); got != 18 {
		t.Errorf("on-birthday: expected 18, got %d", got)
	}
}

func TestComputeAge_DayBeforeBirthday_StillSeventeen(t *testing.T) {
	// Born May 5, 2008. On May 4, 2026 they are still 17.
	now := time.Date(2026, 5, 4, 23, 59, 59, 0, time.UTC)
	dob := time.Date(2008, 5, 5, 0, 0, 0, 0, time.UTC)
	if got := computeAge(dob, now); got != 17 {
		t.Errorf("day before 18th birthday: expected 17, got %d", got)
	}
}

func TestComputeAge_LaterMonthThanBirthMonth(t *testing.T) {
	// Born March 2008, now December 2025: 17 (had birthday this year).
	now := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	dob := time.Date(2008, 3, 15, 0, 0, 0, 0, time.UTC)
	if got := computeAge(dob, now); got != 17 {
		t.Errorf("expected 17, got %d", got)
	}
}

func TestComputeAge_LeapYearBirthdayOnNonLeapYear(t *testing.T) {
	// Born Feb 29, 2004. On Feb 28, 2026 (non-leap) — they haven't had
	// their birthday yet, so they're 21, not 22. Pinning the leap-year
	// case because the original implementation used YearDay() which is
	// well-known leap-year-fragile.
	now := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	dob := time.Date(2004, 2, 29, 0, 0, 0, 0, time.UTC)
	if got := computeAge(dob, now); got != 21 {
		t.Errorf("leap-year DOB before Feb 29 anniversary: expected 21, got %d", got)
	}

	// On March 1, 2026, they've passed it — 22.
	now2 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if got := computeAge(dob, now2); got != 22 {
		t.Errorf("leap-year DOB after Feb 29 anniversary: expected 22, got %d", got)
	}
}

func TestComputeAge_FarPastDate(t *testing.T) {
	// Sanity — large age. Used by the >150 reject in HandleVerifyAge.
	now := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	dob := time.Date(1800, 5, 4, 0, 0, 0, 0, time.UTC)
	if got := computeAge(dob, now); got != 226 {
		t.Errorf("expected 226, got %d", got)
	}
}

// --- sha256OfEmail ---
//
// The hash is the join key between the verify-age refusal write and the
// OAuth login pre-check (PR 2 of this stack). If the two sides normalise
// emails differently, a refused user could re-create their account by
// signing in with a different casing or whitespace variant.

func TestSha256OfEmail_NormalizesCaseAndWhitespace(t *testing.T) {
	want := sha256OfEmail("alice@example.com")

	// Same email with different casing/whitespace must hash identically.
	for _, variant := range []string{
		"ALICE@EXAMPLE.COM",
		"  alice@example.com  ",
		"Alice@Example.com",
		"\talice@example.com\n",
	} {
		if got := sha256OfEmail(variant); got != want {
			t.Errorf("variant %q produced different hash than canonical: got %s, want %s", variant, got, want)
		}
	}
}

func TestSha256OfEmail_DistinctEmailsDistinctHashes(t *testing.T) {
	a := sha256OfEmail("alice@example.com")
	b := sha256OfEmail("bob@example.com")
	if a == b {
		t.Error("distinct emails must produce distinct hashes")
	}
}

func TestSha256OfEmail_HexLengthIs64(t *testing.T) {
	// The under_age_blocks.email_sha256 column is CHAR(64). Pin this so
	// a future change to the hash format (e.g. switching to base64) won't
	// silently break the column-width constraint.
	got := sha256OfEmail("anything@example.com")
	if len(got) != 64 {
		t.Errorf("expected 64-char hex digest, got %d (%q)", len(got), got)
	}
	// Hex digits only.
	for _, c := range got {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("non-hex character %q in digest %q", c, got)
			break
		}
	}
}

// --- oauthProviderForUser ---

func TestOauthProviderForUser_Google(t *testing.T) {
	u := dbgen.User{GoogleID: pgtype.Text{String: "g_123", Valid: true}}
	if got := oauthProviderForUser(u); got != "google" {
		t.Errorf("expected google, got %q", got)
	}
}

func TestOauthProviderForUser_Facebook(t *testing.T) {
	u := dbgen.User{FacebookID: pgtype.Text{String: "fb_123", Valid: true}}
	if got := oauthProviderForUser(u); got != "facebook" {
		t.Errorf("expected facebook, got %q", got)
	}
}

func TestOauthProviderForUser_Apple(t *testing.T) {
	u := dbgen.User{AppleSub: pgtype.Text{String: "ap_123", Valid: true}}
	if got := oauthProviderForUser(u); got != "apple" {
		t.Errorf("expected apple, got %q", got)
	}
}

func TestOauthProviderForUser_Unknown(t *testing.T) {
	// All three columns null/empty — shouldn't happen in production
	// (every user has at least one OAuth identity at insert time) but
	// the audit value for this case is "unknown" rather than the empty
	// string, so the audit log row is still self-describing.
	u := dbgen.User{}
	if got := oauthProviderForUser(u); got != "unknown" {
		t.Errorf("expected unknown, got %q", got)
	}
}

func TestOauthProviderForUser_PrefersGoogleWhenMultipleSet(t *testing.T) {
	// A user may have linked multiple identities (e.g. signed up with
	// Google, later linked Facebook). The audit tag should reflect the
	// "primary" identity in a stable order. We pick google > facebook >
	// apple — alphabetical isn't meaningful here; this just pins the
	// current behaviour so a future refactor of the switch doesn't
	// silently flip the precedence.
	u := dbgen.User{
		GoogleID:   pgtype.Text{String: "g", Valid: true},
		FacebookID: pgtype.Text{String: "fb", Valid: true},
		AppleSub:   pgtype.Text{String: "ap", Valid: true},
	}
	if got := oauthProviderForUser(u); got != "google" {
		t.Errorf("expected google to win the precedence tie, got %q", got)
	}
}

// --- errUnderAge constant ---
//
// The frontend matches against the literal string "under_age" to drive
// the deletion-confirmation screen. Pinning the constant value (not the
// identifier) so a typo'd refactor is caught here.

func TestErrUnderAgeConstantIsStable(t *testing.T) {
	if errUnderAge != "under_age" {
		t.Errorf("errUnderAge contract changed: %q (frontend matches the literal)", errUnderAge)
	}
}

func TestUnderAgeResponseBodyShape(t *testing.T) {
	// Smoke test on the JSON marshalling of the 403 body. The frontend
	// matches on this wire shape — pin both keys here so a struct-tag
	// rename catches before deploy.
	body := verifyAgeUnderAgeResponse{
		Error:   errUnderAge,
		Message: "test",
	}
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, `"error":"under_age"`) {
		t.Errorf("expected error field, got %s", got)
	}
	if !strings.Contains(got, `"message":"test"`) {
		t.Errorf("expected message field, got %s", got)
	}
}
