// Package emailimport resolves which trip a forwarded/imported booking
// email belongs to. It is shared by the IngestEmail RPC (the user pastes or
// forwards a booking confirmation in-app) and the IMAP poller (a dedicated
// mailbox the user forwards confirmations to).
package emailimport

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

// tripLister is the slice of *dbgen.Queries the resolver needs. Kept small
// so tests can inject a stub without Postgres, matching the fail-loud
// test-double pattern used across the backend.
type tripLister interface {
	ListTripsByUserAndStatus(ctx context.Context, arg dbgen.ListTripsByUserAndStatusParams) ([]dbgen.Trip, error)
	ListTripsByUser(ctx context.Context, arg dbgen.ListTripsByUserParams) ([]dbgen.Trip, error)
}

var _ tripLister = (*dbgen.Queries)(nil)

// lookbackTrips bounds how many of a user's trips we scan for a subject
// title match. Someone with hundreds of trips still gets their recent ones
// considered; the fallback covers the rest.
const lookbackTrips = 50

// ResolveTrip picks the trip a booking email should attach to, for the
// given user, using this precedence:
//  1. A trip whose title appears (word-boundary, case-insensitive) in the
//     email subject — the longest such title wins, so "Japan Spring 2027"
//     beats a stray "Japan".
//  2. The most recently created trip still in "planning".
//  3. The most recently created trip of any status.
//  4. "" — no trips at all, so the booking is created unattached.
//
// It never fails the import: a query error degrades to the next tier and,
// ultimately, to an unattached booking.
func ResolveTrip(ctx context.Context, q tripLister, userID uuid.UUID, subject string) string {
	planning, _ := q.ListTripsByUserAndStatus(ctx, dbgen.ListTripsByUserAndStatusParams{
		UserID: userID,
		Status: "planning",
		Limit:  lookbackTrips,
		Offset: 0,
	})
	recent, _ := q.ListTripsByUser(ctx, dbgen.ListTripsByUserParams{
		UserID: userID,
		Limit:  lookbackTrips,
		Offset: 0,
	})

	// 1. Subject title match across all recent trips (planning + others).
	if match := bestTitleMatch(subject, recent); match != "" {
		return match
	}

	// 2. Most recent planning trip.
	if len(planning) > 0 {
		return planning[0].ID.String()
	}

	// 3. Most recent trip of any status.
	if len(recent) > 0 {
		return recent[0].ID.String()
	}

	// 4. Unattached.
	return ""
}

// bestTitleMatch returns the ID of the trip whose title is the longest one
// found as a whole-word phrase in subject, or "" if none match.
func bestTitleMatch(subject string, trips []dbgen.Trip) string {
	if strings.TrimSpace(subject) == "" {
		return ""
	}
	lowerSubject := strings.ToLower(subject)
	var bestID string
	bestLen := 0
	for _, t := range trips {
		title := strings.ToLower(strings.TrimSpace(t.Title))
		if title == "" || len(title) <= bestLen {
			continue
		}
		if containsWholePhrase(lowerSubject, title) {
			bestID = t.ID.String()
			bestLen = len(title)
		}
	}
	return bestID
}

// containsWholePhrase reports whether phrase occurs in s bounded by
// non-alphanumeric characters (or string edges), so "Rome" matches
// "Trip to Rome!" but not "Metronome". Both args must already be lowercase.
func containsWholePhrase(s, phrase string) bool {
	from := 0
	for {
		i := strings.Index(s[from:], phrase)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(phrase)
		// Decode the neighbouring runes (not bytes) so multibyte titles /
		// subjects (accented, CJK) get a correct word boundary.
		leftOK := start == 0
		if !leftOK {
			r, _ := utf8.DecodeLastRuneInString(s[:start])
			leftOK = !isWordChar(r)
		}
		rightOK := end == len(s)
		if !rightOK {
			r, _ := utf8.DecodeRuneInString(s[end:])
			rightOK = !isWordChar(r)
		}
		if leftOK && rightOK {
			return true
		}
		from = start + 1
		if from >= len(s) {
			return false
		}
	}
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
