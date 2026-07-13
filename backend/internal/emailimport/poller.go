package emailimport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/jackc/pgx/v5"

	toquiv1 "github.com/gallowaysoftware/toqui/backend/gen/toqui/v1"
	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui/backend/internal/email"
)

// IMAPConfig configures the booking-import poller.
type IMAPConfig struct {
	Host         string
	Port         int
	Username     string
	Password     string
	Mailbox      string
	PollInterval time.Duration
	TLS          bool
}

// userLookup resolves the sender of a forwarded email to a Toqui account.
// *dbgen.Queries satisfies it.
type userLookup interface {
	GetUserByEmail(ctx context.Context, email string) (dbgen.User, error)
}

// emailIngester is the subset of *handlers.BookingHandler the poller needs.
// Declaring it here (rather than importing handlers) keeps the dependency
// one-directional — handlers imports emailimport for ResolveTrip, not the
// reverse — and avoids an import cycle.
type emailIngester interface {
	IngestEmail(ctx context.Context, req *connect.Request[toquiv1.IngestEmailRequest]) (*connect.Response[toquiv1.IngestEmailResponse], error)
}

// maxRawMessageBytes skips absurdly large messages before they reach the
// parser / AI. Matches the IngestEmail proto's raw_email ceiling.
const maxRawMessageBytes = 500_000

// Poller watches an IMAP mailbox that users forward booking confirmations
// to. Each UNSEEN message is matched to its sender's account and ingested
// via the same pipeline as the IngestEmail RPC. It is an in-process
// background job (mirrors internal/lifecycle Jobs) — no separate binary,
// no service-to-service auth — enabled only when IMAP creds are configured.
type Poller struct {
	cfg      IMAPConfig
	users    userLookup
	ingester emailIngester
}

// NewPoller builds a Poller. Mailbox defaults to INBOX and PollInterval to
// 60s when unset.
func NewPoller(cfg IMAPConfig, users userLookup, ingester emailIngester) *Poller {
	if cfg.Mailbox == "" {
		cfg.Mailbox = "INBOX"
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 60 * time.Second
	}
	return &Poller{cfg: cfg, users: users, ingester: ingester}
}

// Start runs the poll loop until ctx is cancelled. Callers run it in a
// goroutine. Each cycle is best-effort: connection or processing errors are
// logged and the next tick retries.
func (p *Poller) Start(ctx context.Context) {
	slog.Info("imap booking-import poller starting",
		"host", p.cfg.Host, "mailbox", p.cfg.Mailbox, "interval", p.cfg.PollInterval.String(), "tls", p.cfg.TLS)

	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	// Poll once immediately so a restart drains anything that arrived while
	// the server was down.
	p.PollOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			slog.Info("imap booking-import poller stopping")
			return
		case <-ticker.C:
			p.PollOnce(ctx)
		}
	}
}

// PollOnce connects, processes all UNSEEN messages, and disconnects. It
// opens a fresh connection each cycle rather than holding one open — self-
// host mail servers drop idle connections, and reconnecting per cycle is
// cheap at a 60s cadence and far simpler than IDLE + keepalive. Start calls
// it each tick; exported so a manual poll (and end-to-end tests) can drive
// a single cycle.
func (p *Poller) PollOnce(ctx context.Context) {
	c, err := p.dial()
	if err != nil {
		slog.Error("imap: dial failed", "error", err)
		return
	}
	defer func() { _ = c.Logout().Wait(); _ = c.Close() }()

	if err := c.Login(p.cfg.Username, p.cfg.Password).Wait(); err != nil {
		slog.Error("imap: login failed", "error", err)
		return
	}
	if _, err := c.Select(p.cfg.Mailbox, nil).Wait(); err != nil {
		slog.Error("imap: select mailbox failed", "mailbox", p.cfg.Mailbox, "error", err)
		return
	}

	searchData, err := c.UIDSearch(&imap.SearchCriteria{
		NotFlag: []imap.Flag{imap.FlagSeen},
	}, nil).Wait()
	if err != nil {
		slog.Error("imap: search unseen failed", "error", err)
		return
	}
	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		return
	}
	slog.Info("imap: processing unseen messages", "count", len(uids))

	for _, uid := range uids {
		if ctx.Err() != nil {
			return
		}
		p.fetchAndProcess(ctx, c, uid)
	}
}

