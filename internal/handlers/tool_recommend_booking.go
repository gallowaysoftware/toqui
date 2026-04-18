package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gallowaysoftware/toqui-backend/internal/affiliate"
	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

// RecommendBookingTool is a chat tool that generates booking recommendations.
//
// Behaviour by tier:
//
//   - Free tier: returns the affiliate candidate (Skyscanner, Booking.com,
//     etc.) so we earn a commission on conversion. The user sees the
//     standard FTC disclosure ("This is a partner link…").
//
//   - Pro tier (and Explorer/Voyager): prefers a non-affiliate candidate
//     (Google Flights, Google Maps, Wikivoyage, Google search) so the
//     recommendation is genuinely commission-free. Every category in
//     sources.go currently exposes at least one non-affiliate candidate,
//     so today every Pro recommendation carries IndependentDisclosure
//     ("Toqui earns no commission on this link"). The selection logic in
//     SelectForPreference also handles the defensive case where a
//     category has no independent candidate (e.g. if a future change to
//     InsuranceSources removes the Google search fallback): the tool
//     would then return the affiliate source with ProDisclosure
//     ("Recommended for your trip. This is a partner link…") so the
//     partner-link nature is still disclosed honestly.
//
// The candidate pools and selection live in the affiliate package
// (sources.go, ranking.go) so the handler is just glue: pick category,
// fetch candidates, hand off to SelectForPreference, wrap the chosen
// Source as a Recommendation with the right disclosure.
// analyticsTracker is the slice of the analytics client this tool needs.
// Defining a tiny interface here (instead of taking the concrete client)
// lets unit tests inject a recording stub to assert which events fire and
// which properties they carry — important because affiliate_link_generated
// is privacy-sensitive (CLAUDE.md forbids logging destination names).
type analyticsTracker interface {
	Track(userID, event string, properties map[string]any)
}

type RecommendBookingTool struct {
	linkBuilder     *affiliate.LinkBuilder
	userTier        tier.UserTier
	onRecommend     func(rec affiliate.Recommendation)
	tripDestination string // fallback destination from trip context
	tripStartDate   string // fallback start date (YYYY-MM-DD)
	tripEndDate     string // fallback end date (YYYY-MM-DD)
	tripID          string // raw trip ID for sub-ID hashing
	userID          string // raw user ID for analytics (hashed before sending)
	analyticsClient analyticsTracker
}

type recommendBookingArgs struct {
	Category     string `json:"category"`
	Query        string `json:"query"`
	Origin       string `json:"origin"`
	Destination  string `json:"destination"`
	PropertyName string `json:"property_name"`
	DateFrom     string `json:"date_from"`
	DateTo       string `json:"date_to"`
}

// NewRecommendBookingTool creates a recommend_booking tool with the given
// affiliate link builder, user tier, and an optional callback invoked on each
// recommendation.
func NewRecommendBookingTool(linkBuilder *affiliate.LinkBuilder, userTier tier.UserTier, onRecommend func(rec affiliate.Recommendation)) *RecommendBookingTool {
	return &RecommendBookingTool{
		linkBuilder: linkBuilder,
		userTier:    userTier,
		onRecommend: onRecommend,
	}
}

// WithTripContext sets fallback destination/dates and trip ID from the current
// trip so affiliate URLs are pre-populated when the AI doesn't specify them
// (#176). The tripID is hashed for sub-ID tracking in affiliate URLs.
func (t *RecommendBookingTool) WithTripContext(destination, startDate, endDate, tripID string) *RecommendBookingTool {
	cp := *t
	cp.tripDestination = destination
	cp.tripStartDate = startDate
	cp.tripEndDate = endDate
	cp.tripID = tripID
	return &cp
}

// WithAnalytics configures event tracking for affiliate link generation.
// The tracker parameter is an interface (analyticsTracker) rather than the
// concrete *analytics.Client so unit tests can inject a recording stub to
// assert which events fire and which properties they carry. *analytics.Client
// satisfies the interface, so production call sites pass it as before.
func (t *RecommendBookingTool) WithAnalytics(tracker analyticsTracker, userID string) *RecommendBookingTool {
	cp := *t
	cp.analyticsClient = tracker
	cp.userID = userID
	return &cp
}

