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
	instructions := bookingInstructionsForTier(tier.Pro)

	if !strings.Contains(instructions, "best options") {
		t.Errorf("pro tier instructions should mention best options, got %q", instructions)
	}
	if !strings.Contains(instructions, "regardless of affiliate") {
		t.Errorf("pro tier instructions should mention ignoring affiliate partnerships, got %q", instructions)
	}
	if strings.Contains(instructions, "always use the recommend_booking tool to generate affiliate links") {
		t.Errorf("pro tier instructions should not tell AI to always use affiliate links, got %q", instructions)
	}
}

func TestBuildTripContext_IncludesBookingInstructions(t *testing.T) {
	ctx := buildTripContext("Japan Trip", "Two weeks in Japan", "JP", "", "", "planning", []string{"food", "culture"}, nil, nil, 0, tier.Free)

	if !strings.Contains(ctx, "BOOKING RECOMMENDATIONS") {
		t.Error("trip context should include booking recommendations section")
	}
	if !strings.Contains(ctx, "disclosure") {
		t.Error("free tier trip context should mention disclosure requirement")
	}
}

func TestBuildTripContext_ProTier(t *testing.T) {
	ctx := buildTripContext("Japan Trip", "Two weeks in Japan", "JP", "", "", "planning", []string{"food", "culture"}, nil, nil, 0, tier.Pro)

	if !strings.Contains(ctx, "BOOKING RECOMMENDATIONS") {
		t.Error("trip context should include booking recommendations section")
	}
	if !strings.Contains(ctx, "best options") {
		t.Error("pro tier trip context should mention best options")
	}
}

func TestBuildTripContext_Empty_ReturnsEmpty(t *testing.T) {
	ctx := buildTripContext("", "", "", "", "", "", nil, nil, nil, 0, tier.Free)
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
	ctx := buildTripContext("Japan Trip", "", "JP", "", "", "planning", nil, items, nil, 0, tier.Free)

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
	ctx := buildTripContext("Japan Trip", "", "JP", "", "", "planning", nil, nil, bookings, 0, tier.Free)

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
	ctx := buildTripContext("Japan Trip", "", "JP", "", "", "planning", nil, nil, nil, 3, tier.Free)

	if !strings.Contains(ctx, "4 people") {
		t.Error("trip context should show collaborator count (+1 for owner)")
	}
}

func TestBuildTripContext_IncludesStatus(t *testing.T) {
	ctx := buildTripContext("Japan Trip", "", "JP", "", "", "active", nil, nil, nil, 0, tier.Free)

	if !strings.Contains(ctx, "(active)") {
		t.Error("trip context should include trip status")
	}
}

func TestBuildTripContext_CapsItineraryAt20(t *testing.T) {
	items := make([]dbgen.ItineraryItem, 25)
	for i := range items {
		items[i] = dbgen.ItineraryItem{
			DayNumber: pgtype.Int4{Int32: 1, Valid: true},
			Title:     pgtype.Text{String: fmt.Sprintf("Item %d", i+1), Valid: true},
		}
	}
	ctx := buildTripContext("Japan Trip", "", "JP", "", "", "planning", nil, items, nil, 0, tier.Free)

	if !strings.Contains(ctx, "more items not shown") {
		t.Error("trip context should cap itinerary at 20 items")
	}
}
