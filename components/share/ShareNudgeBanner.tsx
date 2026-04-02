import { View, Text, StyleSheet, Pressable } from "react-native";
import { Share2, X } from "lucide-react-native";
import { useTranslation } from "react-i18next";
import { useTheme } from "@/lib/theme";

interface ShareNudgeBannerProps {
  onShare: () => void;
  onDismiss: () => void;
}

/**
 * ShareNudgeBanner -- small dismissible banner shown on the trip detail page
 * when the trip has itinerary items but has never been shared. Nudges the
 * user to share with friends.
 */
export function ShareNudgeBanner({ onShare, onDismiss }: ShareNudgeBannerProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();

  const styles = StyleSheet.create({
    container: {
      backgroundColor: colors.accentSoft,
      borderLeftWidth: 3,
      borderLeftColor: colors.accent,
      borderRadius: 10,
      padding: 12,
      marginBottom: 16,
      flexDirection: "row",
      alignItems: "center",
      gap: 10,
    },
    icon: {
      opacity: 0.8,
    },
    text: {
      flex: 1,
      fontSize: 14,
      color: colors.accent,
      fontWeight: "600",
    },
    dismiss: {
      padding: 4,
    },
  });

  return (
    <View style={styles.container} testID="share-nudge-banner">
      <Share2 color={colors.accent} size={18} style={styles.icon as object} />
      <Pressable style={{ flex: 1 }} onPress={onShare} accessibilityRole="button">
        <Text style={styles.text}>{t("share.nudgeBanner")}</Text>
      </Pressable>
      <Pressable
        style={styles.dismiss}
        onPress={onDismiss}
        accessibilityRole="button"
        accessibilityLabel={t("common.cancel")}
      >
        <X color={colors.textTertiary} size={16} />
      </Pressable>
    </View>
  );
}
