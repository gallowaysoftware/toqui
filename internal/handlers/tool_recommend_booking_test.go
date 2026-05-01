package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/gallowaysoftware/toqui-backend/internal/affiliate"
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
	// Free-tier callers always get the affiliate candidate; the description
	// must not claim "ranked by fit" or "commission-free" because there is
	// no non-affiliate alternative in the free pool.
	if strings.Contains(def.Description, "ranked by fit") {
		t.Errorf("free tier description should not claim ranked-by-fit framing, got %q", def.Description)
	}
	if strings.Contains(def.Description, "commission-free") {
		t.Errorf("free tier description should not claim 'commission-free' (free always uses affiliate), got %q", def.Description)
	}
}

func TestRecommendBookingTool_ProTier_DefinitionDescription(t *testing.T) {
	// Pro tier now diverges from free: the tool prefers non-affiliate
	// sources and the description tells the AI so. The disclosure-inclusion
	// requirement is still present so the AI never strips it.
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{})
	freeTool := NewRecommendBookingTool(lb, tier.Free, nil)
	proTool := NewRecommendBookingTool(lb, tier.Pro, nil)

	if freeTool.Definition().Description == proTool.Definition().Description {
		t.Errorf("pro tier description should diverge from free tier now that tier-weighted ranking ships; got identical=%q",
			proTool.Definition().Description)
	}
	if !strings.Contains(proTool.Definition().Description, "commission-free") {
		t.Errorf("pro tier description should describe behaviour as 'prefers commission-free sources', got %q",
			proTool.Definition().Description)
	}
	if !strings.Contains(proTool.Definition().Description, "independent") {
		t.Errorf("pro tier description should mention independent sources, got %q",
			proTool.Definition().Description)
	}
	if !strings.Contains(proTool.Definition().Description, "disclosure") {
		t.Errorf("pro tier description must still require disclosure inclusion, got %q",
			proTool.Definition().Description)
	}
	// "ranked by fit" is a quality claim the tool does not deliver. PR #331
	// stripped this exact framing because the code didn't back it up — do
	// not let it creep back into the tool description.
	if strings.Contains(proTool.Definition().Description, "ranked by fit") {
		t.Errorf("pro tier description must not claim ranking-by-fit (the tool picks first non-affiliate, no scoring), got %q",
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
	// Pro tier flight: ITA Matrix is the marketed Pro addition (Google's
	// flight backend, the most powerful flight search tool publicly
	// available) and ranks above plain Google Flights via the +0.05
	// Pro-pool addition tiebreak. Both are non-affiliate so the
	// disclosure is still IndependentDisclosure either way; this test
	// pins that the Pro addition wins the tiebreak rather than Google.
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

	if rec.Disclosure != affiliate.IndependentDisclosure {
		t.Errorf("pro tier flight should have independent disclosure, got %q", rec.Disclosure)
	}
	if rec.Partner != affiliate.PartnerITAMatrix {
		t.Errorf("pro tier flight should select ITA Matrix (Pro-pool addition outranking plain Google Flights), got %q", rec.Partner)
	}
	if strings.Contains(rec.URL, "skyscanner.com") {
		t.Errorf("pro tier flight URL should not be Skyscanner, got %q", rec.URL)
	}
	if strings.Contains(rec.URL, "associateid=sky123") {
		t.Errorf("pro tier flight URL must not carry affiliate ID, got %q", rec.URL)
	}
}

func TestRecommendBookingTool_ProTier_HotelDisclosure(t *testing.T) {
	// Pro tier hotel: Hotellook is the marketed Pro addition (a hotel
	// meta-search aggregator that compares prices across booking sites)
	// and ranks above plain Google Maps via the +0.05 Pro-pool addition
	// tiebreak in ranking.go. Both are non-affiliate so the disclosure
	// is still IndependentDisclosure either way; this test pins that
	// the Pro addition wins the tiebreak rather than Google.
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

	if rec.Disclosure != affiliate.IndependentDisclosure {
		t.Errorf("pro tier hotel should have independent disclosure, got %q", rec.Disclosure)
	}
	if rec.Partner != affiliate.PartnerHotellook {
		t.Errorf("pro tier hotel should select Hotellook (Pro-pool addition), got %q", rec.Partner)
	}
	if strings.Contains(rec.URL, "booking.com") {
		t.Errorf("pro tier hotel URL should not be Booking.com, got %q", rec.URL)
	}
}

func TestRecommendBookingTool_ProTier_ActivityDisclosure(t *testing.T) {
	// Pro tier activity request without a city → falls back to Google Maps
	// (still independent, just not Wikivoyage).
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

	if rec.Disclosure != affiliate.IndependentDisclosure {
		t.Errorf("pro tier activity should have independent disclosure, got %q", rec.Disclosure)
	}
	if strings.Contains(rec.URL, "getyourguide.com") {
		t.Errorf("pro tier activity URL should not be GetYourGuide, got %q", rec.URL)
	}
}

func TestRecommendBookingTool_ProTier_CarRentalDisclosure(t *testing.T) {
	// Pro tier prefers Google Maps "car rental near X" over DiscoverCars.
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

	if rec.Disclosure != affiliate.IndependentDisclosure {
		t.Errorf("pro tier car rental should have independent disclosure, got %q", rec.Disclosure)
	}
	if strings.Contains(rec.URL, "discovercars.com") {
		t.Errorf("pro tier car rental URL should not be DiscoverCars, got %q", rec.URL)
	}
	if strings.Contains(rec.URL, "a_aid=dc202") {
		t.Errorf("pro tier car rental URL must not carry affiliate ID, got %q", rec.URL)
	}
}

func TestRecommendBookingTool_ProTier_InsuranceDisclosure(t *testing.T) {
	// Insurance has the weakest independent alternative (a plain Google
	// search), but it IS still an independent URL — clicking it doesn't
	// earn Toqui a commission. So Pro users get IndependentDisclosure here
	// too. If InsuranceSources ever drops the Google fallback, the
	// affiliate candidate is selected and DisclosureFor now produces
	// FTCDisclosure (#190 LB-4) — the same partner-link label free-tier
	// users see, because the URL is commission-earning regardless of
	// tier.
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

	if rec.Disclosure != affiliate.IndependentDisclosure {
		t.Errorf("pro tier insurance should have independent disclosure (Squaremouth fallback exists), got %q", rec.Disclosure)
	}
	if rec.Partner != affiliate.PartnerSquaremouth {
		t.Errorf("pro tier insurance should select Squaremouth (Pro-pool addition outranking plain Google), got %q", rec.Partner)
	}
	if strings.Contains(rec.URL, "safetywing.com") {
		t.Errorf("pro tier insurance URL should not be SafetyWing, got %q", rec.URL)
	}
	if strings.Contains(rec.URL, "referenceID=sw303") {
		t.Errorf("pro tier insurance URL must not carry SafetyWing affiliate ID, got %q", rec.URL)
	}
}

// TestRecommendBookingTool_ProTier_InsuranceFallback_NoGoogleSource
// exercises the documented affiliate-fallback path by hand-constructing a
// scenario where SelectForPreference can't find a non-affiliate source. We
// verify it via the affiliate package's primitives directly because
// InsuranceSources currently always emits a Google search candidate.
func TestRecommendBookingTool_ProTier_InsuranceFallback_NoGoogleSource(t *testing.T) {
	onlyAffiliate := []affiliate.Source{
		{
			ID:          "safetywing",
			Partner:     affiliate.PartnerSafetyWing,
			IsAffiliate: true,
			URL:         "https://safetywing.com/nomad-insurance?referenceID=sw303",
		},
	}
	selected := affiliate.SelectForPreference(true, onlyAffiliate)
	if selected.ID != "safetywing" {
		t.Fatalf("expected affiliate fallback when no independent option exists, got %+v", selected)
	}
	disc := affiliate.DisclosureFor(selected, true)
	if disc != affiliate.FTCDisclosure {
		t.Errorf("affiliate fallback for Pro must use FTCDisclosure — the URL is commission-earning regardless of tier (#190 LB-4); got %q", disc)
	}
}

// TestRecommendBookingTool_ProTier_ActivityWithCity exercises the
// Wikivoyage+Google Maps Pro path: the source builder includes Wikivoyage
// when a city is set, and SelectForPreference takes the first non-affiliate
// (Google Maps) per the affiliate-first ordering. We additionally confirm
// the disclosure is independent.
func TestRecommendBookingTool_ProTier_ActivityWithCity(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{GetYourGuideID: "gyg789"})
	tool := NewRecommendBookingTool(lb, tier.Pro, nil)
	tool = tool.WithTripContext("Prague", "", "", "")

	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "activity",
		"query": "walking tour"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec affiliate.Recommendation
	if err := json.Unmarshal(result, &rec); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if rec.Disclosure != affiliate.IndependentDisclosure {
		t.Errorf("pro tier activity with city should have independent disclosure, got %q", rec.Disclosure)
	}
	if strings.Contains(rec.URL, "getyourguide.com") {
		t.Errorf("pro tier activity URL should not be GetYourGuide, got %q", rec.URL)
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

// recordingTracker is a test stub satisfying the analyticsTracker interface.
// It captures every Track() call so tests can assert exactly which events
// fired and which properties were sent — important for affiliate_link_generated
// because CLAUDE.md privacy rules forbid logging destination names.
type recordingTracker struct {
	events []recordedEvent
}

type recordedEvent struct {
	userID     string
	event      string
	properties map[string]any
}

func (r *recordingTracker) Track(userID, event string, properties map[string]any) {
	r.events = append(r.events, recordedEvent{userID: userID, event: event, properties: properties})
}

func TestRecommendBookingTool_WithAnalytics_FreeTier_TracksAffiliateLink(t *testing.T) {
	// Free-tier flight → Skyscanner (affiliate) → event must fire with
	// exactly the documented properties.
	tracker := &recordingTracker{}
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)
	tool = tool.WithAnalytics(tracker, "user-123")
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

	if len(tracker.events) != 1 {
		t.Fatalf("expected exactly 1 analytics event for free-tier affiliate link, got %d: %+v", len(tracker.events), tracker.events)
	}
	ev := tracker.events[0]
	if ev.event != "affiliate_link_generated" {
		t.Errorf("expected event=affiliate_link_generated, got %q", ev.event)
	}
	if ev.userID != "user-123" {
		t.Errorf("expected raw userID passed to tracker (hashing happens inside the client), got %q", ev.userID)
	}
	if got := ev.properties["partner"]; got != string(affiliate.PartnerSkyscanner) {
		t.Errorf("expected partner=skyscanner, got %v", got)
	}
	if got := ev.properties["category"]; got != "flight" {
		t.Errorf("expected category=flight, got %v", got)
	}
	if got := ev.properties["tier"]; got != string(tier.Free) {
		t.Errorf("expected tier=free, got %v", got)
	}
	// Privacy regression guard: CLAUDE.md forbids logging destination names
	// (or country codes — the field used to carry "France"). The set of keys
	// must be exactly {partner, category, tier} so a future addition of
	// destination/destination_country/region is caught immediately.
	allowedKeys := map[string]bool{"partner": true, "category": true, "tier": true}
	for k := range ev.properties {
		if !allowedKeys[k] {
			t.Errorf("unexpected analytics property %q (CLAUDE.md privacy rules forbid extra context, esp. destinations); got props=%v", k, ev.properties)
		}
	}
}

func TestRecommendBookingTool_WithAnalytics_ProTier_NoTrackingForIndependent(t *testing.T) {
	// Pro-tier flight → Google Flights (independent) → no affiliate link is
	// generated, so the event MUST NOT fire. This is the privacy-positive
	// promise of the Pro tier — verifying with a real recorder, not a no-op.
	tracker := &recordingTracker{}
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	tool := NewRecommendBookingTool(lb, tier.Pro, nil)
	tool = tool.WithAnalytics(tracker, "user-456")

	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "flight",
		"query": "flights to Paris",
		"origin": "JFK",
		"destination": "CDG"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tracker.events) != 0 {
		t.Errorf("Pro-tier independent source must not generate analytics events, got %d events: %+v", len(tracker.events), tracker.events)
	}
}

