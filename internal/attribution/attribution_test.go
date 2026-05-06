package attribution

import (
	"encoding/base64"
	"strings"
	"testing"
)

// encode returns base64-stdlib encoding of the given JSON string. Helper
// to keep test cases readable.
func encode(jsonStr string) string {
	return base64.StdEncoding.EncodeToString([]byte(jsonStr))
}

func TestParse_HappyPath(t *testing.T) {
	in := encode(`{"ref":"producthunt","utm_source":"producthunt","utm_medium":"launch","utm_campaign":"v1","captured_at":"2026-05-06T00:00:00Z"}`)
	got := Parse(in)
	if got.Ref != "producthunt" {
		t.Errorf("Ref: got %q, want producthunt", got.Ref)
	}
	if got.Source != "producthunt" {
		t.Errorf("Source: got %q, want producthunt", got.Source)
	}
	if got.Medium != "launch" {
		t.Errorf("Medium: got %q, want launch", got.Medium)
	}
	if got.Campaign != "v1" {
		t.Errorf("Campaign: got %q, want v1", got.Campaign)
	}
	if got.Empty() {
		t.Error("Empty() returned true for populated attribution")
	}
}

func TestParse_EmptyInput(t *testing.T) {
	got := Parse("")
	if !got.Empty() {
		t.Errorf("expected empty attribution, got %+v", got)
	}
}

func TestParse_PartialFields(t *testing.T) {
	// Only utm_source provided — others should be empty, Empty() should
	// still report false.
	in := encode(`{"utm_source":"twitter"}`)
	got := Parse(in)
	if got.Source != "twitter" {
		t.Errorf("Source: got %q, want twitter", got.Source)
	}
	if got.Ref != "" || got.Medium != "" || got.Campaign != "" {
		t.Errorf("expected only Source set, got %+v", got)
	}
	if got.Empty() {
		t.Error("Empty() should be false when one field is set")
	}
}

func TestParse_MalformedBase64(t *testing.T) {
	got := Parse("!!!not base64!!!")
	if !got.Empty() {
		t.Errorf("expected empty result for bad base64, got %+v", got)
	}
}

func TestParse_MalformedJSON(t *testing.T) {
	in := base64.StdEncoding.EncodeToString([]byte("{not json"))
	got := Parse(in)
	if !got.Empty() {
		t.Errorf("expected empty result for bad JSON, got %+v", got)
	}
}

func TestParse_OversizedInput(t *testing.T) {
	// Construct a base64 string longer than the cap so we bail before
	// even attempting decode.
	big := strings.Repeat("A", maxEncodedLen+1)
	got := Parse(big)
	if !got.Empty() {
		t.Errorf("expected empty result for oversized payload, got %+v", got)
	}
}

func TestParse_OversizedValueIsTruncated(t *testing.T) {
	// utm_campaign of 200 chars — should be truncated to 64.
	long := strings.Repeat("a", 200)
	in := encode(`{"utm_campaign":"` + long + `"}`)
	got := Parse(in)
	if len(got.Campaign) != maxValueLen {
		t.Errorf("Campaign length: got %d, want %d", len(got.Campaign), maxValueLen)
	}
	if got.Campaign != strings.Repeat("a", maxValueLen) {
		t.Errorf("Campaign content unexpected: %q", got.Campaign)
	}
}

func TestParse_NonASCIIDropped(t *testing.T) {
	// Emoji + accented chars should be dropped entirely (not replaced).
	in := encode(`{"utm_source":"café 🎉 launch"}`)
	got := Parse(in)
	// Expected: "caf launch" → "caflaunch" (space dropped; only a-zA-Z0-9-_).
	if got.Source != "caflaunch" {
		t.Errorf("Source: got %q, want %q", got.Source, "caflaunch")
	}
}

func TestParse_LogInjectionDefanged(t *testing.T) {
	// Newlines, control chars, and quote characters MUST be stripped
	// so an attacker can't inject fake log lines via attribution.
	in := encode(`{"utm_source":"producthunt\n[ERROR] fake\""}`)
	got := Parse(in)
	if strings.ContainsAny(got.Source, "\n\r\"") {
		t.Errorf("Source still contains injection chars: %q", got.Source)
	}
	if got.Source != "producthuntERRORfake" {
		t.Errorf("Source: got %q, want producthuntERRORfake", got.Source)
	}
}

func TestParse_ValueBecomesEmptyAfterSanitize(t *testing.T) {
	// utm_source of pure emoji → drops to "". The entire payload should
	// then look empty for downstream callers.
	in := encode(`{"utm_source":"🎉🎉"}`)
	got := Parse(in)
	if got.Source != "" {
		t.Errorf("Source: got %q, want empty", got.Source)
	}
	if !got.Empty() {
		t.Error("Empty() should be true when sanitize drops all fields")
	}
}

func TestParse_URLSafeBase64(t *testing.T) {
	// Some clients may send URL-safe base64 (no padding). Ensure we
	// fall back to that decoding.
	raw := []byte(`{"utm_source":"twitter"}`)
	in := base64.RawURLEncoding.EncodeToString(raw)
	got := Parse(in)
	if got.Source != "twitter" {
		t.Errorf("URL-safe base64 not decoded: got %+v", got)
	}
}

func TestParse_UnknownFieldsIgnored(t *testing.T) {
	// gclid (Google) and email — neither are in the whitelist; both
	// must be silently dropped.
	in := encode(`{"gclid":"abc123","email":"victim@example.com","utm_source":"twitter"}`)
	got := Parse(in)
	if got.Source != "twitter" {
		t.Errorf("Source: got %q, want twitter", got.Source)
	}
	// Verify the Properties() map carries no surprise keys.
	props := got.Properties()
	for k := range props {
		switch k {
		case "attribution_ref", "attribution_source", "attribution_medium", "attribution_campaign":
		default:
			t.Errorf("unexpected key in Properties: %q", k)
		}
	}
}

func TestProperties_OmitsEmpty(t *testing.T) {
	a := Attribution{Source: "twitter"}
	props := a.Properties()
	if len(props) != 1 {
		t.Errorf("expected 1 property, got %d: %+v", len(props), props)
	}
	if props["attribution_source"] != "twitter" {
		t.Errorf("attribution_source: got %v, want twitter", props["attribution_source"])
	}
	if _, ok := props["attribution_ref"]; ok {
		t.Error("attribution_ref should not be present when Ref is empty")
	}
}

func TestProperties_NilWhenEmpty(t *testing.T) {
	var a Attribution
	if a.Properties() != nil {
		t.Errorf("Properties() should be nil for empty attribution")
	}
}
