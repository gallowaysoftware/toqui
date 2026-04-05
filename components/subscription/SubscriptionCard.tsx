import { useState, useCallback } from "react";
import {
  View,
  Text,
  StyleSheet,
  Pressable,
  ActivityIndicator,
} from "react-native";
import { useTranslation } from "react-i18next";
import {
  CheckCircle,
  Crown,
  Globe,
  Zap,
  MessageCircle,
  Users,
  ExternalLink,
  AlertCircle,
} from "lucide-react-native";
import { useTheme } from "@/lib/theme";
import {
  useSubscription,
  type SubscriptionTier,
} from "@/lib/hooks/useSubscription";
import { useAnalytics } from "@/lib/analytics";

const EXPLORER_MONTHLY = 9.99;
const EXPLORER_ANNUAL = 79.99;
const VOYAGER_MONTHLY = 17.99;
const VOYAGER_ANNUAL = 143.99;

function formatDate(date: Date): string {
  return date.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function annualSavingsPercent(monthly: number, annual: number): number {
  return Math.round((1 - annual / (monthly * 12)) * 100);
}

export function SubscriptionCard() {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const { subscription, isLoading, error, subscribe, cancel, manageSubscription } =
    useSubscription();
  const { track } = useAnalytics();
  const [annual, setAnnual] = useState(false);
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  const currentTier: SubscriptionTier = subscription?.tier ?? "free";
  const isActive = subscription?.status === "active" || subscription?.status === "past_due";

  const handleSubscribe = useCallback(
    async (tier: "explorer" | "voyager") => {
      setActionLoading(tier);
      track("subscription_started", { tier, annual });
      try {
        await subscribe(tier, annual);
      } catch {
        // Error captured in hook
      } finally {
        setActionLoading(null);
      }
    },
    [subscribe, annual, track],
  );

  const handleCancel = useCallback(async () => {
    setActionLoading("cancel");
    track("subscription_cancel_started");
    try {
      await cancel();
    } catch {
      // Error captured in hook
    } finally {
      setActionLoading(null);
    }
  }, [cancel, track]);

  const handleManage = useCallback(async () => {
    setActionLoading("manage");
    track("subscription_manage_opened");
    try {
      await manageSubscription();
    } catch {
      // Error captured in hook
    } finally {
      setActionLoading(null);
    }
  }, [manageSubscription, track]);

  const styles = StyleSheet.create({
    container: {
      backgroundColor: colors.surface,
      borderRadius: 12,
      padding: 16,
      borderWidth: 1,
      borderColor: colors.border,
    },
    header: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
      marginBottom: 16,
    },
    title: {
      fontSize: 18,
      fontWeight: "700",
      color: colors.textPrimary,
    },
    toggleRow: {
      flexDirection: "row",
      backgroundColor: colors.surfaceTertiary,
      borderRadius: 8,
      padding: 2,
      marginBottom: 16,
    },
    toggleOption: {
      flex: 1,
      paddingVertical: 8,
      alignItems: "center",
      borderRadius: 6,
    },
    toggleOptionActive: {
      backgroundColor: colors.surface,
    },
    toggleText: {
      fontSize: 13,
      fontWeight: "500",
      color: colors.textTertiary,
    },
    toggleTextActive: {
      color: colors.textPrimary,
      fontWeight: "600",
    },
    savingsBadge: {
      fontSize: 11,
      fontWeight: "700",
      color: colors.success,
      marginTop: 2,
    },
    tierCard: {
      borderWidth: 1,
      borderColor: colors.border,
      borderRadius: 10,
      padding: 14,
      marginBottom: 12,
    },
    tierCardCurrent: {
      borderColor: colors.accent,
      backgroundColor: colors.accentSoft,
    },
    tierHeader: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      marginBottom: 8,
    },
    tierNameRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
    },
    tierName: {
      fontSize: 16,
      fontWeight: "700",
      color: colors.textPrimary,
    },
    currentBadge: {
      backgroundColor: colors.accent,
      paddingHorizontal: 8,
      paddingVertical: 2,
      borderRadius: 10,
    },
    currentBadgeText: {
      fontSize: 11,
      fontWeight: "700",
      color: "#fff",
    },
    tierPrice: {
      fontSize: 18,
      fontWeight: "700",
      color: colors.accent,
    },
    tierPriceUnit: {
      fontSize: 13,
      fontWeight: "400",
      color: colors.textTertiary,
    },
    annualPrice: {
      fontSize: 12,
      color: colors.success,
      fontWeight: "600",
      marginBottom: 8,
    },
    tierFeatures: {
      gap: 6,
      marginBottom: 12,
    },
    featureRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
    },
    featureText: {
      fontSize: 13,
      color: colors.textSecondary,
      flex: 1,
    },
    subscribeButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      paddingVertical: 10,
      alignItems: "center",
    },
    subscribeButtonDisabled: {
      opacity: 0.6,
    },
    subscribeButtonText: {
      color: "#fff",
      fontSize: 14,
      fontWeight: "600",
    },
    statusSection: {
      borderTopWidth: 1,
      borderTopColor: colors.border,
      paddingTop: 14,
      marginTop: 4,
      gap: 8,
    },
    statusRow: {
      flexDirection: "row",
      justifyContent: "space-between",
      alignItems: "center",
    },
    statusLabel: {
      fontSize: 13,
      color: colors.textTertiary,
    },
    statusValue: {
      fontSize: 13,
      fontWeight: "600",
      color: colors.textSecondary,
    },
    statusWarning: {
      color: colors.error,
    },
    cancelingText: {
      fontSize: 13,
      color: colors.error,
      fontStyle: "italic",
    },
    manageButton: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "center",
      gap: 6,
      borderWidth: 1,
      borderColor: colors.accent,
      borderRadius: 8,
      paddingVertical: 10,
      marginTop: 4,
    },
    manageButtonText: {
      color: colors.accent,
      fontSize: 14,
      fontWeight: "600",
    },
    cancelButton: {
      alignItems: "center",
      paddingVertical: 8,
      marginTop: 4,
    },
    cancelButtonText: {
      color: colors.textTertiary,
      fontSize: 13,
    },
    error: {
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
      marginBottom: 12,
    },
    errorText: {
      color: colors.error,
      fontSize: 13,
      flex: 1,
    },
    freeCard: {
      borderWidth: 1,
      borderColor: colors.border,
      borderRadius: 10,
      padding: 14,
      marginBottom: 12,
    },
    freePrice: {
      fontSize: 16,
      fontWeight: "700",
      color: colors.textSecondary,
      marginBottom: 4,
    },
    freeDescription: {
      fontSize: 13,
      color: colors.textTertiary,
    },
  });

  if (isLoading) {
    return (
      <View style={styles.container}>
        <ActivityIndicator size="small" color={colors.accent} />
      </View>
    );
  }

  const explorerMonthly = annual
    ? `$${(EXPLORER_ANNUAL / 12).toFixed(2)}`
    : `$${EXPLORER_MONTHLY.toFixed(2)}`;
  const voyagerMonthly = annual
    ? `$${(VOYAGER_ANNUAL / 12).toFixed(2)}`
    : `$${VOYAGER_MONTHLY.toFixed(2)}`;

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Crown color={colors.accent} size={22} />
        <Text style={styles.title}>{t("subscription.title")}</Text>
      </View>

      {error && (
        <View style={styles.error}>
          <AlertCircle color={colors.error} size={16} />
          <Text style={styles.errorText}>{error}</Text>
        </View>
      )}

      {/* Monthly / Annual toggle */}
      <View style={styles.toggleRow}>
        <Pressable
          style={[styles.toggleOption, !annual && styles.toggleOptionActive]}
          onPress={() => setAnnual(false)}
          accessibilityRole="button"
        >
          <Text style={[styles.toggleText, !annual && styles.toggleTextActive]}>
            {t("subscription.monthly")}
          </Text>
        </Pressable>
        <Pressable
          style={[styles.toggleOption, annual && styles.toggleOptionActive]}
          onPress={() => setAnnual(true)}
          accessibilityRole="button"
        >
          <Text style={[styles.toggleText, annual && styles.toggleTextActive]}>
            {t("subscription.annual")}
          </Text>
          <Text style={styles.savingsBadge}>{t("subscription.saveBadge")}</Text>
        </Pressable>
      </View>

      {/* Free tier */}
      <View style={[styles.freeCard, currentTier === "free" && styles.tierCardCurrent]}>
        <View style={styles.tierHeader}>
          <View style={styles.tierNameRow}>
            <Text style={styles.tierName}>{t("subscription.free.name")}</Text>
            {currentTier === "free" && (
              <View style={styles.currentBadge}>
                <Text style={styles.currentBadgeText}>
                  {t("subscription.currentTier")}
                </Text>
              </View>
            )}
          </View>
          <Text style={styles.freePrice}>$0</Text>
        </View>
        <Text style={styles.freeDescription}>
          {t("subscription.free.description")}
        </Text>
      </View>

      {/* Explorer tier */}
      <View
        style={[
          styles.tierCard,
          currentTier === "explorer" && styles.tierCardCurrent,
        ]}
      >
        <View style={styles.tierHeader}>
          <View style={styles.tierNameRow}>
            <Globe color={colors.accent} size={18} />
            <Text style={styles.tierName}>
              {t("subscription.explorer.name")}
            </Text>
            {currentTier === "explorer" && (
              <View style={styles.currentBadge}>
                <Text style={styles.currentBadgeText}>
                  {t("subscription.currentTier")}
                </Text>
              </View>
            )}
          </View>
          <View>
            <Text style={styles.tierPrice}>
              {explorerMonthly}
              <Text style={styles.tierPriceUnit}> CAD/mo</Text>
            </Text>
          </View>
        </View>
        {annual && (
          <Text style={styles.annualPrice}>
            {t("subscription.explorer.annualPrice", {
              price: `$${EXPLORER_ANNUAL.toFixed(2)}`,
              savings: annualSavingsPercent(EXPLORER_MONTHLY, EXPLORER_ANNUAL),
            })}
          </Text>
        )}
        <View style={styles.tierFeatures}>
          <View style={styles.featureRow}>
            <CheckCircle color={colors.success} size={14} />
            <Text style={styles.featureText}>
              {t("subscription.explorer.feature1")}
            </Text>
          </View>
          <View style={styles.featureRow}>
            <MessageCircle color={colors.success} size={14} />
            <Text style={styles.featureText}>
              {t("subscription.explorer.feature2")}
            </Text>
          </View>
          <View style={styles.featureRow}>
            <Users color={colors.success} size={14} />
            <Text style={styles.featureText}>
              {t("subscription.explorer.feature3")}
            </Text>
          </View>
        </View>
        {currentTier !== "explorer" && currentTier !== "voyager" && (
          <Pressable
            style={[
              styles.subscribeButton,
              actionLoading === "explorer" && styles.subscribeButtonDisabled,
            ]}
            onPress={() => handleSubscribe("explorer")}
            disabled={actionLoading !== null}
            accessibilityRole="button"
          >
            {actionLoading === "explorer" ? (
              <ActivityIndicator size="small" color="#fff" />
            ) : (
              <Text style={styles.subscribeButtonText}>
                {t("subscription.subscribe")}
              </Text>
            )}
          </Pressable>
        )}
      </View>

      {/* Voyager tier */}
      <View
        style={[
          styles.tierCard,
          currentTier === "voyager" && styles.tierCardCurrent,
        ]}
      >
        <View style={styles.tierHeader}>
          <View style={styles.tierNameRow}>
            <Zap color={colors.accent} size={18} />
            <Text style={styles.tierName}>
              {t("subscription.voyager.name")}
            </Text>
            {currentTier === "voyager" && (
              <View style={styles.currentBadge}>
                <Text style={styles.currentBadgeText}>
                  {t("subscription.currentTier")}
                </Text>
              </View>
            )}
          </View>
          <View>
            <Text style={styles.tierPrice}>
              {voyagerMonthly}
              <Text style={styles.tierPriceUnit}> CAD/mo</Text>
            </Text>
          </View>
        </View>
        {annual && (
          <Text style={styles.annualPrice}>
            {t("subscription.voyager.annualPrice", {
              price: `$${VOYAGER_ANNUAL.toFixed(2)}`,
              savings: annualSavingsPercent(VOYAGER_MONTHLY, VOYAGER_ANNUAL),
            })}
          </Text>
        )}
        <View style={styles.tierFeatures}>
          <View style={styles.featureRow}>
            <CheckCircle color={colors.success} size={14} />
            <Text style={styles.featureText}>
              {t("subscription.voyager.feature1")}
            </Text>
          </View>
          <View style={styles.featureRow}>
            <MessageCircle color={colors.success} size={14} />
            <Text style={styles.featureText}>
              {t("subscription.voyager.feature2")}
            </Text>
          </View>
          <View style={styles.featureRow}>
            <Zap color={colors.success} size={14} />
            <Text style={styles.featureText}>
              {t("subscription.voyager.feature3")}
            </Text>
          </View>
        </View>
        {currentTier !== "voyager" && (
          <Pressable
            style={[
              styles.subscribeButton,
              actionLoading === "voyager" && styles.subscribeButtonDisabled,
            ]}
            onPress={() => handleSubscribe("voyager")}
            disabled={actionLoading !== null}
            accessibilityRole="button"
          >
            {actionLoading === "voyager" ? (
              <ActivityIndicator size="small" color="#fff" />
            ) : (
              <Text style={styles.subscribeButtonText}>
                {currentTier === "explorer"
                  ? t("subscription.upgrade")
                  : t("subscription.subscribe")}
              </Text>
            )}
          </Pressable>
        )}
      </View>

      {/* Active subscription status */}
      {isActive && (currentTier === "explorer" || currentTier === "voyager") && (
        <View style={styles.statusSection}>
          <View style={styles.statusRow}>
            <Text style={styles.statusLabel}>{t("subscription.status")}</Text>
            <Text
              style={[
                styles.statusValue,
                subscription?.status === "past_due" && styles.statusWarning,
              ]}
            >
              {subscription?.status === "past_due"
                ? t("subscription.pastDue")
                : t("subscription.active")}
            </Text>
          </View>
          {subscription?.currentPeriodEnd && (
            <View style={styles.statusRow}>
              <Text style={styles.statusLabel}>
                {subscription.cancelAtPeriodEnd
                  ? t("subscription.endsOn")
                  : t("subscription.renewsOn")}
              </Text>
              <Text style={styles.statusValue}>
                {formatDate(subscription.currentPeriodEnd)}
              </Text>
            </View>
          )}
          {subscription?.cancelAtPeriodEnd && (
            <Text style={styles.cancelingText}>
              {t("subscription.cancelingNote")}
            </Text>
          )}

          <Pressable
            style={[
              styles.manageButton,
              actionLoading === "manage" && styles.subscribeButtonDisabled,
            ]}
            onPress={handleManage}
            disabled={actionLoading !== null}
            accessibilityRole="button"
          >
            {actionLoading === "manage" ? (
              <ActivityIndicator size="small" color={colors.accent} />
            ) : (
              <>
                <ExternalLink color={colors.accent} size={14} />
                <Text style={styles.manageButtonText}>
                  {t("subscription.manage")}
                </Text>
              </>
            )}
          </Pressable>

          {!subscription?.cancelAtPeriodEnd && (
            <Pressable
              style={[
                styles.cancelButton,
                actionLoading === "cancel" && styles.subscribeButtonDisabled,
              ]}
              onPress={handleCancel}
              disabled={actionLoading !== null}
              accessibilityRole="button"
            >
              {actionLoading === "cancel" ? (
                <ActivityIndicator size="small" color={colors.textTertiary} />
              ) : (
                <Text style={styles.cancelButtonText}>
                  {t("subscription.cancel")}
                </Text>
              )}
            </Pressable>
          )}
        </View>
      )}
    </View>
  );
}
