import { View, Text, StyleSheet } from "react-native";
import Markdown from "react-native-markdown-display";
import type { ChatMessage } from "@/lib/hooks/useChat";
import { useTheme } from "@/lib/theme";
import { PersonaIntroCard } from "./PersonaIntroCard";

interface MessageBubbleProps {
  message: ChatMessage;
}

export function MessageBubble({ message }: MessageBubbleProps) {
  const { colors } = useTheme();
  const isUser = message.role === "user";
  const isSystem = message.role === "system";

  if (isSystem) {
    if (message.personaIntro) {
      return <PersonaIntroCard persona={message.personaIntro} />;
    }
    return (
      <View style={[styles.systemBubble, { backgroundColor: colors.surfaceTertiary }]}>
        <Text style={[styles.systemText, { color: colors.textTertiary }]}>{message.content}</Text>
      </View>
    );
  }

  const mdStyles = {
    body: { fontSize: 15, color: colors.assistantBubbleText, lineHeight: 22 },
    strong: { fontWeight: "700" as const },
    em: { fontStyle: "italic" as const },
    link: { color: colors.accent },
    heading1: { fontSize: 20, fontWeight: "700" as const, marginBottom: 8, color: colors.textPrimary },
    heading2: { fontSize: 18, fontWeight: "700" as const, marginBottom: 6, color: colors.textPrimary },
    heading3: { fontSize: 16, fontWeight: "600" as const, marginBottom: 4, color: colors.textPrimary },
    bullet_list: { marginBottom: 8 },
    ordered_list: { marginBottom: 8 },
    list_item: { marginBottom: 4 },
    code_inline: { backgroundColor: colors.surfaceTertiary, paddingHorizontal: 4, borderRadius: 3, fontFamily: "monospace" as const },
    fence: { backgroundColor: colors.surfaceTertiary, padding: 8, borderRadius: 6, fontFamily: "monospace" as const },
    blockquote: { borderLeftWidth: 3, borderLeftColor: colors.accent, paddingLeft: 12, marginVertical: 8 },
  };

  return (
    <View style={[
      styles.bubble,
      isUser
        ? [styles.userBubble, { backgroundColor: colors.userBubble }]
        : [styles.assistantBubble, { backgroundColor: colors.assistantBubble, borderColor: colors.assistantBubbleBorder }],
      message.isError && { backgroundColor: colors.errorBg, borderColor: colors.error },
    ]}>
      {message.personaName && !isUser ? (
        <View style={styles.personaHeader}>
          <View style={[styles.personaDot, { backgroundColor: message.personaAccentColor ?? colors.accent }]} />
          <Text style={[styles.personaName, { color: colors.textSecondary }]}>{message.personaName}</Text>
        </View>
      ) : null}
      {isUser ? (
        <Text style={[styles.userText, { color: colors.userBubbleText }]}>{message.content}</Text>
      ) : (
        <Markdown style={mdStyles}>{message.content}</Markdown>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  bubble: { maxWidth: "85%", padding: 12, borderRadius: 16, marginBottom: 8 },
  userBubble: { alignSelf: "flex-end", borderBottomRightRadius: 4 },
  assistantBubble: { alignSelf: "flex-start", borderBottomLeftRadius: 4, borderWidth: 1 },
  systemBubble: {
    alignSelf: "center",
    borderRadius: 12,
    paddingVertical: 6,
    paddingHorizontal: 12,
    marginBottom: 8,
    maxWidth: "90%",
  },
  systemText: { fontSize: 13, fontStyle: "italic", textAlign: "center" },
  userText: { fontSize: 15, lineHeight: 22 },
  personaHeader: { flexDirection: "row", alignItems: "center", gap: 6, marginBottom: 4 },
  personaDot: { width: 8, height: 8, borderRadius: 4 },
  personaName: { fontSize: 12, fontWeight: "600" },
});
