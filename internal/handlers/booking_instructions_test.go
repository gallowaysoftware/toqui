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
	// Pro and Free share the same booking instructions today. The tool
	// returns affiliate-linked URLs for every tier, so Pro users still need
	// the disclosure-inclusion requirement in the system prompt for FTC
	// compliance. When tier-weighted ranking and a widened candidate pool
	// land, this test should assert the new Pro-specific framing.
	proInstructions := bookingInstructionsForTier(tier.Pro)
	freeInstructions := bookingInstructionsForTier(tier.Free)

	if proInstructions != freeInstructions {
		t.Errorf("pro tier instructions should match free tier until tier-weighted ranking ships")
	}
	if !strings.Contains(proInstructions, "recommend_booking") {
		t.Errorf("pro tier instructions should mention the recommend_booking tool, got %q", proInstructions)
	}
	if !strings.Contains(proInstructions, "disclosure") {
		t.Errorf("pro tier instructions must mention disclosure requirement (every tier gets affiliate URLs today), got %q", proInstructions)
	}
	if !strings.Contains(proInstructions, "legal requirement") {
		t.Errorf("pro tier instructions must mention legal requirement for disclosure, got %q", proInstructions)
	}
	if strings.Contains(proInstructions, "regardless of affiliate") {
		t.Errorf("pro tier instructions must not claim ignore-affiliate framing while URLs still carry affiliate IDs, got %q", proInstructions)
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
	// Until tier-weighted ranking ships, Pro and Free share the same booking
	// instructions — including the disclosure-inclusion requirement.
	if !strings.Contains(ctx, "disclosure") {
		t.Error("pro tier trip context should still mention disclosure requirement (affiliate URLs today)")
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
