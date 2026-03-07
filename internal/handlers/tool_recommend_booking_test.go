package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gallowaysoftware/toqui-backend/internal/affiliate"
	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

func TestRecommendBookingTool_Definition(t *testing.T) {
	lb := affiliate.NewLinkBuilder("", "", "")
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

func TestRecommendBookingTool_Execute_InvalidJSON(t *testing.T) {
	lb := affiliate.NewLinkBuilder("", "", "")
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	_, err := tool.Execute(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRecommendBookingTool_Execute_MissingCategory(t *testing.T) {
	lb := affiliate.NewLinkBuilder("", "", "")
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"category": "", "query": "hotels in Prague"}`))
	if err == nil {
		t.Error("expected error for empty category")
	}
}

func TestRecommendBookingTool_Execute_MissingQuery(t *testing.T) {
	lb := affiliate.NewLinkBuilder("", "", "")
	tool := NewRecommendBookingTool(lb, tier.Free, nil)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"category": "hotel", "query": ""}`))
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestRecommendBookingTool_Execute_Flight(t *testing.T) {
	lb := affiliate.NewLinkBuilder("sky123", "", "")
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
	lb := affiliate.NewLinkBuilder("", "book456", "")
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
	lb := affiliate.NewLinkBuilder("", "", "gyg789")
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

func TestRecommendBookingTool_Execute_FlightNoOrigin(t *testing.T) {
	lb := affiliate.NewLinkBuilder("sky123", "", "")
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
	lb := affiliate.NewLinkBuilder("", "book456", "")
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
	lb := affiliate.NewLinkBuilder("", "", "")
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
	lb := affiliate.NewLinkBuilder("", "", "")
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
	lb := affiliate.NewLinkBuilder("", "", "")
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
	lb := affiliate.NewLinkBuilder("", "", "")
	tool := NewRecommendBookingTool(lb, tier.Pro, nil)
	def := tool.Definition()

	if !strings.Contains(def.Description, "best available sources") {
		t.Errorf("pro tier description should mention best available sources, got %q", def.Description)
	}
	if strings.Contains(def.Description, "affiliate") {
		t.Errorf("pro tier description should not mention affiliate links, got %q", def.Description)
	}
}

func TestRecommendBookingTool_FreeTier_Disclosure(t *testing.T) {
	lb := affiliate.NewLinkBuilder("sky123", "", "")
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
	lb := affiliate.NewLinkBuilder("sky123", "", "")
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
	lb := affiliate.NewLinkBuilder("", "book456", "")
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
	lb := affiliate.NewLinkBuilder("", "", "gyg789")
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
