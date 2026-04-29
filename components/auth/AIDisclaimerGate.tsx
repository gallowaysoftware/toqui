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

async function getStorageItem(key: string): Promise<string | null> {
  if (Platform.OS === "web") {
    return localStorage.getItem(key);
  }
  const { getItemAsync } = await import("expo-secure-store");
  return getItemAsync(key);
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
    void getStorageItem(STORAGE_KEY_PREFIX + user.id).then((v) => {
      if (!cancelled) setNeedsAck(v !== "true");
    });
    return () => {
      cancelled = true;
    };
  }, [user?.id]);

  const handleAcknowledge = useCallback(async () => {
    if (!user?.id) return;
    await setStorageItem(STORAGE_KEY_PREFIX + user.id, "true");
    // Server-side audit trail via PostHog. The event timestamp + the
    // hashed distinct_id together prove "this user clicked through this
    // disclaimer at this time" — sufficient for a side-project liability
    // surface; replace with a backend consent row if usage scales.
    track("ai_disclaimer_acknowledged");
    setNeedsAck(false);
  }, [user?.id, track]);

  return (
    <>
      {children}
      <Modal
        visible={needsAck === true}
        animationType="fade"
        transparent
        onRequestClose={() => {
          // Android back button — ignored. The modal must be acknowledged.
        }}
      >
        <View style={styles.backdrop}>
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
