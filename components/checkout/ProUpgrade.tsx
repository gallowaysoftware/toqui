import { useState, useEffect, useCallback, useRef } from "react";
import {
  View,
  Text,
  StyleSheet,
  Pressable,
  Platform,
  ActivityIndicator,
  Linking,
} from "react-native";
import { useTranslation } from "react-i18next";
import { CheckCircle, Star, Mail, BookOpen, ExternalLink, ChevronRight, X } from "lucide-react-native";
import { useCheckout } from "@/lib/hooks/useCheckout";
import { useTheme } from "@/lib/theme";
import { useAnalytics } from "@/lib/analytics";

interface ProUpgradeProps {
  tripId: string;
  onUnlocked?: () => void;
  /** Render as a single-line inline banner instead of the full card */
  compact?: boolean;
  /** Called when the user dismisses the compact banner */
  onDismiss?: () => void;
}

declare global {
  interface Window {
    appendHelcimPayIframe?: (token: string) => void;
    helcimPaySuccess?: () => void;
  }
}

function loadHelcimJS(): Promise<void> {
  return new Promise((resolve, reject) => {
    if (document.getElementById("helcim-pay-js")) {
      resolve();
      return;
    }
    const script = document.createElement("script");
    script.id = "helcim-pay-js";
    script.src = "https://secure.helcim.com/helcim-pay/services/start.js";
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("Failed to load Helcim.js"));
    document.head.appendChild(script);
  });
}

