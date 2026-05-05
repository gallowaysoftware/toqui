package telemetry

import (
	"testing"
)

// resolveTraceSampler reads OTEL_TRACES_SAMPLER_ARG and returns the
// sampler + a short label that's logged at boot. The label is the most
// observable property in the test surface — it's what an operator looks
// at in Cloud Logging to confirm the right sampling rate is in effect.
// Pin the label-string contract so a future refactor can't silently
// shift "0.1" to "10%" or similar without breaking the test.

func TestResolveTraceSampler_Empty(t *testing.T) {
	// Empty / unset env var → AlwaysSample. This is the "no behaviour
	// change" branch — a deploy that doesn't set OTEL_TRACES_SAMPLER_ARG
	// should keep sampling 100% as before.
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "")
	_, label := resolveTraceSampler()
	if label != "always" {
		t.Errorf("expected label=always for empty env, got %q", label)
	}
}

func TestResolveTraceSampler_Unparseable(t *testing.T) {
	// Bad input must not crash the boot path. Falls back to
	// AlwaysSample with a warn log.
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "not-a-number")
	_, label := resolveTraceSampler()
	if label != "always" {
		t.Errorf("expected label=always for unparseable env, got %q", label)
	}
}

func TestResolveTraceSampler_Whitespace(t *testing.T) {
	// Whitespace-only must be treated as "unset", not as a parse error.
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "   ")
	_, label := resolveTraceSampler()
	if label != "always" {
		t.Errorf("expected label=always for whitespace env, got %q", label)
	}
}

func TestResolveTraceSampler_ZeroIsNeverSample(t *testing.T) {
	// "0" is a valid configuration: drop everything except spans
	// inheriting from a sampled traceparent. Useful for cost
	// emergencies where we still want propagated traces but no local
	// emission.
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0")
	_, label := resolveTraceSampler()
	if label != "never" {
		t.Errorf("expected label=never for zero, got %q", label)
	}
}

func TestResolveTraceSampler_NegativeClamped(t *testing.T) {
	// Out-of-range negative clamps to "never" rather than erroring —
	// bad env config shouldn't crash the boot path.
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "-0.5")
	_, label := resolveTraceSampler()
	if label != "never" {
		t.Errorf("expected label=never for negative, got %q", label)
	}
}

func TestResolveTraceSampler_OneIsAlwaysSample(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "1")
	_, label := resolveTraceSampler()
	if label != "always" {
		t.Errorf("expected label=always for 1, got %q", label)
	}
}

func TestResolveTraceSampler_AboveOneClamped(t *testing.T) {
	// "1.5" → AlwaysSample. Clamped, not rejected.
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "1.5")
	_, label := resolveTraceSampler()
	if label != "always" {
		t.Errorf("expected label=always for >1, got %q", label)
	}
}

func TestResolveTraceSampler_FractionalRate(t *testing.T) {
	// The headline case: 10% sampling, the recommended starting point
	// from the toqui-terraform#28 audit.
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.1")
	sampler, label := resolveTraceSampler()
	if label != "0.1" {
		t.Errorf("expected label=0.1, got %q", label)
	}
	if sampler == nil {
		t.Error("expected non-nil sampler")
	}
}

func TestResolveTraceSampler_FractionalLabelStability(t *testing.T) {
	// The label is what gets logged at boot. Pin a few rates so a
	// refactor of strconv formatting can't silently change the
	// observability surface (e.g. "0.10000" instead of "0.1" would
	// break dashboards / alert rules that match on the label).
	cases := []struct {
		in, want string
	}{
		{"0.5", "0.5"},
		{"0.01", "0.01"},
		{"0.25", "0.25"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", tc.in)
			_, label := resolveTraceSampler()
			if label != tc.want {
				t.Errorf("rate %q → label %q, want %q", tc.in, label, tc.want)
			}
		})
	}
}
