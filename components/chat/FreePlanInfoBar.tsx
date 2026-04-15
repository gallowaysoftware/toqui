import { useState } from "react";
import { View, Text, StyleSheet, Pressable } from "react-native";
import { useTranslation } from "react-i18next";
import { Info, X } from "lucide-react-native";
import { useTheme } from "@/lib/theme";

interface FreePlanInfoBarProps {
  /** Navigate to upgrade screen */
  onUpgrade: () => void;
}

/**
 * Subtle info bar shown on the first visit to a new trip chat.
 * Proactively communicates free plan limits (expert persona, messages)
 * so users aren't surprised later.
 */
export function FreePlanInfoBar({ onUpgrade }: FreePlanInfoBarProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const [dismissed, setDismissed] = useState(false);

  if (dismissed) return null;

  const styles = StyleSheet.create({
    container: {
      flexDirection: "row",
      alignItems: "center",
      paddingHorizontal: 14,
      paddingVertical: 10,
      backgroundColor: colors.infoBg,
      borderBottomWidth: 1,
      borderBottomColor: colors.infoBorder,
      gap: 8,
    },
    textContainer: {
      flex: 1,
    },
    text: {
      fontSize: 13,
      color: colors.info,
      lineHeight: 18,
    },
    upgradeLink: {
      fontSize: 13,
      fontWeight: "600",
      color: colors.accent,
    },
    dismissButton: {
      padding: 4,
    },
  });

  return (
    <View style={styles.container} testID="free-plan-info-bar">
      <Info size={14} color={colors.info} />
      <View style={styles.textContainer}>
        <Text style={styles.text}>
          {t("chat.freePlanInfo.message")}{" "}
          <Text style={styles.upgradeLink} onPress={onUpgrade}>
            {t("chat.freePlanInfo.upgradeLink")}
          </Text>
        </Text>
      </View>
      <Pressable
        style={styles.dismissButton}
        onPress={() => setDismissed(true)}
        accessibilityRole="button"
        accessibilityLabel={t("common.dismiss")}
      >
        <X size={14} color={colors.textTertiary} />
      </Pressable>
    </View>
  );
}
