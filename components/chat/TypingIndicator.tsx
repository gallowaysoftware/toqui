import { useEffect, useRef } from "react";
import { View, Text, StyleSheet, Animated } from "react-native";
import { useTheme } from "@/lib/theme";

interface TypingIndicatorProps {
  toolName?: string | null;
}

function PulsingDot({ color, delay }: { color: string; delay: number }) {
  const opacity = useRef(new Animated.Value(0.3)).current;

  useEffect(() => {
    const pulse = Animated.loop(
      Animated.sequence([
        Animated.timing(opacity, {
          toValue: 1,
          duration: 400,
          delay,
          useNativeDriver: true,
        }),
        Animated.timing(opacity, {
          toValue: 0.3,
          duration: 400,
          useNativeDriver: true,
        }),
      ]),
    );
    pulse.start();
    return () => pulse.stop();
  }, [opacity, delay]);

  return (
    <Animated.View
      style={{
        width: 8,
        height: 8,
        borderRadius: 4,
        backgroundColor: color,
        opacity,
      }}
    />
  );
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
          <PulsingDot color={colors.border} delay={0} />
          <PulsingDot color={colors.border} delay={150} />
          <PulsingDot color={colors.border} delay={300} />
        </View>
      )}
    </View>
  );
}
