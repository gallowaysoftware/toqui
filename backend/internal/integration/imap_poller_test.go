//go:build integration

package integration

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui/backend/internal/ai"
	"github.com/gallowaysoftware/toqui/backend/internal/booking"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui/backend/internal/emailimport"
	"github.com/gallowaysoftware/toqui/backend/internal/handlers"
)

const (
	imapUser = "toqui-inbox"
	imapPass = "s3cret"
)

// startMemIMAP spins up an in-memory IMAP server (plaintext, InsecureAuth)
// on a random local port, returns its host:port, and appends the given raw
// messages to INBOX.
func startMemIMAP(t *testing.T, rawMessages ...string) (host string, port int) {
	t.Helper()

	memServer := imapmemserver.New()
	memUser := imapmemserver.NewUser(imapUser, imapPass)
	if err := memUser.Create("INBOX", nil); err != nil {
		t.Fatalf("create INBOX: %v", err)
	}
	memServer.AddUser(memUser)

	server := imapserver.New(&imapserver.Options{
		NewSession: func(_ *imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memServer.NewSession(), nil, nil
		},
		InsecureAuth: true,
		Caps: imap.CapSet{
			imap.CapIMAP4rev1: {},
			imap.CapIMAP4rev2: {},
		},
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(func() { _ = server.Close() })

	addr := ln.Addr().(*net.TCPAddr)

	// Seed INBOX via a throwaway client.
	seed, err := imapclient.DialInsecure(addr.String(), nil)
	if err != nil {
		t.Fatalf("seed dial: %v", err)
	}
	if err := seed.Login(imapUser, imapPass).Wait(); err != nil {
		t.Fatalf("seed login: %v", err)
	}
	for _, raw := range rawMessages {
		cmd := seed.Append("INBOX", int64(len(raw)), nil)
		if _, err := cmd.Write([]byte(raw)); err != nil {
			t.Fatalf("append write: %v", err)
		}
		if err := cmd.Close(); err != nil {
			t.Fatalf("append close: %v", err)
		}
		if _, err := cmd.Wait(); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	_ = seed.Logout().Wait()
	_ = seed.Close()

	return addr.IP.String(), addr.Port
}

// TestIMAPPoller_EndToEnd drives the full poller loop against a live
// in-memory IMAP server + Postgres: it seeds a forwarded booking email,
// runs one poll cycle, and asserts the booking was imported for the sender
// and the message was marked \Seen.
func TestIMAPPoller_EndToEnd(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)

	// Toqui user whose account email matches the forwarding sender.
	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "g_imap_poll", Valid: true},
		Email:    "traveler@example.com",
		Name:     pgtype.Text{String: "Traveler", Valid: true},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := queries.CreateTrip(ctx, dbgen.CreateTripParams{UserID: user.ID, Title: "Barcelona Trip"}); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	forwarded := "From: Traveler <traveler@example.com>\r\n" +
		"Subject: Fwd: Your Barcelona Trip hotel is booked\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Confirmation HTL-7788, Hotel Arts Barcelona, check-in 2027-05-02.\r\n"
	// A second message from a stranger — must be skipped but still marked seen.
	stranger := "From: spam@nowhere.com\r\n" +
		"Subject: hi\r\n" +
		"Content-Type: text/plain\r\n\r\nnot a booking\r\n"

	host, port := startMemIMAP(t, forwarded, stranger)

	parsedJSON := `{"type":"hotel","confirmation_code":"HTL-7788","provider":"Hotel Arts","title":"Hotel Arts Barcelona","start_time":"2027-05-02T15:00:00Z"}`
	svc := booking.NewService(env.Pool, &cannedAIProvider{json: parsedJSON})
	handler := handlers.NewBookingHandler(svc, queries)

	poller := emailimport.NewPoller(emailimport.IMAPConfig{
		Host:     host,
		Port:     port,
		Username: imapUser,
		Password: imapPass,
		Mailbox:  "INBOX",
		TLS:      false,
	}, queries, handler)

	poller.PollOnce(ctx)

	countBookings := func() int {
		var n int
		if err := env.Pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM bookings WHERE user_id = $1 AND source = 'email' AND confirmation_code = 'HTL-7788'", user.ID,
		).Scan(&n); err != nil {
			t.Fatalf("count bookings: %v", err)
		}
		return n
	}

	// A booking was imported for the sender, source=email.
	if got := countBookings(); got != 1 {
		t.Fatalf("expected 1 imported email booking, got %d", got)
	}

	// A second cycle must NOT re-import: both messages are \Seen, so the
	// UNSEEN search skips them and the booking count stays 1.
	poller.PollOnce(ctx)
	if got := countBookings(); got != 1 {
		t.Errorf("second poll re-imported: booking count = %d, want 1", got)
	}

	// Both messages are now \Seen (the booking imported; the stranger skipped
	// but still marked so it isn't reprocessed forever).
	verify, err := imapclient.DialInsecure(net.JoinHostPort(host, strconv.Itoa(port)), nil)
	if err != nil {
		t.Fatalf("verify dial: %v", err)
	}
	defer func() { _ = verify.Logout().Wait(); _ = verify.Close() }()
	if err := verify.Login(imapUser, imapPass).Wait(); err != nil {
		t.Fatalf("verify login: %v", err)
	}
	if _, err := verify.Select("INBOX", nil).Wait(); err != nil {
		t.Fatalf("verify select: %v", err)
	}
	sd, err := verify.UIDSearch(&imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}, nil).Wait()
	if err != nil {
		t.Fatalf("verify search unseen: %v", err)
	}
	if n := len(sd.AllUIDs()); n != 0 {
		t.Errorf("expected 0 unseen messages after poll, got %d", n)
	}
}

