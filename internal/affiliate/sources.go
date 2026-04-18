package affiliate

import (
	"fmt"
	"net/url"
	"strings"
)

// Source is a single booking-recommendation candidate. It bundles the
// generated URL with metadata the ranker and the caller need:
//
//   - Partner — which partner or independent source the URL points to.
//   - IsAffiliate — true iff the URL points at a commission-earning partner
//     domain (Skyscanner, Booking.com, GetYourGuide, DiscoverCars, SafetyWing).
//     This is a STATIC property of the partner, independent of whether the
//     tracking ID is plumbed through for this environment. An un-ID'd
//     booking.com URL is still a commercial booking aggregator; labeling
//     it "independent" would mislead the user. For the orthogonal
//     question "is our tracking ID configured?", see LinkBuilder.HasPartner.
//   - Title/Description — human-readable labels for the recommendation.
//
// Source is purely a value type. No I/O, no side effects. Sources are
// built by the LinkBuilder's per-category methods and then ranked by
// SelectForPreference (see ranking.go).
type Source struct {
	ID          string  // stable identifier, e.g. "skyscanner", "google_flights"
	Partner     Partner // which Partner this source represents
	IsAffiliate bool    // true iff this Partner is a commission-earning partner domain
	URL         string
	Title       string
	Description string
}

// FlightSources returns the candidate sources for a flight search. The
// slice is ordered affiliate-first so free-tier callers that take
// sources[0] get the current behaviour. origin, dest, and date follow
// the same semantics as FlightSearchURL. tripIDHash is appended as a
// sub-ID on the affiliate candidate for conversion attribution.
func (b *LinkBuilder) FlightSources(origin, dest, date, tripIDHash string) []Source {
	origin = strings.TrimSpace(origin)
	dest = strings.TrimSpace(dest)
	date = strings.TrimSpace(date)

	var out []Source

	// Affiliate candidate: Skyscanner. IsAffiliate is a static property
	// of the partner — skyscanner.com is a commercial flight aggregator
	// whether or not our associateid is plumbed in for this environment.
	// Conflating the two previously caused Pro-tier recommendations to
	// be labelled "Independent" on un-ID'd dev/staging envs (see PR #334
	// / follow-up to PR #332).
	skyURL := fmt.Sprintf("https://www.skyscanner.com/transport/flights/%s/%s/%s",
		url.PathEscape(origin), url.PathEscape(dest), url.PathEscape(date))
	if b.skyscannerID != "" {
		skyURL += "?associateid=" + url.QueryEscape(b.skyscannerID)
		if tripIDHash != "" {
			skyURL += "&utm_content=" + url.QueryEscape(tripIDHash)
		}
	}
	out = append(out, Source{
		ID:          "skyscanner",
		Partner:     PartnerSkyscanner,
		IsAffiliate: true,
		URL:         skyURL,
		Title:       fmt.Sprintf("Compare flights: %s to %s", origin, dest),
		Description: fmt.Sprintf("Skyscanner search from %s to %s", origin, dest),
	})

	// Independent candidate: Google Flights. Deterministic URL, no
	// affiliate tracking, no tracking ID. Quoting the human query in
	// the `q` param is the documented way to deep-link.
	gfQuery := fmt.Sprintf("Flights from %s to %s", origin, dest)
	if date != "" && date != "anytime" {
		gfQuery += " on " + date
	}
	out = append(out, Source{
		ID:          "google_flights",
		Partner:     PartnerGoogle,
		IsAffiliate: false,
		URL:         "https://www.google.com/travel/flights?q=" + url.QueryEscape(gfQuery),
		Title:       fmt.Sprintf("Search flights on Google Flights: %s to %s", origin, dest),
		Description: "Independent flight search on Google Flights — Toqui earns no commission.",
	})

	return out
}

