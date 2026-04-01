import { useState, useCallback, useRef, useEffect } from "react";
import { Platform } from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { createClient, Code, ConnectError } from "@connectrpc/connect";
import { useTransport } from "@/lib/transport";
import { ChatService, ChatMode } from "@gen/toqui/v1/chat_pb";
import type { ChatMessage as ProtoChatMessage } from "@gen/toqui/v1/chat_pb";
import type { Persona } from "@gen/toqui/v1/persona_pb";

export interface Recommendation {
  partner: string;
  category: string;
  title: string;
  description: string;
  url: string;
  price?: string;
  disclosure?: string;
}

export interface PersonaIntroData {
  name: string;
  specialties: string[];
  accentColor: string;
  avatarUrl: string;
  handoffMessage: string;
}

export interface ChatMessage {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
  isError?: boolean;
  personaId?: string;
  personaName?: string;
  personaAvatar?: string;
  personaAccentColor?: string;
  recommendation?: Recommendation;
  personaIntro?: PersonaIntroData;
}

export interface ToolActivity {
  toolName: string;
  status: "calling" | "done";
}

export interface ActivePersona {
  id: string;
  name: string;
  avatarUrl: string;
  accentColor: string;
  specialties: string[];
}

export interface CreatedTrip {
  id: string;
  title: string;
  description: string;
}

export interface SelectedTrip {
  id: string;
  title: string;
  description: string;
}