export function ProUpgrade({ tripId, onUnlocked, compact, onDismiss }: ProUpgradeProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const { initCheckout, validatePayment, checkStatus, isLoading, error } = useCheckout(tripId);
  const { track } = useAnalytics();
  const [unlocked, setUnlocked] = useState<boolean | null>(null);
  const [checkingStatus, setCheckingStatus] = useState(true);
  const statusChecked = useRef(false);

  useEffect(() => {
    if (statusChecked.current) return;
    statusChecked.current = true;
    let cancelled = false;
    checkStatus()
      .then((status) => {
        if (!cancelled) setUnlocked(status.unlocked);
      })
      .catch(() => {
        if (!cancelled) setUnlocked(false);
      })
      .finally(() => {
        if (!cancelled) setCheckingStatus(false);
      });
    return () => {
      cancelled = true;
    };
  }, [checkStatus]);

  // Track when the upgrade UI is viewed (not already unlocked)
  useEffect(() => {
    if (!checkingStatus && unlocked === false) {
      track("upgrade_viewed");
      track("upgrade_prompt_shown", { trigger: compact ? "inline" : "settings" });
    }
  }, [checkingStatus, unlocked, track, compact]);

  const handleCheckout = useCallback(async () => {
    if (Platform.OS !== "web") return;

    track("upgrade_started");

    try {
      const checkout = await initCheckout();
      track("checkout_initiated");
      await loadHelcimJS();

      window.helcimPaySuccess = async () => {
        try {
          const responseElement = document.getElementById("helcimPayJsIdentityToken") as HTMLInputElement | null;
          const hashElement = document.getElementById("helcimPayJsHash") as HTMLInputElement | null;
          const response = responseElement?.value ?? "";
          const hash = hashElement?.value ?? "";

          const result = await validatePayment(response, hash);
          if (result.unlocked) {
            track("payment_completed");
            setUnlocked(true);
            onUnlocked?.();
          }
        } catch {
          track("payment_abandoned");
        }
      };

      if (window.appendHelcimPayIframe) {
        window.appendHelcimPayIframe(checkout.checkoutToken);
      }
    } catch {
      // Error is already captured in the hook
    }
  }, [initCheckout, validatePayment, onUnlocked, track]);

  const styles = StyleSheet.create({
    container: {
      backgroundColor: colors.surface,
      borderRadius: 12,
      padding: 16,
      marginBottom: 20,
      borderWidth: 1,
      borderColor: colors.border,
    },
    header: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
      marginBottom: 12,
    },
    title: {
      fontSize: 18,
      fontWeight: "700",
      color: colors.textPrimary,
    },
    price: {
      fontSize: 24,
      fontWeight: "700",
      color: colors.accent,
      marginBottom: 2,
    },
    priceDescription: {
      fontSize: 13,
      color: colors.textTertiary,
      marginBottom: 16,
    },
    benefits: {
      gap: 10,
      marginBottom: 20,
    },
    benefitRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: 10,
    },
    benefitText: {
      fontSize: 14,
      color: colors.textSecondary,
      flex: 1,
    },
    unlockButton: {
      backgroundColor: colors.accent,
      borderRadius: 10,
      paddingVertical: 14,
      alignItems: "center",
    },
    unlockButtonDisabled: {
      opacity: 0.6,
    },
    unlockButtonText: {
      color: "#fff",
      fontSize: 16,
      fontWeight: "600",
    },
    error: {
      color: colors.error,
      fontSize: 13,
      marginBottom: 12,
      textAlign: "center",
    },
    webOnly: {
      alignItems: "center",
      gap: 8,
    },
    webOnlyText: {
      fontSize: 14,
      color: colors.textSecondary,
      textAlign: "center",
    },
    webOnlyLink: {
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
    },
    webOnlyLinkText: {
      fontSize: 14,
      color: colors.accent,
      fontWeight: "500",
    },
    successContainer: {
      backgroundColor: colors.successBg,
      borderRadius: 12,
      padding: 20,
      marginBottom: 20,
      alignItems: "center",
      gap: 8,
      borderWidth: 1,
      borderColor: colors.border,
    },
    successTitle: {
      fontSize: 18,
      fontWeight: "700",
      color: colors.success,
    },
    successDescription: {
      fontSize: 14,
      color: colors.textSecondary,
      textAlign: "center",
    },
    compactContainer: {
      flexDirection: "row",
      alignItems: "center",
      backgroundColor: colors.accentSoft,
      borderRadius: 10,
      paddingVertical: 10,
      paddingHorizontal: 14,
      marginBottom: 16,
      gap: 8,
    },
    compactText: {
      flex: 1,
      fontSize: 14,
      color: colors.accent,
      fontWeight: "600",
    },
    compactDismiss: {
      padding: 4,
    },
  });

  if (checkingStatus) {
    return (
      <View style={styles.container}>
        <ActivityIndicator size="small" color={colors.accent} />
      </View>
    );
  }

  if (unlocked) {
    return (
      <View style={styles.successContainer}>
        <CheckCircle color={colors.success} size={28} />
        <Text style={styles.successTitle}>{t("checkout.success")}</Text>
        <Text style={styles.successDescription}>
          {t("checkout.successDescription")}
        </Text>
      </View>
    );
  }

  if (compact) {
    return (
      <View style={styles.compactContainer as object}>
        <Star color={colors.accent} size={16} />
        <Pressable
          style={{ flex: 1 }}
          onPress={() => {
            // In compact mode, trigger full checkout flow on web
            if (Platform.OS === "web") {
              void handleCheckout();
            }
          }}
          accessibilityRole="button"
        >
          <Text style={styles.compactText}>
            {t("checkout.unlockInline")}
          </Text>
        </Pressable>
        <ChevronRight color={colors.accent} size={16} />
        {onDismiss && (
          <Pressable
            style={styles.compactDismiss}
            onPress={onDismiss}
            accessibilityRole="button"
            accessibilityLabel={t("common.dismiss")}
          >
            <X color={colors.textTertiary} size={14} />
          </Pressable>
        )}
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Star color={colors.accent} size={22} />
        <Text style={styles.title}>{t("checkout.title")}</Text>
      </View>

      <Text style={styles.price}>{t("checkout.price")}</Text>
      <Text style={styles.priceDescription}>
        {t("checkout.priceDescription")}
      </Text>

      <View style={styles.benefits}>
        <View style={styles.benefitRow}>
          <BookOpen color={colors.textSecondary} size={16} />
          <Text style={styles.benefitText}>{t("checkout.benefits.experts")}</Text>
        </View>
        <View style={styles.benefitRow}>
          <CheckCircle color={colors.textSecondary} size={16} />
          <Text style={styles.benefitText}>{t("checkout.benefits.bookings")}</Text>
        </View>
        <View style={styles.benefitRow}>
          <Mail color={colors.textSecondary} size={16} />
          <Text style={styles.benefitText}>{t("checkout.benefits.email")}</Text>
        </View>
      </View>

      {error && <Text style={styles.error}>{t("checkout.error")}</Text>}

      {Platform.OS === "web" ? (
        <Pressable
          style={[styles.unlockButton, isLoading && styles.unlockButtonDisabled]}
          onPress={handleCheckout}
          disabled={isLoading}
        >
          {isLoading ? (
            <ActivityIndicator size="small" color="#fff" />
          ) : (
            <Text style={styles.unlockButtonText}>
              {t("checkout.unlockButton")}
            </Text>
          )}
        </Pressable>
      ) : (
        <View style={styles.webOnly}>
          <Text style={styles.webOnlyText}>{t("checkout.webOnly")}</Text>
          <Pressable
            style={styles.webOnlyLink}
            onPress={() => Linking.openURL("https://toqui.app")}
          >
            <ExternalLink color={colors.accent} size={14} />
            <Text style={styles.webOnlyLinkText}>
              {t("checkout.webOnlyLink")}
            </Text>
          </Pressable>
        </View>
      )}
    </View>
  );
}
