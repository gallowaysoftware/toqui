import { View, Text, TextInput, Pressable, StyleSheet, Platform } from "react-native";
import { useState, useCallback, useRef } from "react";
import { Send, Paperclip, X } from "lucide-react-native";

interface AttachmentFile {
  filename: string;
  mediaType: string;
  data: Uint8Array;
}

interface ChatInputProps {
  onSend: (message: string, attachments?: AttachmentFile[]) => void;
  disabled?: boolean;
  placeholder?: string;
}

const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10MB
const ALLOWED_TYPES = [
  "image/jpeg", "image/png", "image/gif", "image/webp",
  "application/pdf", "text/plain", "text/csv",
];

function readFileAsUint8Array(file: File): Promise<Uint8Array> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(new Uint8Array(reader.result as ArrayBuffer));
    reader.onerror = reject;
    reader.readAsArrayBuffer(file);
  });
}

function validateFile(file: File): string | null {
  if (file.size > MAX_FILE_SIZE) return `${file.name} is too large (max 10MB)`;
  if (!ALLOWED_TYPES.includes(file.type) && !file.name.match(/\.(jpg|jpeg|png|gif|webp|pdf|txt|csv)$/i)) {
    return `${file.name}: unsupported file type`;
  }
  return null;
}

export function ChatInput({ onSend, disabled, placeholder = "Type a message..." }: ChatInputProps) {
  const [text, setText] = useState("");
  const [attachments, setAttachments] = useState<AttachmentFile[]>([]);
  const [isDragging, setIsDragging] = useState(false);
  const [error, setError] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);

  const addFiles = useCallback(async (files: FileList | File[]) => {
    setError("");
    const newAttachments: AttachmentFile[] = [];
    for (const file of Array.from(files)) {
      if (attachments.length + newAttachments.length >= 5) {
        setError("Maximum 5 files per message");
        break;
      }
      const validationError = validateFile(file);
      if (validationError) {
        setError(validationError);
        continue;
      }
      const data = await readFileAsUint8Array(file);
      newAttachments.push({
        filename: file.name,
        mediaType: file.type || "application/octet-stream",
        data,
      });
    }
    if (newAttachments.length > 0) {
      setAttachments((prev) => [...prev, ...newAttachments]);
    }
  }, [attachments.length]);

  const removeAttachment = useCallback((index: number) => {
    setAttachments((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const handleSend = useCallback(() => {
    const trimmed = text.trim();
    if ((!trimmed && attachments.length === 0) || disabled) return;
    onSend(trimmed, attachments.length > 0 ? attachments : undefined);
    setText("");
    setAttachments([]);
    setError("");
  }, [text, attachments, disabled, onSend]);

  const handleFileSelect = useCallback(() => {
    if (Platform.OS === "web") {
      fileInputRef.current?.click();
    } else {
      // Native: use expo-document-picker
      void (async () => {
        const { getDocumentAsync } = await import("expo-document-picker");
        const result = await getDocumentAsync({
          multiple: true,
          type: ALLOWED_TYPES,
        });
        if (result.canceled || !result.assets) return;
        const files: AttachmentFile[] = [];
        for (const asset of result.assets) {
          if (asset.size && asset.size > MAX_FILE_SIZE) {
            setError(`${asset.name} is too large (max 10MB)`);
            continue;
          }
          const ExpoFS = await import("expo-file-system");
          const base64 = await ExpoFS.default.readAsStringAsync(asset.uri, { encoding: "base64" as const });
          const binary = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0));
          files.push({
            filename: asset.name,
            mediaType: asset.mimeType || "application/octet-stream",
            data: binary,
          });
        }
        if (files.length > 0) {
          setAttachments((prev) => [...prev, ...files].slice(0, 5));
        }
      })();
    }
  }, []);

  // Web-only drag-and-drop handlers
  const webDragProps = Platform.OS === "web" ? {
    onDragOver: (e: React.DragEvent) => { e.preventDefault(); setIsDragging(true); },
    onDragLeave: () => setIsDragging(false),
    onDrop: (e: React.DragEvent) => {
      e.preventDefault();
      setIsDragging(false);
      if (e.dataTransfer.files.length > 0) {
        void addFiles(e.dataTransfer.files);
      }
    },
  } as Record<string, unknown> : {};

  return (
    <View style={styles.wrapper}>
      {attachments.length > 0 && (
        <View style={styles.attachmentRow}>
          {attachments.map((a, i) => (
            <View key={`${a.filename}-${i}`} style={styles.attachmentChip}>
              <Text style={styles.attachmentName} numberOfLines={1}>{a.filename}</Text>
              <Pressable
                onPress={() => removeAttachment(i)}
                hitSlop={8}
                accessibilityLabel={`Remove ${a.filename}`}
                accessibilityRole="button"
              >
                <X color="#666" size={14} />
              </Pressable>
            </View>
          ))}
        </View>
      )}
      {error ? <Text style={styles.errorText} accessibilityLiveRegion="polite">{error}</Text> : null}
      <View
        style={[styles.container, isDragging && styles.dragging]}
        {...webDragProps}
      >
        <Pressable
          style={styles.attachButton}
          onPress={handleFileSelect}
          disabled={disabled}
          accessibilityLabel="Attach file"
          accessibilityRole="button"
        >
          <Paperclip color={disabled ? "#ccc" : "#999"} size={20} />
        </Pressable>
        <TextInput
          style={styles.input}
          value={text}
          onChangeText={setText}
          placeholder={isDragging ? "Drop files here..." : placeholder}
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
          accessibilityLabel="Send message"
          style={[styles.sendButton, (!text.trim() && attachments.length === 0 || disabled) && styles.disabledButton]}
          onPress={handleSend}
          disabled={(!text.trim() && attachments.length === 0) || disabled}
        >
          <Send color="#fff" size={18} />
        </Pressable>
        {Platform.OS === "web" && (
          <input
            ref={fileInputRef as React.RefObject<HTMLInputElement>}
            type="file"
            multiple
            accept={ALLOWED_TYPES.join(",")}
            style={{ display: "none" }}
            onChange={(e) => {
              if (e.target.files) void addFiles(e.target.files);
              e.target.value = "";
            }}
          />
        )}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  wrapper: {
    backgroundColor: "#fff",
    borderTopWidth: 1,
    borderTopColor: "#e0e0e0",
  },
  container: {
    flexDirection: "row",
    padding: 12,
    alignItems: "flex-end",
  },
  dragging: {
    backgroundColor: "#fef3f0",
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
  attachButton: {
    width: 44,
    height: 44,
    justifyContent: "center",
    alignItems: "center",
    marginRight: 4,
  },
  sendButton: {
    backgroundColor: "#BF4028",
    borderRadius: 22,
    width: 44,
    height: 44,
    justifyContent: "center",
    alignItems: "center",
    marginLeft: 8,
  },
  disabledButton: {
    opacity: 0.4,
  },
  attachmentRow: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 6,
    paddingHorizontal: 12,
    paddingTop: 8,
  },
  attachmentChip: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    backgroundColor: "#f0f0f0",
    borderRadius: 16,
    paddingHorizontal: 10,
    paddingVertical: 5,
    maxWidth: 200,
  },
  attachmentName: {
    fontSize: 12,
    color: "#444",
    flex: 1,
  },
  errorText: {
    fontSize: 12,
    color: "#ef4444",
    paddingHorizontal: 12,
    paddingTop: 4,
  },
});
