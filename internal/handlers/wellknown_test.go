package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The two well-known files are public contracts with iOS and Android — Apple
// and Google fetch them as static JSON during install/upgrade and silently
// reject Universal Links if the shape is wrong. Pin both wire shapes so a
// well-meaning refactor (renaming `appID`, dropping `applinks`, etc.) gets
// caught here instead of breaking deep links in production.

// --- Apple App Site Association (AASA) ---

func TestHandleAppleAppSiteAssociation_OKAndJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleAppleAppSiteAssociation(rec, httptest.NewRequest(http.MethodGet, "/.well-known/apple-app-site-association", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json (Apple is strict about MIME)", got)
	}

	body, _ := io.ReadAll(rec.Body)
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}

	applinks, ok := parsed["applinks"].(map[string]any)
	if !ok {
		t.Fatalf("missing or wrong-typed `applinks` key: %v", parsed)
	}
	details, ok := applinks["details"].([]any)
	if !ok || len(details) == 0 {
		t.Fatalf("missing or empty `applinks.details` array: %v", applinks)
	}
	first, ok := details[0].(map[string]any)
	if !ok {
		t.Fatalf("first details entry is not an object: %v", details[0])
	}

	// `appID` is what Apple matches against the installed app's
	// signature. The placeholder shape `<TEAM_ID>.<bundle>` must round-trip
	// through marshalling unchanged.
	appID, ok := first["appID"].(string)
	if !ok || appID == "" {
		t.Errorf("appID missing from first entry: %v", first)
	}

	paths, ok := first["paths"].([]any)
	if !ok {
		t.Fatalf("paths is not an array: %v", first)
	}
	// Both routes must be present — these are the surfaces that need
	// universal-link handling today.
	got := make(map[string]bool, len(paths))
	for _, p := range paths {
		if s, ok := p.(string); ok {
			got[s] = true
		}
	}
	for _, want := range []string{"/shared/*", "/trips/invite*"} {
		if !got[want] {
			t.Errorf("missing path %q in AASA — Apple won't deep-link it", want)
		}
	}
}

// --- Android Asset Links ---

func TestHandleAssetLinks_OKAndJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleAssetLinks(rec, httptest.NewRequest(http.MethodGet, "/.well-known/assetlinks.json", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	body, _ := io.ReadAll(rec.Body)
	var parsed []map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body is not a JSON array: %v", err)
	}
	if len(parsed) == 0 {
		t.Fatal("assetlinks array is empty — Android won't accept an empty file")
	}

	// Required Android-app-link relation.
	relations, ok := parsed[0]["relation"].([]any)
	if !ok {
		t.Fatalf("relation is not an array: %v", parsed[0])
	}
	hasURLDelegate := false
	for _, r := range relations {
		if s, ok := r.(string); ok && s == "delegate_permission/common.handle_all_urls" {
			hasURLDelegate = true
			break
		}
	}
	if !hasURLDelegate {
		t.Error("missing `delegate_permission/common.handle_all_urls` relation — Android won't honor app links")
	}

	target, ok := parsed[0]["target"].(map[string]any)
	if !ok {
		t.Fatalf("target is not an object: %v", parsed[0])
	}
	if got := target["namespace"]; got != "android_app" {
		t.Errorf("namespace = %v, want android_app", got)
	}
	if got, _ := target["package_name"].(string); got == "" {
		t.Errorf("package_name missing — Android needs the app package to verify the signature")
	}
	// sha256_cert_fingerprints should be present as an array, even if empty
	// in this scaffold (real values fill in once we ship signed APKs).
	if _, ok := target["sha256_cert_fingerprints"].([]any); !ok {
		t.Errorf("sha256_cert_fingerprints is not an array: %v", target["sha256_cert_fingerprints"])
	}
}

// Both endpoints serve the same payload regardless of HTTP method. The
// stores fetch via GET, but we don't gate on method (no destructive side
// effects, and rejecting POST would just add noise on robotic probes).
// Pin the current behavior so a future "GET only" tightening is a deliberate
// choice rather than a silent break.

func TestHandleAppleAppSiteAssociation_AcceptsAnyMethod(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
		rec := httptest.NewRecorder()
		HandleAppleAppSiteAssociation(rec, httptest.NewRequest(method, "/.well-known/apple-app-site-association", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("method %s: status = %d, want 200", method, rec.Code)
		}
	}
}

func TestHandleAssetLinks_AcceptsAnyMethod(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
		rec := httptest.NewRecorder()
		HandleAssetLinks(rec, httptest.NewRequest(method, "/.well-known/assetlinks.json", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("method %s: status = %d, want 200", method, rec.Code)
		}
	}
}
