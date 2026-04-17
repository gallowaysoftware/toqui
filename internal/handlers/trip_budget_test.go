package handlers

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

func TestTripToProto_BudgetFields(t *testing.T) {
	trip := &dbgen.Trip{
		Title:       "Budget Trip",
		Status:      "planning",
		BudgetCents: pgtype.Int8{Int64: 200000, Valid: true},
		Currency:    pgtype.Text{String: "USD", Valid: true},
	}

	proto := tripToProto(trip)

	if proto.BudgetCents == nil {
		t.Fatal("expected BudgetCents to be set")
	}
	if *proto.BudgetCents != 200000 {
		t.Errorf("BudgetCents = %d, want 200000", *proto.BudgetCents)
	}
	if proto.Currency != "USD" {
		t.Errorf("Currency = %q, want %q", proto.Currency, "USD")
	}
}

func TestTripToProto_NoBudget(t *testing.T) {
	trip := &dbgen.Trip{
		Title:  "No Budget Trip",
		Status: "planning",
	}

	proto := tripToProto(trip)

	if proto.BudgetCents != nil {
		t.Errorf("expected nil BudgetCents, got %d", *proto.BudgetCents)
	}
	if proto.Currency != "" {
		t.Errorf("expected empty Currency, got %q", proto.Currency)
	}
}

func TestTripToProto_EuroBudget(t *testing.T) {
	trip := &dbgen.Trip{
		Title:       "Euro Trip",
		Status:      "planning",
		BudgetCents: pgtype.Int8{Int64: 150000, Valid: true},
		Currency:    pgtype.Text{String: "EUR", Valid: true},
	}

	proto := tripToProto(trip)

	if proto.BudgetCents == nil || *proto.BudgetCents != 150000 {
		t.Errorf("BudgetCents = %v, want 150000", proto.BudgetCents)
	}
	if proto.Currency != "EUR" {
		t.Errorf("Currency = %q, want %q", proto.Currency, "EUR")
	}
}

func TestItineraryToProto_CostFields(t *testing.T) {
	items := []dbgen.ItineraryItem{
		{
			DayNumber:          pgtype.Int4{Int32: 1, Valid: true},
			Title:              pgtype.Text{String: "Temple Visit", Valid: true},
			Type:               pgtype.Text{String: "sightseeing", Valid: true},
			EstimatedCostCents: pgtype.Int8{Int64: 500, Valid: true},
			CostCurrency:       pgtype.Text{String: "USD", Valid: true},
		},
		{
			DayNumber: pgtype.Int4{Int32: 1, Valid: true},
			Title:     pgtype.Text{String: "Free Walk", Valid: true},
			Type:      pgtype.Text{String: "activity", Valid: true},
			// No cost fields — should remain nil/empty in proto
		},
	}

	itin := itineraryToProto("test-trip-id", items, nil)

	if len(itin.Days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(itin.Days))
	}
	day := itin.Days[0]
	if len(day.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(day.Items))
	}

	// First item should have cost
	if day.Items[0].EstimatedCostCents == nil {
		t.Fatal("expected EstimatedCostCents to be set for first item")
	}
	if *day.Items[0].EstimatedCostCents != 500 {
		t.Errorf("EstimatedCostCents = %d, want 500", *day.Items[0].EstimatedCostCents)
	}
	if day.Items[0].CostCurrency != "USD" {
		t.Errorf("CostCurrency = %q, want %q", day.Items[0].CostCurrency, "USD")
	}

	// Second item should have no cost
	if day.Items[1].EstimatedCostCents != nil {
		t.Errorf("expected nil EstimatedCostCents for second item, got %d", *day.Items[1].EstimatedCostCents)
	}
	if day.Items[1].CostCurrency != "" {
		t.Errorf("expected empty CostCurrency for second item, got %q", day.Items[1].CostCurrency)
	}
}

func TestBuildTripContext_IncludesBudget(t *testing.T) {
	budget := int64(200000)
	ctx := buildTripContext("Japan Trip", "", "JP", nil, "", "", "planning", nil, nil, nil, 0, tier.Free, &budget, "USD", false)

	if !strings.Contains(ctx, "Trip budget") {
		t.Error("trip context should include budget when set")
	}
	if !strings.Contains(ctx, "$2000.00") {
		t.Errorf("trip context should format budget in dollars, got:\n%s", ctx)
	}
	if !strings.Contains(ctx, "USD") {
		t.Error("trip context should include currency code")
	}
}

func TestBuildTripContext_NoBudget(t *testing.T) {
	ctx := buildTripContext("Japan Trip", "", "JP", nil, "", "", "planning", nil, nil, nil, 0, tier.Free, nil, "", false)

	if strings.Contains(ctx, "Trip budget") {
		t.Error("trip context should not include budget when not set")
	}
}

func TestBuildTripContext_EuroBudget(t *testing.T) {
	budget := int64(150000)
	ctx := buildTripContext("Euro Trip", "", "FR", nil, "", "", "planning", nil, nil, nil, 0, tier.Free, &budget, "EUR", false)

	if !strings.Contains(ctx, "Trip budget") {
		t.Error("trip context should include budget")
	}
	if !strings.Contains(ctx, "1500.00") {
		t.Errorf("trip context should format EUR budget correctly, got:\n%s", ctx)
	}
	if !strings.Contains(ctx, "EUR") {
		t.Error("trip context should include EUR currency code")
	}
}

func TestBuildTripContext_ItineraryWithCosts(t *testing.T) {
	items := []dbgen.ItineraryItem{
		{
			DayNumber:          pgtype.Int4{Int32: 1, Valid: true},
			Title:              pgtype.Text{String: "Sushi Dinner", Valid: true},
			EstimatedCostCents: pgtype.Int8{Int64: 5000, Valid: true},
			CostCurrency:       pgtype.Text{String: "USD", Valid: true},
		},
		{
			DayNumber: pgtype.Int4{Int32: 1, Valid: true},
			Title:     pgtype.Text{String: "Temple Visit", Valid: true},
			// No cost
		},
	}
	ctx := buildTripContext("Japan Trip", "", "JP", nil, "", "", "planning", nil, items, nil, 0, tier.Free, nil, "", false)

	if !strings.Contains(ctx, "Sushi Dinner ($50.00)") {
		t.Errorf("trip context should include cost for priced items, got:\n%s", ctx)
	}
	// Temple Visit should not have a cost annotation
	if strings.Contains(ctx, "Temple Visit ($") || strings.Contains(ctx, "Temple Visit (0") {
		t.Error("items without cost should not show cost annotation")
	}
}

func TestCurrencySymbol(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"USD", "$"},
		{"EUR", "\u20ac"},
		{"GBP", "\u00a3"},
		{"JPY", "\u00a5"},
		{"CAD", "CA$"},
		{"AUD", "A$"},
		{"SEK", ""},
		{"usd", "$"}, // case insensitive
	}

	for _, tt := range tests {
		got := currencySymbol(tt.code)
		if got != tt.want {
			t.Errorf("currencySymbol(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestCreateItineraryToolDefinition_IncludesCostFields(t *testing.T) {
	tool := NewCreateItineraryTool(nil, [16]byte{}, [16]byte{}, nil)
	def := tool.Definition()

	params := string(def.Parameters)
	if !strings.Contains(params, "estimated_cost_cents") {
		t.Error("create_itinerary_items tool should include estimated_cost_cents parameter")
	}
	if !strings.Contains(params, "cost_currency") {
		t.Error("create_itinerary_items tool should include cost_currency parameter")
	}
}
