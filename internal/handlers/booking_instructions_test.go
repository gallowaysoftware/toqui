package handlers

import (
	"strings"
	"testing"

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
	ctx := buildTripContext("Japan Trip", "Two weeks in Japan", "JP", []string{"food", "culture"}, tier.Free)

	if !strings.Contains(ctx, "BOOKING RECOMMENDATIONS") {
		t.Error("trip context should include booking recommendations section")
	}
	if !strings.Contains(ctx, "disclosure") {
		t.Error("free tier trip context should mention disclosure requirement")
	}
}

func TestBuildTripContext_ProTier(t *testing.T) {
	ctx := buildTripContext("Japan Trip", "Two weeks in Japan", "JP", []string{"food", "culture"}, tier.Pro)

	if !strings.Contains(ctx, "BOOKING RECOMMENDATIONS") {
		t.Error("trip context should include booking recommendations section")
	}
	if !strings.Contains(ctx, "best options") {
		t.Error("pro tier trip context should mention best options")
	}
}

func TestBuildTripContext_Empty_ReturnsEmpty(t *testing.T) {
	ctx := buildTripContext("", "", "", nil, tier.Free)
	if ctx != "" {
		t.Errorf("expected empty string for empty trip context, got %q", ctx)
	}
}
