import {
  View,
  Text,
  TextInput,
  Pressable,
  FlatList,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { useState, useCallback, useRef } from "react";
import { useLocalSearchParams } from "expo-router";
import { createClient, Code, ConnectError } from "@connectrpc/connect";
import { useTransport } from "@/lib/transport";
import { ChatService, ChatMode } from "@gen/toqui/v1/chat_pb";

interface Message {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
  isError?: boolean;
}

function uuid(): string {
  if (
    typeof crypto !== "undefined" &&
    typeof crypto.randomUUID === "function"
  ) {
    return crypto.randomUUID();
  }
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const h = [...bytes].map((b) => b.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

/**
 * Minimal chat screen to validate ConnectRPC server-side streaming
 * works on React Native (iOS, Android, and Web).
 *
 * This uses the same `for await` pattern as the Next.js useChat hook.
 */
export default function ChatScreen() {
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const transport = useTransport();
  const [messages, setMessages] = useState<Message[]>([]);
  const [streamingText, setStreamingText] = useState("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [input, setInput] = useState("");
  const [toolActivity, setToolActivity] = useState<string | null>(null);
  const sessionIdRef = useRef("");
  const isSendingRef = useRef(false);
  const flatListRef = useRef<FlatList>(null);

  const sendMessage = useCallback(
    async (content: string) => {
      if (!content.trim() || isSendingRef.current) return;
      isSendingRef.current = true;

      const userMsg: Message = { id: uuid(), role: "user", content };
      setMessages((prev) => [...prev, userMsg]);
      setInput("");
      setIsStreaming(true);
      setStreamingText("");
      setToolActivity(null);

      try {
        const client = createClient(ChatService, transport);
        let fullText = "";

        // This is the critical test: server-side streaming via ConnectRPC
        // Uses the same `for await` pattern as the existing Next.js app
        for await (const event of client.sendMessage({
          sessionId: sessionIdRef.current,
          tripId: tripId ?? "",
          content,
          mode: ChatMode.SELECTION,
        })) {
          switch (event.event.case) {
            case "textDelta":
              fullText += event.event.value.text;
              setStreamingText(fullText);
              break;

            case "sessionCreated":
              sessionIdRef.current = event.event.value.sessionId;
              break;

            case "toolCall":
              setToolActivity(event.event.value.toolName);
              break;

            case "toolResult":
              setToolActivity(null);
              break;

            case "messageComplete":
              if (event.event.value.fullContent) {
                fullText = event.event.value.fullContent;
              }
              setToolActivity(null);
              break;

            case "personaSwitch": {
              const ps = event.event.value;
              if (ps.handoffMessage) {
                setMessages((prev) => [
                  ...prev,
                  { id: uuid(), role: "system", content: ps.handoffMessage },
                ]);
              }
              break;
            }

            case "error":
              console.error("Stream error event:", event.event.value.message);
              break;
          }
        }

        // Stream complete — add the full assistant message
        if (fullText) {
          setMessages((prev) => [
            ...prev,
            { id: uuid(), role: "assistant", content: fullText },
          ]);
        }
      } catch (error) {
        console.error("Chat streaming error:", error);
        const isRateLimit =
          error instanceof ConnectError &&
          error.code === Code.ResourceExhausted;
        setMessages((prev) => [
          ...prev,
          {
            id: uuid(),
            role: "assistant",
            content: isRateLimit
              ? "Rate limit reached. Please try again later."
              : `Error: ${error instanceof Error ? error.message : "Unknown error"}`,
            isError: true,
          },
        ]);
      } finally {
        setStreamingText("");
        setIsStreaming(false);
        setToolActivity(null);
        isSendingRef.current = false;
      }
    },
    [transport, tripId],
  );

  const renderMessage = useCallback(
    ({ item }: { item: Message }) => (
      <View
        style={[
          styles.messageBubble,
          item.role === "user" ? styles.userBubble : styles.assistantBubble,
          item.isError && styles.errorBubble,
          item.role === "system" && styles.systemBubble,
        ]}
      >
        <Text
          style={[
            styles.messageText,
            item.role === "user" && styles.userText,
          ]}
        >
          {item.content}
        </Text>
      </View>
    ),
    [],
  );

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
        onContentSizeChange={() =>
          flatListRef.current?.scrollToEnd({ animated: true })
        }
        ListEmptyComponent={
          <View style={styles.emptyContainer}>
            <Text style={styles.emptyTitle}>Streaming PoC</Text>
            <Text style={styles.emptySubtitle}>
              Send a message to test ConnectRPC server-side streaming on this
              platform.
            </Text>
          </View>
        }
        ListFooterComponent={
          <>
            {toolActivity && (
              <View style={styles.toolBubble}>
                <Text style={styles.toolText}>
                  Calling {toolActivity}...
                </Text>
              </View>
            )}
            {streamingText ? (
              <View style={[styles.messageBubble, styles.assistantBubble]}>
                <Text style={styles.messageText}>{streamingText}</Text>
                <Text style={styles.streamingIndicator}>●</Text>
              </View>
            ) : null}
          </>
        }
      />

      <View style={styles.inputRow}>
        <TextInput
          style={styles.textInput}
          value={input}
          onChangeText={setInput}
          placeholder="Type a message..."
          multiline
          editable={!isStreaming}
          onSubmitEditing={() => sendMessage(input)}
          blurOnSubmit={false}
        />
        <Pressable
          style={[
            styles.sendButton,
            (!input.trim() || isStreaming) && styles.disabledButton,
          ]}
          onPress={() => sendMessage(input)}
          disabled={!input.trim() || isStreaming}
        >
          <Text style={styles.sendButtonText}>
            {isStreaming ? "..." : "Send"}
          </Text>
        </Pressable>
      </View>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: "#f5f5f5",
  },
  messageList: {
    padding: 16,
    flexGrow: 1,
  },
  emptyContainer: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    paddingTop: 100,
  },
  emptyTitle: {
    fontSize: 20,
    fontWeight: "bold",
    color: "#e8654a",
    marginBottom: 8,
  },
  emptySubtitle: {
    fontSize: 14,
    color: "#666",
    textAlign: "center",
    paddingHorizontal: 40,
  },
  messageBubble: {
    maxWidth: "80%",
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
  systemBubble: {
    backgroundColor: "#f0f0f0",
    alignSelf: "center",
    maxWidth: "90%",
  },
  errorBubble: {
    backgroundColor: "#fee2e2",
    borderColor: "#fca5a5",
  },
  messageText: {
    fontSize: 15,
    color: "#333",
    lineHeight: 22,
  },
  userText: {
    color: "#fff",
  },
  streamingIndicator: {
    color: "#e8654a",
    fontSize: 8,
    marginTop: 4,
  },
  toolBubble: {
    backgroundColor: "#f0f0f0",
    alignSelf: "flex-start",
    padding: 8,
    borderRadius: 12,
    marginBottom: 8,
  },
  toolText: {
    fontSize: 12,
    color: "#888",
    fontStyle: "italic",
  },
  inputRow: {
    flexDirection: "row",
    padding: 12,
    borderTopWidth: 1,
    borderTopColor: "#e0e0e0",
    backgroundColor: "#fff",
    alignItems: "flex-end",
  },
  textInput: {
    flex: 1,
    borderWidth: 1,
    borderColor: "#ddd",
    borderRadius: 20,
    paddingHorizontal: 16,
    paddingVertical: 10,
    fontSize: 15,
    maxHeight: 100,
  },
  sendButton: {
    backgroundColor: "#e8654a",
    borderRadius: 20,
    paddingHorizontal: 20,
    paddingVertical: 10,
    marginLeft: 8,
  },
  disabledButton: {
    opacity: 0.5,
  },
  sendButtonText: {
    color: "#fff",
    fontWeight: "600",
    fontSize: 15,
  },
});
