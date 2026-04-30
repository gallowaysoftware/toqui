package config

import (
	"testing"
	"time"
)

// The getEnv* helpers are tiny but hot — every config field reads through
// them at startup. A regression that mishandled empty-string vs unset, or
// silently treated "false" as true, would corrupt every environment's
// boot. Each test pins the contract for one specific failure mode.

func TestGetEnv_FallsBackWhenUnsetOrEmpty(t *testing.T) {
	const key = "TOQUI_TEST_GETENV_UNSET"
	t.Setenv(key, "") // explicitly empty — the same fallback path as unset

	if got := getEnv(key, "fallback-value"); got != "fallback-value" {
		t.Errorf("getEnv(empty) = %q, want fallback-value", got)
	}
}

func TestGetEnv_ReturnsValueWhenSet(t *testing.T) {
	const key = "TOQUI_TEST_GETENV_SET"
	t.Setenv(key, "actual-value")

	if got := getEnv(key, "fallback-value"); got != "actual-value" {
		t.Errorf("getEnv = %q, want actual-value", got)
	}
}

func TestGetEnvInt_FallsBackOnEmpty(t *testing.T) {
	const key = "TOQUI_TEST_INT_EMPTY"
	t.Setenv(key, "")

	if got := getEnvInt(key, 42); got != 42 {
		t.Errorf("getEnvInt(empty) = %d, want fallback 42", got)
	}
}

func TestGetEnvInt_FallsBackOnGarbage(t *testing.T) {
	// Critical contract: a malformed value MUST NOT crash — it logs a
	// warning and falls back. This is what keeps a typo'd env file from
	// taking the whole service down at boot.
	const key = "TOQUI_TEST_INT_GARBAGE"
	t.Setenv(key, "not-a-number")

	if got := getEnvInt(key, 42); got != 42 {
		t.Errorf("getEnvInt(garbage) = %d, want fallback 42", got)
	}
}

func TestGetEnvInt_ParsesValidInteger(t *testing.T) {
	const key = "TOQUI_TEST_INT_VALID"
	t.Setenv(key, "1900")

	if got := getEnvInt(key, 0); got != 1900 {
		t.Errorf("getEnvInt = %d, want 1900", got)
	}
}

func TestGetEnvInt_HandlesNegative(t *testing.T) {
	// Some env vars (e.g. AI budget margins) accept negatives; the
	// helper must parse them cleanly rather than treating "-1" as garbage.
	const key = "TOQUI_TEST_INT_NEG"
	t.Setenv(key, "-1")

	if got := getEnvInt(key, 0); got != -1 {
		t.Errorf("getEnvInt(-1) = %d, want -1", got)
	}
}

func TestGetEnvBool_FallsBackOnEmpty(t *testing.T) {
	const key = "TOQUI_TEST_BOOL_EMPTY"
	t.Setenv(key, "")

	if got := getEnvBool(key, true); got != true {
		t.Errorf("getEnvBool(empty) = %v, want fallback true", got)
	}
}

func TestGetEnvBool_FallsBackOnGarbage(t *testing.T) {
	const key = "TOQUI_TEST_BOOL_GARBAGE"
	t.Setenv(key, "yesplease")

	if got := getEnvBool(key, false); got != false {
		t.Errorf("getEnvBool(garbage) = %v, want fallback false", got)
	}
}

func TestGetEnvBool_ParsesAllStandardForms(t *testing.T) {
	// strconv.ParseBool accepts: 1, t, T, TRUE, true, True, 0, f, F,
	// FALSE, false, False. Lock the truthy + falsy mappings so a
	// future contributor that "improves" the helper doesn't silently
	// flip them.
	const key = "TOQUI_TEST_BOOL_FORMS"
	cases := map[string]bool{
		"true":  true,
		"True":  true,
		"TRUE":  true,
		"1":     true,
		"t":     true,
		"false": false,
		"False": false,
		"FALSE": false,
		"0":     false,
		"f":     false,
	}
	for v, want := range cases {
		t.Setenv(key, v)
		if got := getEnvBool(key, !want); got != want {
			t.Errorf("getEnvBool(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestParseCSVEnv_ReturnsNilOnEmpty(t *testing.T) {
	const key = "TOQUI_TEST_CSV_EMPTY"
	t.Setenv(key, "")

	got := parseCSVEnv(key)
	if got != nil {
		t.Errorf("parseCSVEnv(empty) = %v, want nil (NOT empty slice)", got)
	}
}

func TestParseCSVEnv_TrimsAndSkipsBlanks(t *testing.T) {
	// Real-world value shape: env files often have ", , a , b , ,c"
	// from copy-paste. The helper must strip whitespace AND drop
	// empty entries — pin both since either alone would let a bogus
	// entry slip into the allow/deny lists.
	const key = "TOQUI_TEST_CSV_TRIM"
	t.Setenv(key, " a@example.com , , b@example.com,, c@example.com ,")

	got := parseCSVEnv(key)
	want := []string{"a@example.com", "b@example.com", "c@example.com"}
	if len(got) != len(want) {
		t.Fatalf("parseCSVEnv = %v (len=%d), want %v (len=%d)", got, len(got), want, len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("parseCSVEnv[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGetEnvDuration_FallsBackOnEmpty(t *testing.T) {
	const key = "TOQUI_TEST_DUR_EMPTY"
	t.Setenv(key, "")

	if got := getEnvDuration(key, 5*time.Second); got != 5*time.Second {
		t.Errorf("getEnvDuration(empty) = %v, want 5s fallback", got)
	}
}

func TestGetEnvDuration_FallsBackOnGarbage(t *testing.T) {
	const key = "TOQUI_TEST_DUR_GARBAGE"
	t.Setenv(key, "five seconds")

	if got := getEnvDuration(key, 5*time.Second); got != 5*time.Second {
		t.Errorf("getEnvDuration(garbage) = %v, want 5s fallback", got)
	}
}

func TestGetEnvDuration_ParsesGoDurationSyntax(t *testing.T) {
	// Pin the standard time.ParseDuration shapes that show up in env
	// files: bare "30s", combined "1h30m", millisecond "500ms".
	const key = "TOQUI_TEST_DUR_VALID"
	cases := map[string]time.Duration{
		"30s":   30 * time.Second,
		"1h30m": 90 * time.Minute,
		"500ms": 500 * time.Millisecond,
		"2h":    2 * time.Hour,
	}
	for v, want := range cases {
		t.Setenv(key, v)
		if got := getEnvDuration(key, time.Hour); got != want {
			t.Errorf("getEnvDuration(%q) = %v, want %v", v, got, want)
		}
	}
}
