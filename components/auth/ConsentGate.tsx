import { useCallback, useState } from "react";
import {
  ActivityIndicator,
  Modal,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from "react-native";
import * as WebBrowser from "expo-web-browser";
import { useTranslation } from "react-i18next";
import { useAuth } from "@/lib/auth";
import { useAnalytics } from "@/lib/analytics";
import { useConsentSignal } from "@/lib/transport";
import { useTheme } from "@/lib/theme";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";

// ---------------------------------------------------------------------------
// ConsentGate
//
// Pops a blocking modal whenever `useConsentSignal().consentRequired` is
// true (flipped by the transport interceptor on
// `FailedPrecondition("consent_required")` from the backend). The user
// cannot dismiss it — the only exits are "I agree" (records consent
// server-side, then acknowledges the signal) or logout.
//
// Contract with backend (see `internal/handlers/consent.go` →
// `HandleBatchConsent`). POST /auth/consent is the BATCH endpoint; its
// body shape is deliberately different from the per-type REST handler
// at /api/privacy/consents:
//
//   POST /auth/consent  →  { terms_accepted: bool, privacy_accepted: bool,
//                             marketing_opt_in?: bool }
//
// The handler requires terms_accepted AND privacy_accepted to be true
// and records both consent types atomically server-side (one handler
// invocation, one audit trail entry per consent). This matches the
// onboarding flow's implicit-accept model — the user clicks "I agree"
// once and we capture both legally-required consents in one call.
// marketing_opt_in is optional; we don't collect it in this modal
// because analytics consent has its own settings-screen flow.
//
// Copy re-uses the onboarding implicit-accept language from PR #192 so
// the wording users see mid-session matches what they originally saw at
// onboarding. That matters for a consent flow — the user has already
// "agreed" implicitly when they first signed up, this modal is the
// explicit capture we skipped at the time.
// ---------------------------------------------------------------------------

const TERMS_URL = "https://toqui.travel/terms";
const PRIVACY_URL = "https://toqui.travel/privacy";

interface ConsentGateProps {
  children: React.ReactNode;
}

export function ConsentGate({ children }: ConsentGateProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const { accessToken, logout } = useAuth();
  const { track } = useAnalytics();
  const { consentRequired, acknowledgeConsent } = useConsentSignal();

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const openTerms = useCallback(() => {
    void WebBrowser.openBrowserAsync(TERMS_URL);
  }, []);

  const openPrivacy = useCallback(() => {
    void WebBrowser.openBrowserAsync(PRIVACY_URL);
  }, []);

  const handleAgree = useCallback(async () => {
    if (!accessToken || submitting) return;
    setSubmitting(true);
    setError(null);
    try {
      const apiUrl = getConfig().apiUrl;
      // Single batch call — POST /auth/consent records both required
      // consents server-side in one handler invocation.
      const res = await authFetch(`${apiUrl}/auth/consent`, accessToken, {
        method: "POST",
        body: JSON.stringify({
          terms_accepted: true,
          privacy_accepted: true,
        }),
      });
      if (!res.ok) {
        throw new Error(`Consent record failed: status=${res.status}`);
      }
      track("consent_recorded");
      acknowledgeConsent();
    } catch (e) {
      setError(t("consentGate.error"));
      // Don't ack — leave the gate up so the user can retry.
      // eslint-disable-next-line no-console
      console.error("ConsentGate submit failed", e);
    } finally {
      setSubmitting(false);
    }
  }, [accessToken, submitting, track, acknowledgeConsent, t]);

  const handleLogout = useCallback(async () => {
    try {
      await logout();
    } catch (e) {
      // Swallow: if logout fails we still clear auth state locally (the
      // AuthProvider does that in its `logout()` implementation before any
      // network call). Rethrowing would surface as an unhandled promise
      // rejection because the click handler is fire-and-forget.
      // eslint-disable-next-line no-console
      console.error("ConsentGate logout failed", e);
    } finally {
      // Clearing the signal here is defensive: logout tears down auth so
      // no RPC should fire again until the next login. But we still clear
      // so the gate unmounts immediately for UX.
      acknowledgeConsent();
    }
  }, [logout, acknowledgeConsent]);

  return (
    <>
      {children}
      <Modal
        visible={consentRequired}
        animationType="fade"
        transparent
        onRequestClose={() => {
          // Android back button — ignored. The user must accept or log out.
        }}
      >
        <View style={styles.backdrop}>
          <View
            style={[styles.card, { backgroundColor: colors.surface }]}
            testID="consent-gate"
          >
            <ScrollView contentContainerStyle={styles.scroll}>
              <Text style={[styles.title, { color: colors.accent }]}>
                {t("consentGate.title")}
              </Text>
              <Text
                style={[styles.subtitle, { color: colors.textSecondary }]}
              >
                {t("consentGate.subtitle")}
              </Text>

              <Text style={[styles.body, { color: colors.textSecondary }]}>
                {t("consentGate.termsNoticePrefix")}
                <Text
                  style={[styles.link, { color: colors.accent }]}
                  onPress={openTerms}
                  accessibilityRole="link"
                  testID="consent-gate-terms-link"
                >
                  {t("consentGate.termsLink")}
                </Text>
                {t("consentGate.termsNoticeSeparator")}
                <Text
                  style={[styles.link, { color: colors.accent }]}
                  onPress={openPrivacy}
                  accessibilityRole="link"
                  testID="consent-gate-privacy-link"
                >
                  {t("consentGate.privacyLink")}
                </Text>
                {t("consentGate.termsNoticeSuffix")}
              </Text>

              {error ? (
                <Text
                  style={[styles.error, { color: colors.error }]}
                  testID="consent-gate-error"
                >
                  {error}
                </Text>
              ) : null}

              <Pressable
                style={[
                  styles.primaryButton,
                  { backgroundColor: colors.accent },
                  submitting && styles.primaryButtonDisabled,
                ]}
                onPress={handleAgree}
                disabled={submitting}
                accessibilityRole="button"
                testID="consent-gate-agree"
              >
                {submitting ? (
                  <ActivityIndicator color={colors.userBubbleText} />
                ) : (
                  <Text
                    style={[
                      styles.primaryButtonText,
                      { color: colors.userBubbleText },
                    ]}
                  >
                    {t("consentGate.agree")}
                  </Text>
                )}
              </Pressable>

              <Pressable
                style={styles.secondaryButton}
                onPress={handleLogout}
                accessibilityRole="button"
                testID="consent-gate-logout"
              >
                <Text
                  style={[
                    styles.secondaryButtonText,
                    { color: colors.textSecondary },
                  ]}
                >
                  {t("consentGate.logout")}
                </Text>
              </Pressable>
            </ScrollView>
          </View>
        </View>
      </Modal>
    </>
  );
}

const styles = StyleSheet.create({
  backdrop: {
    flex: 1,
    backgroundColor: "rgba(0,0,0,0.55)",
    justifyContent: "center",
    alignItems: "center",
    padding: 16,
  },
  card: {
    width: "100%",
    maxWidth: 440,
    maxHeight: "90%",
    borderRadius: 16,
    overflow: "hidden",
    ...Platform.select({
      web: {
        boxShadow: "0 10px 30px rgba(0,0,0,0.25)",
      },
      default: {
        shadowColor: "#000",
        shadowOffset: { width: 0, height: 10 },
        shadowOpacity: 0.25,
        shadowRadius: 20,
        elevation: 10,
      },
    }),
  },
  scroll: {
    padding: 24,
  },
  title: {
    fontSize: 22,
    fontWeight: "700",
    textAlign: "center",
    marginBottom: 10,
  },
  subtitle: {
    fontSize: 15,
    lineHeight: 22,
    textAlign: "center",
    marginBottom: 16,
  },
  body: {
    fontSize: 14,
    lineHeight: 21,
    textAlign: "center",
    marginBottom: 20,
  },
  link: {
    fontWeight: "600",
    textDecorationLine: "underline",
  },
  error: {
    fontSize: 14,
    textAlign: "center",
    marginBottom: 12,
  },
  primaryButton: {
    borderRadius: 10,
    paddingVertical: 14,
    alignItems: "center",
    marginBottom: 10,
  },
  primaryButtonDisabled: {
    opacity: 0.6,
  },
  primaryButtonText: {
    fontSize: 16,
    fontWeight: "600",
  },
  secondaryButton: {
    paddingVertical: 10,
    alignItems: "center",
  },
  secondaryButtonText: {
    fontSize: 14,
    fontWeight: "500",
  },
});
