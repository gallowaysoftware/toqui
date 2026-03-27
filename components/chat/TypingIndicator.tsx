import { View, Text, StyleSheet } from "react-native";

interface TypingIndicatorProps {
  toolName?: string | null;
}

export function TypingIndicator({ toolName }: TypingIndicatorProps) {
  return (
    <View style={styles.container}>
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

const styles = StyleSheet.create({
  container: {
    alignSelf: "flex-start",
    backgroundColor: "#fff",
    borderRadius: 16,
    borderBottomLeftRadius: 4,
    borderWidth: 1,
    borderColor: "#e0e0e0",
    paddingVertical: 10,
    paddingHorizontal: 14,
    marginBottom: 8,
  },
  text: {
    fontSize: 13,
    color: "#888",
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
    backgroundColor: "#ccc",
  },
  dot1: { opacity: 0.4 },
  dot2: { opacity: 0.6 },
  dot3: { opacity: 0.8 },
});
