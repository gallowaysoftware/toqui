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
//
// includePro controls whether Pro-tier-only sources are appended. When
// true, the slice gains ITA Matrix (Google's flight backend, the most
// powerful flight search tool publicly available) and Momondo (price
// breadth that Skyscanner sometimes misses). Both are non-affiliate so
// they're invisible to free-tier users — that's the marketed Pro "wider
// candidate pool" deliverable per toqui-backend#386.
func (b *LinkBuilder) FlightSources(origin, dest, date, tripIDHash string, includePro bool) []Source {
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

	if includePro {
		// Pro: ITA Matrix — Google's underlying flight search engine,
		// far more powerful than Google Flights for complex routings
		// (codeshares, multi-city, fare-class filtering). No affiliate
		// program; deep-link uses the documented &p= path-segment
		// shape with a routing language string.
		itaQuery := url.Values{}
		itaQuery.Set("p", fmt.Sprintf("%s %s %s", origin, dest, date))
		out = append(out, Source{
			ID:          "ita_matrix",
			Partner:     PartnerITAMatrix,
			IsAffiliate: false,
			URL:         "https://matrix.itasoftware.com/search?" + itaQuery.Encode(),
			Title:       fmt.Sprintf("ITA Matrix advanced search: %s to %s", origin, dest),
			Description: "Pro: deep flight search via ITA Matrix (Google's flight backend) — independent, no commission.",
		})
		// Pro: Momondo — different inventory mix from Skyscanner, often
		// surfaces fares neither finds. Plain destination-search URL,
		// no tracking ID.
		mqQuery := url.Values{}
		mqQuery.Set("Search", "true")
		mqQuery.Set("TripType", "2") // round-trip
		mqQuery.Set("SegNo", "2")
		mqQuery.Set("SO0", origin)
		mqQuery.Set("SD0", dest)
		if date != "" && date != "anytime" {
			mqQuery.Set("SDP0", date)
		}
		out = append(out, Source{
			ID:          "momondo",
			Partner:     PartnerMomondo,
			IsAffiliate: false,
			URL:         "https://www.momondo.com/flightsearch?" + mqQuery.Encode(),
			Title:       fmt.Sprintf("Momondo: %s to %s", origin, dest),
			Description: "Pro: Momondo flight comparison — different inventory mix from Skyscanner.",
		})
	}

	return out
}

// HotelSources returns the candidate sources for a hotel search.
// propertyName takes precedence over city when non-empty (for deep-linked
// property searches). Ordered affiliate-first (Booking.com), with Google
// Maps hotels as the independent fallback. When includePro=true, adds
// Hotellook (multi-aggregator meta-search) — Pro users get price
// comparison across booking platforms in one extra source per
// toqui-backend#386.
func (b *LinkBuilder) HotelSources(propertyName, city, checkin, checkout, tripIDHash string, includePro bool) []Source {
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

	if includePro {
		// Pro: Hotellook — meta-search aggregating Booking.com, Hotels.com,
		// Agoda, Hostelworld, etc. Different inventory shape than
		// Booking.com alone. No affiliate ID configured here so Pro
		// users see truly independent comparison; if we later sign on
		// with TravelPayouts, flip IsAffiliate=true and add the marker
		// query param.
		hlSearch := searchStr
		if hlSearch == "" {
			hlSearch = "hotels"
		}
		hlParams := url.Values{}
		hlParams.Set("destination", hlSearch)
		if checkin != "" {
			hlParams.Set("checkIn", checkin)
		}
		if checkout != "" {
			hlParams.Set("checkOut", checkout)
		}
		out = append(out, Source{
			ID:          "hotellook",
			Partner:     PartnerHotellook,
			IsAffiliate: false,
			URL:         "https://search.hotellook.com/?" + hlParams.Encode(),
			Title:       fmt.Sprintf("Compare hotel prices: %s (Hotellook)", hlSearch),
			Description: "Pro: Hotellook compares prices across Booking.com, Hotels.com, Agoda, and others.",
		})
	}

	return out
}