// HotelSources returns the candidate sources for a hotel search.
// propertyName takes precedence over city when non-empty (for deep-linked
// property searches). Ordered affiliate-first (Booking.com), with Google
// Maps hotels as the independent fallback.
func (b *LinkBuilder) HotelSources(propertyName, city, checkin, checkout, tripIDHash string) []Source {
	propertyName = strings.TrimSpace(propertyName)
	city = strings.TrimSpace(city)
	searchStr := propertyName
	if searchStr == "" {
		searchStr = city
	}

	var out []Source

	// Affiliate: Booking.com.
	bkParams := url.Values{}
	bkParams.Set("ss", searchStr)
	if checkin != "" {
		bkParams.Set("checkin", checkin)
	}
	if checkout != "" {
		bkParams.Set("checkout", checkout)
	}
	if b.bookingComID != "" {
		bkParams.Set("aid", b.bookingComID)
		if tripIDHash != "" {
			bkParams.Set("label", tripIDHash)
		}
	}
	bkURL := "https://www.booking.com/searchresults.html?" + bkParams.Encode()
	bkTitle := fmt.Sprintf("Search hotels on Booking.com: %s", searchStr)
	if propertyName != "" {
		bkTitle = fmt.Sprintf("Find %s on Booking.com", propertyName)
	}
	out = append(out, Source{
		ID:          "booking_com",
		Partner:     PartnerBookingCom,
		IsAffiliate: true, // booking.com is always a commercial aggregator
		URL:         bkURL,
		Title:       bkTitle,
		Description: fmt.Sprintf("Booking.com results for %s", searchStr),
	})

	// Independent: Google Maps hotels. If the user named a specific
	// property we prefer a Google search for that name; otherwise a
	// hotels-in-CITY Maps search.
	var gURL, gTitle, gDesc string
	if propertyName != "" {
		gURL = "https://www.google.com/search?q=" + url.QueryEscape(propertyName+" hotel")
		gTitle = fmt.Sprintf("Find %s on Google", propertyName)
		gDesc = fmt.Sprintf("Independent Google search for %s — Toqui earns no commission.", propertyName)
	} else {
		gURL = "https://www.google.com/maps/search/" + url.PathEscape("hotels in "+city)
		gTitle = fmt.Sprintf("Hotels in %s on Google Maps", city)
		gDesc = fmt.Sprintf("Independent Google Maps search for hotels in %s — Toqui earns no commission.", city)
	}
	out = append(out, Source{
		ID:          "google_hotels",
		Partner:     PartnerGoogle,
		IsAffiliate: false,
		URL:         gURL,
		Title:       gTitle,
		Description: gDesc,
	})

	return out
}

// ActivitySources returns the candidate sources for an activity search.
// Ordered: GetYourGuide (affiliate), then Google Maps (independent), then
// Wikivoyage (independent, deep local knowledge). city may be empty — if
// so we fall back to the raw query.
func (b *LinkBuilder) ActivitySources(query, city, tripIDHash string) []Source {
	query = strings.TrimSpace(query)
	city = strings.TrimSpace(city)

	var out []Source

	// Affiliate: GetYourGuide.
	gygParams := url.Values{}
	gygParams.Set("q", query)
	if b.getYourGuideID != "" {
		gygParams.Set("partner_id", b.getYourGuideID)
		if tripIDHash != "" {
			gygParams.Set("cmp", tripIDHash)
		}
	}
	out = append(out, Source{
		ID:          "getyourguide",
		Partner:     PartnerGetYourGuide,
		IsAffiliate: true, // getyourguide.com is always a commercial aggregator
		URL:         "https://www.getyourguide.com/s/?" + gygParams.Encode(),
		Title:       fmt.Sprintf("Search activities on GetYourGuide: %s", query),
		Description: fmt.Sprintf("GetYourGuide results for %s", query),
	})

	// Independent: Google Maps "things to do" search.
	mapsQ := "things to do"
	if city != "" {
		mapsQ = fmt.Sprintf("%s in %s", query, city)
	} else if query != "" {
		mapsQ = query
	}
	out = append(out, Source{
		ID:          "google_activities",
		Partner:     PartnerGoogle,
		IsAffiliate: false,
		URL:         "https://www.google.com/maps/search/" + url.PathEscape(mapsQ),
		Title:       fmt.Sprintf("Find %s on Google Maps", query),
		Description: "Independent Google Maps search — Toqui earns no commission.",
	})

	// Independent: Wikivoyage city page. Wikivoyage is a volunteer-run
	// travel wiki under a Creative Commons licence — genuinely
	// independent, no commercial pressure.
	if city != "" {
		// Wikivoyage page titles use underscores for spaces and keep
		// punctuation as-is; url.PathEscape handles the rest.
		slug := strings.ReplaceAll(city, " ", "_")
		out = append(out, Source{
			ID:          "wikivoyage",
			Partner:     PartnerWikivoyage,
			IsAffiliate: false,
			URL:         "https://en.wikivoyage.org/wiki/" + url.PathEscape(slug),
			Title:       fmt.Sprintf("Wikivoyage: %s", city),
			Description: fmt.Sprintf("Volunteer-written travel guide for %s — independent and ad-free.", city),
		})
	}

	return out
}

// CarRentalSources returns the candidate sources for a car rental search.
// Ordered: DiscoverCars (affiliate), then Google Maps (independent).
func (b *LinkBuilder) CarRentalSources(location, pickupDate, dropoffDate string) []Source {
	location = strings.TrimSpace(location)

	var out []Source

	// Affiliate: DiscoverCars.
	dcParams := url.Values{}
	dcParams.Set("location", location)
	if pickupDate != "" {
		dcParams.Set("pickup_date", pickupDate)
	}
	if dropoffDate != "" {
		dcParams.Set("dropoff_date", dropoffDate)
	}
	if b.discoverCarsID != "" {
		dcParams.Set("a_aid", b.discoverCarsID)
	}
	out = append(out,
		Source{
			ID:          "discovercars",
			Partner:     PartnerDiscoverCars,
			IsAffiliate: true, // discovercars.com is always a commercial aggregator
			URL:         "https://www.discovercars.com/?" + dcParams.Encode(),
			Title:       fmt.Sprintf("Compare car rentals in %s on DiscoverCars", location),
			Description: fmt.Sprintf("DiscoverCars search for %s", location),
		},
		// Independent: Google Maps "car rental near X".
		Source{
			ID:          "google_car_rental",
			Partner:     PartnerGoogle,
			IsAffiliate: false,
			URL:         "https://www.google.com/maps/search/" + url.PathEscape("car rental near "+location),
			Title:       fmt.Sprintf("Car rentals near %s on Google Maps", location),
			Description: "Independent Google Maps search — Toqui earns no commission.",
		},
	)

	return out
}

