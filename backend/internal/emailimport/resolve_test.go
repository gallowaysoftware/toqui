package emailimport

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

// stubLister returns canned trip lists. planning is returned for the
// status="planning" query; all is returned for the unfiltered query.
type stubLister struct {
	planning []dbgen.Trip
	all      []dbgen.Trip
}

func (s *stubLister) ListTripsByUserAndStatus(_ context.Context, _ dbgen.ListTripsByUserAndStatusParams) ([]dbgen.Trip, error) {
	return s.planning, nil
}

func (s *stubLister) ListTripsByUser(_ context.Context, _ dbgen.ListTripsByUserParams) ([]dbgen.Trip, error) {
	return s.all, nil
}

func trip(title string, created time.Time) dbgen.Trip {
	return dbgen.Trip{ID: uuid.New(), Title: title, CreatedAt: created}
}

func TestResolveTrip(t *testing.T) {
	now := time.Now()
	japan := trip("Japan Spring 2027", now.Add(-2*time.Hour))
	greece := trip("Greece", now.Add(-time.Hour)) // more recent planning
	oldRome := trip("Rome", now.Add(-100*time.Hour))

	ctx := context.Background()
	uid := uuid.New()

	t.Run("subject title match wins over recency", func(t *testing.T) {
		// Both trips are planning; Greece is more recent, but the subject
		// names Japan → Japan wins.
		s := &stubLister{
			planning: []dbgen.Trip{greece, japan},
			all:      []dbgen.Trip{greece, japan},
		}
		got := ResolveTrip(ctx, s, uid, "Fwd: Your flight to Japan Spring 2027 is confirmed")
		if got != japan.ID.String() {
			t.Errorf("got %s, want japan %s", got, japan.ID.String())
		}
	})

	t.Run("longest matching title wins", func(t *testing.T) {
		japanShort := trip("Japan", now.Add(-3*time.Hour))
		s := &stubLister{all: []dbgen.Trip{japanShort, japan}}
		got := ResolveTrip(ctx, s, uid, "booking for Japan Spring 2027")
		if got != japan.ID.String() {
			t.Errorf("got %s, want the longer 'Japan Spring 2027' %s", got, japan.ID.String())
		}
	})

	t.Run("whole-word only — no substring false match", func(t *testing.T) {
		rome := trip("Rome", now.Add(-time.Hour))
		fallback := trip("Beach Week", now) // most recent → expected fallback
		s := &stubLister{all: []dbgen.Trip{fallback, rome}}
		// "Chrome" contains "rome" as a substring but not as a whole word,
		// so it must NOT match the Rome trip — we fall back to most-recent.
		got := ResolveTrip(ctx, s, uid, "Your Chrome browser update is ready")
		if got == rome.ID.String() {
			t.Errorf("substring 'rome' inside 'Chrome' should not match the Rome trip")
		}
		if got != fallback.ID.String() {
			t.Errorf("got %s, want fallback %s", got, fallback.ID.String())
		}
	})

	t.Run("no subject match falls back to most recent planning", func(t *testing.T) {
		s := &stubLister{
			planning: []dbgen.Trip{greece, japan}, // greece first = most recent
			all:      []dbgen.Trip{greece, japan, oldRome},
		}
		got := ResolveTrip(ctx, s, uid, "Generic receipt, no trip named")
		if got != greece.ID.String() {
			t.Errorf("got %s, want most-recent-planning greece %s", got, greece.ID.String())
		}
	})

	t.Run("no planning trips falls back to most recent any", func(t *testing.T) {
		s := &stubLister{
			planning: nil,
			all:      []dbgen.Trip{oldRome},
		}
		got := ResolveTrip(ctx, s, uid, "Generic receipt")
		if got != oldRome.ID.String() {
			t.Errorf("got %s, want oldRome %s", got, oldRome.ID.String())
		}
	})

	t.Run("no trips at all returns empty (unattached)", func(t *testing.T) {
		s := &stubLister{}
		if got := ResolveTrip(ctx, s, uid, "anything"); got != "" {
			t.Errorf("got %s, want empty", got)
		}
	})

	t.Run("empty subject skips title match", func(t *testing.T) {
		s := &stubLister{planning: []dbgen.Trip{greece}}
		if got := ResolveTrip(ctx, s, uid, ""); got != greece.ID.String() {
			t.Errorf("got %s, want greece via planning fallback", got)
		}
	})

	t.Run("multibyte title matches at a real word boundary", func(t *testing.T) {
		// A CJK-titled trip must match when the subject names it, and the
		// rune-aware boundary check must not panic or mis-slice.
		nihon := trip("日本", now.Add(-time.Hour))
		s := &stubLister{all: []dbgen.Trip{nihon}}
		got := ResolveTrip(ctx, s, uid, "予約確認 日本 ホテル")
		if got != nihon.ID.String() {
			t.Errorf("got %s, want 日本 trip %s", got, nihon.ID.String())
		}
	})

	t.Run("accented title does not false-match a longer word", func(t *testing.T) {
		// "Café" must not match inside "Cafétéria" (é is multibyte, so the
		// byte-cast bug would have mishandled the boundary).
		cafe := trip("Café", now.Add(-time.Hour))
		fallback := trip("Somewhere", now)
		s := &stubLister{all: []dbgen.Trip{fallback, cafe}}
		got := ResolveTrip(ctx, s, uid, "Your Cafétéria order is ready")
		if got == cafe.ID.String() {
			t.Errorf("'Café' should not match inside 'Cafétéria'")
		}
	})
}