// fetchAndProcess fetches one message's raw bytes, ingests it, and marks it
// \Seen unless the failure was transient (so a later cycle retries).
func (p *Poller) fetchAndProcess(ctx context.Context, c *imapclient.Client, uid imap.UID) {
	uidSet := imap.UIDSetNum(uid)
	section := &imap.FetchItemBodySection{} // empty section = whole message
	msgs, err := c.Fetch(uidSet, &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{section},
	}).Collect()
	if err != nil || len(msgs) == 0 {
		slog.Error("imap: fetch message failed", "uid", uid, "error", err)
		return
	}
	raw := msgs[0].FindBodySection(section)
	if len(raw) == 0 {
		slog.Warn("imap: empty message body, marking seen", "uid", uid)
		p.markSeen(c, uid)
		return
	}
	if len(raw) > maxRawMessageBytes {
		slog.Warn("imap: message exceeds size cap, skipping", "uid", uid, "bytes", len(raw))
		p.markSeen(c, uid)
		return
	}

	markSeen := p.processMessage(ctx, raw)
	if markSeen {
		p.markSeen(c, uid)
	}
}

// processMessage routes one raw message to its sender's account and ingests
// it. It returns whether the message should be marked \Seen: true on
// success or a permanent rejection (unknown sender, unroutable, empty body),
// false on a transient failure so a later cycle retries. It's separated from
// the IMAP transport so it can be unit-tested without a mail server.
func (p *Poller) processMessage(ctx context.Context, raw []byte) (markSeen bool) {
	parsed := email.Parse(string(raw))
	if parsed.FromAddress == "" {
		slog.Warn("imap: message has no From address, skipping")
		return true // unroutable — don't retry forever
	}

	user, err := p.users.GetUserByEmail(ctx, parsed.FromAddress)
	if errors.Is(err, pgx.ErrNoRows) {
		slog.Info("imap: sender is not a Toqui user, skipping", "from", pseudonymize(parsed.FromAddress))
		return true // unknown sender — don't retry forever
	}
	if err != nil {
		slog.Error("imap: user lookup failed", "error", err)
		return false // transient DB error — retry next cycle
	}

	authCtx := auth.ContextWithUserID(ctx, user.ID)
	_, err = p.ingester.IngestEmail(authCtx, connect.NewRequest(&toquiv1.IngestEmailRequest{
		RawEmail: string(raw),
	}))
	if err == nil {
		slog.Info("imap: imported booking from forwarded email", "user_id", user.ID)
		return true
	}

	switch connect.CodeOf(err) {
	case connect.CodeInvalidArgument, connect.CodePermissionDenied:
		// Nothing parseable, or not allowed — permanent for this message.
		slog.Warn("imap: message rejected, marking seen", "user_id", user.ID, "code", connect.CodeOf(err).String())
		return true
	default:
		// AI provider down, DB blip, etc. — leave unseen to retry.
		slog.Error("imap: ingest failed (transient), will retry", "user_id", user.ID, "error", err)
		return false
	}
}

func (p *Poller) markSeen(c *imapclient.Client, uid imap.UID) {
	if err := c.Store(imap.UIDSetNum(uid), &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagSeen},
	}, nil).Close(); err != nil {
		slog.Error("imap: mark seen failed", "uid", uid, "error", err)
	}
}

func (p *Poller) dial() (*imapclient.Client, error) {
	addr := fmt.Sprintf("%s:%d", p.cfg.Host, p.cfg.Port)
	if p.cfg.TLS {
		return imapclient.DialTLS(addr, nil)
	}
	return imapclient.DialInsecure(addr, nil)
}

// pseudonymize reduces an email address to a non-identifying hint for logs
// (privacy: never log full addresses). "alice@example.com" → "a***@example.com".
func pseudonymize(addr string) string {
	at := -1
	for i, r := range addr {
		if r == '@' {
			at = i
			break
		}
	}
	if at <= 0 {
		return "***"
	}
	return addr[:1] + "***" + addr[at:]
}