// failingAIProvider always emits a stream error, so IngestEmail fails with a
// transient (Internal) code — used to exercise the poller's retry cap.
type failingAIProvider struct{ calls int }

func (f *failingAIProvider) ChatStream(_ context.Context, _ *ai.ChatRequest) (<-chan ai.Event, error) {
	f.calls++
	ch := make(chan ai.Event, 1)
	ch <- ai.Event{Type: ai.EventError, Error: errors.New("provider down")}
	close(ch)
	return ch, nil
}

func (f *failingAIProvider) Name() string { return "failing" }

// TestIMAPPoller_DeadLettersAfterRetryCap verifies that a message which keeps
// failing transiently is retried a bounded number of times and then marked
// \Seen (dead-lettered), rather than re-hitting the AI every cycle forever.
func TestIMAPPoller_DeadLettersAfterRetryCap(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)

	_, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "g_deadletter", Valid: true},
		Email:    "traveler@example.com",
		Name:     pgtype.Text{String: "Traveler", Valid: true},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	msg := "From: traveler@example.com\r\nSubject: Booking\r\n" +
		"Content-Type: text/plain\r\n\r\nConfirmation ABC\r\n"
	host, port := startMemIMAP(t, msg)

	failing := &failingAIProvider{}
	svc := booking.NewService(env.Pool, failing)
	handler := handlers.NewBookingHandler(svc, queries)
	poller := emailimport.NewPoller(emailimport.IMAPConfig{
		Host: host, Port: port, Username: imapUser, Password: imapPass, Mailbox: "INBOX", TLS: false,
	}, queries, handler)

	// Poll more than the cap; the message should be retried up to the cap
	// and then dead-lettered.
	for i := 0; i < 8; i++ {
		poller.PollOnce(ctx)
	}

	// The AI was called at most the cap number of times, not 8.
	if failing.calls == 0 || failing.calls > 5 {
		t.Errorf("AI called %d times; want 1..5 (bounded by the retry cap)", failing.calls)
	}

	// The message is now \Seen (dead-lettered) — no unseen mail remains.
	verify, err := imapclient.DialInsecure(net.JoinHostPort(host, strconv.Itoa(port)), nil)
	if err != nil {
		t.Fatalf("verify dial: %v", err)
	}
	defer func() { _ = verify.Logout().Wait(); _ = verify.Close() }()
	if err := verify.Login(imapUser, imapPass).Wait(); err != nil {
		t.Fatalf("verify login: %v", err)
	}
	if _, err := verify.Select("INBOX", nil).Wait(); err != nil {
		t.Fatalf("verify select: %v", err)
	}
	sd, err := verify.UIDSearch(&imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}, nil).Wait()
	if err != nil {
		t.Fatalf("verify search: %v", err)
	}
	if n := len(sd.AllUIDs()); n != 0 {
		t.Errorf("dead-lettered message still unseen: %d unseen", n)
	}
}
