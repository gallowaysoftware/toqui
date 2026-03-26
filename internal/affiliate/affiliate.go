package affiliate

import (
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

// ProDisclosure is the disclosure text for Pro-tier users who receive unbiased
// recommendations with no affiliate links attached.
const ProDisclosure = "Unbiased recommendation \u2014 no affiliate links."

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
func (b *LinkBuilder) FlightSearchURL(origin, dest, date string) string {
	u := fmt.Sprintf("https://www.skyscanner.com/transport/flights/%s/%s/%s",
		url.PathEscape(origin), url.PathEscape(dest), url.PathEscape(date))
	if b.skyscannerID != "" {
		u += "?associateid=" + url.QueryEscape(b.skyscannerID)
	}
	return u
}

// HotelSearchURL returns a Booking.com hotel search URL with affiliate tracking.
// city is the destination city name. checkin and checkout are YYYY-MM-DD format.
func (b *LinkBuilder) HotelSearchURL(city, checkin, checkout string) string {
	params := url.Values{}
	params.Set("ss", city)
	if checkin != "" {
		params.Set("checkin", checkin)
	}
	if checkout != "" {
		params.Set("checkout", checkout)
	}
	if b.bookingComID != "" {
		params.Set("aid", b.bookingComID)
	}
	return "https://www.booking.com/searchresults.html?" + params.Encode()
}

// ActivityURL returns a GetYourGuide activity search URL with affiliate tracking.
// query is a search term like "walking tour Prague" or "cooking class Tokyo".
func (b *LinkBuilder) ActivityURL(query string) string {
	params := url.Values{}
	params.Set("q", query)
	if b.getYourGuideID != "" {
		params.Set("partner_id", b.getYourGuideID)
	}
	return "https://www.getyourguide.com/s/?" + params.Encode()
}

// ViatorActivityURL returns a Viator activity search URL with affiliate tracking.
// query is a search term like "food tour Rome" or "snorkeling Bali".
func (b *LinkBuilder) ViatorActivityURL(query string) string {
	params := url.Values{}
	params.Set("text", query)
	if b.viatorID != "" {
		params.Set("pid", b.viatorID)
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
