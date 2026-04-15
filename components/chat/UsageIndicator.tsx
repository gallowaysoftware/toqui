import { View, Text, StyleSheet, Pressable } from "react-native";
import { useTranslation } from "react-i18next";
import { Info } from "lucide-react-native";
import { useTheme } from "@/lib/theme";

interface UsageIndicatorProps {
  used: number;
  limit: number;
  tier: string;
  /** Navigate to upgrade screen */
  onUpgrade: () => void;
}

const COLOR_AMBER = "#d97706";

/**
 * Proactive usage counter shown at the top of chat.
 * Always visible for free users so limits are never a surprise.
 */
export function UsageIndicator({ used, limit, tier, onUpgrade }: UsageIndicatorProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();

  // Don't show for paid tiers with no limit
  if (limit <= 0) return null;
  // Don't show for subscribers (explorer/voyager get unlimited)
  if (tier === "explorer" || tier === "voyager") return null;

  const ratio = limit > 0 ? used / limit : 0;
  const isNearLimit = ratio >= 0.8 && ratio < 1;
  const isAtLimit = ratio >= 1;

  const indicatorColor = isAtLimit
    ? colors.error
    : isNearLimit
      ? COLOR_AMBER
      : colors.textTertiary;

  const bgColor = isAtLimit
    ? colors.errorBg
    : isNearLimit
      ? colors.warningBg
      : colors.infoBg;

  const borderColor = isAtLimit
    ? colors.error
    : isNearLimit
      ? colors.warningBorder
      : colors.infoBorder;

  const styles = StyleSheet.create({
    container: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      paddingHorizontal: 16,
      paddingVertical: 8,
      backgroundColor: bgColor,
      borderBottomWidth: 1,
      borderBottomColor: borderColor,
    },
    left: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
      flex: 1,
    },
    usageText: {
      fontSize: 13,
      color: indicatorColor,
      fontWeight: "500",
    },
    upgradeLink: {
      fontSize: 13,
      fontWeight: "600",
      color: colors.accent,
    },
  });

  const label = isAtLimit
    ? t("chat.usageIndicator.atLimit")
    : isNearLimit
      ? t("chat.usageIndicator.nearLimit", { used, limit })
      : t("chat.usageIndicator.normal", { used, limit });

  return (
    <View style={styles.container} testID="usage-indicator">
      <View style={styles.left}>
        <Info size={14} color={indicatorColor} />
        <Text style={styles.usageText}>{label}</Text>
      </View>
      {(isNearLimit || isAtLimit) && (
        <Pressable onPress={onUpgrade} accessibilityRole="button">
          <Text style={styles.upgradeLink}>
            {isAtLimit
              ? t("chat.usageIndicator.upgradeCta")
              : t("chat.upgrade")}
          </Text>
        </Pressable>
      )}
    </View>
  );
}
