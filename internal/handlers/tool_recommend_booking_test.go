package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gallowaysoftware/toqui-backend/internal/affiliate"
	"github.com/gallowaysoftware/toqui-backend/internal/analytics"
	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

func TestRecommendBookingTool_Definition(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)
	def := tool.Definition()

	if def.Name != "recommend_booking" {
		t.Errorf("expected name %q, got %q", "recommend_booking", def.Name)
	}

	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	// Verify parameters is valid JSON
	var params map[string]any
	if err := json.Unmarshal(def.Parameters, &params); err != nil {
		t.Fatalf("parameters is not valid JSON: %v", err)
	}

	// Check required fields
	required, ok := params["required"].([]any)
	if !ok {
		t.Fatal("expected required field in parameters")
	}
	requiredMap := make(map[string]bool)
	for _, r := range required {
		requiredMap[r.(string)] = true
	}
	if !requiredMap["category"] {
		t.Error("expected 'category' in required fields")
	}
	if !requiredMap["query"] {
		t.Error("expected 'query' in required fields")
	}
}

func TestRecommendBookingTool_Definition_IncludesNewCategories(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)
	def := tool.Definition()

	var params map[string]any
	if err := json.Unmarshal(def.Parameters, &params); err != nil {
		t.Fatalf("parameters is not valid JSON: %v", err)
	}

	props := params["properties"].(map[string]any)
	category := props["category"].(map[string]any)
	enumValues := category["enum"].([]any)

	enumSet := make(map[string]bool)
	for _, v := range enumValues {
		enumSet[v.(string)] = true
	}

	for _, expected := range []string{"flight", "hotel", "activity", "car_rental", "insurance"} {
		if !enumSet[expected] {
			t.Errorf("expected %q in category enum, got %v", expected, enumValues)
		}
	}
}

func TestRecommendBookingTool_Execute_InvalidJSON(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	_, err := tool.Execute(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRecommendBookingTool_Execute_MissingCategory(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"category": "", "query": "hotels in Prague"}`))
	if err == nil {
		t.Error("expected error for empty category")
	}
}

func TestRecommendBookingTool_Execute_MissingQuery(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"category": "hotel", "query": ""}`))
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestRecommendBookingTool_Execute_Flight(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	var captured affiliate.Recommendation
	tool := NewRecommendBookingTool(lb, tier.Free, func(rec affiliate.Recommendation) {
		captured = rec
	})

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "flight",
		"query": "flights from NYC to Prague",
		"origin": "JFK",
		"destination": "PRG",
		"date_from": "2026-06-15"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if rec.Category != "flight" {
		t.Errorf("expected category %q, got %q", "flight", rec.Category)
	}
	if rec.Partner != affiliate.PartnerSkyscanner {
		t.Errorf("expected partner %q, got %q", affiliate.PartnerSkyscanner, rec.Partner)
	}
	if !strings.Contains(rec.URL, "skyscanner.com") {
		t.Errorf("expected skyscanner URL, got %q", rec.URL)
	}
	if !strings.Contains(rec.URL, "associateid=sky123") {
		t.Errorf("expected affiliate ID in URL, got %q", rec.URL)
	}
	if rec.Disclosure != affiliate.FTCDisclosure {
		t.Errorf("expected FTC disclosure, got %q", rec.Disclosure)
	}

	// Verify callback was invoked
	if captured.Category != "flight" {
		t.Error("expected onRecommend callback to be called with flight category")
	}
}

