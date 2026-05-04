import { View, Text, TextInput, Pressable, StyleSheet } from "react-native";
import { useState, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { useTheme } from "@/lib/theme";
import { useAuth } from "@/lib/auth";
import { useAnalytics } from "@/lib/analytics";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";

/**
 * AgeGate — 18+ enforcement.
 *
 * The gate is now POST-OAUTH ONLY. A logged-out visitor sees marketing /
 * sign-in screens unchanged; the gate has no opinion until there's a
 * real account behind the request. This is the redesign tracked in
 * toqui-backend#420 — the original placement (cold-start, pre-login)
 * demanded a birthday before the user knew what the app was, and ran
 * on a localStorage cache they could clear to bypass.
 *
 * Behaviour now:
 *
 *   - Logged out → render `children` (no gate). The /auth/* and
 *     onboarding screens behind us are public.
 *   - Logged in, `user.ageVerifiedAt` set → render `children`. Returning
 *     verified users never see this component's UI again.
 *   - Logged in, NOT verified → render the DOB form with the new
 *     contextual copy. The backend tells us this state via
 *     `age_verification_required` on the login response, but here we
 *     derive it from `user.ageVerifiedAt` directly — both are
 *     equivalent today and going via the proto field would mean
 *     plumbing it through the auth context.
 *   - Logged in, submitted under-18 DOB → backend hard-deletes the
 *     account (server-side action, see age_verify.go's
 *     handleUnderAge). Frontend receives a typed 403
 *     `{"error": "under_age", "message": "..."}`, shows the deletion
 *     confirmation screen with the message verbatim, and clears local
 *     auth state. The user can't continue — there's nothing to
 *     continue to, since the backend has already wiped them.
 *
 * No more localStorage. No more client-side "is this user verified"
 * cache. The user proto's `ageVerifiedAt` is the source of truth on
 * every render — if a user verifies on another device, the next time
 * this app refreshes its login state the gate disappears without us
 * having to migrate any storage.
 */
interface AgeGateProps {
  children: React.ReactNode;
}

function calculateAge(dob: Date): number {
  const today = new Date();
  let age = today.getFullYear() - dob.getFullYear();
  const monthDiff = today.getMonth() - dob.getMonth();
  if (monthDiff < 0 || (monthDiff === 0 && today.getDate() < dob.getDate())) {
    age--;
  }
  return age;
}

function parseDate(y: string, m: string, d: string): Date | null {
  const year = parseInt(y, 10);
  const month = parseInt(m, 10);
  const day = parseInt(d, 10);
  if (isNaN(year) || isNaN(month) || isNaN(day)) return null;
  if (month < 1 || month > 12 || day < 1 || day > 31) return null;
  if (year < 1900 || year > new Date().getFullYear()) return null;
  const date = new Date(year, month - 1, day);
  // Round-trip validation: catches invalid dates like Feb 30
  if (date.getFullYear() !== year || date.getMonth() !== month - 1 || date.getDate() !== day) return null;
  return date;
}

export function AgeGate({ children }: AgeGateProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const { accessToken, user, logout } = useAuth();
  const { track } = useAnalytics();

  // Local UI state. We don't cache "verified" anywhere — `user.ageVerifiedAt`
  // is checked on every render so a backend update propagates instantly.
  const [year, setYear] = useState("");
  const [month, setMonth] = useState("");
  const [day, setDay] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  // `deleted` flips to true when the backend confirms the under-18
  // account-deletion path. Once set, we render the deletion-confirmation
  // screen and the user is effectively logged out (we call `logout()` in
  // the same handler).
  const [deleted, setDeleted] = useState(false);

  // Logged-out users are not gated — the marketing/sign-in screens
  // behind us must render normally. This is THE structural change of
  // the redesign.
  if (!accessToken) {
    return <>{children}</>;
  }

  // Backend says "you're verified" → we believe it, no UI shown.
  if (user?.ageVerifiedAt) {
    return <>{children}</>;
  }

  const handleVerify = useCallback(async () => {
    setError("");
    const dob = parseDate(year, month, day);
    if (!dob) {
      setError(t("ageGate.invalidDate"));
      return;
    }

    // We deliberately DO NOT short-circuit on age < 18 in the client.
    // The backend is the single enforcement point: it does the deletion
    // + block-list write atomically. Showing a "you're under 18" screen
    // without telling the backend would leave the user logged in with
    // their data intact, defeating the redesign.
    if (!accessToken) {
      // Should be unreachable given the early return above, but pin
      // the invariant — without a token we can't authenticate the call.
      setError(t("ageGate.invalidDate"));
      return;
    }

    const dobStr = `${dob.getFullYear()}-${String(dob.getMonth() + 1).padStart(2, "0")}-${String(dob.getDate()).padStart(2, "0")}`;
    setSubmitting(true);
    try {
      const res = await authFetch(`${getConfig().apiUrl}/auth/verify-age`, accessToken, {
        method: "POST",
        body: JSON.stringify({ date_of_birth: dobStr }),
      });

      if (res.ok) {
        track("age_gate_passed");
        // The auth context's `user` will repopulate on next refresh;
        // until then, force a re-render by clearing form state. The
        // proto's `ageVerifiedAt` is what controls the gate next render.
        setYear("");
        setMonth("");
        setDay("");
        return;
      }

      // 403 with body { error: "under_age" } → backend already deleted
      // the account. Show the deletion confirmation, then clear local
      // auth state so a refresh doesn't try to use a now-invalid token.
      if (res.status === 403) {
        try {
          const body = await res.json();
          if (body && body.error === "under_age") {
            setDeleted(true);
            track("age_gate_under_age_refused");
            // Fire-and-forget logout. The token is already invalid
            // server-side (the user row is gone) — we just need the
            // client to stop sending it. Errors here are non-fatal.
            void logout();
            return;
          }
        } catch {
          // Fallthrough to generic error path below.
        }
      }

      // Anything else: show a generic try-again. Don't claim deletion
      // when we don't know what happened.
      setError(t("ageGate.tryAgain"));
    } catch {
      setError(t("ageGate.tryAgain"));
    } finally {
      setSubmitting(false);
    }
  }, [year, month, day, accessToken, t, track, logout]);

  if (deleted) {
    // Post-deletion confirmation. Note we keep this in the AgeGate
    // tree even though the user is now logged out — the parent layout
    // will re-render once `accessToken` clears, but until then this
    // screen explains what just happened. After the next render cycle
    // the !accessToken branch above takes over and the user lands on
    // the marketing/sign-in screens.
    return (
      <View style={[styles.container, { backgroundColor: colors.surface }]}>
        <Text style={[styles.title, { color: colors.error }]}>{t("ageGate.deletedTitle")}</Text>
        <Text style={[styles.subtitle, { color: colors.textSecondary }]}>
          {t("ageGate.deletedSubtitle")}
        </Text>
      </View>
    );
  }

  return (
    <View style={[styles.container, { backgroundColor: colors.surface }]}>
      <Text style={[styles.title, { color: colors.accent }]}>{t("ageGate.title")}</Text>
      <Text style={[styles.subtitle, { color: colors.textSecondary }]}>
        {t("ageGate.subtitle")}
      </Text>
      <Text style={[styles.privacyNote, { color: colors.textTertiary }]}>
        {t("ageGate.privacyNote")}
      </Text>

      <View style={styles.dateRow}>
        <View style={styles.dateField}>
          <Text style={[styles.label, { color: colors.textSecondary }]}>{t("ageGate.month")}</Text>
          <TextInput
            style={[styles.input, { backgroundColor: colors.inputBg, borderColor: colors.inputBorder, color: colors.textPrimary }]}
            placeholder="MM"
            placeholderTextColor={colors.textTertiary}
            value={month}
            onChangeText={setMonth}
            keyboardType="number-pad"
            maxLength={2}
          />
        </View>
        <View style={styles.dateField}>
          <Text style={[styles.label, { color: colors.textSecondary }]}>{t("ageGate.day")}</Text>
          <TextInput
            style={[styles.input, { backgroundColor: colors.inputBg, borderColor: colors.inputBorder, color: colors.textPrimary }]}
            placeholder="DD"
            placeholderTextColor={colors.textTertiary}
            value={day}
            onChangeText={setDay}
            keyboardType="number-pad"
            maxLength={2}
          />
        </View>
        <View style={[styles.dateField, { flex: 1.5 }]}>
          <Text style={[styles.label, { color: colors.textSecondary }]}>{t("ageGate.year")}</Text>
          <TextInput
            style={[styles.input, { backgroundColor: colors.inputBg, borderColor: colors.inputBorder, color: colors.textPrimary }]}
            placeholder="YYYY"
            placeholderTextColor={colors.textTertiary}
            value={year}
            onChangeText={setYear}
            keyboardType="number-pad"
            maxLength={4}
          />
        </View>
      </View>

      {error ? <Text style={[styles.error, { color: colors.error }]}>{error}</Text> : null}

      <Pressable
        style={[styles.button, { backgroundColor: colors.accent, opacity: submitting ? 0.6 : 1 }]}
        onPress={handleVerify}
        disabled={submitting}
      >
        <Text style={[styles.buttonText, { color: colors.userBubbleText }]}>
          {submitting ? t("ageGate.verifying") : t("ageGate.verifyAge")}
        </Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: "center", padding: 24 },
  title: { fontSize: 24, fontWeight: "bold", textAlign: "center", marginBottom: 12 },
  subtitle: { fontSize: 15, textAlign: "center", lineHeight: 22, marginBottom: 12 },
  privacyNote: { fontSize: 13, textAlign: "center", lineHeight: 18, marginBottom: 32, fontStyle: "italic" },
  dateRow: { flexDirection: "row", gap: 12, marginBottom: 16 },
  dateField: { flex: 1 },
  label: { fontSize: 13, fontWeight: "500", marginBottom: 4 },
  input: {
    borderWidth: 1,
    borderRadius: 8,
    padding: 12,
    fontSize: 18,
    textAlign: "center",
    fontWeight: "600",
  },
  error: { fontSize: 14, textAlign: "center", marginBottom: 12 },
  button: { borderRadius: 8, padding: 14, alignItems: "center" },
  buttonText: { fontSize: 16, fontWeight: "600" },
});
