import { View, TextInput, Pressable, StyleSheet, Platform } from "react-native";
import { useState, useCallback } from "react";
import { Send } from "lucide-react-native";

interface ChatInputProps {
  onSend: (message: string) => void;
  disabled?: boolean;
  placeholder?: string;
}

export function ChatInput({ onSend, disabled, placeholder = "Type a message..." }: ChatInputProps) {
  const [text, setText] = useState("");

  const handleSend = useCallback(() => {
    const trimmed = text.trim();
    if (!trimmed || disabled) return;
    onSend(trimmed);
    setText("");
  }, [text, disabled, onSend]);

  return (
    <View style={styles.container}>
      <TextInput
        style={styles.input}
        value={text}
        onChangeText={setText}
        placeholder={placeholder}
        placeholderTextColor="#999"
        multiline
        maxLength={10000}
        editable={!disabled}
        onSubmitEditing={Platform.OS === "web" ? handleSend : undefined}
        blurOnSubmit={false}
      />
      <Pressable
        role="button"
        aria-label="Send"
        style={[styles.sendButton, (!text.trim() || disabled) && styles.disabledButton]}
        onPress={handleSend}
        disabled={!text.trim() || disabled}
      >
        <Send color="#fff" size={18} />
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: "row",
    padding: 12,
    borderTopWidth: 1,
    borderTopColor: "#e0e0e0",
    backgroundColor: "#fff",
    alignItems: "flex-end",
  },
  input: {
    flex: 1,
    borderWidth: 1,
    borderColor: "#ddd",
    borderRadius: 20,
    paddingHorizontal: 16,
    paddingVertical: 10,
    fontSize: 15,
    maxHeight: 120,
    color: "#333",
  },
  sendButton: {
    backgroundColor: "#BF4028",
    borderRadius: 20,
    width: 40,
    height: 40,
    justifyContent: "center",
    alignItems: "center",
    marginLeft: 8,
  },
  disabledButton: {
    opacity: 0.4,
  },
});