func TestRecommendBookingTool_Execute_Hotel(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{BookingComID: "book456"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "hotel",
		"query": "hotels in Reykjavik",
		"destination": "Reykjavik",
		"date_from": "2026-07-01",
		"date_to": "2026-07-10"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if rec.Category != "hotel" {
		t.Errorf("expected category %q, got %q", "hotel", rec.Category)
	}
	if rec.Partner != affiliate.PartnerBookingCom {
		t.Errorf("expected partner %q, got %q", affiliate.PartnerBookingCom, rec.Partner)
	}
	if !strings.Contains(rec.URL, "booking.com") {
		t.Errorf("expected booking.com URL, got %q", rec.URL)
	}
	if !strings.Contains(rec.URL, "aid=book456") {
		t.Errorf("expected affiliate ID in URL, got %q", rec.URL)
	}
	if !strings.Contains(rec.URL, "ss=Reykjavik") {
		t.Errorf("expected city in URL, got %q", rec.URL)
	}
	if rec.Disclosure != affiliate.FTCDisclosure {
		t.Errorf("expected FTC disclosure, got %q", rec.Disclosure)
	}
}

func TestRecommendBookingTool_Execute_Activity(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{GetYourGuideID: "gyg789"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "activity",
		"query": "walking tour in Prague Old Town"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if rec.Category != "activity" {
		t.Errorf("expected category %q, got %q", "activity", rec.Category)
	}
	if rec.Partner != affiliate.PartnerGetYourGuide {
		t.Errorf("expected partner %q, got %q", affiliate.PartnerGetYourGuide, rec.Partner)
	}
	if !strings.Contains(rec.URL, "getyourguide.com") {
		t.Errorf("expected getyourguide URL, got %q", rec.URL)
	}
	if !strings.Contains(rec.URL, "partner_id=gyg789") {
		t.Errorf("expected partner ID in URL, got %q", rec.URL)
	}
	if rec.Disclosure != affiliate.FTCDisclosure {
		t.Errorf("expected FTC disclosure, got %q", rec.Disclosure)
	}
}

func TestRecommendBookingTool_Execute_CarRental(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{DiscoverCarsID: "dc202"})
	var captured affiliate.Recommendation
	tool := NewRecommendBookingTool(lb, tier.Free, func(rec affiliate.Recommendation) {
		captured = rec
	})

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "car_rental",
		"query": "car rental in Lisbon",
		"destination": "Lisbon",
		"date_from": "2026-07-01",
		"date_to": "2026-07-10"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if rec.Category != "car_rental" {
		t.Errorf("expected category %q, got %q", "car_rental", rec.Category)
	}
	if rec.Partner != affiliate.PartnerDiscoverCars {
		t.Errorf("expected partner %q, got %q", affiliate.PartnerDiscoverCars, rec.Partner)
	}
	if !strings.Contains(rec.URL, "discovercars.com") {
		t.Errorf("expected discovercars URL, got %q", rec.URL)
	}
	if !strings.Contains(rec.URL, "a_aid=dc202") {
		t.Errorf("expected affiliate ID in URL, got %q", rec.URL)
	}
	if !strings.Contains(rec.URL, "location=Lisbon") {
		t.Errorf("expected location in URL, got %q", rec.URL)
	}
	if rec.Disclosure != affiliate.FTCDisclosure {
		t.Errorf("expected FTC disclosure, got %q", rec.Disclosure)
	}

	// Verify callback was invoked
	if captured.Category != "car_rental" {
		t.Error("expected onRecommend callback to be called with car_rental category")
	}
}

func TestRecommendBookingTool_Execute_CarRentalNoDates(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{DiscoverCarsID: "dc202"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "car_rental",
		"query": "car rental in Tokyo",
		"destination": "Tokyo"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if !strings.Contains(rec.URL, "location=Tokyo") {
		t.Errorf("expected location in URL, got %q", rec.URL)
	}
	// Description should not mention dates
	if strings.Contains(rec.Description, "from") && strings.Contains(rec.Description, "to") {
		t.Errorf("description should not mention dates when none provided: %q", rec.Description)
	}
}

func TestRecommendBookingTool_Execute_Insurance(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SafetyWingID: "sw303"})
	var captured affiliate.Recommendation
	tool := NewRecommendBookingTool(lb, tier.Free, func(rec affiliate.Recommendation) {
		captured = rec
	})

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "insurance",
		"query": "travel insurance for Japan trip",
		"destination": "Japan"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if rec.Category != "insurance" {
		t.Errorf("expected category %q, got %q", "insurance", rec.Category)
	}
	if rec.Partner != affiliate.PartnerSafetyWing {
		t.Errorf("expected partner %q, got %q", affiliate.PartnerSafetyWing, rec.Partner)
	}
	if !strings.Contains(rec.URL, "safetywing.com") {
		t.Errorf("expected safetywing URL, got %q", rec.URL)
	}
	if !strings.Contains(rec.URL, "referenceID=sw303") {
		t.Errorf("expected reference ID in URL, got %q", rec.URL)
	}
	if rec.Disclosure != affiliate.FTCDisclosure {
		t.Errorf("expected FTC disclosure, got %q", rec.Disclosure)
	}

	// Verify callback was invoked
	if captured.Category != "insurance" {
		t.Error("expected onRecommend callback to be called with insurance category")
	}
}

func TestRecommendBookingTool_Execute_InsuranceNoDestination(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SafetyWingID: "sw303"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "insurance",
		"query": "travel insurance for my trip"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if !strings.Contains(rec.Title, "your trip") {
		t.Errorf("expected fallback destination in title, got %q", rec.Title)
	}
}

