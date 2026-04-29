import { useCallback, useEffect, useState } from "react";
import {
  Modal,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from "react-native";
import { useTranslation } from "react-i18next";
import { useAuth } from "@/lib/auth";
import { useAnalytics } from "@/lib/analytics";
import { useTheme } from "@/lib/theme";

// ---------------------------------------------------------------------------
// AIDisclaimerGate
//
// One-time blocking modal that fires after a user signs in for the first
// time on this device, before they use any AI feature. Surfaces the
// "AI may be wrong, especially on visa/health/safety; verify before
// booking" disclaimer that's the legal complement to Trip Pro's $19
// charge — without it, every AI-hallucinated visa rule that costs a
// user a flight is a liability surface for a sole proprietor.
//
// Acceptance is stored locally (expo-secure-store on native, localStorage
// on web) AND tracked via PostHog with the user's pseudonymised ID so we
// have a server-side log of who acknowledged when. Legal-grade evidence
// would require a backend consent record (mirroring ConsentGate's
// /auth/consent flow with an "ai_disclaimer" consent type) — that's a
// follow-up that adds backend work; for now PostHog is sufficient
// signal that the prompt was seen and acknowledged at a specific time.
//
// Storage key includes a version suffix so we can re-prompt every user
// after material wording changes without writing a migration.
// ---------------------------------------------------------------------------

const STORAGE_KEY_PREFIX = "toqui_ai_disclaimer_acked_v1_";

// Storage helpers swallow read errors and surface them as null so the
// caller's "no value found" branch handles the same way as "couldn't
// read" — see useEffect below for the fail-CLOSED policy. Write errors
// rethrow so the caller can decide whether to surface them; in this
// component we swallow them in handleAcknowledge to avoid trapping the
// user on the modal (the audit trail is the PostHog event, not the
// local flag — see issue #198).
async function getStorageItem(key: string): Promise<string | null> {
  try {
    if (Platform.OS === "web") {
      return localStorage.getItem(key);
    }
    const { getItemAsync } = await import("expo-secure-store");
    return getItemAsync(key);
  } catch {
    return null;
  }
}

async function setStorageItem(key: string, value: string): Promise<void> {
  if (Platform.OS === "web") {
    localStorage.setItem(key, value);
    return;
  }
  const { setItemAsync } = await import("expo-secure-store");
  await setItemAsync(key, value);
}

interface AIDisclaimerGateProps {
  children: React.ReactNode;
}

export function AIDisclaimerGate({ children }: AIDisclaimerGateProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const { user } = useAuth();
  const { track } = useAnalytics();

  // tri-state: null = loading (don't render modal yet), true = show modal,
  // false = already acknowledged or no user
  const [needsAck, setNeedsAck] = useState<boolean | null>(null);

  useEffect(() => {
    if (!user?.id) {
      setNeedsAck(false);
      return;
    }
    let cancelled = false;
    // Fail-CLOSED via getStorageItem's swallow-to-null: a read error
    // (iOS Safari private mode, quota exceeded, secure-store import
    // failure) returns null, which `null !== "true"` evaluates to true,
    // so the modal shows. Better to over-prompt than to bypass the
    // legal gate (issue #198).
    void getStorageItem(STORAGE_KEY_PREFIX + user.id).then((v) => {
      if (!cancelled) setNeedsAck(v !== "true");
    });
    return () => {
      cancelled = true;
    };
  }, [user?.id]);

  const handleAcknowledge = useCallback(async () => {
    if (!user?.id) return;
    // Track FIRST — PostHog is the audit trail; the local storage write
    // is just a "don't re-prompt" UX nicety. If the storage write throws
    // (quota, private mode, etc.) we still want the audit-trail event
    // to land and we still want the user to proceed past the modal —
    // they'll see it again on next session, which is acceptable.
    // Pre-fix, a thrown setItemAsync would leave the user stuck on the
    // modal forever (issue #198).
    track("ai_disclaimer_acknowledged");
    try {
      await setStorageItem(STORAGE_KEY_PREFIX + user.id, "true");
    } catch {
      // intentionally swallow — see comment above
    }
    setNeedsAck(false);
  }, [user?.id, track]);

  return (
    <>
      {children}
      <Modal
        visible={needsAck === true}
        animationType="fade"
        transparent
        // accessibilityViewIsModal tells iOS VoiceOver to trap focus
        // inside the modal — without it, swipe-to-explore can land on
        // the backgrounded children and screen-reader users can dismiss
        // the modal accidentally without acknowledging.
        // eslint-disable-next-line react/no-unknown-property
        accessibilityViewIsModal
        onRequestClose={() => {
          // Android back button — ignored. The modal must be acknowledged.
        }}
      >
        <View
          style={styles.backdrop}
          // role="dialog" on web (react-native-web maps it from accessibilityRole),
          // ignored on native. Combined with accessibilityViewIsModal above this
          // makes the modal announce itself to assistive tech on every platform.
          accessibilityRole={Platform.OS === "web" ? ("dialog" as never) : undefined}
          aria-modal={Platform.OS === "web" ? true : undefined}
        >
          <View
            style={[styles.card, { backgroundColor: colors.surface }]}
            testID="ai-disclaimer-gate"
          >
            <ScrollView contentContainerStyle={styles.scroll}>
              <Text style={[styles.title, { color: colors.accent }]}>
                {t("aiDisclaimer.title")}
              </Text>
              <Text style={[styles.subtitle, { color: colors.textSecondary }]}>
                {t("aiDisclaimer.subtitle")}
              </Text>

              <View style={styles.bulletGroup}>
                <BulletRow
                  color={colors.textSecondary}
                  emphasis={colors.accent}
                  emphasisText={t("aiDisclaimer.bullet1Heading")}
                  text={t("aiDisclaimer.bullet1Body")}
                />
                <BulletRow
                  color={colors.textSecondary}
                  emphasis={colors.accent}
                  emphasisText={t("aiDisclaimer.bullet2Heading")}
                  text={t("aiDisclaimer.bullet2Body")}
                />
                <BulletRow
                  color={colors.textSecondary}
                  emphasis={colors.accent}
                  emphasisText={t("aiDisclaimer.bullet3Heading")}
                  text={t("aiDisclaimer.bullet3Body")}
                />
              </View>

              <Pressable
                style={[styles.primaryButton, { backgroundColor: colors.accent }]}
                onPress={() => void handleAcknowledge()}
                accessibilityRole="button"
                testID="ai-disclaimer-acknowledge"
              >
                <Text
                  style={[styles.primaryButtonText, { color: colors.userBubbleText }]}
                >
                  {t("aiDisclaimer.acknowledge")}
                </Text>
              </Pressable>
            </ScrollView>
          </View>
        </View>
      </Modal>
    </>
  );
}

interface BulletRowProps {
  color: string;
  emphasis: string;
  emphasisText: string;
  text: string;
}

function BulletRow({ color, emphasis, emphasisText, text }: BulletRowProps) {
  return (
    <Text style={[styles.bullet, { color }]}>
      <Text style={[styles.bulletEmphasis, { color: emphasis }]}>
        {emphasisText}
      </Text>{" "}
      {text}
    </Text>
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
    marginBottom: 18,
  },
  bulletGroup: {
    marginBottom: 22,
  },
  bullet: {
    fontSize: 14,
    lineHeight: 21,
    marginBottom: 12,
  },
  bulletEmphasis: {
    fontWeight: "700",
  },
  primaryButton: {
    borderRadius: 10,
    paddingVertical: 14,
    alignItems: "center",
  },
  primaryButtonText: {
    fontSize: 16,
    fontWeight: "600",
  },
});