// ActivitySources returns the candidate sources for an activity search.
// Ordered: GetYourGuide (affiliate), then Google Maps (independent), then
// Wikivoyage (independent, deep local knowledge). city may be empty — if
// so we fall back to the raw query. When includePro=true, adds Atlas
// Obscura and Time Out — editorial sources surfacing experiences that
// don't appear in commercial activity aggregators (#386).
func (b *LinkBuilder) ActivitySources(query, city, tripIDHash string, includePro bool) []Source {
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

	if includePro {
		// Pro: Atlas Obscura — editorial coverage of the unusual
		// experiences that GetYourGuide doesn't index (museum-of-X
		// type things, hidden ruins, tucked-away workshops). No
		// affiliate program; deep-link via search.
		aoQuery := query
		if city != "" {
			aoQuery = city + " " + query
		}
		out = append(out, Source{
			ID:          "atlas_obscura",
			Partner:     PartnerAtlasObscura,
			IsAffiliate: false,
			URL:         "https://www.atlasobscura.com/search?q=" + url.QueryEscape(strings.TrimSpace(aoQuery)),
			Title:       fmt.Sprintf("Atlas Obscura: %s", query),
			Description: "Pro: editorial coverage of unusual experiences — Toqui earns no commission.",
		})
		// Pro: Time Out city page — editorial things-to-do curated by
		// local editors. Major cities only; for unsupported cities
		// the link still 404-pages gracefully (Time Out renders a
		// search form). Slug shape is the city name lowercased and
		// hyphenated, but Time Out's URLs are fairly forgiving.
		if city != "" {
			toSlug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(city), " ", "-"))
			out = append(out, Source{
				ID:          "timeout",
				Partner:     PartnerTimeOut,
				IsAffiliate: false,
				URL:         "https://www.timeout.com/" + url.PathEscape(toSlug),
				Title:       fmt.Sprintf("Time Out %s", city),
				Description: fmt.Sprintf("Pro: locally-edited things to do in %s — independent of any booking aggregator.", city),
			})
		}
	}

	return out
}

// CarRentalSources returns the candidate sources for a car rental search.
// Ordered: DiscoverCars (affiliate), then Google Maps (independent). When
// includePro=true, adds Turo (peer-to-peer rental — different inventory
// shape from agency rentals) and AutoEurope (specialty broker, strong in
// Europe).
func (b *LinkBuilder) CarRentalSources(location, pickupDate, dropoffDate string, includePro bool) []Source {
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

	if includePro {
		// Pro: Turo — peer-to-peer car rental marketplace, different
		// inventory shape (specialty/luxury, no airport queue, often
		// cheaper for longer stays). No affiliate ID.
		turoParams := url.Values{}
		turoParams.Set("location", location)
		if pickupDate != "" {
			turoParams.Set("startDate", pickupDate)
		}
		if dropoffDate != "" {
			turoParams.Set("endDate", dropoffDate)
		}
		out = append(out, Source{
			ID:          "turo",
			Partner:     PartnerTuro,
			IsAffiliate: false,
			URL:         "https://turo.com/us/en/search?" + turoParams.Encode(),
			Title:       fmt.Sprintf("Turo peer-to-peer rentals in %s", location),
			Description: "Pro: peer-to-peer car rentals — no airport queue, often cheaper for longer stays.",
		})
		// Pro: AutoEurope — specialty broker, strong in Europe. Their
		// search URL takes "ToCity" as the pickup location.
		aeParams := url.Values{}
		aeParams.Set("ToCity", location)
		if pickupDate != "" {
			aeParams.Set("PickupDate", pickupDate)
		}
		if dropoffDate != "" {
			aeParams.Set("DropoffDate", dropoffDate)
		}
		out = append(out, Source{
			ID:          "auto_europe",
			Partner:     PartnerAutoEurope,
			IsAffiliate: false,
			URL:         "https://www.autoeurope.com/results?" + aeParams.Encode(),
			Title:       fmt.Sprintf("AutoEurope car rental in %s", location),
			Description: "Pro: specialty car-rental broker, strongest inventory in Europe.",
		})
	}

	return out
}

