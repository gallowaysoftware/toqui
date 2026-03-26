import {
  View,
  Text,
  FlatList,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
  ActivityIndicator,
} from "react-native";
import { useCallback, useRef } from "react";
import { useLocalSearchParams } from "expo-router";
import Markdown from "react-native-markdown-display";
import { useChat } from "@/lib/hooks/useChat";
import { MessageBubble } from "@/components/chat/MessageBubble";
import { ChatInput } from "@/components/chat/ChatInput";
import { TypingIndicator } from "@/components/chat/TypingIndicator";
import { RecommendationCard } from "@/components/chat/RecommendationCard";
import type { ChatMessage } from "@/lib/hooks/useChat";

export default function ChatScreen() {
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const {
    messages,
    streamingText,
    isStreaming,
    isLoadingHistory,
    toolActivity,
    sendMessage,
  } = useChat(tripId, "planning");

  const flatListRef = useRef<FlatList>(null);

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
              <ActivityIndicator size="large" color="#e8654a" />
            </View>
          ) : (
            <View style={styles.emptyContainer}>
              <Text style={styles.emptyTitle}>Plan your trip</Text>
              <Text style={styles.emptySubtitle}>
                Ask me anything about your destination, and I'll help you build the perfect itinerary.
              </Text>
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
          </>
        }
      />
      <ChatInput onSend={sendMessage} disabled={isStreaming} />
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f5f5f5" },
  messageList: { padding: 16, flexGrow: 1 },
  loadingContainer: { flex: 1, justifyContent: "center", alignItems: "center", paddingTop: 100 },
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
