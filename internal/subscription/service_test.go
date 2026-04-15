package subscription

import (
	"testing"

	"github.com/stripe/stripe-go/v82"

	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

func TestParseBillingPeriod(t *testing.T) {
	tests := []struct {
		input string
		want  BillingPeriod
	}{
		{"monthly", BillingPeriodMonthly},
		{"annual", BillingPeriodAnnual},
		{"", BillingPeriodMonthly},
		{"yearly", BillingPeriodMonthly},  // unrecognised defaults to monthly
		{"ANNUAL", BillingPeriodMonthly},  // case-sensitive
		{"Monthly", BillingPeriodMonthly}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseBillingPeriod(tt.input)
			if got != tt.want {
				t.Errorf("ParseBillingPeriod(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIntervalToBillingPeriod(t *testing.T) {
	tests := []struct {
		name     string
		interval stripe.PriceRecurringInterval
		want     BillingPeriod
	}{
		{"year", stripe.PriceRecurringIntervalYear, BillingPeriodAnnual},
		{"month", stripe.PriceRecurringIntervalMonth, BillingPeriodMonthly},
		{"day", stripe.PriceRecurringIntervalDay, BillingPeriodMonthly},
		{"week", stripe.PriceRecurringIntervalWeek, BillingPeriodMonthly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intervalToBillingPeriod(tt.interval)
			if got != tt.want {
				t.Errorf("intervalToBillingPeriod(%q) = %q, want %q", tt.interval, got, tt.want)
			}
		})
	}
}

func TestResolvePriceID(t *testing.T) {
	svc := &Service{
		prices: ProductConfig{
			ExplorerMonthly: "price_explorer_monthly",
			ExplorerAnnual:  "price_explorer_annual",
			VoyagerMonthly:  "price_voyager_monthly",
			VoyagerAnnual:   "price_voyager_annual",
		},
	}

	tests := []struct {
		name    string
		tier    tier.UserTier
		annual  bool
		want    string
		wantErr bool
	}{
		{"explorer monthly", tier.Explorer, false, "price_explorer_monthly", false},
		{"explorer annual", tier.Explorer, true, "price_explorer_annual", false},
		{"voyager monthly", tier.Voyager, false, "price_voyager_monthly", false},
		{"voyager annual", tier.Voyager, true, "price_voyager_annual", false},
		{"free tier rejected", tier.Free, false, "", true},
		{"pro tier rejected", tier.Pro, false, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.resolvePriceID(tt.tier, tt.annual)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolvePriceID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("resolvePriceID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolvePriceID_MissingConfig(t *testing.T) {
	// Service with only monthly prices configured (annual missing).
	svc := &Service{
		prices: ProductConfig{
			ExplorerMonthly: "price_explorer_monthly",
			VoyagerMonthly:  "price_voyager_monthly",
		},
	}

	// Monthly should work.
	if _, err := svc.resolvePriceID(tier.Explorer, false); err != nil {
		t.Errorf("expected no error for monthly, got: %v", err)
	}

	// Annual should fail with empty price ID.
	if _, err := svc.resolvePriceID(tier.Explorer, true); err == nil {
		t.Error("expected error for unconfigured annual price, got nil")
	}
}

func TestResolveTierFromPriceID(t *testing.T) {
	svc := &Service{
		prices: ProductConfig{
			ExplorerMonthly: "price_explorer_monthly",
			ExplorerAnnual:  "price_explorer_annual",
			VoyagerMonthly:  "price_voyager_monthly",
			VoyagerAnnual:   "price_voyager_annual",
		},
	}

	tests := []struct {
		name    string
		priceID string
		want    tier.UserTier
		wantOK  bool
	}{
		{"explorer monthly", "price_explorer_monthly", tier.Explorer, true},
		{"explorer annual", "price_explorer_annual", tier.Explorer, true},
		{"voyager monthly", "price_voyager_monthly", tier.Voyager, true},
		{"voyager annual", "price_voyager_annual", tier.Voyager, true},
		{"unknown price", "price_unknown", tier.UserTier(""), false},
		{"empty price", "", tier.UserTier(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := svc.resolveTierFromPriceID(tt.priceID)
			if ok != tt.wantOK {
				t.Errorf("resolveTierFromPriceID(%q) ok = %v, want %v", tt.priceID, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("resolveTierFromPriceID(%q) = %q, want %q", tt.priceID, got, tt.want)
			}
		})
	}
}

func TestResolveTierFromPriceID_PartialConfig(t *testing.T) {
	// Only Explorer configured — Voyager prices should not match.
	svc := &Service{
		prices: ProductConfig{
			ExplorerMonthly: "price_explorer_monthly",
			ExplorerAnnual:  "price_explorer_annual",
		},
	}

	if got, ok := svc.resolveTierFromPriceID("price_explorer_monthly"); !ok || got != tier.Explorer {
		t.Errorf("expected Explorer, got %q (ok=%v)", got, ok)
	}

	// Empty VoyagerMonthly should not match an empty string input.
	if _, ok := svc.resolveTierFromPriceID(""); ok {
		t.Error("expected no match for empty priceID")
	}
}

func TestBillingPeriodConstants(t *testing.T) {
	if BillingPeriodMonthly != "monthly" {
		t.Errorf("BillingPeriodMonthly = %q, want %q", BillingPeriodMonthly, "monthly")
	}
	if BillingPeriodAnnual != "annual" {
		t.Errorf("BillingPeriodAnnual = %q, want %q", BillingPeriodAnnual, "annual")
	}
}
