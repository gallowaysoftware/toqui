package emailimport

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	toquiv1 "github.com/gallowaysoftware/toqui/backend/gen/toqui/v1"
	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

type fakeUserLookup struct {
	user dbgen.User
	err  error
}

func (f *fakeUserLookup) GetUserByEmail(_ context.Context, _ string) (dbgen.User, error) {
	return f.user, f.err
}

type fakeIngester struct {
	err         error
	gotUserID   uuid.UUID
	gotRawEmail string
	callCount   int
}

func (f *fakeIngester) IngestEmail(ctx context.Context, req *connect.Request[toquiv1.IngestEmailRequest]) (*connect.Response[toquiv1.IngestEmailResponse], error) {
	f.callCount++
	f.gotUserID, _ = auth.UserIDFromContext(ctx)
	f.gotRawEmail = req.Msg.RawEmail
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(&toquiv1.IngestEmailResponse{}), nil
}

func newPoller(users userLookup, ing emailIngester) *Poller {
	return &Poller{cfg: IMAPConfig{Mailbox: "INBOX"}, users: users, ingester: ing}
}

const sampleEmail = "From: traveler@example.com\r\n" +
	"Subject: Booking\r\n" +
	"Content-Type: text/plain\r\n\r\n" +
	"Confirmation ABC123\r\n"

func TestProcessMessage_Success(t *testing.T) {
	uid := uuid.New()
	users := &fakeUserLookup{user: dbgen.User{ID: uid, Email: "traveler@example.com"}}
	ing := &fakeIngester{}
	p := newPoller(users, ing)

	markSeen := p.processMessage(context.Background(), []byte(sampleEmail))
	if !markSeen {
		t.Error("successful ingest should mark the message seen")
	}
	if ing.callCount != 1 {
		t.Errorf("ingester called %d times, want 1", ing.callCount)
	}
	if ing.gotUserID != uid {
		t.Errorf("ingest ran with user %s, want the looked-up user %s", ing.gotUserID, uid)
	}
	if ing.gotRawEmail != sampleEmail {
		t.Error("raw email not passed through verbatim")
	}
}

func TestProcessMessage_UnknownSender(t *testing.T) {
	users := &fakeUserLookup{err: pgx.ErrNoRows}
	ing := &fakeIngester{}
	p := newPoller(users, ing)

	markSeen := p.processMessage(context.Background(), []byte(sampleEmail))
	if !markSeen {
		t.Error("unknown sender should mark seen (don't retry forever)")
	}
	if ing.callCount != 0 {
		t.Error("ingester must not be called for an unknown sender")
	}
}

func TestProcessMessage_NoFromAddress(t *testing.T) {
	users := &fakeUserLookup{err: errors.New("should not be called")}
	ing := &fakeIngester{}
	p := newPoller(users, ing)

	// A bare body with no headers → no From address.
	markSeen := p.processMessage(context.Background(), []byte("just some text, no headers"))
	if !markSeen {
		t.Error("unroutable message should mark seen")
	}
	if ing.callCount != 0 {
		t.Error("ingester must not be called without a sender")
	}
}

func TestProcessMessage_UserLookupTransientError(t *testing.T) {
	users := &fakeUserLookup{err: errors.New("db connection reset")}
	ing := &fakeIngester{}
	p := newPoller(users, ing)

	markSeen := p.processMessage(context.Background(), []byte(sampleEmail))
	if markSeen {
		t.Error("a transient DB error must leave the message unseen for retry")
	}
	if ing.callCount != 0 {
		t.Error("ingester must not be called when the lookup failed")
	}
}

func TestProcessMessage_IngestPermanentError(t *testing.T) {
	users := &fakeUserLookup{user: dbgen.User{ID: uuid.New()}}
	for _, code := range []connect.Code{connect.CodeInvalidArgument, connect.CodePermissionDenied} {
		ing := &fakeIngester{err: connect.NewError(code, fmt.Errorf("rejected"))}
		p := newPoller(users, ing)
		if !p.processMessage(context.Background(), []byte(sampleEmail)) {
			t.Errorf("%s should mark seen (permanent rejection)", code)
		}
	}
}

func TestProcessMessage_IngestTransientError(t *testing.T) {
	users := &fakeUserLookup{user: dbgen.User{ID: uuid.New()}}
	for _, code := range []connect.Code{connect.CodeUnavailable, connect.CodeInternal} {
		ing := &fakeIngester{err: connect.NewError(code, fmt.Errorf("ai down"))}
		p := newPoller(users, ing)
		if p.processMessage(context.Background(), []byte(sampleEmail)) {
			t.Errorf("%s should leave the message unseen for retry", code)
		}
	}
}

func TestPseudonymize(t *testing.T) {
	cases := map[string]string{
		"alice@example.com": "a***@example.com",
		"bob@toqui.travel":  "b***@toqui.travel",
		"nodomain":          "***",
		"":                  "***",
	}
	for in, want := range cases {
		if got := pseudonymize(in); got != want {
			t.Errorf("pseudonymize(%q) = %q, want %q", in, got, want)
		}
	}
}