function uuid(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const h = [...bytes].map((b) => b.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

function personaToActive(p: Persona): ActivePersona {
  return {
    id: p.id,
    name: p.name,
    avatarUrl: p.avatarUrl,
    accentColor: p.accentColor,
    specialties: p.specialties,
  };
}

const modeToProto: Record<string, ChatMode> = {
  planning: ChatMode.PLANNING,
  companion: ChatMode.COMPANION,
  selection: ChatMode.SELECTION,
};

function protoToFrontendMessage(msg: ProtoChatMessage): ChatMessage | null {
  const role = msg.role as ChatMessage["role"];
  if (role !== "user" && role !== "assistant" && role !== "system") return null;
  return {
    id: msg.id,
    role,
    content: msg.content,
    personaId: msg.metadata["persona_id"] || undefined,
    personaName: msg.metadata["persona_name"] || undefined,
    personaAvatar: msg.metadata["persona_avatar"] || undefined,
    personaAccentColor: msg.metadata["persona_accent_color"] || undefined,
  };
}

export interface FailedMessage {
  content: string;
  attachments?: { filename: string; mediaType: string; data: Uint8Array }[];
}

// Persist session IDs across remounts so navigating away and back resumes the same session.
async function getPersistedSessionId(tripId?: string): Promise<string> {
  const key = `toqui_session_${tripId ?? "companion"}`;
  if (Platform.OS === "web") return sessionStorage.getItem(key) ?? "";
  return (await AsyncStorage.getItem(key)) ?? "";
}

async function persistSessionId(tripId: string | undefined, id: string): Promise<void> {
  const key = `toqui_session_${tripId ?? "companion"}`;
  if (Platform.OS === "web") {
    sessionStorage.setItem(key, id);
    return;
  }
  await AsyncStorage.setItem(key, id);
}

interface UseChatOptions {
  onResourceExhausted?: () => void;
  onExpertLimitReached?: () => void;
}

export function useChat(
  tripId: string | undefined,
  mode: "planning" | "companion" | "selection",
  options?: UseChatOptions,
) {
  const transport = useTransport();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [streamingText, setStreamingText] = useState("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [isLoadingHistory, setIsLoadingHistory] = useState(false);
  const [activePersona, setActivePersona] = useState<ActivePersona | null>(null);
  const [toolActivity, setToolActivity] = useState<ToolActivity | null>(null);
  const [createdTrip, setCreatedTrip] = useState<CreatedTrip | null>(null);
  const [selectedTrip, setSelectedTrip] = useState<SelectedTrip | null>(null);
  const [hasMoreHistory, setHasMoreHistory] = useState(false);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [historyError, setHistoryError] = useState<string | null>(null);
  const [lastFailedMessage, setLastFailedMessage] = useState<FailedMessage | null>(null);
  // Incremented by retryHistory() to force a fresh initial load.
  const [historyKey, setHistoryKey] = useState(0);
  const isSendingRef = useRef(false);
  const isLoadingMoreRef = useRef(false);
  const abortControllerRef = useRef<AbortController | null>(null);
  const sessionIdRef = useRef("");
  const activePersonaRef = useRef<ActivePersona | null>(null);
  const historyLoadedRef = useRef<string | null>(null);
  // Last historyKey value for which we started a load — prevents double-loads on re-renders.
  const historyKeyLoadedRef = useRef(-1);
  const nextPageTokenRef = useRef("");
  // Stable ID of the assistant message being built by textDelta events.
  const streamingMessageIdRef = useRef<string | null>(null);
  const onResourceExhaustedRef = useRef(options?.onResourceExhausted);
  const onExpertLimitReachedRef = useRef(options?.onExpertLimitReached);
  useEffect(() => {
    onResourceExhaustedRef.current = options?.onResourceExhausted;
    onExpertLimitReachedRef.current = options?.onExpertLimitReached;
  }, [options?.onResourceExhausted, options?.onExpertLimitReached]);

  // Reset state when tripId changes
  useEffect(() => {
    setMessages([]);
    setStreamingText("");
    setIsStreaming(false);
    setActivePersona(null);
    setToolActivity(null);
    setCreatedTrip(null);
    setSelectedTrip(null);
    setHasMoreHistory(false);
    setHistoryError(null);
    setLastFailedMessage(null);
    sessionIdRef.current = "";
    activePersonaRef.current = null;
    historyLoadedRef.current = null;
    historyKeyLoadedRef.current = -1;
    nextPageTokenRef.current = "";
    streamingMessageIdRef.current = null;
    isSendingRef.current = false;
  }, [tripId]);

  // Hydrate sessionIdRef from persistent storage so remounts resume the same session.
  useEffect(() => {
    let cancelled = false;
    void getPersistedSessionId(tripId).then((id) => {
      if (!cancelled) sessionIdRef.current = id;
    });
    return () => { cancelled = true; };
  }, [tripId]);

  // Load chat history
  useEffect(() => {
    if (!tripId) return;
    // Skip if we already fired a load for this exact (tripId, historyKey) combination.
    if (historyKeyLoadedRef.current === historyKey && historyLoadedRef.current === tripId) return;
    historyKeyLoadedRef.current = historyKey;
    let cancelled = false;
    const loadHistory = async () => {
      setIsLoadingHistory(true);
      try {
        const client = createClient(ChatService, transport);
        const res = await client.getChatHistory({
          tripId,
          sessionId: "",
          pagination: { pageSize: 100, pageToken: "" },
        });
        if (cancelled) return;
        const loaded: ChatMessage[] = [];
        for (const msg of res.messages) {
          const converted = protoToFrontendMessage(msg);
          if (converted) loaded.push(converted);
        }
        if (loaded.length > 0) {
          // Deduplicate: the backend tool loop can store intermediate
          // and final versions of the same assistant message. Keep only
          // the longest version when two messages share a content prefix.
          const deduped: ChatMessage[] = [];
          for (const msg of loaded) {
            const dupIdx = deduped.findIndex(
              (m) =>
                m.role === msg.role &&
                m.role === "assistant" &&
                (m.content.startsWith(msg.content) || msg.content.startsWith(m.content)),
            );
            if (dupIdx >= 0) {
              // Keep the longer version
              if (msg.content.length > deduped[dupIdx].content.length) {
                deduped[dupIdx] = msg;
              }
            } else {
              deduped.push(msg);
            }
          }
          setMessages((prev) => {
            if (prev.length === 0) return deduped;
            const existingIds = new Set(prev.map((m) => m.id));
            const newFromHistory = deduped.filter((m) => !existingIds.has(m.id));
            return [...newFromHistory, ...prev];
          });
        }
        const nextToken = res.pagination?.nextPageToken ?? "";
        nextPageTokenRef.current = nextToken;
        setHasMoreHistory(nextToken !== "");
        historyLoadedRef.current = tripId;
        setHistoryError(null);
      } catch (error) {
        console.error("Failed to load chat history:", error);
        if (!cancelled) setHistoryError("Failed to load chat history. Please try again.");
      } finally {
        if (!cancelled) setIsLoadingHistory(false);
      }
    };
    void loadHistory();
    return () => { cancelled = true; };
  }, [tripId, transport, historyKey]);

  const loadMoreHistory = useCallback(async () => {
    if (!tripId || !nextPageTokenRef.current || isLoadingMoreRef.current) return;
    isLoadingMoreRef.current = true;
    setIsLoadingMore(true);
    try {
      const client = createClient(ChatService, transport);
      const res = await client.getChatHistory({
        tripId,
        sessionId: "",
        pagination: { pageSize: 100, pageToken: nextPageTokenRef.current },
      });
      const loaded: ChatMessage[] = [];
      for (const msg of res.messages) {
        const converted = protoToFrontendMessage(msg);
        if (converted) loaded.push(converted);
      }
      if (loaded.length > 0) {
        setMessages((prev) => {
          const existingIds = new Set(prev.map((m) => m.id));
          const newFromHistory = loaded.filter((m) => !existingIds.has(m.id));
          return [...newFromHistory, ...prev];
        });
      }
      const nextToken = res.pagination?.nextPageToken ?? "";
      nextPageTokenRef.current = nextToken;
      setHasMoreHistory(nextToken !== "");
    } catch (error) {
      console.error("Failed to load more chat history:", error);
    } finally {
      isLoadingMoreRef.current = false;
      setIsLoadingMore(false);
    }
  }, [tripId, transport]);

  // Reset history state and re-trigger the initial load (used by the retry button).
  const retryHistory = useCallback(() => {
    historyLoadedRef.current = null;
    historyKeyLoadedRef.current = -1;
    nextPageTokenRef.current = "";
    setMessages([]);
    setHistoryError(null);
    setHistoryKey((k) => k + 1);
  }, []);

  const sendMessage = useCallback(
    async (content: string, attachments?: { filename: string; mediaType: string; data: Uint8Array }[]) => {
      if (isSendingRef.current) return;
      isSendingRef.current = true;

      const controller = new AbortController();
      abortControllerRef.current = controller;
      const timeout = setTimeout(() => controller.abort(), 60_000);

      setLastFailedMessage(null);
      const displayContent = attachments?.length
        ? `${content}${content ? "\n" : ""}[${attachments.map((a) => a.filename).join(", ")}]`
        : content;
      const userMsg: ChatMessage = { id: uuid(), role: "user", content: displayContent };
      setMessages((prev) => [...prev, userMsg]);
      setIsStreaming(true);
      setStreamingText("");
      setCreatedTrip(null);
      setSelectedTrip(null);
      streamingMessageIdRef.current = null;

      try {
        const client = createClient(ChatService, transport);
        let fullText = "";

        const protoAttachments = (attachments ?? []).map((a) => ({
          filename: a.filename,
          mediaType: a.mediaType,
          data: a.data,
          sizeBytes: BigInt(a.data.length),
        }));

        for await (const event of client.sendMessage({
          sessionId: sessionIdRef.current,
          tripId: tripId ?? "",
          content,
          mode: modeToProto[mode] ?? ChatMode.SELECTION,
          attachments: protoAttachments,
        }, { signal: controller.signal })) {
          const resp = event;
          switch (resp.event.case) {
            case "textDelta": {
              fullText += resp.event.value.text;
              // First delta: push a placeholder message and record its ID.
              // Subsequent deltas: update it in-place. Content lives in messages[], not streamingText.
              if (!streamingMessageIdRef.current) {
                const newId = uuid();
                streamingMessageIdRef.current = newId;
                const persona = activePersonaRef.current;
                setMessages((prev) => [
                  ...prev,
                  {
                    id: newId,
                    role: "assistant",
                    content: fullText,
                    personaId: persona?.id,
                    personaName: persona?.name,
                    personaAvatar: persona?.avatarUrl,
                    personaAccentColor: persona?.accentColor,
                  },
                ]);
              } else {
                const id = streamingMessageIdRef.current;
                setMessages((prev) =>
                  prev.map((m) => (m.id === id ? { ...m, content: fullText } : m)),
                );
              }
              break;
            }
            case "sessionCreated":
              sessionIdRef.current = resp.event.value.sessionId;
              // Persist so remounts resume this session rather than starting a new one.
              void persistSessionId(tripId, resp.event.value.sessionId);
              break;
            case "toolCall":
              setToolActivity({ toolName: resp.event.value.toolName, status: "calling" });
              break;
            case "toolResult": {
              const toolResult = resp.event.value;
              // Brief "done" state lets the UI animate out before the indicator disappears.
              setToolActivity({ toolName: toolResult.toolName, status: "done" });
              setTimeout(() => setToolActivity(null), 300);
              if (toolResult.toolName === "suggest_expert" && toolResult.resultJson) {
                try {
                  const parsed = JSON.parse(toolResult.resultJson);
                  if (parsed.error === "trip_pro_required") {
                    onExpertLimitReachedRef.current?.();
                  }
                } catch { /* ignore malformed JSON */ }
              }
              if (toolResult.toolName === "recommend_booking" && toolResult.resultJson) {
                try {
                  const parsed = JSON.parse(toolResult.resultJson);
                  const rec = parsed.recommendation;
                  if (rec?.url && rec?.title) {
                    setMessages((prev) => [
                      ...prev,
                      {
                        id: uuid(),
                        role: "assistant",
                        content: "",
                        recommendation: {
                          partner: rec.partner ?? "",
                          category: rec.category ?? "",
                          title: rec.title,
                          description: rec.description ?? "",
                          url: rec.url,
                          price: rec.price,
                          disclosure: rec.disclosure,
                        },
                      },
                    ]);
                  }
                } catch { /* ignore malformed JSON */ }
              }
              break;
            }
            case "tripCreated": {
              const trip = resp.event.value.trip;
              if (trip) {
                setCreatedTrip({ id: trip.id, title: trip.title, description: trip.description });
              }
              break;
            }
            case "tripSelected": {
              const trip = resp.event.value.trip;
              if (trip) {
                setSelectedTrip({ id: trip.id, title: trip.title, description: trip.description });
              }
              break;
            }
            case "messageComplete": {
              // Update the in-progress message rather than appending a duplicate.
              if (resp.event.value.fullContent) fullText = resp.event.value.fullContent;
              setToolActivity(null);
              if (streamingMessageIdRef.current) {
                const id = streamingMessageIdRef.current;
                const persona = activePersonaRef.current;
                setMessages((prev) =>
                  prev.map((m) =>
                    m.id === id
                      ? {
                          ...m,
                          content: fullText,
                          personaId: persona?.id,
                          personaName: persona?.name,
                          personaAvatar: persona?.avatarUrl,
                          personaAccentColor: persona?.accentColor,
                        }
                      : m,
                  ),
                );
                streamingMessageIdRef.current = null;
              } else if (fullText) {
                // Cached / no-delta path: messageComplete arrived with no prior textDelta
                const persona = activePersonaRef.current;
                setMessages((prev) => [
                  ...prev,
                  {
                    id: uuid(),
                    role: "assistant",
                    content: fullText,
                    personaId: persona?.id,
                    personaName: persona?.name,
                    personaAvatar: persona?.avatarUrl,
                    personaAccentColor: persona?.accentColor,
                  },
                ]);
              }
              break;
            }
            case "personaSwitch": {
              const ps = resp.event.value;
              if (ps.newPersona) {
                const newActive = personaToActive(ps.newPersona);
                activePersonaRef.current = newActive;
                setActivePersona(newActive);
              }
              if (ps.handoffMessage) {
                const persona = ps.newPersona;
                setMessages((prev) => [
                  ...prev,
                  {
                    id: uuid(),
                    role: "system",
                    content: ps.handoffMessage,
                    personaIntro: persona
                      ? {
                          name: persona.name,
                          specialties: persona.specialties,
                          accentColor: persona.accentColor,
                          avatarUrl: persona.avatarUrl,
                          handoffMessage: ps.handoffMessage,
                        }
                      : undefined,
                  },
                ]);
              }
              break;
            }
            case "error": {
              const errMsg = resp.event.case === "error" ? resp.event.value.message : "";
              console.error("Stream error:", errMsg);
              setMessages((prev) => [
                ...prev,
                {
                  id: uuid(),
                  role: "assistant",
                  content: errMsg || "Sorry, something went wrong. Please try again.",
                  isError: true,
                },
              ]);
              break;
            }
          }
        }

        // Stream ended without messageComplete (no error): finalize the in-progress message.
        if (streamingMessageIdRef.current && fullText) {
          const id = streamingMessageIdRef.current;
          const persona = activePersonaRef.current;
          setMessages((prev) =>
            prev.map((m) =>
              m.id === id
                ? {
                    ...m,
                    content: fullText,
                    personaId: persona?.id,
                    personaName: persona?.name,
                    personaAvatar: persona?.avatarUrl,
                    personaAccentColor: persona?.accentColor,
                  }
                : m,
            ),
          );
          streamingMessageIdRef.current = null;
        }
      } catch (error) {
        // Remove the partial streaming message so only the error message is shown.
        if (streamingMessageIdRef.current) {
          const id = streamingMessageIdRef.current;
          setMessages((prev) => prev.filter((m) => m.id !== id));
          streamingMessageIdRef.current = null;
        }

        console.error("Chat error:", error);
        const isAbort =
          (error instanceof DOMException && error.name === "AbortError") ||
          (error instanceof Error && error.name === "AbortError");
        if (isAbort) {
          setMessages((prev) => [
            ...prev,
            { id: uuid(), role: "assistant", content: "Stream timed out. Please try again.", isError: true },
          ]);
        } else if (error instanceof ConnectError && error.code === Code.ResourceExhausted) {
          onResourceExhaustedRef.current?.();
          const errMsg = error.message;
          const isDailyLimit = errMsg.includes("daily message limit");
          setMessages((prev) => [
            ...prev,
            {
              id: uuid(),
              role: "assistant",
              content: isDailyLimit
                ? "You\u2019ve used your 30 messages for today. Try again tomorrow! Your messages reset at midnight UTC."
                : "Our AI service has reached its daily capacity \u2014 please try again tomorrow.",
              isError: true,
            },
          ]);
        } else {
          setLastFailedMessage({ content, attachments });
          setMessages((prev) => [
            ...prev,
            { id: uuid(), role: "assistant", content: "Sorry, something went wrong. Please try again.", isError: true },
          ]);
        }
      } finally {
        clearTimeout(timeout);
        abortControllerRef.current = null;
        setStreamingText("");
        setIsStreaming(false);
        setToolActivity(null);
        streamingMessageIdRef.current = null;
        isSendingRef.current = false;
      }
    },
    [tripId, mode, transport],
  );

  const abortStream = useCallback(() => {
    abortControllerRef.current?.abort();
  }, []);

  const clearLastFailedMessage = useCallback(() => {
    setLastFailedMessage(null);
  }, []);

  return {
    messages,
    streamingText,
    isStreaming,
    isLoadingHistory,
    isLoadingMore,
    historyError,
    activePersona,
    toolActivity,
    createdTrip,
    selectedTrip,
    sendMessage,
    abortStream,
    hasMoreHistory,
    loadMoreHistory,
    retryHistory,
    lastFailedMessage,
    clearLastFailedMessage,
  };
}
