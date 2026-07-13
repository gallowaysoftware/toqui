//go:build integration

package integration

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	"github.com/jackc/pgx/v5/pgtype"

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

	// Run one cycle (exported wrapper below drives the unexported pollOnce).
	poller.PollOnce(ctx)

	// A booking was imported for the sender, source=email.
	var count int
	if err := env.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM bookings WHERE user_id = $1 AND source = 'email' AND confirmation_code = 'HTL-7788'", user.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count bookings: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 imported email booking, got %d", count)
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
	// Small settle for the STORE to land.
	time.Sleep(50 * time.Millisecond)
	sd, err := verify.UIDSearch(&imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}, nil).Wait()
	if err != nil {
		t.Fatalf("verify search unseen: %v", err)
	}
	if n := len(sd.AllUIDs()); n != 0 {
		t.Errorf("expected 0 unseen messages after poll, got %d", n)
	}
}
