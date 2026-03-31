import { View, Text, StyleSheet } from "react-native";
import { useTheme } from "@/lib/theme";

interface TypingIndicatorProps {
  toolName?: string | null;
}

export function TypingIndicator({ toolName }: TypingIndicatorProps) {
  const { colors } = useTheme();
  const label = toolName ? `Using ${toolName}` : "AI is typing";

  const styles = StyleSheet.create({
    container: {
      alignSelf: "flex-start",
      backgroundColor: colors.assistantBubble,
      borderRadius: 16,
      borderBottomLeftRadius: 4,
      borderWidth: 1,
      borderColor: colors.assistantBubbleBorder,
      paddingVertical: 10,
      paddingHorizontal: 14,
      marginBottom: 8,
    },
    text: {
      fontSize: 13,
      color: colors.textTertiary,
      fontStyle: "italic",
    },
    dots: {
      flexDirection: "row",
      gap: 4,
      alignItems: "center",
    },
    dot: {
      width: 8,
      height: 8,
      borderRadius: 4,
      backgroundColor: colors.border,
    },
    dot1: { opacity: 0.4 },
    dot2: { opacity: 0.6 },
    dot3: { opacity: 0.8 },
  });

  return (
    <View
      style={styles.container}
      accessibilityLiveRegion="polite"
      accessibilityLabel={label}
    >
      {toolName ? (
        <Text style={styles.text}>Using {toolName}...</Text>
      ) : (
        <View style={styles.dots}>
          <View style={[styles.dot, styles.dot1]} />
          <View style={[styles.dot, styles.dot2]} />
          <View style={[styles.dot, styles.dot3]} />
        </View>
      )}
    </View>
  );
}
