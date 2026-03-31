import {
  View,
  Text,
  FlatList,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
  ActivityIndicator,
  Pressable,
} from "react-native";
import { useCallback, useMemo, useRef, useState } from "react";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { MapPin, Utensils, Compass, Briefcase } from "lucide-react-native";
import Markdown from "react-native-markdown-display";
import { useChat } from "@/lib/hooks/useChat";
import { MessageBubble } from "@/components/chat/MessageBubble";
import { ChatInput } from "@/components/chat/ChatInput";
import { TypingIndicator } from "@/components/chat/TypingIndicator";
import { RecommendationCard } from "@/components/chat/RecommendationCard";
import { SuggestionChips } from "@/components/chat/SuggestionChips";
import type { ChatMessage } from "@/lib/hooks/useChat";

const CHAT_SUGGESTION_DEFS = [
  { key: "itinerary", icon: MapPin },
  { key: "restaurants", icon: Utensils },
  { key: "hiddenGems", icon: Compass },
  { key: "packing", icon: Briefcase },
] as const;

export default function ChatScreen() {
  const { t } = useTranslation();
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const router = useRouter();
  const [showExpertBanner, setShowExpertBanner] = useState(false);
  const {
    messages,
    streamingText,
    isStreaming,
    isLoadingHistory,
    historyError,
    toolActivity,
    sendMessage,
    abortStream,
    loadMoreHistory,
  } = useChat(tripId, "planning", {
    onExpertLimitReached: () => setShowExpertBanner(true),
  });

  const flatListRef = useRef<FlatList>(null);

  const suggestions = useMemo(
    () => CHAT_SUGGESTION_DEFS.map((s) => ({ ...s, label: t(`chat.suggestions.${s.key}`) })),
    [t],
  );

  const renderMessage = useCallback(({ item }: { item: ChatMessage }) => {
    if (item.recommendation) {
      return <RecommendationCard recommendation={item.recommendation} />;
    }
    return <MessageBubble message={item} />;
  }, []);

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
          isLoadingHistory ? (
            <View style={styles.loadingContainer}>
              <ActivityIndicator size="large" color="#BF4028" />
            </View>
          ) : historyError && !isStreaming ? (
            <View style={styles.emptyContainer}>
              <Text style={styles.errorTitle}>Could not load messages</Text>
              <Text style={styles.emptySubtitle}>{historyError}</Text>
              <Pressable style={styles.retryButton} onPress={loadMoreHistory}>
                <Text style={styles.retryButtonText}>Retry</Text>
              </Pressable>
            </View>
          ) : (
            <View style={styles.emptyContainer}>
              <Text style={styles.emptyTitle}>Plan your trip</Text>
              <Text style={styles.emptySubtitle}>
                Ask me anything about your destination, and I'll help you build the perfect itinerary.
              </Text>
              <SuggestionChips suggestions={suggestions} onSelect={sendMessage} />
            </View>
          )
        }
        ListFooterComponent={
          <>
            {toolActivity && (
              <TypingIndicator toolName={toolActivity.toolName} />
            )}
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
      {showExpertBanner && tripId && (
        <View style={styles.expertBanner}>
          <Text style={styles.expertBannerText}>{t("chat.expertLimitReached")}</Text>
          <Pressable
            onPress={() => router.push(`/trips/${tripId}`)}
            style={styles.expertBannerButton}
          >
            <Text style={styles.expertBannerButtonText}>{t("chat.expertLimitCta")}</Text>
          </Pressable>
        </View>
      )}
      <ChatInput
        onSend={(text, attachments) => sendMessage(text, attachments)}
        disabled={isStreaming}
      />
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f5f5f5" },
  messageList: { padding: 16, flexGrow: 1 },
  loadingContainer: { flex: 1, justifyContent: "center", alignItems: "center", paddingTop: 100 },
  emptyContainer: { alignItems: "center", paddingTop: 100, paddingBottom: 24 },
  emptyTitle: { fontSize: 20, fontWeight: "bold", color: "#BF4028", marginBottom: 8 },
  emptySubtitle: { fontSize: 14, color: "#666", textAlign: "center", paddingHorizontal: 40, marginBottom: 20 },
  errorTitle: { fontSize: 20, fontWeight: "bold", color: "#c0392b", marginBottom: 8 },
  retryButton: { backgroundColor: "#BF4028", borderRadius: 16, paddingVertical: 8, paddingHorizontal: 24 },
  retryButtonText: { color: "#fff", fontSize: 14, fontWeight: "600" },
  expertBanner: {
    backgroundColor: "#fff8f0",
    borderTopWidth: 1,
    borderTopColor: "#BF4028",
    paddingVertical: 10,
    paddingHorizontal: 16,
    alignItems: "center",
  },
  expertBannerText: { fontSize: 13, color: "#333", marginBottom: 6, textAlign: "center" },
  expertBannerButton: {
    backgroundColor: "#BF4028",
    borderRadius: 16,
    paddingVertical: 6,
    paddingHorizontal: 16,
  },
  expertBannerButtonText: { color: "#fff", fontSize: 13, fontWeight: "600" },
  stopButton: {
    alignSelf: "center",
    marginTop: 4,
    marginBottom: 8,
    paddingVertical: 6,
    paddingHorizontal: 16,
    borderRadius: 16,
    borderWidth: 1,
    borderColor: "#BF4028",
  },
  stopButtonText: { fontSize: 13, color: "#BF4028", fontWeight: "600" },
  streamingBubble: {
    maxWidth: "85%",
    padding: 12,
    borderRadius: 16,
    borderBottomLeftRadius: 4,
    backgroundColor: "#fff",
    borderWidth: 1,
    borderColor: "#e0e0e0",
    alignSelf: "flex-start",
    marginBottom: 8,
  },
});

const markdownStyles = StyleSheet.create({
  body: { fontSize: 15, color: "#333", lineHeight: 22 },
  strong: { fontWeight: "700" },
  link: { color: "#BF4028" },
});