func TestRecommendBookingTool_Execute_FlightNoOrigin(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "flight",
		"query": "flights to Tokyo",
		"destination": "NRT"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	// Should use "anywhere" as origin fallback
	if !strings.Contains(rec.URL, "anywhere") {
		t.Errorf("expected 'anywhere' fallback in URL for missing origin, got %q", rec.URL)
	}
}

func TestRecommendBookingTool_Execute_HotelNoDates(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{BookingComID: "book456"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "hotel",
		"query": "hotels in Tokyo",
		"destination": "Tokyo"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if !strings.Contains(rec.URL, "ss=Tokyo") {
		t.Errorf("expected city in URL, got %q", rec.URL)
	}
	// Description should not mention dates
	if strings.Contains(rec.Description, "from") && strings.Contains(rec.Description, "to") {
		t.Errorf("description should not mention dates when none provided: %q", rec.Description)
	}
}

func TestRecommendBookingTool_Execute_NoCallback(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{})
	// nil callback should not panic
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "activity",
		"query": "cooking class Tokyo"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if rec.Category != "activity" {
		t.Errorf("expected category %q, got %q", "activity", rec.Category)
	}
}

func TestRecommendBookingTool_Execute_NoAffiliateIDs(t *testing.T) {
	// All empty IDs — URLs still work, just without affiliate tracking
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "flight",
		"query": "flights to London",
		"origin": "YYZ",
		"destination": "LHR",
		"date_from": "2026-09-01"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	// URL should still be generated, just without affiliate params
	if !strings.Contains(rec.URL, "skyscanner.com") {
		t.Errorf("expected skyscanner URL even without affiliate ID, got %q", rec.URL)
	}
	if strings.Contains(rec.URL, "associateid") {
		t.Errorf("should not contain affiliate param when ID is empty: %s", rec.URL)
	}
}

// --- Tier-gated tests ---

func TestRecommendBookingTool_FreeTier_DefinitionDescription(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)
	def := tool.Definition()

	if !strings.Contains(def.Description, "affiliate") {
		t.Errorf("free tier description should mention affiliate links, got %q", def.Description)
	}
	if strings.Contains(def.Description, "best available sources") {
		t.Errorf("free tier description should not mention best available sources, got %q", def.Description)
	}
}

func TestRecommendBookingTool_ProTier_DefinitionDescription(t *testing.T) {
	// Pro and Free share the same tool description today. The Execute path
	// returns affiliate URLs for every tier, so the description must not
	// claim Pro results come from a non-affiliate source. When tier-weighted
	// ranking and a widened candidate pool land, this test should be updated
	// to assert the new Pro-specific framing.
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{})
	freeTool := NewRecommendBookingTool(lb, tier.Free, nil)
	proTool := NewRecommendBookingTool(lb, tier.Pro, nil)

	if freeTool.Definition().Description != proTool.Definition().Description {
		t.Errorf("pro tier description should match free tier until tier-weighted ranking ships; got free=%q pro=%q",
			freeTool.Definition().Description, proTool.Definition().Description)
	}
	if !strings.Contains(proTool.Definition().Description, "affiliate") {
		t.Errorf("pro tier description must still mention affiliate (today every tier returns affiliate URLs), got %q",
			proTool.Definition().Description)
	}
	if strings.Contains(proTool.Definition().Description, "best available sources") {
		t.Errorf("pro tier description must not claim best-available-source framing while URLs still carry affiliate IDs, got %q",
			proTool.Definition().Description)
	}
}

func TestRecommendBookingTool_FreeTier_Disclosure(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "flight",
		"query": "flights to Paris",
		"origin": "JFK",
		"destination": "CDG",
		"date_from": "2026-08-01"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if rec.Disclosure != affiliate.FTCDisclosure {
		t.Errorf("free tier should have FTC disclosure, got %q", rec.Disclosure)
	}
}

