package affiliate

import (
	"sort"
	"strings"
)

// ScoreContext bundles the request-time signals the ranker uses to weight
// candidate sources. It's intentionally a flat plain-data type with no
// dependencies on the tier / handlers packages so the affiliate package
// stays a leaf in the import graph (handlers compute these booleans and
// hand them in).
//
// Today's signals are deliberately minimal — affiliate preference and a
// pair of "do we have specific search terms" booleans. The ranker is
// designed so adding new signals (e.g. PriceSensitivity, IsLuxury,
// DurationDays) is a matter of adding a field and one weighting line in
// scoreOne; the public API stays stable.
type ScoreContext struct {
	// PreferNonAffiliate is the Pro-tier signal. When true, non-affiliate
	// sources get a large positive score bump so they outrank affiliate
	// candidates of comparable fit. When false (free tier) the bump is
	// reversed so affiliate sources lead — preserving free-tier
	// behaviour.
	PreferNonAffiliate bool

	// HasSpecificDates is true iff the caller passed a concrete date /
	// date-range to the source builder (vs the empty-string "anytime"
	// fallback). Search-engine-style sources (Google Flights, ITA Matrix,
	// Momondo) score better when dates are specific because that's what
	// they're built for; editorial sources (Atlas Obscura, Wikivoyage)
	// don't benefit either way.
	HasSpecificDates bool

	// HasSpecificCity is true iff a city name was supplied. City-keyed
	// editorial sources (Time Out, Atlas Obscura) need this to produce a
	// useful URL; without it they're either omitted by the source builder
	// or fall back to a generic landing page that ranks worse.
	HasSpecificCity bool
}

// ScoredSource pairs a Source with the score the ranker assigned and a
// short human-readable rationale (the per-component contributions joined
// with ", "). The rationale is what PR 3 will surface in the booking
// recommendation tool result so the AI can explain why a particular
// source was chosen.
type ScoredSource struct {
	Source
	Score     float64
	Rationale string
}

// ScoreSources ranks the candidate sources for a single category and
// returns them sorted from best fit to worst. Sources with equal scores
// keep their input order (sort.SliceStable), so the existing
// affiliate-first ordering inside `sources.go` continues to act as a
// deterministic tiebreaker.
//
// The ranker is a small linear combination of weighted signals — it is
// NOT a learning system. The weights are tuned to satisfy two
// requirements drawn from #386:
//
//  1. Free tier behaviour MUST stay identical to the pre-ranker world:
//     `ScoreSources(ctx{free}, sources)[0]` must be the same source that
//     `sources[0]` was. That's why the affiliate bump for free tier
//     (+0.5) is small enough that fit signals can't reorder a healthy
//     pool, but large enough to consistently lead a homogeneous one.
//
//  2. Pro tier MUST prefer non-affiliate UNLESS no non-affiliate source
//     exists, in which case the affiliate fallback wins (the same
//     defensive path SelectForPreference has always had — see #190 LB-4).
//     The non-affiliate bump (+1.5) is large enough to dominate every
//     other weighting line so this property holds even when fit signals
//     pull the other direction.
//
// Returning a sorted slice (instead of just the top pick) lets the
// caller surface the full ranking — useful for the AI explaining "I
// chose X over Y because…" in PR 3 — without re-running the scorer.
func ScoreSources(ctx ScoreContext, sources []Source) []ScoredSource {
	if len(sources) == 0 {
		return nil
	}
	out := make([]ScoredSource, len(sources))
	for i, s := range sources {
		out[i] = scoreOne(ctx, s)
	}
	// Stable sort: equal scores preserve input order, so affiliate-first
	// ordering inside sources.go is the natural tiebreaker on free tier.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})
	return out
}

