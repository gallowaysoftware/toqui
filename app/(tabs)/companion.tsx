import {
  View,
  Text,
  FlatList,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { useCallback, useRef } from "react";
import Markdown from "react-native-markdown-display";
import { useChat } from "@/lib/hooks/useChat";
import { useAuth } from "@/lib/auth";
import { MessageBubble } from "@/components/chat/MessageBubble";
import { ChatInput } from "@/components/chat/ChatInput";
import { TypingIndicator } from "@/components/chat/TypingIndicator";
import { RecommendationCard } from "@/components/chat/RecommendationCard";
import type { ChatMessage } from "@/lib/hooks/useChat";

export default function CompanionScreen() {
  const { accessToken } = useAuth();
  const {
    messages,
    streamingText,
    isStreaming,
    toolActivity,
    sendMessage,
  } = useChat(undefined, "companion");

  const flatListRef = useRef<FlatList>(null);

  const renderMessage = useCallback(({ item }: { item: ChatMessage }) => {
    if (item.recommendation) {
      return <RecommendationCard recommendation={item.recommendation} />;
    }
    return <MessageBubble message={item} />;
  }, []);

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

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f5f5f5" },
  center: { flex: 1, justifyContent: "center", alignItems: "center" },
  emptyText: { fontSize: 16, color: "#666" },
  messageList: { padding: 16, flexGrow: 1 },
  emptyContainer: { flex: 1, justifyContent: "center", alignItems: "center", paddingTop: 100 },
  emptyTitle: { fontSize: 20, fontWeight: "bold", color: "#e8654a", marginBottom: 8 },
  emptySubtitle: { fontSize: 14, color: "#666", textAlign: "center", paddingHorizontal: 40 },
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
  link: { color: "#e8654a" },
});