func TestRecommendBookingTool_ProTier_Disclosure(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	tool := NewRecommendBookingTool(lb, tier.Pro, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "flight",
		"query": "flights to Paris",
		"origin": "JFK",
		"destination": "CDG",
		"date_from": "2026-08-01"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if rec.Disclosure != affiliate.ProDisclosure {
		t.Errorf("pro tier should have pro disclosure, got %q", rec.Disclosure)
	}
}

func TestRecommendBookingTool_ProTier_HotelDisclosure(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{BookingComID: "book456"})
	tool := NewRecommendBookingTool(lb, tier.Pro, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "hotel",
		"query": "hotels in Berlin",
		"destination": "Berlin"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if rec.Disclosure != affiliate.ProDisclosure {
		t.Errorf("pro tier hotel should have pro disclosure, got %q", rec.Disclosure)
	}
}

func TestRecommendBookingTool_ProTier_ActivityDisclosure(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{GetYourGuideID: "gyg789"})
	tool := NewRecommendBookingTool(lb, tier.Pro, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "activity",
		"query": "wine tasting in Bordeaux"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if rec.Disclosure != affiliate.ProDisclosure {
		t.Errorf("pro tier activity should have pro disclosure, got %q", rec.Disclosure)
	}
}

func TestRecommendBookingTool_ProTier_CarRentalDisclosure(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{DiscoverCarsID: "dc202"})
	tool := NewRecommendBookingTool(lb, tier.Pro, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "car_rental",
		"query": "car rental in Lisbon",
		"destination": "Lisbon"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if rec.Disclosure != affiliate.ProDisclosure {
		t.Errorf("pro tier car rental should have pro disclosure, got %q", rec.Disclosure)
	}
}

func TestRecommendBookingTool_ProTier_InsuranceDisclosure(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SafetyWingID: "sw303"})
	tool := NewRecommendBookingTool(lb, tier.Pro, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "insurance",
		"query": "travel insurance for Japan",
		"destination": "Japan"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if rec.Disclosure != affiliate.ProDisclosure {
		t.Errorf("pro tier insurance should have pro disclosure, got %q", rec.Disclosure)
	}
}

// --- Sub-ID tracking tests ---

func TestRecommendBookingTool_FlightWithTripID_HasSubID(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)
	tool = tool.WithTripContext("France", "2026-06-15", "2026-06-20", "550e8400-e29b-41d4-a716-446655440000")

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "flight",
		"query": "flights from NYC to Paris",
		"origin": "JFK",
		"destination": "CDG",
		"date_from": "2026-06-15"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	expectedHash := affiliate.HashTripID("550e8400-e29b-41d4-a716-446655440000")
	if !strings.Contains(rec.URL, "utm_content="+expectedHash) {
		t.Errorf("expected utm_content sub-ID in URL, got %q", rec.URL)
	}
}

func TestRecommendBookingTool_HotelWithTripID_HasSubID(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{BookingComID: "book456"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)
	tool = tool.WithTripContext("France", "", "", "trip-uuid-123")

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "hotel",
		"query": "hotels in Paris",
		"destination": "Paris"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	expectedHash := affiliate.HashTripID("trip-uuid-123")
	if !strings.Contains(rec.URL, "label="+expectedHash) {
		t.Errorf("expected label sub-ID in URL, got %q", rec.URL)
	}
}

func TestRecommendBookingTool_ActivityWithTripID_HasSubID(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{GetYourGuideID: "gyg789"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)
	tool = tool.WithTripContext("Czech Republic", "", "", "trip-uuid-456")

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "activity",
		"query": "walking tour Prague"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	expectedHash := affiliate.HashTripID("trip-uuid-456")
	if !strings.Contains(rec.URL, "cmp="+expectedHash) {
		t.Errorf("expected cmp sub-ID in URL, got %q", rec.URL)
	}
}

func TestRecommendBookingTool_NoTripID_NoSubID(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	// No WithTripContext call — tripID is empty
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "flight",
		"query": "flights to London",
		"origin": "JFK",
		"destination": "LHR",
		"date_from": "2026-09-01"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if strings.Contains(rec.URL, "utm_content=") {
		t.Errorf("should not contain utm_content when no trip ID: %s", rec.URL)
	}
}

// --- Analytics tracking tests ---

func TestRecommendBookingTool_WithAnalytics_FreeTier_TracksEvent(t *testing.T) {
	// Use a no-op analytics client (disabled) — just verify the wiring doesn't panic
	client := analytics.NewClient("")
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)
	tool = tool.WithAnalytics(client, "user-123")
	tool = tool.WithTripContext("France", "", "", "trip-123")

	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "flight",
		"query": "flights to Paris",
		"origin": "JFK",
		"destination": "CDG"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecommendBookingTool_WithAnalytics_ProTier_NoTracking(t *testing.T) {
	// Pro-tier users should NOT have affiliate events tracked since they
	// don't receive affiliate links
	client := analytics.NewClient("")
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	tool := NewRecommendBookingTool(lb, tier.Pro, nil)
	tool = tool.WithAnalytics(client, "user-456")

	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "flight",
		"query": "flights to Paris",
		"origin": "JFK",
		"destination": "CDG"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecommendBookingTool_NilAnalytics_NoPanic(t *testing.T) {
	// Verify that nil analytics client doesn't cause a panic
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)
	// Don't call WithAnalytics — analyticsClient is nil

	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "flight",
		"query": "flights to Paris",
		"origin": "JFK",
		"destination": "CDG"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