// InsuranceSources returns the candidate sources for travel insurance.
// Ordered: SafetyWing (affiliate), then a Google search (independent).
// Insurance is the category with the weakest independent alternative —
// there is no widely-used non-affiliate comparison site with a stable
// URL scheme, so the independent option is a plain Google search.
func (b *LinkBuilder) InsuranceSources(destination string) []Source {
	destination = strings.TrimSpace(destination)

	var out []Source

	// Affiliate: SafetyWing.
	swBase := "https://safetywing.com/nomad-insurance"
	swURL := swBase
	if b.safetyWingID != "" {
		swURL += "?referenceID=" + url.QueryEscape(b.safetyWingID)
	}
	out = append(out, Source{
		ID:          "safetywing",
		Partner:     PartnerSafetyWing,
		IsAffiliate: true, // safetywing.com is always a commercial partner
		URL:         swURL,
		Title:       fmt.Sprintf("Travel insurance for %s (SafetyWing)", destination),
		Description: fmt.Sprintf("SafetyWing Nomad Insurance covers %s.", destination),
	})

	// Independent: a Google search for travel insurance comparisons.
	q := "travel insurance"
	if destination != "" {
		q = fmt.Sprintf("travel insurance for %s", destination)
	}
	out = append(out, Source{
		ID:          "google_insurance",
		Partner:     PartnerGoogle,
		IsAffiliate: false,
		URL:         "https://www.google.com/search?q=" + url.QueryEscape(q),
		Title:       fmt.Sprintf("Compare travel insurance for %s on Google", destination),
		Description: "Independent Google search — Toqui earns no commission.",
	})

	return out
}

// VacationRentalSources returns the candidate sources for a vacation
// rental search (houses, cabins, villas — the segment Booking.com is
// weak in). Ordered: VRBO (affiliate via Partnerize), then a Google
// search (independent). Airbnb is deliberately omitted until we have a
// separate Impact.com partnership; the current Partnerize publisher ID
// covers Expedia-group brands only.
func (b *LinkBuilder) VacationRentalSources(city, checkin, checkout, tripIDHash string) []Source {
	city = strings.TrimSpace(city)
	checkin = strings.TrimSpace(checkin)
	checkout = strings.TrimSpace(checkout)

	var out []Source

	// Title formats degrade gracefully when city is empty — upstream
	// in the chat tool we fall back to "your destination", but source
	// builders are also called directly in tests and future callers,
	// so guard here against the "Vacation rentals in  (VRBO)"
	// double-space before committing to a Title/Description string.
	vrboTitle := "Vacation rentals (VRBO)"
	vrboDesc := "VRBO search for homes, cabins, and villas."
	if city != "" {
		vrboTitle = fmt.Sprintf("Vacation rentals in %s (VRBO)", city)
		vrboDesc = fmt.Sprintf("VRBO search for homes, cabins, and villas in %s.", city)
	}

	// Affiliate: VRBO via Partnerize. IsAffiliate is a static property
	// of the partner — vrbo.com is a commercial aggregator whether or
	// not our Partnerize camref is plumbed through for this env.
	out = append(out, Source{
		ID:          "vrbo",
		Partner:     PartnerVRBO,
		IsAffiliate: true,
		URL:         b.VacationRentalURL(city, checkin, checkout, tripIDHash),
		Title:       vrboTitle,
		Description: vrboDesc,
	})

	// Independent: Google search.
	q := "vacation rental"
	if city != "" {
		q = fmt.Sprintf("vacation rental %s", city)
	}
	if checkin != "" && checkout != "" {
		q += fmt.Sprintf(" %s to %s", checkin, checkout)
	}
	googleTitle := "Compare vacation rentals on Google"
	if city != "" {
		googleTitle = fmt.Sprintf("Compare vacation rentals in %s on Google", city)
	}
	out = append(out, Source{
		ID:          "google_vacation_rental",
		Partner:     PartnerGoogle,
		IsAffiliate: false,
		URL:         "https://www.google.com/search?q=" + url.QueryEscape(q),
		Title:       googleTitle,
		Description: "Independent Google search — Toqui earns no commission.",
	})

	return out
}
