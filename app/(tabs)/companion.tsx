import {
  View,
  Text,
  FlatList,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { useCallback, useMemo, useRef } from "react";
import { useTranslation } from "react-i18next";
import { MapPin, Utensils, Compass, Globe } from "lucide-react-native";
import Markdown from "react-native-markdown-display";
import { useChat } from "@/lib/hooks/useChat";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { MessageBubble } from "@/components/chat/MessageBubble";
import { ChatInput } from "@/components/chat/ChatInput";
import { TypingIndicator } from "@/components/chat/TypingIndicator";
import { RecommendationCard } from "@/components/chat/RecommendationCard";
import { SuggestionChips } from "@/components/chat/SuggestionChips";
import type { ChatMessage } from "@/lib/hooks/useChat";

const COMPANION_SUGGESTION_DEFS = [
  { key: "nearby", icon: MapPin },
  { key: "eat", icon: Utensils },
  { key: "navigate", icon: Compass },
  { key: "translate", icon: Globe },
] as const;

export default function CompanionScreen() {
  const { t } = useTranslation();
  const { accessToken } = useAuth();
  const { colors } = useTheme();
  const {
    messages,
    streamingText,
    isStreaming,
    toolActivity,
    sendMessage,
  } = useChat(undefined, "companion");

  const flatListRef = useRef<FlatList>(null);

  const suggestions = useMemo(
    () => COMPANION_SUGGESTION_DEFS.map((s) => ({ ...s, label: t(`companion.suggestions.${s.key}`) })),
    [t],
  );

  const renderMessage = useCallback(({ item }: { item: ChatMessage }) => {
    if (item.recommendation) {
      return <RecommendationCard recommendation={item.recommendation} />;
    }
    return <MessageBubble message={item} />;
  }, []);

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    center: { flex: 1, justifyContent: "center", alignItems: "center" },
    emptyText: { fontSize: 16, color: colors.textSecondary },
    messageList: { padding: 16, flexGrow: 1 },
    emptyContainer: { flex: 1, justifyContent: "center", alignItems: "center", paddingTop: 100 },
    emptyTitle: { fontSize: 20, fontWeight: "bold", color: colors.accent, marginBottom: 8 },
    emptySubtitle: { fontSize: 14, color: colors.textSecondary, textAlign: "center", paddingHorizontal: 40, marginBottom: 20 },
    streamingBubble: {
      maxWidth: "85%",
      padding: 12,
      borderRadius: 16,
      borderBottomLeftRadius: 4,
      backgroundColor: colors.assistantBubble,
      borderWidth: 1,
      borderColor: colors.assistantBubbleBorder,
      alignSelf: "flex-start",
      marginBottom: 8,
    },
  });

  const markdownStyles = {
    body: { fontSize: 15, color: colors.assistantBubbleText, lineHeight: 22 },
    strong: { fontWeight: "700" as const },
    link: { color: colors.accent },
  };

  if (!accessToken) {
    return (
      <View style={styles.center}>
        <Text style={styles.emptyText}>Sign in to use companion mode</Text>
      </View>
    );
  }

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
      keyboardVerticalOffset={90}
    >
      <FlatList
        ref={flatListRef}
        data={messages}
        renderItem={renderMessage}
        keyExtractor={(item) => item.id}
        contentContainerStyle={styles.messageList}
        onContentSizeChange={() => flatListRef.current?.scrollToEnd({ animated: true })}
        ListEmptyComponent={
          <View style={styles.emptyContainer}>
            <Text style={styles.emptyTitle}>Travel Companion</Text>
            <Text style={styles.emptySubtitle}>
              Ask me anything about your surroundings, get directions, find restaurants, or discover hidden gems nearby.
            </Text>
            <SuggestionChips suggestions={suggestions} onSelect={sendMessage} />
          </View>
        }
        ListFooterComponent={
          <>
            {toolActivity && <TypingIndicator toolName={toolActivity.toolName} />}
            {streamingText ? (
              <View style={styles.streamingBubble}>
                <Markdown style={markdownStyles}>{streamingText}</Markdown>
              </View>
            ) : isStreaming && !toolActivity ? (
              <TypingIndicator />
            ) : null}
          </>
        }
      />
      <ChatInput onSend={sendMessage} disabled={isStreaming} placeholder="Ask your companion..." />
    </KeyboardAvoidingView>
  );
}