func (t *RecommendBookingTool) Definition() ai.ToolDefinition {
	// Pro-tier callers receive a candidate pool that includes non-affiliate
	// options (Google Flights, Google Maps, Wikivoyage, Google search) and
	// the tool prefers the non-affiliate source. Telling the AI this matters:
	// the model adapts its phrasing ("here's an independent search you can
	// use…") and uses the IndependentDisclosure text the tool returns rather
	// than defaulting to partner-link language. The AI does not need to
	// reason about which source was picked — the tool returns the disclosure
	// string already matched to the URL, and the AI must include it verbatim.
	description := "Generate affiliate-linked booking recommendations. Use when the user asks about flights, hotels, activities, car rentals, or travel insurance. Returns a single recommendation with the search URL and a disclosure string — always include the disclosure verbatim in your reply to the user."
	if t.userTier.IsPro() {
		description = "Generate a booking recommendation that prefers commission-free sources. Use when the user asks about flights, hotels, activities, car rentals, or travel insurance. For Pro users this picks an independent source (Google Flights, Google Maps, Wikivoyage) when one is available, falling back to an affiliate partner only when no independent option exists. Returns a single recommendation with the search URL and a disclosure string — always include the disclosure verbatim in your reply to the user."
	}

	return ai.ToolDefinition{
		Name:        "recommend_booking",
		Description: description,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"category": {
					"type": "string",
					"enum": ["flight", "hotel", "vacation_rental", "activity", "car_rental", "insurance"],
					"description": "Type of booking to recommend. Use 'hotel' for hotels (Booking.com — best global coverage) and 'vacation_rental' for houses/cabins/villas/longer stays (VRBO — vacation-home inventory Booking.com lacks). Pick based on the user's intent: 'we want a house for a week' → vacation_rental; 'find me a hotel in Rome' → hotel."
				},
				"query": {
					"type": "string",
					"description": "Search description, e.g., 'flights from NYC to Prague in June' or 'hotels in Reykjavik'"
				},
				"origin": {
					"type": "string",
					"description": "For flights: origin city or airport code"
				},
				"destination": {
					"type": "string",
					"description": "Destination city or airport code"
				},
				"property_name": {
					"type": "string",
					"description": "For hotels: the specific property name if the user mentioned one (e.g. 'St. Regis Vommuli'). Leave empty for generic destination searches."
				},
				"date_from": {
					"type": "string",
					"description": "Start date (YYYY-MM-DD)"
				},
				"date_to": {
					"type": "string",
					"description": "End date (YYYY-MM-DD)"
				}
			},
			"required": ["category", "query"]
		}`),
	}
}

func (t *RecommendBookingTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params recommendBookingArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	if params.Category == "" {
		return nil, fmt.Errorf("category is required")
	}
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	slog.Debug("recommend_booking tool executing",
		"category", params.Category,
		"query", params.Query,
		"origin", params.Origin,
		"destination", params.Destination,
		"date_from", params.DateFrom,
		"date_to", params.DateTo,
		"user_tier", string(t.userTier),
	)

	rec, isAffiliate := t.buildRecommendation(params)

	// Track affiliate link generation (async, non-blocking, privacy-safe).
	// Gate on the source's IsAffiliate flag — the URL the user actually
	// sees — rather than comparing the disclosure string, so a future
	// rewording of IndependentDisclosure can't silently flip the analytics
	// behaviour. Pro-tier users selecting an independent source (every
	// category today) won't trigger this event; Pro users falling back to
	// an affiliate source (the defensive path in SelectForPreference) will.
	//
	// Properties are intentionally minimal — partner / category / tier.
	// CLAUDE.md privacy rules forbid logging destination names ("travel
	// data is inherently sensitive under GDPR Article 9"), so we never
	// include the destination country/city/region in this event.
	if t.analyticsClient != nil && isAffiliate {
		props := map[string]any{
			"partner":  string(rec.Partner),
			"category": rec.Category,
			"tier":     string(t.userTier),
		}
		t.analyticsClient.Track(t.userID, "affiliate_link_generated", props)
	}

	if t.onRecommend != nil {
		t.onRecommend(rec)
	}

	return json.Marshal(rec)
}

// buildRecommendation collects the per-category candidate sources, applies
// tier-aware ranking, and wraps the selected Source as a Recommendation with
// the appropriate disclosure. It also returns isAffiliate so the caller can
// gate analytics on the actual selected-source flag (rather than re-deriving
// it from the disclosure string, which couples analytics behaviour to
// user-visible copy).
//
// The category-specific arguments (origin, dest, dates, property name) follow
// the same fallback semantics they did in the legacy single-URL path: missing
// origin → "anywhere", missing destination → trip context → "anywhere",
// missing date → trip context → "anytime". The fallbacks live here so the
// affiliate package's source builders can stay free of trip-context coupling.
func (t *RecommendBookingTool) buildRecommendation(params recommendBookingArgs) (affiliate.Recommendation, bool) {
	tripHash := affiliate.HashTripID(t.tripID)

	var sources []affiliate.Source
	var fallbackTitle, fallbackDescription string
	var fallbackPartner = affiliate.PartnerForCategory(params.Category)

	switch params.Category {
	case "flight":
		origin := params.Origin
		if origin == "" {
			origin = "anywhere"
		}
		dest := params.Destination
		if dest == "" {
			dest = t.tripDestination
		}
		if dest == "" {
			dest = "anywhere"
		}
		date := params.DateFrom
		if date == "" {
			date = t.tripStartDate
		}
		if date == "" {
			date = "anytime"
		}
		sources = t.linkBuilder.FlightSources(origin, dest, date, tripHash)
		fallbackTitle = fmt.Sprintf("Search flights: %s to %s", origin, dest)
		fallbackDescription = fmt.Sprintf("Find and compare flight prices from %s to %s", origin, dest)
		if params.DateFrom != "" {
			fallbackDescription += fmt.Sprintf(" departing %s", params.DateFrom)
		}

	case "hotel":
		city := params.Destination
		if city == "" {
			city = t.tripDestination
		}
		if city == "" {
			city = "your destination"
		}
		sources = t.linkBuilder.HotelSources(params.PropertyName, city, params.DateFrom, params.DateTo, tripHash)
		if params.PropertyName != "" {
			fallbackTitle = fmt.Sprintf("Book %s", params.PropertyName)
			fallbackDescription = fmt.Sprintf("View rates and availability for %s in %s", params.PropertyName, city)
		} else {
			fallbackTitle = fmt.Sprintf("Search hotels in %s", city)
			fallbackDescription = fmt.Sprintf("Browse and compare hotel options in %s", city)
		}
		if params.DateFrom != "" && params.DateTo != "" {
			fallbackDescription += fmt.Sprintf(" from %s to %s", params.DateFrom, params.DateTo)
		}

	case "vacation_rental":
		city := params.Destination
		if city == "" {
			city = t.tripDestination
		}
		if city == "" {
			city = "your destination"
		}
		sources = t.linkBuilder.VacationRentalSources(city, params.DateFrom, params.DateTo, tripHash)
		fallbackTitle = fmt.Sprintf("Vacation rentals in %s", city)
		fallbackDescription = fmt.Sprintf("Browse houses, cabins, and villas in %s", city)
		if params.DateFrom != "" && params.DateTo != "" {
			fallbackDescription += fmt.Sprintf(" from %s to %s", params.DateFrom, params.DateTo)
		}

	case "activity":
		city := params.Destination
		if city == "" {
			city = t.tripDestination
		}
		sources = t.linkBuilder.ActivitySources(params.Query, city, tripHash)
		fallbackTitle = fmt.Sprintf("Search activities: %s", params.Query)
		fallbackDescription = fmt.Sprintf("Discover tours, experiences, and activities: %s", params.Query)

	case "car_rental":
		location := params.Destination
		if location == "" {
			location = t.tripDestination
		}
		if location == "" {
			location = "your destination"
		}
		sources = t.linkBuilder.CarRentalSources(location, params.DateFrom, params.DateTo)
		fallbackTitle = fmt.Sprintf("Search car rentals in %s", location)
		fallbackDescription = fmt.Sprintf("Compare car rental prices and options in %s", location)
		if params.DateFrom != "" && params.DateTo != "" {
			fallbackDescription += fmt.Sprintf(" from %s to %s", params.DateFrom, params.DateTo)
		}

	case "insurance":
		dest := params.Destination
		if dest == "" {
			dest = t.tripDestination
		}
		if dest == "" {
			dest = "your trip"
		}
		sources = t.linkBuilder.InsuranceSources(dest)
		fallbackTitle = fmt.Sprintf("Travel insurance for %s", dest)
		fallbackDescription = fmt.Sprintf("Get travel medical insurance coverage for %s with SafetyWing Nomad Insurance", dest)

	default:
		// Unknown category — should not happen given the enum constraint on
		// the tool parameters. Return an empty-URL recommendation so the AI
		// can still respond gracefully. No URL means no affiliate link, so
		// isAffiliate=false (don't fire the affiliate_link_generated event
		// for a recommendation that doesn't actually link anywhere).
		return affiliate.Recommendation{
			Partner:     fallbackPartner,
			Title:       params.Query,
			Description: params.Query,
			Category:    params.Category,
			Disclosure:  affiliate.FTCDisclosure,
		}, false
	}

	// Pick the source the user will actually see. Pro users prefer
	// non-affiliate; everyone else takes sources[0] (affiliate-first).
	preferNonAffiliate := t.userTier.IsPro()
	selected := affiliate.SelectForPreference(preferNonAffiliate, sources)

	// Defensive: if the source builder returned nothing, fall back to the
	// legacy partner-only behaviour so we never emit a Recommendation with a
	// blank URL. This is unreachable today (every source builder always
	// produces at least one candidate) but cheap insurance. isAffiliate is
	// false here because the URL itself is empty — there's nothing to track
	// as an affiliate link generation.
	if selected.URL == "" {
		return affiliate.Recommendation{
			Partner:     fallbackPartner,
			Title:       fallbackTitle,
			Description: fallbackDescription,
			Category:    params.Category,
			Disclosure:  affiliate.FTCDisclosure,
		}, false
	}

	// Use the selected source's title/description when present (they're
	// crafted by the source builder to fit the URL — e.g. Wikivoyage's
	// title says "Wikivoyage: <city>" rather than "Search flights"). Keep
	// the legacy fallbacks for safety.
	title := selected.Title
	if title == "" {
		title = fallbackTitle
	}
	description := selected.Description
	if description == "" {
		description = fallbackDescription
	}

	return affiliate.Recommendation{
		Partner:     selected.Partner,
		Title:       title,
		Description: description,
		URL:         selected.URL,
		Category:    params.Category,
		Disclosure:  affiliate.DisclosureFor(selected, preferNonAffiliate),
	}, selected.IsAffiliate
}
