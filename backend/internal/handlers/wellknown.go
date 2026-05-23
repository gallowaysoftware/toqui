package handlers

import (
	"encoding/json"
	"net/http"
)

// HandleAppleAppSiteAssociation serves the Apple App Site Association file
// for iOS Universal Links. This must be served at /.well-known/apple-app-site-association
// without authentication.
func HandleAppleAppSiteAssociation(w http.ResponseWriter, r *http.Request) {
	aasa := map[string]any{
		"applinks": map[string]any{
			"apps": []string{},
			"details": []map[string]any{
				{
					"appID": "TEAM_ID_PLACEHOLDER.travel.toqui.app",
					"paths": []string{"/shared/*", "/trips/invite*"},
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(aasa)
}

// HandleAssetLinks serves the Android Asset Links file for Android App Links.
// This must be served at /.well-known/assetlinks.json without authentication.
func HandleAssetLinks(w http.ResponseWriter, r *http.Request) {
	links := []map[string]any{
		{
			"relation": []string{"delegate_permission/common.handle_all_urls"},
			"target": map[string]any{
				"namespace":                "android_app",
				"package_name":             "travel.toqui.app",
				"sha256_cert_fingerprints": []string{},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}
