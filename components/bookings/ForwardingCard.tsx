import { View, Text, StyleSheet, Pressable, Platform } from "react-native";
import { useState, useCallback, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Mail, Copy, Check, ChevronDown, ChevronUp, X } from "lucide-react-native";
import { useTheme } from "@/lib/theme";

const FORWARDING_EMAIL = "bookings@mail.toqui.travel";

/** Key used to persist the dismissed state per trip. */
function dismissedKey(tripId: string) {
  return `toqui_forwarding_dismissed_${tripId}`;
}

async function loadDismissed(tripId: string): Promise<boolean> {
  if (Platform.OS === "web") {
    return localStorage.getItem(dismissedKey(tripId)) === "1";
  }
  try {
    const AsyncStorage = (await import("@react-native-async-storage/async-storage")).default;
    return (await AsyncStorage.getItem(dismissedKey(tripId))) === "1";
  } catch {
    return false;
  }
}

async function persistDismissed(tripId: string): Promise<void> {
  if (Platform.OS === "web") {
    localStorage.setItem(dismissedKey(tripId), "1");
    return;
  }
  try {
    const AsyncStorage = (await import("@react-native-async-storage/async-storage")).default;
    await AsyncStorage.setItem(dismissedKey(tripId), "1");
  } catch {
    // Silently fail — non-critical persistence.
  }
}

interface ForwardingCardProps {
  tripId: string;
}

export default function ForwardingCard({ tripId }: ForwardingCardProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const [copied, setCopied] = useState(false);
  const [dismissed, setDismissed] = useState(true); // start hidden until loaded
  const [howItWorksOpen, setHowItWorksOpen] = useState(false);

  useEffect(() => {
    loadDismissed(tripId).then((d) => setDismissed(d));
  }, [tripId]);

  const handleCopy = useCallback(async () => {
    try {
      if (Platform.OS === "web") {
        await navigator.clipboard.writeText(FORWARDING_EMAIL);
      } else {
        const Clipboard = await import("expo-clipboard");
        await Clipboard.setStringAsync(FORWARDING_EMAIL);
      }
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard not available — fail silently.
    }
  }, []);

  const handleDismiss = useCallback(() => {
    setDismissed(true);
    void persistDismissed(tripId);
  }, [tripId]);

  if (dismissed) return null;

  const styles = StyleSheet.create({
    card: {
      backgroundColor: colors.infoBg,
      borderRadius: 12,
      padding: 16,
      marginBottom: 12,
      borderWidth: 1,
      borderColor: colors.infoBorder,
    },
    header: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
    },
    headerLeft: {
      flexDirection: "row",
      alignItems: "center",
      gap: 10,
    },
    title: {
      fontSize: 15,
      fontWeight: "600",
      color: colors.textPrimary,
    },
    description: {
      fontSize: 13,
      color: colors.textSecondary,
      marginTop: 8,
      lineHeight: 18,
    },
    emailRow: {
      flexDirection: "row",
      alignItems: "center",
      backgroundColor: colors.surface,
      borderRadius: 8,
      borderWidth: 1,
      borderColor: colors.border,
      marginTop: 12,
      overflow: "hidden",
    },
    emailText: {
      flex: 1,
      fontSize: 13,
      fontFamily: Platform.OS === "ios" ? "Menlo" : "monospace",
      color: colors.textPrimary,
      paddingHorizontal: 12,
      paddingVertical: 10,
    },
    copyButton: {
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
      paddingHorizontal: 12,
      paddingVertical: 10,
      backgroundColor: colors.accent,
      borderRadius: 0,
    },
    copyButtonCopied: {
      backgroundColor: colors.success,
    },
    copyText: {
      fontSize: 13,
      fontWeight: "600",
      color: "#fff",
    },
    howItWorks: {
      marginTop: 12,
    },
    howItWorksToggle: {
      flexDirection: "row",
      alignItems: "center",
      gap: 4,
    },
    howItWorksToggleText: {
      fontSize: 13,
      fontWeight: "600",
      color: colors.info,
    },
    howItWorksContent: {
      marginTop: 8,
    },
    step: {
      flexDirection: "row",
      alignItems: "flex-start",
      marginBottom: 6,
    },
    stepNumber: {
      fontSize: 12,
      fontWeight: "700",
      color: colors.info,
      width: 20,
    },
    stepText: {
      flex: 1,
      fontSize: 13,
      color: colors.textSecondary,
      lineHeight: 18,
    },
  });

  return (
    <View style={styles.card}>
      <View style={styles.header}>
        <View style={styles.headerLeft}>
          <Mail color={colors.info} size={20} />
          <Text style={styles.title}>{t("bookings.forwarding.title")}</Text>
        </View>
        <Pressable onPress={handleDismiss} hitSlop={8} accessibilityLabel={t("common.cancel")}>
          <X color={colors.textTertiary} size={18} />
        </Pressable>
      </View>

      <Text style={styles.description}>
        {t("bookings.forwarding.description")}
      </Text>

      <View style={styles.emailRow}>
        <Text style={styles.emailText} selectable>
          {FORWARDING_EMAIL}
        </Text>
        <Pressable
          style={[styles.copyButton, copied && styles.copyButtonCopied]}
          onPress={handleCopy}
          accessibilityRole="button"
          accessibilityLabel={copied ? t("referral.copied") : t("bookings.forwarding.copy")}
        >
          {copied ? (
            <>
              <Check color="#fff" size={14} />
              <Text style={styles.copyText}>{t("referral.copied")}</Text>
            </>
          ) : (
            <>
              <Copy color="#fff" size={14} />
              <Text style={styles.copyText}>{t("bookings.forwarding.copy")}</Text>
            </>
          )}
        </Pressable>
      </View>

      <View style={styles.howItWorks}>
        <Pressable
          style={styles.howItWorksToggle}
          onPress={() => setHowItWorksOpen((o) => !o)}
          accessibilityRole="button"
        >
          <Text style={styles.howItWorksToggleText}>
            {t("bookings.forwarding.howItWorks")}
          </Text>
          {howItWorksOpen ? (
            <ChevronUp color={colors.info} size={16} />
          ) : (
            <ChevronDown color={colors.info} size={16} />
          )}
        </Pressable>

        {howItWorksOpen && (
          <View style={styles.howItWorksContent}>
            <View style={styles.step}>
              <Text style={styles.stepNumber}>1.</Text>
              <Text style={styles.stepText}>{t("bookings.forwarding.step1")}</Text>
            </View>
            <View style={styles.step}>
              <Text style={styles.stepNumber}>2.</Text>
              <Text style={styles.stepText}>{t("bookings.forwarding.step2")}</Text>
            </View>
            <View style={styles.step}>
              <Text style={styles.stepNumber}>3.</Text>
              <Text style={styles.stepText}>{t("bookings.forwarding.step3")}</Text>
            </View>
          </View>
        )}
      </View>
    </View>
  );
}
