import { View, Text, StyleSheet, Pressable, ActivityIndicator } from "react-native";
import { Share2 } from "lucide-react-native";
import { useTranslation } from "react-i18next";
import { useTheme } from "@/lib/theme";

interface SharePromptCardProps {
  onShare: () => void;
  isSharing?: boolean;
}

/**
 * SharePromptCard -- inline card shown in the chat after the AI creates
 * itinerary items. Prompts the user to share the trip with companions.
 * Shown once per trip (caller manages persistence via AsyncStorage).
 */
export function SharePromptCard({ onShare, isSharing }: SharePromptCardProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();

  const styles = StyleSheet.create({
    container: {
      backgroundColor: colors.accentSoft,
      borderRadius: 12,
      padding: 14,
      marginVertical: 8,
      flexDirection: "row",
      alignItems: "center",
      gap: 12,
      borderWidth: 1,
      borderColor: colors.accent + "30",
    },
    content: {
      flex: 1,
    },
    title: {
      fontSize: 14,
      fontWeight: "600",
      color: colors.textPrimary,
      marginBottom: 2,
    },
    subtitle: {
      fontSize: 13,
      color: colors.textSecondary,
    },
    button: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      paddingVertical: 8,
      paddingHorizontal: 14,
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
    },
    buttonText: {
      color: "#fff",
      fontSize: 13,
      fontWeight: "600",
    },
  });

  return (
    <View style={styles.container} testID="share-prompt-card">
      <View style={styles.content}>
        <Text style={styles.title}>{t("share.promptTitle")}</Text>
        <Text style={styles.subtitle}>{t("share.promptSubtitle")}</Text>
      </View>
      <Pressable
        style={({ pressed }) => [styles.button, { opacity: pressed ? 0.85 : 1 }]}
        onPress={onShare}
        disabled={isSharing}
        accessibilityRole="button"
        accessibilityLabel={t("referral.share")}
      >
        {isSharing ? (
          <ActivityIndicator size="small" color="#fff" />
        ) : (
          <Share2 color="#fff" size={16} />
        )}
        <Text style={styles.buttonText}>{t("referral.share")}</Text>
      </Pressable>
    </View>
  );
}
