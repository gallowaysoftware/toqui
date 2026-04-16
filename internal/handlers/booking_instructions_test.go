package handlers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

func TestBookingInstructionsForTier_Free(t *testing.T) {
	instructions := bookingInstructionsForTier(tier.Free)

	if !strings.Contains(instructions, "recommend_booking") {
		t.Errorf("free tier instructions should mention the recommend_booking tool, got %q", instructions)
	}
	if !strings.Contains(instructions, "disclosure") {
		t.Errorf("free tier instructions should mention disclosure requirement, got %q", instructions)
	}
	if !strings.Contains(instructions, "legal requirement") {
		t.Errorf("free tier instructions should mention legal requirement, got %q", instructions)
	}
	if strings.Contains(instructions, "regardless of affiliate") {
		t.Errorf("free tier instructions should not mention ignoring affiliate partnerships, got %q", instructions)
	}
}

func TestBookingInstructionsForTier_Pro(t *testing.T) {
	// Pro tier now diverges from free: the tool prefers non-affiliate
	// sources and the system prompt tells the AI so. The disclosure rule
	// stays — for insurance and other affiliate-fallback categories the
	// disclosure is still mandatory, and for independent sources the AI
	// must include the IndependentDisclosure verbatim.
	proInstructions := bookingInstructionsForTier(tier.Pro)
	freeInstructions := bookingInstructionsForTier(tier.Free)

	if proInstructions == freeInstructions {
		t.Errorf("pro tier instructions should diverge from free tier now that tier-weighted ranking ships")
	}
	if !strings.Contains(proInstructions, "recommend_booking") {
		t.Errorf("pro tier instructions should mention the recommend_booking tool, got %q", proInstructions)
	}
	if !strings.Contains(proInstructions, "prefers independent sources") {
		t.Errorf("pro tier instructions should describe behaviour as 'prefers independent sources', got %q", proInstructions)
	}
	// "ranks by fit" is a quality claim the tool does not deliver — it
	// picks the first non-affiliate candidate from a hand-curated list, no
	// scoring is involved. PR #331 stripped this exact phrasing because the
	// code didn't back it up; do not let it creep back in.
	if strings.Contains(proInstructions, "ranks") || strings.Contains(proInstructions, "ranked by fit") {
		t.Errorf("pro tier instructions must not claim ranking-by-fit (the tool does not score sources), got %q", proInstructions)
	}
	if !strings.Contains(proInstructions, "disclosure") {
		t.Errorf("pro tier instructions must still mention disclosure requirement, got %q", proInstructions)
	}
	if !strings.Contains(proInstructions, "legal requirement") {
		t.Errorf("pro tier instructions must mention legal requirement for disclosure, got %q", proInstructions)
	}
	if strings.Contains(proInstructions, "regardless of affiliate") {
		t.Errorf("pro tier instructions must not use the old 'regardless of affiliate' framing, got %q", proInstructions)
	}
}

// TestBookingInstructionsForTier_Explorer_Voyager_PreferIndependent verifies
// that the higher subscription tiers also get the Pro-style framing — they
// inherit IsPro() == true, so the system prompt should match Pro's.
func TestBookingInstructionsForTier_Explorer_Voyager_PreferIndependent(t *testing.T) {
	proInstructions := bookingInstructionsForTier(tier.Pro)
	for _, ut := range []tier.UserTier{tier.Explorer, tier.Voyager} {
		got := bookingInstructionsForTier(ut)
		if got != proInstructions {
			t.Errorf("%s tier instructions should match Pro (both prefer independent sources), got divergent text", ut)
		}
	}
}

func TestBuildTripContext_IncludesBookingInstructions(t *testing.T) {
	ctx := buildTripContext("Japan Trip", "Two weeks in Japan", "JP", nil, "", "", "planning", []string{"food", "culture"}, nil, nil, 0, tier.Free, nil, "", false)

	if !strings.Contains(ctx, "BOOKING RECOMMENDATIONS") {
		t.Error("trip context should include booking recommendations section")
	}
	if !strings.Contains(ctx, "disclosure") {
		t.Error("free tier trip context should mention disclosure requirement")
	}
}

func TestBuildTripContext_ProTier(t *testing.T) {
	ctx := buildTripContext("Japan Trip", "Two weeks in Japan", "JP", nil, "", "", "planning", []string{"food", "culture"}, nil, nil, 0, tier.Pro, nil, "", false)

	if !strings.Contains(ctx, "BOOKING RECOMMENDATIONS") {
		t.Error("trip context should include booking recommendations section")
	}
	// Pro context describes the actual behaviour: prefers independent sources.
	if !strings.Contains(ctx, "prefers independent sources") {
		t.Error("pro tier trip context should describe behaviour as 'prefers independent sources'")
	}
	if strings.Contains(ctx, "ranks") || strings.Contains(ctx, "ranked by fit") {
		t.Error("pro tier trip context must not claim ranking-by-fit (the tool does not score sources)")
	}
	if !strings.Contains(ctx, "disclosure") {
		t.Error("pro tier trip context should still mention disclosure requirement (affiliate fallback exists)")
	}
}

