package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOAuthResultCookie_SameSiteConsistency(t *testing.T) {
	// SetOAuthResultCookie and ClearOAuthResultCookie must use the same
	// SameSite mode, otherwise the browser treats them as different cookies
	// and the clear has no effect.

	setRec := httptest.NewRecorder()
	SetOAuthResultCookie(setRec, "test-value", true)

	clearRec := httptest.NewRecorder()
	ClearOAuthResultCookie(clearRec, true)

	setCookies := setRec.Result().Cookies()
	clearCookies := clearRec.Result().Cookies()

	if len(setCookies) == 0 || len(clearCookies) == 0 {
		t.Fatal("expected cookies to be set")
	}

	setSameSite := findCookieSameSite(setCookies, OAuthResultCookie)
	clearSameSite := findCookieSameSite(clearCookies, OAuthResultCookie)

	if setSameSite != clearSameSite {
		t.Errorf("SameSite mismatch: SetOAuthResultCookie uses %v, ClearOAuthResultCookie uses %v",
			setSameSite, clearSameSite)
	}
}

func findCookieSameSite(cookies []*http.Cookie, name string) http.SameSite {
	for _, c := range cookies {
		if c.Name == name {
			return c.SameSite
		}
	}
	return 0
}
