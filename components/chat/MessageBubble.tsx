import { View, Text, StyleSheet } from "react-native";
import Markdown from "react-native-markdown-display";
import type { ChatMessage } from "@/lib/hooks/useChat";

interface MessageBubbleProps {
  message: ChatMessage;
}

export function MessageBubble({ message }: MessageBubbleProps) {
  const isUser = message.role === "user";
  const isSystem = message.role === "system";

  if (isSystem) {
    return (
      <View style={styles.systemBubble}>
        <Text style={styles.systemText}>{message.content}</Text>
      </View>
    );
  }

  return (
    <View style={[styles.bubble, isUser ? styles.userBubble : styles.assistantBubble, message.isError && styles.errorBubble]}>
      {message.personaName && !isUser ? (
        <View style={styles.personaHeader}>
          <View style={[styles.personaDot, { backgroundColor: message.personaAccentColor ?? "#e8654a" }]} />
          <Text style={styles.personaName}>{message.personaName}</Text>
        </View>
      ) : null}
      {isUser ? (
        <Text style={styles.userText}>{message.content}</Text>
      ) : (
        <Markdown style={markdownStyles}>{message.content}</Markdown>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  bubble: {
    maxWidth: "85%",
    padding: 12,
    borderRadius: 16,
    marginBottom: 8,
  },
  userBubble: {
    backgroundColor: "#e8654a",
    alignSelf: "flex-end",
    borderBottomRightRadius: 4,
  },
  assistantBubble: {
    backgroundColor: "#fff",
    alignSelf: "flex-start",
    borderBottomLeftRadius: 4,
    borderWidth: 1,
    borderColor: "#e0e0e0",
  },
  errorBubble: {
    backgroundColor: "#fee2e2",
    borderColor: "#fca5a5",
  },
  systemBubble: {
    alignSelf: "center",
    backgroundColor: "#f0f0f0",
    borderRadius: 12,
    paddingVertical: 6,
    paddingHorizontal: 12,
    marginBottom: 8,
    maxWidth: "90%",
  },
  systemText: {
    fontSize: 13,
    color: "#888",
    fontStyle: "italic",
    textAlign: "center",
  },
  userText: {
    fontSize: 15,
    color: "#fff",
    lineHeight: 22,
  },
  personaHeader: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    marginBottom: 4,
  },
  personaDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
  },
  personaName: {
    fontSize: 12,
    fontWeight: "600",
    color: "#666",
  },
});

const markdownStyles = StyleSheet.create({
  body: { fontSize: 15, color: "#333", lineHeight: 22 },
  strong: { fontWeight: "700" },
  em: { fontStyle: "italic" },
  link: { color: "#e8654a" },
  heading1: { fontSize: 20, fontWeight: "700", marginBottom: 8, color: "#333" },
  heading2: { fontSize: 18, fontWeight: "700", marginBottom: 6, color: "#333" },
  heading3: { fontSize: 16, fontWeight: "600", marginBottom: 4, color: "#333" },
  bullet_list: { marginBottom: 8 },
  ordered_list: { marginBottom: 8 },
  list_item: { marginBottom: 4 },
  code_inline: { backgroundColor: "#f0f0f0", paddingHorizontal: 4, borderRadius: 3, fontFamily: "monospace" },
  fence: { backgroundColor: "#f0f0f0", padding: 8, borderRadius: 6, fontFamily: "monospace" },
  blockquote: { borderLeftWidth: 3, borderLeftColor: "#e8654a", paddingLeft: 12, marginVertical: 8 },
});
