package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

type OAuthHandler struct {
	authSvc        *auth.Service
	queries        *dbgen.Queries
	frontendURL    string
	secureCookies  bool
	maxFreeUsers   int
	allowedDomains []string
}

func NewOAuthHandler(authSvc *auth.Service, pool *pgxpool.Pool, frontendURL string, secureCookies bool, maxFreeUsers int, allowedDomains []string) *OAuthHandler {
	return &OAuthHandler{
		authSvc:        authSvc,
		queries:        dbgen.New(pool),
		frontendURL:    frontendURL,
		secureCookies:  secureCookies,
		maxFreeUsers:   maxFreeUsers,
		allowedDomains: allowedDomains,
	}
}

func (h *OAuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state := generateState()
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.authSvc.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// oauthResult is the JSON payload stored in the temporary cookie
// and returned by HandleExchange.
type oauthResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
	Email        string `json:"email"`
	Name         string `json:"name,omitempty"`
	AvatarURL    string `json:"avatar_url,omitempty"`
}

func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Redirect(w, r, h.frontendURL+"/?error=invalid_state", http.StatusTemporaryRedirect)
		return
	}

	// Clear the state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, h.frontendURL+"/?error=missing_code", http.StatusTemporaryRedirect)
		return
	}

	info, err := h.authSvc.ExchangeCode(r.Context(), code)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/?error=exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	// Domain allowlist: reject signups from unauthorized email domains.
	if !isEmailDomainAllowed(info.Email, h.allowedDomains) {
		slog.Info("user denied: email domain not allowed", "email", info.Email)
		redirectURL := h.frontendURL + "/waitlist?" + url.Values{
			"reason": []string{"domain_not_allowed"},
			"email":  []string{info.Email},
		}.Encode()
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}

	// Capacity check: if user doesn't already exist and we're at capacity,
	// check for a valid invite code before allowing registration.
	if h.maxFreeUsers > 0 {
		_, existErr := h.queries.GetUserByGoogleID(r.Context(), info.ID)
		if errors.Is(existErr, pgx.ErrNoRows) {
			// New user — check capacity
			userCount, countErr := h.queries.CountUsers(r.Context())
			if countErr != nil {
				slog.Error("count users for capacity check failed", "error", countErr)
				http.Redirect(w, r, h.frontendURL+"/?error=db_error", http.StatusTemporaryRedirect)
				return
			}
			if int(userCount) >= h.maxFreeUsers {
				// At capacity — check if user has a valid invite
				waitlistEntry, wlErr := h.queries.GetWaitlistByEmail(r.Context(), info.Email)
				if wlErr != nil || !waitlistEntry.InviteCode.Valid {
					slog.Info("user denied: at capacity, no invite",
						"email", info.Email,
						"user_count", userCount,
						"max_free_users", h.maxFreeUsers,
					)
					redirectURL := h.frontendURL + "/waitlist?" + url.Values{
						"reason": []string{"at_capacity"},
						"email":  []string{info.Email},
					}.Encode()
					http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
					return
				}
				// Has valid invite — allow through, mark accepted
				if markErr := h.queries.MarkWaitlistAccepted(r.Context(), info.Email); markErr != nil {
					slog.Error("mark waitlist accepted failed", "email", info.Email, "error", markErr)
				}
				slog.Info("user admitted via invite code",
					"email", info.Email,
					"invite_code", waitlistEntry.InviteCode.String,
				)
			}
		}
	}

	user, err := h.queries.UpsertUserByGoogleID(r.Context(), dbgen.UpsertUserByGoogleIDParams{
		GoogleID:  info.ID,
		Email:     info.Email,
		Name:      pgtype.Text{String: info.Name, Valid: info.Name != ""},
		AvatarUrl: pgtype.Text{String: info.AvatarURL, Valid: info.AvatarURL != ""},
	})
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/?error=db_error", http.StatusTemporaryRedirect)
		return
	}

	accessToken, err := h.authSvc.GenerateAccessToken(user.ID)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/?error=token_error", http.StatusTemporaryRedirect)
		return
	}

	refreshToken, err := h.authSvc.GenerateRefreshToken(user.ID)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/?error=token_error", http.StatusTemporaryRedirect)
		return
	}

	// Store auth result in a short-lived HttpOnly cookie instead of URL params.
	result := oauthResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID.String(),
		Email:        user.Email,
	}
	if user.Name.Valid {
		result.Name = user.Name.String
	}
	if user.AvatarUrl.Valid {
		result.AvatarURL = user.AvatarUrl.String
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		slog.Error("marshal oauth result", "error", err)
		http.Redirect(w, r, h.frontendURL+"/?error=internal", http.StatusTemporaryRedirect)
		return
	}

	auth.SetOAuthResultCookie(w, string(resultJSON), h.secureCookies)
	http.Redirect(w, r, h.frontendURL+"/auth/callback", http.StatusTemporaryRedirect)
}

// HandleExchange reads the temporary OAuth result cookie, returns the tokens
// in the response body, and clears the cookie. This is the secure replacement
// for passing tokens in URL query parameters.
func (h *OAuthHandler) HandleExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resultStr := auth.OAuthResultFromCookies(r)
	if resultStr == "" {
		http.Error(w, "no pending auth result", http.StatusBadRequest)
		return
	}

	// Clear the one-time cookie immediately
	auth.ClearOAuthResultCookie(w, h.secureCookies)

	w.Header().Set("Content-Type", "application/json")
	// resultStr is already valid JSON — write it directly
	w.Write([]byte(resultStr))
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// isEmailDomainAllowed checks if the email's domain is in the allowlist.
// An empty allowlist permits all domains.
func isEmailDomainAllowed(email string, allowedDomains []string) bool {
	if len(allowedDomains) == 0 {
		return true
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return false
	}
	domain := strings.ToLower(parts[1])
	for _, allowed := range allowedDomains {
		if domain == allowed {
			return true
		}
	}
	return false
}
