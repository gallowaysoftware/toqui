import { View, Text, StyleSheet, Pressable, Share, ActivityIndicator } from "react-native";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import * as Clipboard from "expo-clipboard";
import { Gift, Copy, Share2, Users } from "lucide-react-native";
import { useReferral } from "@/lib/hooks/useReferral";
import { useTheme } from "@/lib/theme";

export default function ReferralCard() {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const { code, successfulReferrals, rewardsEarned, isLoading, error } = useReferral();
  const [copied, setCopied] = useState(false);
  const copiedTimer = useRef<ReturnType<typeof setTimeout>>(undefined);

  const shareLink = code
    ? `https://toqui.travel?ref=${code}`
    : null;

  const handleCopy = async () => {
    if (!shareLink) return;
    await Clipboard.setStringAsync(shareLink);
    setCopied(true);
    clearTimeout(copiedTimer.current);
    copiedTimer.current = setTimeout(() => setCopied(false), 2000);
  };

  const handleShare = async () => {
    if (!shareLink || !code) return;
    await Share.share({
      message: t("referral.shareMessage", { code, link: shareLink }),
    });
  };

  const styles = StyleSheet.create({
    loadingContainer: {
      padding: 20,
      alignItems: "center",
    },
    description: {
      fontSize: 14,
      color: colors.textSecondary,
      lineHeight: 20,
      marginBottom: 14,
    },
    codeContainer: {
      backgroundColor: colors.surfaceSecondary,
      borderRadius: 8,
      padding: 12,
      alignItems: "center",
      marginBottom: 14,
    },
    codeLabel: {
      fontSize: 12,
      color: colors.textTertiary,
      marginBottom: 4,
    },
    codeValue: {
      fontSize: 20,
      fontWeight: "700",
      color: colors.accent,
      letterSpacing: 2,
    },
    buttonRow: {
      flexDirection: "row",
      gap: 10,
      marginBottom: 14,
    },
    actionButton: {
      flex: 1,
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "center",
      gap: 6,
      borderWidth: 1,
      borderColor: colors.accent,
      borderRadius: 8,
      padding: 10,
    },
    actionButtonText: {
      color: colors.accent,
      fontWeight: "600",
      fontSize: 14,
    },
    statsRow: {
      flexDirection: "row",
      justifyContent: "space-around",
    },
    stat: {
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
    },
    statText: {
      fontSize: 13,
      color: colors.textSecondary,
    },
  });

  if (isLoading) {
    return (
      <View style={styles.loadingContainer}>
        <ActivityIndicator color={colors.accent} size="small" />
      </View>
    );
  }

  if (error || !code) {
    return null;
  }

  return (
    <View>
      <Text style={styles.description}>{t("referral.description")}</Text>

      <View style={styles.codeContainer}>
        <Text style={styles.codeLabel}>{t("referral.yourCode")}</Text>
        <Text style={styles.codeValue}>{code}</Text>
      </View>

      <View style={styles.buttonRow}>
        <Pressable style={styles.actionButton} onPress={handleCopy}>
          <Copy color={colors.accent} size={16} />
          <Text style={styles.actionButtonText}>
            {copied ? t("referral.copied") : t("referral.copyLink")}
          </Text>
        </Pressable>
        <Pressable style={styles.actionButton} onPress={handleShare}>
          <Share2 color={colors.accent} size={16} />
          <Text style={styles.actionButtonText}>{t("referral.share")}</Text>
        </Pressable>
      </View>

      <View style={styles.statsRow}>
        <View style={styles.stat}>
          <Users color={colors.textSecondary} size={16} />
          <Text style={styles.statText}>
            {t("referral.friendsInvited", { count: successfulReferrals })}
          </Text>
        </View>
        <View style={styles.stat}>
          <Gift color={colors.textSecondary} size={16} />
          <Text style={styles.statText}>
            {t("referral.rewardsEarned", { count: rewardsEarned })}
          </Text>
        </View>
      </View>
    </View>
  );
}
