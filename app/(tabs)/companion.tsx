import {
  View,
  Text,
  FlatList,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ActivityIndicator,
} from "react-native";
import { useCallback, useMemo, useRef, useState } from "react";
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
import FeedbackModal from "@/components/feedback/FeedbackModal";
import type { ChatMessage } from "@/lib/hooks/useChat";

const COMPANION_SUGGESTION_DEFS = [
  { key: "nearby", icon: MapPin },
  { key: "eat", icon: Utensils },
  { key: "navigate", icon: Compass },
  { key: "translate", icon: Globe },
] as const;

export default function CompanionScreen() {
  const { t } = useTranslation();
  const { accessToken, isLoading: authLoading } = useAuth();
  const { colors } = useTheme();
  const [feedbackOpen, setFeedbackOpen] = useState(false);
  const {
    messages,
    streamingText,
    isStreaming,
    toolActivity,
    sendMessage,
    abortStream,
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
    feedbackLink: { marginTop: 8, paddingVertical: 4, paddingHorizontal: 8 },
    feedbackLinkText: { fontSize: 12, color: colors.textTertiary, textDecorationLine: "underline" },
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
    stopButton: {
      alignSelf: "center",
      marginTop: 4,
      marginBottom: 8,
      paddingVertical: 6,
      paddingHorizontal: 16,
      borderRadius: 16,
      borderWidth: 1,
      borderColor: colors.accent,
    },
    stopButtonText: { fontSize: 13, color: colors.accent, fontWeight: "600" },
  });

  const markdownStyles = {
    body: { fontSize: 15, color: colors.assistantBubbleText, lineHeight: 22 },
    strong: { fontWeight: "700" as const },
    link: { color: colors.accent },
  };

  if (authLoading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color={colors.accent} />
      </View>
    );
  }

  if (!accessToken) {
    return (
      <>
        <View style={styles.center}>
          <Text style={styles.emptyText}>Sign in to use companion mode</Text>
          <Pressable
            onPress={() => setFeedbackOpen(true)}
            style={styles.feedbackLink}
            accessibilityRole="button"
          >
            <Text style={styles.feedbackLinkText}>Having issues? Let us know</Text>
          </Pressable>
        </View>
        <FeedbackModal visible={feedbackOpen} onClose={() => setFeedbackOpen(false)} />
      </>
    );
  }

  return (
    <>
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
            <Pressable
              onPress={() => setFeedbackOpen(true)}
              style={styles.feedbackLink}
              accessibilityRole="button"
            >
              <Text style={styles.feedbackLinkText}>Having issues? Let us know</Text>
            </Pressable>
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
            {isStreaming && (
              <Pressable
                style={styles.stopButton}
                onPress={abortStream}
                accessibilityLabel="Stop generating"
                accessibilityRole="button"
              >
                <Text style={styles.stopButtonText}>Stop generating</Text>
              </Pressable>
            )}
          </>
        }
      />
      <ChatInput onSend={sendMessage} disabled={isStreaming} placeholder="Ask your companion..." />
    </KeyboardAvoidingView>
    <FeedbackModal visible={feedbackOpen} onClose={() => setFeedbackOpen(false)} />
    </>
  );
}