// scoreOne computes the score for a single source. Each weighting line
// appends a short reason fragment so the joined rationale reads as a
// trace of why the score is what it is. The reason fragments are also
// the contract for PR 3's surfaced rationale, so changes here are
// observable to callers and AI prompts — keep them stable and concise.
func scoreOne(ctx ScoreContext, s Source) ScoredSource {
	var score float64
	var reasons []string

	// --- Affiliate preference (the dominant signal) ---
	switch {
	case ctx.PreferNonAffiliate && !s.IsAffiliate:
		score += 1.5
		reasons = append(reasons, "non-affiliate (Pro)")
	case ctx.PreferNonAffiliate && s.IsAffiliate:
		// No bump; affiliate sources are still ranked relative to each
		// other and to other affiliate sources by fit signals below.
		// We do NOT subtract here — the affiliate fallback still has
		// to win when no non-affiliate source exists.
		reasons = append(reasons, "affiliate (Pro)")
	case !ctx.PreferNonAffiliate && s.IsAffiliate:
		score += 0.5
		reasons = append(reasons, "affiliate (free)")
	default:
		// Free tier + non-affiliate. No bump; falls below affiliate
		// candidates of comparable fit, matching the pre-ranker
		// "sources[0] is affiliate" ordering.
		reasons = append(reasons, "non-affiliate (free)")
	}

	// --- Source-fit: dates ---
	// Search aggregators (Skyscanner, Google Flights, ITA Matrix,
	// Momondo, Hotellook, etc.) are built around dated queries. Editorial
	// sources (Atlas Obscura, Wikivoyage, Time Out) and partner-landing
	// pages (SafetyWing, GetYourGuide) don't degrade without a date.
	if ctx.HasSpecificDates && isSearchAggregator(s.Partner) {
		score += 0.3
		reasons = append(reasons, "dated query fits aggregator")
	}

	// --- Source-fit: city ---
	// Editorial city guides (Time Out, Atlas Obscura) are useless without
	// a city. We can't know that the URL was constructed with a city
	// from inside this function, so we lean on Partner identity: when
	// these partners appear in the candidate pool they were built with
	// a city (the source builders gate on city presence), so this fires
	// reliably.
	if ctx.HasSpecificCity && isCityCurated(s.Partner) {
		score += 0.4
		reasons = append(reasons, "city-curated content")
	}

	// --- Established vs scaffolded ---
	// Scaffolded partners (today only Airbnb, listed without an affiliate
	// ID until the Impact.com partnership lands) are penalized so they
	// don't outrank established options. Without this, a Pro user
	// asking for vacation rentals could be routed to an Airbnb URL we
	// can't track or attribute conversions on.
	if isScaffolded(s.Partner) {
		score -= 0.2
		reasons = append(reasons, "scaffolded partner")
	}

	// --- Pro-pool addition tiebreak ---
	// The 10 partners added in #386 PR 1 are the marketed Pro value-add
	// (ITA Matrix, Momondo, Hotellook, Atlas Obscura, Time Out,
	// Squaremouth, InsureMyTrip, Turo, Auto Europe, Airbnb). Without an
	// explicit boost they tied with the free-pool non-affiliate sources
	// (mostly Google) on Pro tier and lost on stable-sort input order —
	// so the marketing claim "Pro: Atlas Obscura editorial coverage"
	// was hollow because Wikivoyage always won the tie. The bump is
	// deliberately tiny (+0.05) so it ONLY moves things when scores
	// would otherwise tie; it can't outrank the affiliate-status
	// signal or the fit signals.
	//
	// Note: Airbnb is a Pro addition AND scaffolded — the -0.2
	// scaffolded penalty still dominates the +0.05 here, so Airbnb
	// continues to rank below established alternatives. By design.
	if isProAddition(s.Partner) {
		score += 0.05
		reasons = append(reasons, "Pro-pool addition")
	}

	return ScoredSource{
		Source:    s,
		Score:     score,
		Rationale: strings.Join(reasons, ", "),
	}
}

// isSearchAggregator reports whether the partner is primarily a
// search-engine-style aggregator that benefits from dated queries.
// Defined as a function (not a map) so the relationship between
// partners and aggregator-status lives next to the score weighting
// that depends on it.
func isSearchAggregator(p Partner) bool {
	switch p {
	case PartnerSkyscanner,
		PartnerBookingCom,
		PartnerDiscoverCars,
		PartnerExpedia,
		PartnerVRBO,
		PartnerGoogle, // Google Flights, Google Maps, Google search
		PartnerITAMatrix,
		PartnerMomondo,
		PartnerHotellook,
		PartnerSquaremouth,
		PartnerInsureMyTrip,
		PartnerAutoEurope,
		PartnerTuro:
		return true
	}
	return false
}

