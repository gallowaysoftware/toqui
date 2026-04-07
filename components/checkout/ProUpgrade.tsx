import { useState, useEffect, useCallback, useRef } from "react";
import {
  View,
  Text,
  StyleSheet,
  Pressable,
  ActivityIndicator,
  Linking,
} from "react-native";
import { useTranslation } from "react-i18next";
import { CheckCircle, Star, Mail, BookOpen, ChevronRight, X } from "lucide-react-native";
import { useCheckout } from "@/lib/hooks/useCheckout";
import { useSubscription } from "@/lib/hooks/useSubscription";
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

export function ProUpgrade({ tripId, onUnlocked, compact, onDismiss }: ProUpgradeProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const { initCheckout, checkStatus, isLoading, error } = useCheckout(tripId);
  const { subscription } = useSubscription();
  const { track, getFeatureFlag } = useAnalytics();
  const isSubscriber =
    subscription?.tier === "explorer" || subscription?.tier === "voyager";
  const [unlocked, setUnlocked] = useState<boolean | null>(null);
  const [checkingStatus, setCheckingStatus] = useState(true);
  const statusChecked = useRef(false);

  // A/B price test: PostHog flag provides the initial display value, but
  // the backend is the source of truth for the actual charge amount.
  // Once checkStatus returns, we use the backend-reported price.
  const rawFlag = getFeatureFlag("trip-pro-price");
  const VALID_PRICES = ["15", "19", "24"] as const;
  const flagValue = typeof rawFlag === "string" ? rawFlag : "19";
  const priceVariant = VALID_PRICES.includes(flagValue as typeof VALID_PRICES[number])
    ? flagValue
    : "19";

  // Backend-authoritative price (set after status check)
  const [serverPriceCents, setServerPriceCents] = useState<number | null>(null);
  const priceDisplay = serverPriceCents
    ? `$${Math.round(serverPriceCents / 100)} CAD`
    : `$${priceVariant} CAD`;

  useEffect(() => {
    if (statusChecked.current) return;
    statusChecked.current = true;
    let cancelled = false;
    checkStatus()
      .then((status) => {
        if (!cancelled) {
          setUnlocked(status.unlocked);
          if (status.price_cents) setServerPriceCents(status.price_cents);
        }
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
      track("upgrade_prompt_shown", { trigger: compact ? "inline" : "settings", price_variant: priceVariant });
    }
  }, [checkingStatus, unlocked, track, compact, priceVariant]);

  const [withdrawalConsent, setWithdrawalConsent] = useState(false);
  const [consentTimestamp, setConsentTimestamp] = useState<string | null>(null);

  // Whether payment was initiated but unlock not yet confirmed
  const [paymentPending, setPaymentPending] = useState(false);

  // Poll for unlock status after returning from Stripe checkout
  const pollForUnlock = useCallback(async () => {
    setPaymentPending(true);
    const maxAttempts = 10;
    const intervalMs = 2000;
    for (let i = 0; i < maxAttempts; i++) {
      try {
        const status = await checkStatus();
        if (status.unlocked) {
          track("payment_completed");
          setUnlocked(true);
          setPaymentPending(false);
          onUnlocked?.();
          return;
        }
      } catch {
        // Keep polling on transient errors
      }
      await new Promise((resolve) => setTimeout(resolve, intervalMs));
    }
    // Polling exhausted — payment may still be processing on Stripe's side
    setPaymentPending(true);
  }, [checkStatus, onUnlocked, track]);

  const handleCheckout = useCallback(async () => {
    if (!withdrawalConsent) return;

    if (!consentTimestamp) {
      setConsentTimestamp(new Date().toISOString());
    }

    track("upgrade_started", { withdrawal_consent: true });

    try {
      const checkout = await initCheckout(priceVariant);
      track("checkout_initiated");

      // Redirect to Stripe hosted checkout
      await Linking.openURL(checkout.url);

      // Poll for unlock after user completes payment and returns
      void pollForUnlock();
    } catch {
      // Error is already captured in the hook
    }
  }, [initCheckout, withdrawalConsent, consentTimestamp, pollForUnlock, track, priceVariant]);

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
    upsellContainer: {
      backgroundColor: colors.accentSoft,
      borderRadius: 10,
      padding: 14,
      marginTop: 12,
      gap: 6,
    },
    upsellTitle: {
      fontSize: 14,
      fontWeight: "700",
      color: colors.accent,
    },
    upsellDescription: {
      fontSize: 13,
      color: colors.textSecondary,
    },
    upsellLink: {
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
      marginTop: 4,
    },
    upsellLinkText: {
      fontSize: 14,
      fontWeight: "600",
      color: colors.accent,
    },
    pendingContainer: {
      backgroundColor: colors.accentSoft,
      borderRadius: 10,
      padding: 14,
      marginBottom: 12,
      alignItems: "center",
      gap: 8,
    },
    pendingText: {
      fontSize: 13,
      color: colors.textSecondary,
      textAlign: "center",
    },
    retryButton: {
      paddingVertical: 6,
      paddingHorizontal: 16,
      borderRadius: 6,
      borderWidth: 1,
      borderColor: colors.accent,
    },
    retryButtonText: {
      fontSize: 13,
      color: colors.accent,
      fontWeight: "600",
    },
    euConsentRow: {
      flexDirection: "row",
      alignItems: "flex-start",
      gap: 10,
      marginBottom: 16,
      padding: 12,
      backgroundColor: colors.surfaceSecondary,
      borderRadius: 8,
      borderWidth: 1,
      borderColor: colors.border,
    },
    euCheckbox: {
      width: 20,
      height: 20,
      borderRadius: 4,
      borderWidth: 2,
      borderColor: colors.accent,
      alignItems: "center",
      justifyContent: "center",
      marginTop: 1,
      flexShrink: 0,
    },
    euCheckboxChecked: {
      backgroundColor: colors.accent,
    },
    euCheckboxMark: {
      color: "#fff",
      fontSize: 13,
      fontWeight: "700",
      lineHeight: 14,
    },
    euConsentText: {
      flex: 1,
      fontSize: 12,
      color: colors.textSecondary,
      lineHeight: 18,
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
      <View>
        <View style={styles.successContainer}>
          <CheckCircle color={colors.success} size={28} />
          <Text style={styles.successTitle}>{t("checkout.success")}</Text>
          <Text style={styles.successDescription}>
            {t("checkout.successDescription")}
          </Text>
        </View>
        {!isSubscriber && (
          <View style={styles.upsellContainer as object}>
            <Text style={styles.upsellTitle}>
              {t("subscription.upsell.title")}
            </Text>
            <Text style={styles.upsellDescription}>
              {t("subscription.upsell.description")}
            </Text>
            <Pressable
              style={styles.upsellLink as object}
              onPress={() => {
                // Navigate to settings where SubscriptionCard lives
                // Using Linking to avoid deep router dependency
              }}
              accessibilityRole="button"
            >
              <ChevronRight color={colors.accent} size={14} />
              <Text style={styles.upsellLinkText}>
                {t("subscription.subscribe")}
              </Text>
            </Pressable>
          </View>
        )}
      </View>
    );
  }

  if (compact) {
    return (
      <View style={styles.compactContainer as object}>
        <Star color={colors.accent} size={16} />
        <Pressable
          style={{ flex: 1 }}
          onPress={() => void handleCheckout()}
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

      <Text style={styles.price}>{priceDisplay}</Text>
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

      <Pressable
        style={styles.euConsentRow}
        onPress={() => setWithdrawalConsent((v) => !v)}
        accessibilityRole="checkbox"
        accessibilityState={{ checked: withdrawalConsent }}
        accessibilityLabel="Digital content consent"
      >
        <View style={[styles.euCheckbox, withdrawalConsent && styles.euCheckboxChecked]}>
          {withdrawalConsent ? <Text style={styles.euCheckboxMark}>✓</Text> : null}
        </View>
        <Text style={styles.euConsentText}>
          I consent to immediate access to digital content and acknowledge that I waive
          my 14-day right of withdrawal once I begin using the service.
        </Text>
      </Pressable>

      {error && <Text style={styles.error}>{t("checkout.error")}</Text>}

      {paymentPending && !unlocked && (
        <View style={styles.pendingContainer}>
          <ActivityIndicator size="small" color={colors.accent} />
          <Text style={styles.pendingText}>
            {t("checkout.paymentProcessing")}
          </Text>
          <Pressable
            style={styles.retryButton}
            onPress={() => void pollForUnlock()}
            accessibilityRole="button"
          >
            <Text style={styles.retryButtonText}>{t("common.retry")}</Text>
          </Pressable>
        </View>
      )}

      <Pressable
        style={[
          styles.unlockButton,
          (isLoading || paymentPending || !withdrawalConsent) && styles.unlockButtonDisabled,
        ]}
        onPress={handleCheckout}
        disabled={isLoading || paymentPending || !withdrawalConsent}
      >
        {isLoading ? (
          <ActivityIndicator size="small" color="#fff" />
        ) : (
          <Text style={styles.unlockButtonText}>
            {t("checkout.unlockButton")}
          </Text>
        )}
      </Pressable>
    </View>
  );
}