func TestBuildTripContext_Empty_ReturnsEmpty(t *testing.T) {
	ctx := buildTripContext("", "", "", nil, "", "", "", nil, nil, nil, 0, tier.Free, nil, "", false)
	if ctx != "" {
		t.Errorf("expected empty string for empty trip context, got %q", ctx)
	}
}

func TestBuildTripContext_IncludesItinerary(t *testing.T) {
	items := []dbgen.ItineraryItem{
		{
			DayNumber: pgtype.Int4{Int32: 1, Valid: true},
			Title:     pgtype.Text{String: "Visit Temple", Valid: true},
		},
		{
			DayNumber: pgtype.Int4{Int32: 1, Valid: true},
			Title:     pgtype.Text{String: "Lunch at Ramen Shop", Valid: true},
		},
		{
			DayNumber: pgtype.Int4{Int32: 2, Valid: true},
			Title:     pgtype.Text{String: "Mount Fuji Day Trip", Valid: true},
		},
	}
	ctx := buildTripContext("Japan Trip", "", "JP", nil, "", "", "planning", nil, items, nil, 0, tier.Free, nil, "", false)

	if !strings.Contains(ctx, "Existing itinerary") {
		t.Error("trip context should include itinerary section")
	}
	if !strings.Contains(ctx, "Visit Temple") {
		t.Error("trip context should include itinerary item titles")
	}
	if !strings.Contains(ctx, "Day 1:") {
		t.Error("trip context should group items by day")
	}
	if !strings.Contains(ctx, "3 items") {
		t.Error("trip context should show item count")
	}
}

func TestBuildTripContext_IncludesBookings(t *testing.T) {
	bookings := []dbgen.Booking{
		{
			Type:      "flight",
			Title:     "NYC to Tokyo",
			StartTime: pgtype.Timestamptz{Time: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), Valid: true},
		},
		{
			Type:      "hotel",
			Title:     "Park Hyatt Tokyo",
			StartTime: pgtype.Timestamptz{Time: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), Valid: true},
			EndTime:   pgtype.Timestamptz{Time: time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC), Valid: true},
		},
	}
	ctx := buildTripContext("Japan Trip", "", "JP", nil, "", "", "planning", nil, nil, bookings, 0, tier.Free, nil, "", false)

	if !strings.Contains(ctx, "Existing bookings") {
		t.Error("trip context should include bookings section")
	}
	if !strings.Contains(ctx, "NYC to Tokyo") {
		t.Error("trip context should include booking titles")
	}
	if !strings.Contains(ctx, "flight") {
		t.Error("trip context should include booking types")
	}
}

func TestBuildTripContext_IncludesCollaborators(t *testing.T) {
	ctx := buildTripContext("Japan Trip", "", "JP", nil, "", "", "planning", nil, nil, nil, 3, tier.Free, nil, "", false)

	if !strings.Contains(ctx, "4 people") {
		t.Error("trip context should show collaborator count (+1 for owner)")
	}
}

func TestBuildTripContext_IncludesStatus(t *testing.T) {
	ctx := buildTripContext("Japan Trip", "", "JP", nil, "", "", "active", nil, nil, nil, 0, tier.Free, nil, "", false)

	if !strings.Contains(ctx, "(active)") {
		t.Error("trip context should include trip status")
	}
}

func TestBuildTripContext_TrialExpiredNudge(t *testing.T) {
	ctx := buildTripContext("Japan Trip", "", "JP", nil, "", "", "planning", nil, nil, nil, 0, tier.Free, nil, "", true)

	if !strings.Contains(ctx, "free trial has expired") {
		t.Error("trip context should include trial expired nudge when trialExpired is true")
	}
	if !strings.Contains(ctx, "Trip Pro") {
		t.Error("trip context should mention Trip Pro upgrade when trial expired")
	}
}

func TestBuildTripContext_NoTrialExpiredNudge(t *testing.T) {
	ctx := buildTripContext("Japan Trip", "", "JP", nil, "", "", "planning", nil, nil, nil, 0, tier.Free, nil, "", false)

	if strings.Contains(ctx, "free trial has expired") {
		t.Error("trip context should not include trial expired nudge when trialExpired is false")
	}
}

func TestBuildTripContext_CapsItineraryAt60(t *testing.T) {
	items := make([]dbgen.ItineraryItem, 65)
	for i := range items {
		items[i] = dbgen.ItineraryItem{
			DayNumber: pgtype.Int4{Int32: 1, Valid: true},
			Title:     pgtype.Text{String: fmt.Sprintf("Item %d", i+1), Valid: true},
		}
	}
	ctx := buildTripContext("Japan Trip", "", "JP", nil, "", "", "planning", nil, items, nil, 0, tier.Free, nil, "", false)

	if !strings.Contains(ctx, "more items not shown") {
		t.Error("trip context should cap itinerary at 60 items")
	}
	// Verify items beyond the cap are not shown
	if strings.Contains(ctx, "Item 61") {
		t.Error("items beyond the cap should not appear in context")
	}
	// Verify items within the cap ARE shown
	if !strings.Contains(ctx, "Item 1") {
		t.Error("items within the cap should appear in context")
	}
}
