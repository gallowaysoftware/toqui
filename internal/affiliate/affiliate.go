package affiliate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
)

// Partner identifies an affiliate partner.
type Partner string

const (
	PartnerSkyscanner   Partner = "skyscanner"
	PartnerBookingCom   Partner = "booking_com"
	PartnerGetYourGuide Partner = "getyourguide"
	PartnerViator       Partner = "viator"
	PartnerDiscoverCars Partner = "discovercars"
	PartnerSafetyWing   Partner = "safetywing"
	PartnerGeneric      Partner = "generic"

	// Non-affiliate "independent" sources used when a user prefers a
	// commission-free recommendation (e.g. Pro tier). These do not pay Toqui
	// a commission and do not carry any tracking ID.
	PartnerGoogle      Partner = "google"
	PartnerWikivoyage  Partner = "wikivoyage"
	PartnerOfficialGov Partner = "official_gov"
)

// Recommendation is a booking recommendation with an affiliate link.
type Recommendation struct {
	Partner     Partner `json:"partner"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	URL         string  `json:"url"`
	Price       string  `json:"price,omitempty"`
	Category    string  `json:"category"`
	ImageURL    string  `json:"image_url,omitempty"`
	Disclosure  string  `json:"disclosure"`
}

// FTCDisclosure is the standard disclosure text included with every affiliate recommendation.
const FTCDisclosure = "This is a partner link. Toqui may earn a commission at no extra cost to you."

// ProDisclosure is the disclosure text shown to Pro-tier users when the
// recommendation is delivered via an affiliate link. Today every category
// in sources.go exposes at least one independent candidate, so Pro
// recommendations almost always carry IndependentDisclosure instead. This
// constant exists for the defensive path in SelectForPreference: if a
// future change leaves a category with only affiliate sources, Pro users
// see this softer label and the partner-link nature is still disclosed
// honestly (and accurately — not as "ranked by fit", which the tool does
// not do).
const ProDisclosure = "Recommended for your trip. This is a partner link \u2014 Toqui may earn a commission at no extra cost to you."

// IndependentDisclosure is the disclosure text shown when the
// recommendation is delivered via a non-affiliate (independent) source —
// e.g. a Google Flights search URL or a Wikivoyage page. Toqui earns no
// commission on these clicks. Pro-tier users see this in place of
// ProDisclosure whenever the selected source is non-affiliate.
const IndependentDisclosure = "Independent source \u2014 Toqui earns no commission on this link."

// LinkBuilder generates affiliate URLs for each supported partner.
type LinkBuilder struct {
	skyscannerID   string
	bookingComID   string
	getYourGuideID string
	viatorID       string
	discoverCarsID string
	safetyWingID   string
}

// LinkBuilderConfig holds affiliate partner IDs for constructing a LinkBuilder.
type LinkBuilderConfig struct {
	SkyscannerID   string
	BookingComID   string
	GetYourGuideID string
	ViatorID       string
	DiscoverCarsID string
	SafetyWingID   string
}

// NewLinkBuilder creates a LinkBuilder with the given partner IDs.
// Empty IDs disable that partner's affiliate tracking (plain URLs are returned instead).
func NewLinkBuilder(cfg LinkBuilderConfig) *LinkBuilder {
	return &LinkBuilder{
		skyscannerID:   cfg.SkyscannerID,
		bookingComID:   cfg.BookingComID,
		getYourGuideID: cfg.GetYourGuideID,
		viatorID:       cfg.ViatorID,
		discoverCarsID: cfg.DiscoverCarsID,
		safetyWingID:   cfg.SafetyWingID,
	}
}

// FlightSearchURL returns a Skyscanner flight search URL with affiliate tracking.
// origin and dest are IATA airport codes or city names. date is YYYY-MM-DD format.
// tripIDHash, if non-empty, is appended as a utm_content sub-ID for conversion attribution.
func (b *LinkBuilder) FlightSearchURL(origin, dest, date string, tripIDHash ...string) string {
	u := fmt.Sprintf("https://www.skyscanner.com/transport/flights/%s/%s/%s",
		url.PathEscape(origin), url.PathEscape(dest), url.PathEscape(date))
	if b.skyscannerID != "" {
		u += "?associateid=" + url.QueryEscape(b.skyscannerID)
		if len(tripIDHash) > 0 && tripIDHash[0] != "" {
			u += "&utm_content=" + url.QueryEscape(tripIDHash[0])
		}
	}
	return u
}

// HotelSearchURL returns a Booking.com hotel search URL with affiliate tracking.
// propertyName, if provided, is used as the search string and takes precedence
// over city — this produces a property-specific deep link for known hotels
// (#176). When propertyName is empty, the destination city is used instead.
// checkin and checkout are YYYY-MM-DD format.
// tripIDHash, if non-empty, is appended as a label sub-ID for conversion attribution.
func (b *LinkBuilder) HotelSearchURL(propertyName, city, checkin, checkout string, tripIDHash ...string) string {
	params := url.Values{}
	searchStr := propertyName
	if searchStr == "" {
		searchStr = city
	}
	params.Set("ss", searchStr)
	if checkin != "" {
		params.Set("checkin", checkin)
	}
	if checkout != "" {
		params.Set("checkout", checkout)
	}
	if b.bookingComID != "" {
		params.Set("aid", b.bookingComID)
		if len(tripIDHash) > 0 && tripIDHash[0] != "" {
			params.Set("label", tripIDHash[0])
		}
	}
	return "https://www.booking.com/searchresults.html?" + params.Encode()
}

// ActivityURL returns a GetYourGuide activity search URL with affiliate tracking.
// query is a search term like "walking tour Prague" or "cooking class Tokyo".
// tripIDHash, if non-empty, is appended as a cmp sub-ID for conversion attribution.
func (b *LinkBuilder) ActivityURL(query string, tripIDHash ...string) string {
	params := url.Values{}
	params.Set("q", query)
	if b.getYourGuideID != "" {
		params.Set("partner_id", b.getYourGuideID)
		if len(tripIDHash) > 0 && tripIDHash[0] != "" {
			params.Set("cmp", tripIDHash[0])
		}
	}
	return "https://www.getyourguide.com/s/?" + params.Encode()
}

// ViatorActivityURL returns a Viator activity search URL with affiliate tracking.
// query is a search term like "food tour Rome" or "snorkeling Bali".
// tripIDHash, if non-empty, is appended as a cmp sub-ID for conversion attribution.
func (b *LinkBuilder) ViatorActivityURL(query string, tripIDHash ...string) string {
	params := url.Values{}
	params.Set("text", query)
	if b.viatorID != "" {
		params.Set("pid", b.viatorID)
		if len(tripIDHash) > 0 && tripIDHash[0] != "" {
			params.Set("cmp", tripIDHash[0])
		}
	}
	return "https://www.viator.com/search/" + query + "?" + params.Encode()
}

// CarRentalURL returns a DiscoverCars car rental search URL with affiliate tracking.
// location is the pickup city or airport. pickupDate and dropoffDate are YYYY-MM-DD format.
func (b *LinkBuilder) CarRentalURL(location, pickupDate, dropoffDate string) string {
	params := url.Values{}
	params.Set("location", location)
	if pickupDate != "" {
		params.Set("pickup_date", pickupDate)
	}
	if dropoffDate != "" {
		params.Set("dropoff_date", dropoffDate)
	}
	if b.discoverCarsID != "" {
		params.Set("a_aid", b.discoverCarsID)
	}
	return "https://www.discovercars.com/?" + params.Encode()
}

// TravelInsuranceURL returns a SafetyWing travel insurance URL with affiliate tracking.
// destination is the destination country or region.
func (b *LinkBuilder) TravelInsuranceURL(destination string) string {
	base := "https://safetywing.com/nomad-insurance"
	if b.safetyWingID != "" {
		return base + "?referenceID=" + url.QueryEscape(b.safetyWingID)
	}
	return base
}

// HasPartner returns true if the given partner has a configured affiliate ID.
func (b *LinkBuilder) HasPartner(p Partner) bool {
	switch p {
	case PartnerSkyscanner:
		return b.skyscannerID != ""
	case PartnerBookingCom:
		return b.bookingComID != ""
	case PartnerGetYourGuide:
		return b.getYourGuideID != ""
	case PartnerViator:
		return b.viatorID != ""
	case PartnerDiscoverCars:
		return b.discoverCarsID != ""
	case PartnerSafetyWing:
		return b.safetyWingID != ""
	default:
		return false
	}
}

// HashTripID produces a short, privacy-safe identifier from a raw trip ID
// (typically a UUID). Algorithm: SHA-256 → first 6 bytes → hex-encoded (12 chars).
// This is used as a sub-ID in affiliate URLs so we can correlate conversions
// back to trips without exposing the raw UUID.
func HashTripID(tripID string) string {
	if tripID == "" {
		return ""
	}
	h := sha256.Sum256([]byte(tripID))
	return hex.EncodeToString(h[:6]) // 12 hex chars = 48 bits
}

// PartnerForCategory returns the default affiliate partner for a booking category.
func PartnerForCategory(category string) Partner {
	switch category {
	case "flight":
		return PartnerSkyscanner
	case "hotel":
		return PartnerBookingCom
	case "activity":
		return PartnerGetYourGuide
	case "car_rental":
		return PartnerDiscoverCars
	case "insurance":
		return PartnerSafetyWing
	default:
		return PartnerGeneric
	}
}
