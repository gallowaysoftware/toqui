import { View, Text, Image, StyleSheet } from "react-native";
import Markdown from "react-native-markdown-display";
import type { ChatMessage } from "@/lib/hooks/useChat";
import { useTheme } from "@/lib/theme";
import { useAuth } from "@/lib/auth";
import { PersonaIntroCard } from "./PersonaIntroCard";

interface MessageBubbleProps {
  message: ChatMessage;
  showAvatar?: boolean;
}

const AVATAR_SIZE = 28;
const AVATAR_GAP = 6;
const AVATAR_SLOT = AVATAR_SIZE + AVATAR_GAP;

function colorFromString(str: string): string {
  const palette = ["#2563eb","#16a34a","#d97706","#dc2626","#7c3aed","#0891b2","#be185d"];
  let hash = 0;
  for (let i = 0; i < str.length; i++) hash = str.charCodeAt(i) + ((hash << 5) - hash);
  return palette[Math.abs(hash) % palette.length];
}

function SpeakerAvatar({ initial, color, imageUri, size = 28 }: {
  initial: string;
  color: string;
  imageUri?: string;
  size?: number;
}) {
  const avatarStyle = {
    width: size,
    height: size,
    borderRadius: size / 2,
    backgroundColor: color,
    justifyContent: "center" as const,
    alignItems: "center" as const,
    overflow: "hidden" as const,
  };
  if (imageUri) {
    return <Image source={{ uri: imageUri }} style={avatarStyle} />;
  }
  return (
    <View style={avatarStyle}>
      <Text style={{ color: "#fff", fontSize: size * 0.4, fontWeight: "700" }}>{initial}</Text>
    </View>
  );
}

function getUserInitial(user: { name?: string | null; email?: string | null } | null): string {
  if (user?.name) return user.name.trim()[0].toUpperCase();
  if (user?.email) return user.email[0].toUpperCase();
  return "U";
}

export function MessageBubble({ message, showAvatar = true }: MessageBubbleProps) {
  const { colors } = useTheme();
  const { user } = useAuth();
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

  if (isUser) {
    const userInitial = getUserInitial(user);
    return (
      <View style={styles.rowUser} accessibilityLabel="Your message">
        <View style={[
          styles.bubble,
          styles.userBubble,
          { backgroundColor: colors.userBubble, marginRight: AVATAR_GAP },
        ]}>
          <Text style={[styles.userText, { color: colors.userBubbleText }]}>{message.content}</Text>
        </View>
        {showAvatar ? (
          <SpeakerAvatar initial={userInitial} color={colors.accent} size={AVATAR_SIZE} />
        ) : (
          <View style={styles.avatarSpacer} />
        )}
      </View>
    );
  }

  const personaName = message.personaName;
  const personaAvatar = message.personaAvatar;
  const avatarColor = message.personaAccentColor
    ?? (personaName ? colorFromString(personaName) : colors.accent);
  const avatarInitial = personaName ? personaName[0].toUpperCase() : "T";
  const speakerLabel = personaName ?? "Toqui";

  return (
    <View style={styles.rowAssistant} accessibilityLabel={personaName ? `${personaName} says` : "Assistant message"}>
      <View style={styles.avatarSlot}>
        {showAvatar ? (
          <SpeakerAvatar
            initial={avatarInitial}
            color={avatarColor}
            imageUri={personaAvatar || undefined}
            size={AVATAR_SIZE}
          />
        ) : (
          <View style={styles.avatarSpacer} />
        )}
      </View>
      <View style={styles.assistantContent}>
        {showAvatar ? (
          <Text style={[styles.speakerLabel, { color: colors.textSecondary }]}>{speakerLabel}</Text>
        ) : null}
        <View style={[
          styles.bubble,
          styles.assistantBubble,
          { backgroundColor: colors.assistantBubble, borderColor: colors.assistantBubbleBorder },
          message.isError && { backgroundColor: colors.errorBg, borderColor: colors.error },
        ]}>
          <Markdown style={mdStyles}>{message.content}</Markdown>
        </View>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  rowUser: {
    flexDirection: "row",
    justifyContent: "flex-end",
    alignItems: "flex-end",
    marginBottom: 8,
  },
  rowAssistant: {
    flexDirection: "row",
    justifyContent: "flex-start",
    alignItems: "flex-start",
    marginBottom: 8,
  },
  avatarSlot: {
    width: AVATAR_SLOT,
    alignItems: "flex-start",
    paddingTop: 2,
  },
  avatarSpacer: {
    width: AVATAR_SIZE,
  },
  assistantContent: {
    flex: 1,
    maxWidth: "85%",
  },
  speakerLabel: {
    fontSize: 12,
    fontWeight: "600",
    marginBottom: 3,
  },
  bubble: { padding: 12, borderRadius: 16 },
  userBubble: { borderBottomRightRadius: 4, maxWidth: "85%" },
  assistantBubble: { borderBottomLeftRadius: 4, borderWidth: 1 },
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
});