func TestRecommendBookingTool_WithAnalytics_ProTier_DoesNotLogDestinationForIndependent(t *testing.T) {
	// Defensive: even when a Pro user has trip context with a destination,
	// the event suppression and the no-destination-properties rule both
	// hold. This is the test that would catch a regression if a future
	// change started firing the event for independent sources OR added
	// destination_country back to the props.
	tracker := &recordingTracker{}
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{
		SkyscannerID: "sky123", BookingComID: "book456", GetYourGuideID: "gyg789",
		DiscoverCarsID: "dc202", SafetyWingID: "sw303",
	})
	tool := NewRecommendBookingTool(lb, tier.Pro, nil)
	tool = tool.WithAnalytics(tracker, "user-pro")
	tool = tool.WithTripContext("Japan", "2026-06-15", "2026-06-20", "trip-uuid-xyz")

	for _, category := range []string{"flight", "hotel", "activity", "car_rental", "insurance"} {
		args := fmt.Sprintf(`{"category": %q, "query": "test", "destination": "Tokyo"}`, category)
		_, err := tool.Execute(context.Background(), json.RawMessage(args))
		if err != nil {
			t.Fatalf("unexpected error for category %q: %v", category, err)
		}
	}

	// All five Pro categories prefer independent today, so zero events.
	if len(tracker.events) != 0 {
		t.Errorf("Pro tier across all categories must not generate any analytics events today (every category has an independent fallback), got %d events: %+v",
			len(tracker.events), tracker.events)
	}
}

