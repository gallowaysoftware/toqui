import { View, Text, StyleSheet, Pressable, ScrollView, TextInput, Alert, ActivityIndicator, Linking } from "react-native";
import { useMemo, useState } from "react";
import { useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { useMutation } from "@tanstack/react-query";
import { createClient } from "@connectrpc/connect";
import { LogOut, Download, Trash2, User, FileText, Shield, Sun, Moon, Monitor, CreditCard, ExternalLink, Gift, MessageSquare, BarChart2, Crown } from "lucide-react-native";
import FeedbackModal from "@/components/feedback/FeedbackModal";
import ReferralCard from "@/components/referral/ReferralCard";
import { SubscriptionCard } from "@/components/subscription/SubscriptionCard";
import { useAuth } from "@/lib/auth";
import { useTransport } from "@/lib/transport";
import { useTheme } from "@/lib/theme";
import { useUsage, formatTimeUntilReset } from "@/lib/hooks/useUsage";
import { useSubscription } from "@/lib/hooks/useSubscription";
import { AuthService } from "@gen/toqui/v1/auth_pb";

const COLOR_WARNING = "#f59e0b";

export default function SettingsScreen() {
  const { t } = useTranslation();
  const { user, logout, accessToken } = useAuth();
  const transport = useTransport();
  const { colors, mode, setMode } = useTheme();
  const router = useRouter();
  const client = useMemo(() => createClient(AuthService, transport), [transport]);
  const [deleteConfirm, setDeleteConfirm] = useState("");
  const [feedbackVisible, setFeedbackVisible] = useState(false);
  const isPro = user?.tier === "pro";
  const { used, limit, resetsAt } = useUsage();
  const { subscription } = useSubscription();
  const subscriptionTier = subscription?.tier ?? "free";
  const isSubscriber = subscriptionTier === "explorer" || subscriptionTier === "voyager";

  const exportData = useMutation({
    mutationFn: async () => {
      await client.exportData({});
    },
  });

  const deleteAccount = useMutation({
    mutationFn: async () => {
      await client.deleteAccount({});
      await logout();
    },
  });

  const handleDelete = () => {
    if (deleteConfirm !== "DELETE") return;
    Alert.alert(
      t("settings.deleteAccount"),
      t("settings.deleteWarning"),
      [
        { text: t("common.cancel"), style: "cancel" },
        {
          text: t("settings.deleteConfirm"),
          style: "destructive",
          onPress: () => deleteAccount.mutate(),
        },
      ],
    );
  };

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    content: { padding: 16, paddingBottom: 40 },
    center: { flex: 1, justifyContent: "center", alignItems: "center" },
    emptyText: { fontSize: 16, color: colors.textSecondary },
    section: {
      backgroundColor: colors.surface,
      borderRadius: 12,
      padding: 16,
      marginBottom: 16,
      borderWidth: 1,
      borderColor: colors.border,
    },
    dangerSection: { borderColor: colors.error },
    sectionHeader: { flexDirection: "row", alignItems: "center", gap: 8, marginBottom: 12 },
    sectionTitle: { fontSize: 16, fontWeight: "600", color: colors.textSecondary },
    accountInfo: { marginBottom: 12 },
    userName: { fontSize: 16, fontWeight: "600", color: colors.textSecondary },
    userEmail: { fontSize: 14, color: colors.textSecondary, marginTop: 2 },
    actionRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: 10,
      paddingVertical: 10,
      borderTopWidth: 1,
      borderTopColor: colors.surfaceTertiary,
    },
    actionText: { fontSize: 15, color: colors.accent, fontWeight: "500" },
    linkText: { fontSize: 15, color: colors.textSecondary },
    outlineButton: {
      borderWidth: 1,
      borderColor: colors.accent,
      borderRadius: 8,
      padding: 12,
      alignItems: "center",
    },
    disabledButton: { opacity: 0.5 },
    outlineButtonText: { color: colors.accent, fontWeight: "600" },
    dangerWarning: { fontSize: 14, color: colors.textSecondary, marginBottom: 12, lineHeight: 20 },
    dangerLabel: { fontSize: 13, color: colors.textTertiary, marginBottom: 6 },
    dangerInput: {
      borderWidth: 1,
      borderColor: colors.error,
      borderRadius: 8,
      padding: 10,
      fontSize: 14,
      marginBottom: 12,
      color: colors.textPrimary,
    },
    deleteButton: {
      borderWidth: 1,
      borderColor: colors.error,
      borderRadius: 8,
      padding: 12,
      alignItems: "center",
    },
    deleteText: { color: colors.error, fontWeight: "600" },
    themeRow: { flexDirection: "row", gap: 10 },
    themeOption: {
      flex: 1,
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "center",
      gap: 6,
      padding: 10,
      borderRadius: 8,
      borderWidth: 1,
    },
    themeLabel: { fontSize: 13, fontWeight: "500" },
    billingPlanRow: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      marginBottom: 10,
    },
    billingLabel: { fontSize: 14, color: colors.textSecondary },
    planBadge: {
      backgroundColor: colors.surfaceTertiary,
      paddingHorizontal: 12,
      paddingVertical: 4,
      borderRadius: 12,
    },
    planBadgePro: { backgroundColor: colors.accent },
    planBadgeText: { fontSize: 13, fontWeight: "600", color: colors.textSecondary },
    planBadgeTextPro: { color: "#fff" },
    billingDescription: { fontSize: 14, color: colors.textTertiary, lineHeight: 20 },
    learnMoreRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
      marginTop: 10,
      paddingVertical: 4,
    },
    learnMoreText: { fontSize: 14, fontWeight: "500", color: colors.accent },
    usageRow: { flexDirection: "row", justifyContent: "space-between", marginBottom: 8 },
    usageLabel: { fontSize: 14, color: colors.textSecondary },
    usageCount: { fontSize: 14, fontWeight: "600", color: colors.textSecondary },
    progressTrack: { height: 4, borderRadius: 2, backgroundColor: colors.surfaceTertiary, marginBottom: 6 },
    progressFill: { height: 4, borderRadius: 2 },
    usageResetText: { fontSize: 12, color: colors.textTertiary },
  });

  if (!accessToken) {
    return (
      <View style={styles.center}>
        <Text style={styles.emptyText}>Sign in to view settings</Text>
      </View>
    );
  }

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {/* Account Info */}
      <View style={styles.section}>
        <View style={styles.sectionHeader}>
          <User color={colors.textSecondary} size={20} />
          <Text style={styles.sectionTitle}>{t("settings.account")}</Text>
        </View>
        {user && (
          <View style={styles.accountInfo}>
            <Text style={styles.userName}>{user.name}</Text>
            <Text style={styles.userEmail}>{user.email}</Text>
          </View>
        )}
        <Pressable style={styles.actionRow} onPress={logout}>
          <LogOut color={colors.accent} size={18} />
          <Text style={styles.actionText}>{t("common.signOut")}</Text>
        </Pressable>
      </View>

      {/* Plan & Billing */}
      <View style={styles.section}>
        <View style={styles.sectionHeader}>
          <CreditCard color={colors.textSecondary} size={20} />
          <Text style={styles.sectionTitle}>{t("settings.billing.title")}</Text>
        </View>
        <View style={styles.billingPlanRow}>
          <Text style={styles.billingLabel}>{t("settings.billing.currentPlan")}</Text>
          <View style={[styles.planBadge, (isPro || isSubscriber) && styles.planBadgePro]}>
            <Text style={[styles.planBadgeText, (isPro || isSubscriber) && styles.planBadgeTextPro]}>
              {isSubscriber
                ? t(`subscription.${subscriptionTier}.name`)
                : isPro
                  ? t("settings.billing.pro")
                  : t("settings.billing.free")}
            </Text>
          </View>
        </View>
        {isSubscriber && subscription?.currentPeriodEnd ? (
          <>
            <Text style={styles.billingDescription}>
              {subscription.billingPeriod === "annual"
                ? t("subscription.annual")
                : t("subscription.monthly")}{" "}
              {t("settings.billing.plan").toLowerCase()}
              {" \u2014 "}
              {subscription.cancelAtPeriodEnd
                ? t("subscription.endsOn").toLowerCase()
                : t("subscription.renewsOn").toLowerCase()}{" "}
              {subscription.currentPeriodEnd.toLocaleDateString(undefined, {
                year: "numeric",
                month: "short",
                day: "numeric",
              })}
            </Text>
          </>
        ) : isPro ? (
          <Text style={styles.billingDescription}>{t("settings.billing.proDescription")}</Text>
        ) : (
          <Text style={styles.billingDescription}>{t("settings.billing.freeDescription")}</Text>
        )}
      </View>

      {/* Subscription Plans */}
      <View style={{ marginBottom: 16 }}>
        <SubscriptionCard />
      </View>

      {/* Usage */}
      {limit > 0 && (
        <View style={styles.section}>
          <View style={styles.sectionHeader}>
            <BarChart2 color={colors.textSecondary} size={20} />
            <Text style={styles.sectionTitle}>Usage</Text>
          </View>
          <View style={styles.usageRow}>
            <Text style={styles.usageLabel}>Messages today</Text>
            <Text style={styles.usageCount}>{used} / {limit}</Text>
          </View>
          <View style={styles.progressTrack}>
            <View
              style={[
                styles.progressFill,
                {
                  width: `${Math.min(100, (used / limit) * 100)}%` as `${number}%`,
                  backgroundColor:
                    used / limit >= 0.9
                      ? colors.error
                      : used / limit >= 0.75
                        ? COLOR_WARNING
                        : colors.success,
                },
              ]}
            />
          </View>
          {resetsAt && (
            <Text style={styles.usageResetText}>Resets {formatTimeUntilReset(resetsAt)}</Text>
          )}
        </View>
      )}

      {/* Refer a Friend */}
      <View style={styles.section}>
        <View style={styles.sectionHeader}>
          <Gift color={colors.accent} size={20} />
          <Text style={styles.sectionTitle}>{t("referral.title")}</Text>
        </View>
        <ReferralCard />
      </View>

      {/* Data */}
      <View style={styles.section}>
        <View style={styles.sectionHeader}>
          <Download color={colors.textSecondary} size={20} />
          <Text style={styles.sectionTitle}>{t("settings.exportData")}</Text>
        </View>
        <Pressable
          style={[styles.outlineButton, exportData.isPending && styles.disabledButton]}
          onPress={() => exportData.mutate()}
          disabled={exportData.isPending}
        >
          {exportData.isPending ? (
            <ActivityIndicator color={colors.accent} size="small" />
          ) : (
            <Text style={styles.outlineButtonText}>
              {exportData.isSuccess ? t("settings.exported") : t("settings.exportData")}
            </Text>
          )}
        </Pressable>
      </View>

      {/* Appearance */}
      <View style={[styles.section, { backgroundColor: colors.surface, borderColor: colors.border }]}>
        <View style={styles.sectionHeader}>
          <Sun color={colors.textPrimary} size={20} />
          <Text style={[styles.sectionTitle, { color: colors.textPrimary }]}>Appearance</Text>
        </View>
        <View style={styles.themeRow}>
          {([
            { key: "light" as const, label: "Light", Icon: Sun },
            { key: "dark" as const, label: "Dark", Icon: Moon },
            { key: "system" as const, label: "System", Icon: Monitor },
          ]).map(({ key, label, Icon }) => (
            <Pressable
              key={key}
              style={[
                styles.themeOption,
                { borderColor: mode === key ? colors.accent : colors.border },
                mode === key && { backgroundColor: colors.accentSoft },
              ]}
              onPress={() => setMode(key)}
            >
              <Icon color={mode === key ? colors.accent : colors.textSecondary} size={18} />
              <Text style={[styles.themeLabel, { color: mode === key ? colors.accent : colors.textSecondary }]}>{label}</Text>
            </Pressable>
          ))}
        </View>
      </View>

      {/* Legal */}
      <View style={styles.section}>
        <View style={styles.sectionHeader}>
          <FileText color={colors.textSecondary} size={20} />
          <Text style={styles.sectionTitle}>Legal</Text>
        </View>
        <Pressable style={styles.actionRow} onPress={() => router.push("/privacy" as never)}>
          <Shield color={colors.textSecondary} size={16} />
          <Text style={styles.linkText}>Privacy Policy</Text>
        </Pressable>
        <Pressable style={styles.actionRow} onPress={() => router.push("/terms" as never)}>
          <FileText color={colors.textSecondary} size={16} />
          <Text style={styles.linkText}>Terms of Service</Text>
        </Pressable>
        <Pressable style={styles.actionRow} onPress={() => Linking.openURL("https://toqui.travel/affiliate-disclosure")}>
          <ExternalLink color={colors.textSecondary} size={16} />
          <Text style={styles.linkText}>Affiliate Disclosure</Text>
        </Pressable>
      </View>

      {/* Help & Feedback */}
      <View style={styles.section}>
        <View style={styles.sectionHeader}>
          <MessageSquare color={colors.textSecondary} size={20} />
          <Text style={styles.sectionTitle}>{t("feedback.title")}</Text>
        </View>
        <Pressable style={styles.outlineButton} onPress={() => setFeedbackVisible(true)}>
          <Text style={styles.outlineButtonText}>{t("feedback.title")}</Text>
        </Pressable>
      </View>

      <FeedbackModal
        visible={feedbackVisible}
        onClose={() => setFeedbackVisible(false)}
      />

      {/* Danger Zone */}
      <View style={[styles.section, styles.dangerSection]}>
        <View style={styles.sectionHeader}>
          <Trash2 color={colors.error} size={20} />
          <Text style={[styles.sectionTitle, { color: colors.error }]}>{t("settings.deleteAccount")}</Text>
        </View>
        <Text style={styles.dangerWarning}>{t("settings.deleteWarning")}</Text>
        <Text style={styles.dangerLabel}>{t("settings.typeDelete")}</Text>
        <TextInput
          style={styles.dangerInput}
          value={deleteConfirm}
          onChangeText={setDeleteConfirm}
          placeholder="DELETE"
          placeholderTextColor={colors.borderStrong}
          autoCapitalize="characters"
        />
        <Pressable
          style={[styles.deleteButton, (deleteConfirm !== "DELETE" || deleteAccount.isPending) && styles.disabledButton]}
          onPress={handleDelete}
          disabled={deleteConfirm !== "DELETE" || deleteAccount.isPending}
        >
          <Text style={styles.deleteText}>
            {deleteAccount.isPending ? t("settings.deleting") : t("settings.deleteConfirm")}
          </Text>
        </Pressable>
      </View>
    </ScrollView>
  );
}