// isCityCurated reports whether the partner produces useful output only
// when a city name is supplied. These are editorial / city-guide sources
// rather than booking engines.
func isCityCurated(p Partner) bool {
	switch p {
	case PartnerAtlasObscura,
		PartnerTimeOut,
		PartnerWikivoyage:
		return true
	}
	return false
}

// isScaffolded reports whether the partner is in the candidate pool
// without a working affiliate ID / tracking, and so should be ranked
// below otherwise-equal established alternatives. Today this is only
// Airbnb (waiting on Impact.com partnership). Update this list when
// new partners are scaffolded; remove a partner from here the moment
// its tracking is plumbed.
func isScaffolded(p Partner) bool {
	return p == PartnerAirbnb
}

// isProAddition reports whether the partner is one of the 10 sources
// added in #386 PR 1 to widen the Pro candidate pool. These partners
// are only ever in the candidate pool when the source builder was
// called with includePro=true, so this predicate is effectively
// "is this partner unique to Pro tier?". The +0.05 tiebreak in
// scoreOne uses this list to ensure the marketed Pro additions
// outrank the free-pool non-affiliate sources (mostly Google) when
// scores would otherwise tie — without altering the dominant
// affiliate-status or fit signals.
//
// Add new partners here as they're added to the Pro pool.
func isProAddition(p Partner) bool {
	switch p {
	case PartnerITAMatrix,
		PartnerMomondo,
		PartnerHotellook,
		PartnerAtlasObscura,
		PartnerTimeOut,
		PartnerSquaremouth,
		PartnerInsureMyTrip,
		PartnerTuro,
		PartnerAutoEurope,
		PartnerAirbnb:
		return true
	}
	return false
}

// SelectForPreference picks a single Source from a candidate list built by
// the per-category source builders in sources.go.
//
// As of #386 PR 2 it is implemented as a thin wrapper around
// ScoreSources, preserving every previous behaviour:
//
//   - Empty input → zero Source (callers treat as "no recommendation").
//   - preferNonAffiliate=true → first non-affiliate source wins; if no
//     non-affiliate source exists, affiliate fallback wins (the
//     defensive path that DisclosureFor pairs with FTCDisclosure to
//     avoid #190 LB-4).
//   - preferNonAffiliate=false → sources[0] wins (affiliate-first
//     ordering inside sources.go is preserved by the stable sort).
//
// The new ranker is observable through ScoreSources — callers that
// want the full ordering / rationale (PR 3's tool-result surfacing)
// should call ScoreSources directly and skip this wrapper.
//
// Why a bool instead of a tier.UserTier argument? The affiliate package
// must stay a leaf in the import graph — it cannot depend on the tier
// package without introducing a cycle. Passing a plain bool keeps the
// policy decision at the call site (handlers) where tier information
// lives, and keeps this package focused on URL construction + ranking.
func SelectForPreference(preferNonAffiliate bool, sources []Source) Source {
	scored := ScoreSources(ScoreContext{PreferNonAffiliate: preferNonAffiliate}, sources)
	if len(scored) == 0 {
		return Source{}
	}
	return scored[0].Source
}

// DisclosureFor returns the disclosure text that should be displayed with
// the selected Source. The rule is deliberately simple and keyed ONLY off
// the source's IsAffiliate flag — NEVER the user tier:
//
//   - Non-affiliate source → IndependentDisclosure. Toqui earns no
//     commission; the user sees that explicitly.
//   - Affiliate source → FTCDisclosure. The partner-link label that
//     satisfies 16 CFR Part 255, irrespective of whether the caller is
//     free or Pro.
//
// The Pro-tier rule (`preferNonAffiliate`) already influenced WHICH
// source got selected via SelectForPreference. Once we've selected, the
// disclosure must match the URL, not the tier. A Pro user who gets
// bumped to the affiliate fallback (because the category has no
// independent option) is looking at a commission-earning URL and must
// be labelled accordingly — the previous ProDisclosure softened the
// label in a way that could under-disclose the commercial relationship.
//
// Finding: #190 LB-4. preferNonAffiliate is intentionally unused here
// so the bug can't come back via a future "well, just for Pro…" tweak.
// Kept in the signature for call-site stability.
func DisclosureFor(selected Source, _ bool) string {
	if !selected.IsAffiliate {
		return IndependentDisclosure
	}
	return FTCDisclosure
}