func TestRecommendBookingTool_WithAnalytics_AffiliateFallback_TracksWithoutDestination(t *testing.T) {
	// Hand-construct the defensive Pro-tier affiliate-fallback path so we
	// can verify (a) the event DOES fire when the selected source carries
	// an affiliate ID, and (b) the properties are still scrubbed of any
	// destination context. The natural "Pro insurance" path does not hit
	// this branch today (Google search is non-affiliate), so we exercise
	// the handler logic by injecting a tracker and using a free-tier
	// insurance call which DOES select SafetyWing.
	tracker := &recordingTracker{}
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SafetyWingID: "sw303"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)
	tool = tool.WithAnalytics(tracker, "user-789")
	tool = tool.WithTripContext("Japan", "", "", "trip-789")

	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "insurance",
		"query": "travel insurance for Japan",
		"destination": "Japan"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tracker.events) != 1 {
		t.Fatalf("expected 1 event for affiliate insurance, got %d: %+v", len(tracker.events), tracker.events)
	}
	ev := tracker.events[0]
	if ev.event != "affiliate_link_generated" {
		t.Errorf("expected event=affiliate_link_generated, got %q", ev.event)
	}
	if got := ev.properties["partner"]; got != string(affiliate.PartnerSafetyWing) {
		t.Errorf("expected partner=safetywing, got %v", got)
	}
	// Privacy regression guard: even with a destination set in trip context
	// AND in the args, the event must not carry it.
	for _, forbidden := range []string{"destination", "destination_country", "destination_city", "country", "region"} {
		if _, present := ev.properties[forbidden]; present {
			t.Errorf("analytics event must not include %q (CLAUDE.md privacy rules), got props=%v", forbidden, ev.properties)
		}
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

// --- Rationale propagation (#386 PR 3) ---

// The scored ranker emits a per-source rationale string and the tool result
// must surface it. The AI sees the rationale field in the tool result and
// can paraphrase it in the user-facing reply ("I picked Skyscanner because
// affiliate, dated query fits aggregator" → "Skyscanner is best for dated
// flight searches"). The tool description tells the AI to paraphrase, not
// quote raw — but the wire contract is simply "rationale field is non-empty
// for any selected source". Test that.

func TestRecommendBookingTool_RationaleIsSurfaced_FreeTierFlight(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

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

	if rec.Rationale == "" {
		t.Errorf("expected non-empty rationale on free-tier flight (selected: %q), got empty", rec.Partner)
	}
	// Free-tier affiliate-first selection should mention "affiliate (free)"
	// per ranking.go's scoreOne. The test couples to this exact phrase
	// because PR 3 promises stable rationale fragments — they're part of
	// the public contract for the AI prompt.
	if !strings.Contains(rec.Rationale, "affiliate (free)") {
		t.Errorf("free-tier flight rationale should mention 'affiliate (free)', got %q", rec.Rationale)
	}
}

func TestRecommendBookingTool_RationaleIsSurfaced_ProTierFlight(t *testing.T) {
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	tool := NewRecommendBookingTool(lb, tier.Pro, nil)

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

	if rec.Rationale == "" {
		t.Errorf("expected non-empty rationale on pro-tier flight (selected: %q), got empty", rec.Partner)
	}
	if !strings.Contains(rec.Rationale, "non-affiliate (Pro)") {
		t.Errorf("pro-tier flight rationale should mention 'non-affiliate (Pro)', got %q", rec.Rationale)
	}
}

func TestRecommendBookingTool_RationaleMentionsDates_WhenDatedAggregator(t *testing.T) {
	// Skyscanner is a search aggregator and the user supplied dates: the
	// rationale should include the "dated query fits aggregator" fragment.
	// Without this signal the AI can't justify aggregator picks for dated
	// searches.
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{SkyscannerID: "sky123"})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	result, _ := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "flight",
		"query": "flights",
		"origin": "JFK",
		"destination": "PRG",
		"date_from": "2026-06-15"
	}`))

	var rec affiliate.Recommendation
	_ = json.Unmarshal(result, &rec)

	if !strings.Contains(rec.Rationale, "dated query fits aggregator") {
		t.Errorf("expected rationale to mention 'dated query fits aggregator' for dated free-tier flight, got %q", rec.Rationale)
	}
}

func TestRecommendBookingTool_RationaleOmitted_WhenFallbackUsed(t *testing.T) {
	// Defensive path: an unknown category hits the fallback that
	// short-circuits before ScoreSources is called. The Recommendation
	// returned has no rationale, and JSON serialization should omit
	// the field via omitempty (no empty "rationale": "" key).
	lb := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{})
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	result, _ := tool.Execute(context.Background(), json.RawMessage(`{
		"category": "unknown_category",
		"query": "something"
	}`))

	if strings.Contains(string(result), `"rationale"`) {
		t.Errorf("rationale field should be omitted (omitempty) when no source was selected; got result %s", string(result))
	}
}
