import { useState } from "react";
import { View, Text, StyleSheet, Pressable } from "react-native";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronUp, Star, Globe, Zap } from "lucide-react-native";
import { useTheme } from "@/lib/theme";

/**
 * Expandable "Which plan is right for me?" comparison card.
 * Helps users choose between Trip Pro (one-time) and subscriptions.
 */
export function PlanComparisonCard() {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const [expanded, setExpanded] = useState(false);

  const styles = StyleSheet.create({
    container: {
      backgroundColor: colors.surface,
      borderRadius: 10,
      borderWidth: 1,
      borderColor: colors.border,
      marginBottom: 12,
      overflow: "hidden",
    },
    header: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      paddingHorizontal: 14,
      paddingVertical: 12,
    },
    headerText: {
      fontSize: 14,
      fontWeight: "600",
      color: colors.accent,
    },
    content: {
      paddingHorizontal: 14,
      paddingBottom: 14,
      gap: 12,
    },
    planRow: {
      flexDirection: "row",
      gap: 8,
    },
    planIcon: {
      marginTop: 2,
    },
    planContent: {
      flex: 1,
    },
    planName: {
      fontSize: 14,
      fontWeight: "700",
      color: colors.textPrimary,
      marginBottom: 2,
    },
    planBestFor: {
      fontSize: 13,
      color: colors.textSecondary,
      lineHeight: 18,
    },
    planPrice: {
      fontSize: 13,
      fontWeight: "600",
      color: colors.accent,
      marginTop: 2,
    },
    divider: {
      height: 1,
      backgroundColor: colors.border,
    },
  });

  return (
    <View style={styles.container} testID="plan-comparison-card">
      <Pressable
        style={styles.header}
        onPress={() => setExpanded((v) => !v)}
        accessibilityRole="button"
        accessibilityLabel={t("subscription.comparison.title")}
      >
        <Text style={styles.headerText}>
          {t("subscription.comparison.title")}
        </Text>
        {expanded ? (
          <ChevronUp size={18} color={colors.accent} />
        ) : (
          <ChevronDown size={18} color={colors.accent} />
        )}
      </Pressable>

      {expanded && (
        <View style={styles.content}>
          {/* Trip Pro */}
          <View style={styles.planRow}>
            <View style={styles.planIcon}>
              <Star size={16} color={colors.accent} />
            </View>
            <View style={styles.planContent}>
              <Text style={styles.planName}>
                {t("subscription.comparison.tripPro.name")}
              </Text>
              <Text style={styles.planBestFor}>
                {t("subscription.comparison.tripPro.bestFor")}
              </Text>
              <Text style={styles.planPrice}>
                {t("subscription.comparison.tripPro.price")}
              </Text>
            </View>
          </View>

          <View style={styles.divider} />

          {/* Explorer */}
          <View style={styles.planRow}>
            <View style={styles.planIcon}>
              <Globe size={16} color={colors.accent} />
            </View>
            <View style={styles.planContent}>
              <Text style={styles.planName}>
                {t("subscription.comparison.explorer.name")}
              </Text>
              <Text style={styles.planBestFor}>
                {t("subscription.comparison.explorer.bestFor")}
              </Text>
              <Text style={styles.planPrice}>
                {t("subscription.comparison.explorer.price")}
              </Text>
            </View>
          </View>

          <View style={styles.divider} />

          {/* Voyager */}
          <View style={styles.planRow}>
            <View style={styles.planIcon}>
              <Zap size={16} color={colors.accent} />
            </View>
            <View style={styles.planContent}>
              <Text style={styles.planName}>
                {t("subscription.comparison.voyager.name")}
              </Text>
              <Text style={styles.planBestFor}>
                {t("subscription.comparison.voyager.bestFor")}
              </Text>
              <Text style={styles.planPrice}>
                {t("subscription.comparison.voyager.price")}
              </Text>
            </View>
          </View>
        </View>
      )}
    </View>
  );
}
