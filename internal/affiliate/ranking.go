package affiliate

// SelectForPreference picks a single Source from a candidate list built by
// the per-category source builders in sources.go.
//
// Selection rules:
//
//   - If sources is empty, returns the zero Source. Callers should treat
//     this as "no recommendation available" and surface a graceful error.
//   - If preferNonAffiliate is true, returns the first Source with
//     IsAffiliate=false. This is the Pro-tier path — commission-free
//     sources (Google Flights, Google Maps, Wikivoyage, Google search)
//     take priority over affiliate partners. Today every category in
//     sources.go exposes at least one non-affiliate candidate, so this
//     branch always finds a match in production. The fallback below is
//     defensive insurance for future categories (or removed Google
//     options): if no non-affiliate source is available, it returns
//     sources[0] and the caller pairs it with ProDisclosure (the soft
//     "Recommended for your trip — partner link" label).
//   - Otherwise, returns sources[0]. The source builders order slices
//     affiliate-first, so this preserves the original free-tier
//     behaviour: the affiliate candidate is always selected when one
//     exists, maximising commission opportunities while still carrying
//     the FTC disclosure.
//
// Why a bool instead of a tier.UserTier argument? The affiliate package
// must stay a leaf in the import graph — it cannot depend on the tier
// package without introducing a cycle (tier → affiliate is already
// implied by callers). Passing a plain bool keeps the policy decision
// at the call site (handlers) where tier information lives, and keeps
// this package focused on URL construction + selection.
func SelectForPreference(preferNonAffiliate bool, sources []Source) Source {
	if len(sources) == 0 {
		return Source{}
	}
	if preferNonAffiliate {
		for _, s := range sources {
			if !s.IsAffiliate {
				return s
			}
		}
	}
	return sources[0]
}

// DisclosureFor returns the disclosure text that should be displayed with
// the selected Source. The rules encode the honest-labelling policy:
//
//   - Non-affiliate source → IndependentDisclosure. Toqui earns no
//     commission; the user sees that explicitly.
//   - Affiliate source + Pro-tier caller → ProDisclosure. Softer framing
//     for Pro users ("Recommended for your trip") but the partner-link
//     nature is still disclosed.
//   - Affiliate source + free-tier caller → FTCDisclosure. The standard
//     "This is a partner link" label that satisfies 16 CFR Part 255.
//
// Callers pass `preferNonAffiliate` (i.e. tier.IsPro()) so this function
// can pick between ProDisclosure and FTCDisclosure for affiliate sources
// without needing to know about the tier package.
func DisclosureFor(selected Source, preferNonAffiliate bool) string {
	if !selected.IsAffiliate {
		return IndependentDisclosure
	}
	if preferNonAffiliate {
		return ProDisclosure
	}
	return FTCDisclosure
}