// InsuranceSources returns the candidate sources for travel insurance.
// Ordered: SafetyWing (affiliate), then a Google search (independent).
// Insurance is the category with the weakest independent alternative —
// there is no widely-used non-affiliate comparison site with a stable
// URL scheme for the free tier, so the independent option is a plain
// Google search. When includePro=true, adds Squaremouth and InsureMyTrip
// — comparison-shopping sites that show side-by-side quotes from
// multiple insurers (which is what users actually want to do here).
func (b *LinkBuilder) InsuranceSources(destination string, includePro bool) []Source {
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

	if includePro {
		// Pro: Squaremouth — side-by-side quote comparison from 25+
		// insurers, far better experience than a Google search. No
		// affiliate ID configured (commission-free for Pro users).
		smParams := url.Values{}
		if destination != "" {
			smParams.Set("destination", destination)
		}
		out = append(out, Source{
			ID:          "squaremouth",
			Partner:     PartnerSquaremouth,
			IsAffiliate: false,
			URL:         "https://www.squaremouth.com/?" + smParams.Encode(),
			Title:       fmt.Sprintf("Compare insurance quotes (Squaremouth): %s", destination),
			Description: "Pro: side-by-side quotes from 25+ insurers — Toqui earns no commission.",
		})
		// Pro: InsureMyTrip — same comparison-shop pattern, different
		// insurer mix.
		imtParams := url.Values{}
		if destination != "" {
			imtParams.Set("destination", destination)
		}
		out = append(out, Source{
			ID:          "insuremytrip",
			Partner:     PartnerInsureMyTrip,
			IsAffiliate: false,
			URL:         "https://www.insuremytrip.com/?" + imtParams.Encode(),
			Title:       fmt.Sprintf("Compare insurance quotes (InsureMyTrip): %s", destination),
			Description: "Pro: side-by-side quotes from a different insurer panel than Squaremouth.",
		})
	}

	return out
}

// VacationRentalSources returns the candidate sources for a vacation
// rental search (houses, cabins, villas — the segment Booking.com is
// weak in). Ordered: VRBO (affiliate via Partnerize), then a Google
// search (independent). When includePro=true, adds an Airbnb deep-link
// (scaffolded WITHOUT an affiliate ID until a separate Impact.com
// partnership is signed; Pro users get a plain commission-free Airbnb
// search until then).
func (b *LinkBuilder) VacationRentalSources(city, checkin, checkout, tripIDHash string, includePro bool) []Source {
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

	if includePro {
		// Pro: Airbnb — different inventory mix from VRBO (urban
		// apartments / unique stays vs. VRBO's house/cabin slant).
		// Scaffolded WITHOUT an affiliate ID — current Partnerize
		// publisher ID only covers Expedia-group brands. When/if an
		// Impact.com Airbnb partnership lands, flip IsAffiliate=true
		// and append the marker query param.
		abParams := url.Values{}
		if city != "" {
			abParams.Set("query", city)
		}
		if checkin != "" {
			abParams.Set("checkin", checkin)
		}
		if checkout != "" {
			abParams.Set("checkout", checkout)
		}
		out = append(out, Source{
			ID:          "airbnb",
			Partner:     PartnerAirbnb,
			IsAffiliate: false, // no affiliate ID; flip when Impact.com partnership signs
			URL:         "https://www.airbnb.com/s/homes?" + abParams.Encode(),
			Title:       fmt.Sprintf("Airbnb stays in %s", city),
			Description: "Pro: Airbnb apartments + unique stays — different inventory mix from VRBO.",
		})
	}

	return out
}
