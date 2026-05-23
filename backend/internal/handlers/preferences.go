package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

// validPreferenceKeys defines the allowed preference key values.
// Keys are limited to a known set to prevent arbitrary data storage
// and to keep the AI context injection predictable.
var validPreferenceKeys = map[string]bool{
	"dietary":       true, // e.g. "vegan", "gluten-free", "halal", "no restrictions"
	"budget":        true, // e.g. "budget", "moderate", "luxury"
	"pace":          true, // e.g. "relaxed", "moderate", "fast"
	"mobility":      true, // e.g. "no restrictions", "limited walking", "wheelchair"
	"accommodation": true, // e.g. "hostel", "hotel", "boutique", "luxury resort"
	"interests":     true, // e.g. "history, food, nightlife"
}

// maxPreferenceValueLength limits the length of preference values to prevent abuse.
const maxPreferenceValueLength = 500

// PreferencesHandler handles user preference CRUD endpoints.
type PreferencesHandler struct {
	authSvc *auth.Service
	queries *dbgen.Queries
}

// NewPreferencesHandler creates a new PreferencesHandler.
func NewPreferencesHandler(authSvc *auth.Service, pool *pgxpool.Pool) *PreferencesHandler {
	return &PreferencesHandler{
		authSvc: authSvc,
		queries: dbgen.New(pool),
	}
}

// HandleGetPreferences handles GET /api/preferences — returns all user preferences as a JSON object.
func (h *PreferencesHandler) HandleGetPreferences(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	result, err := loadPreferencesMap(r.Context(), h.queries, userID)
	if err != nil {
		slog.Error("get preferences failed", "error", err, "user_id", userID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		slog.Error("failed to encode preferences response", "error", err)
	}
}

// HandlePutPreferences handles PUT /api/preferences — upserts preferences from a JSON body.
// Request body: {"dietary": "vegan", "budget": "moderate", "pace": "relaxed"}
// Keys not present in the body are left unchanged. To remove a preference, set its value to "".
func (h *PreferencesHandler) HandlePutPreferences(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(body) == 0 {
		http.Error(w, "request body must contain at least one preference", http.StatusBadRequest)
		return
	}

	// Validate all keys before making any changes.
	for key := range body {
		if !validPreferenceKeys[key] {
			http.Error(w, "invalid preference key: "+key, http.StatusBadRequest)
			return
		}
	}

	// Validate value lengths.
	for key, value := range body {
		if len(value) > maxPreferenceValueLength {
			http.Error(w, "preference value too long for key: "+key, http.StatusBadRequest)
			return
		}
	}

	for key, value := range body {
		if value == "" {
			// Empty value means delete the preference.
			if err := h.queries.DeletePreference(r.Context(), dbgen.DeletePreferenceParams{
				UserID: userID,
				Key:    key,
			}); err != nil {
				slog.Error("delete preference failed", "error", err, "user_id", userID, "key", key)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		} else {
			if _, err := h.queries.UpsertPreference(r.Context(), dbgen.UpsertPreferenceParams{
				UserID: userID,
				Key:    key,
				Value:  value,
			}); err != nil {
				slog.Error("upsert preference failed", "error", err, "user_id", userID, "key", key)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}
	}

	slog.Info("preferences updated", "user_id", userID, "keys_updated", len(body))

	// Return the full current set of preferences after the update.
	result, err := loadPreferencesMap(r.Context(), h.queries, userID)
	if err != nil {
		slog.Error("get preferences after update failed", "error", err, "user_id", userID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		slog.Error("failed to encode preferences response", "error", err)
	}
}

// preferenceLabels maps preference keys to human-readable labels for the AI prompt.
var preferenceLabels = map[string]string{
	"dietary":       "Dietary",
	"budget":        "Budget",
	"pace":          "Pace",
	"mobility":      "Mobility",
	"accommodation": "Accommodation",
	"interests":     "Interests",
}

// buildPreferencesContext formats user preferences as a system prompt section
// for AI context injection. Returns an empty string if the map is empty.
func buildPreferencesContext(prefs map[string]string) string {
	if len(prefs) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\nUSER PREFERENCES (remembered from previous sessions):\n")
	// Use a stable key order based on the preferenceLabels map.
	for _, key := range []string{"dietary", "budget", "pace", "mobility", "accommodation", "interests"} {
		if value, ok := prefs[key]; ok {
			label := preferenceLabels[key]
			sb.WriteString("- ")
			sb.WriteString(label)
			sb.WriteString(": ")
			sb.WriteString(sanitizeForPrompt(value, 200))
			sb.WriteString("\n")
		}
	}
	sb.WriteString("Use these preferences without asking again unless the user explicitly changes them.\n")
	return sb.String()
}

// loadPreferencesMap loads user preferences as a simple key-value map.
// Used by both the preferences handler and the chat handler (for AI context injection).
// Returns an empty map (not an error) if the query returns no rows.
func loadPreferencesMap(ctx context.Context, queries *dbgen.Queries, userID uuid.UUID) (map[string]string, error) {
	prefs, err := queries.GetPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(prefs))
	for _, p := range prefs {
		result[p.Key] = p.Value
	}
	return result, nil
}
