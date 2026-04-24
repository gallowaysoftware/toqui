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
//     sources[0] and the caller pairs it with FTCDisclosure — the same
//     partner-link label free-tier users see on the same URL (#190
//     LB-4).
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
