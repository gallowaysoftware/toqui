package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

type OAuthHandler struct {
	authSvc     *auth.Service
	queries     *dbgen.Queries
	frontendURL string
}

func NewOAuthHandler(authSvc *auth.Service, pool *pgxpool.Pool, frontendURL string) *OAuthHandler {
	return &OAuthHandler{
		authSvc:     authSvc,
		queries:     dbgen.New(pool),
		frontendURL: frontendURL,
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
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.authSvc.AuthCodeURL(state), http.StatusTemporaryRedirect)
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

	callbackURL, _ := url.Parse(h.frontendURL + "/auth/callback")
	q := callbackURL.Query()
	q.Set("access_token", accessToken)
	q.Set("refresh_token", refreshToken)
	q.Set("user_id", user.ID.String())
	q.Set("email", user.Email)
	if user.Name.Valid {
		q.Set("name", user.Name.String)
	}
	callbackURL.RawQuery = q.Encode()

	http.Redirect(w, r, callbackURL.String(), http.StatusTemporaryRedirect)
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
