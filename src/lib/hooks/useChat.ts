"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import { createClient, Code, ConnectError } from "@connectrpc/connect";
import { useTransport } from "@/components/providers/GrpcProvider";
import { ChatService, ChatMode } from "@/gen/toqui/v1/chat_pb";
import type { ChatMessage as ProtoChatMessage } from "@/gen/toqui/v1/chat_pb";
import type { Persona } from "@/gen/toqui/v1/persona_pb";

import type { Recommendation } from "@/components/chat/RecommendationCard";

/**
 * Generate a UUID v4, with fallback for non-secure contexts (HTTP).
 * uuid() requires a secure context (HTTPS or localhost).
 * crypto.getRandomValues() works everywhere.
 */
function uuid(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  // Fallback: build a v4 UUID from crypto.getRandomValues
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6] & 0x0f) | 0x40; // version 4
  bytes[8] = (bytes[8] & 0x3f) | 0x80; // variant 1
  const h = [...bytes].map((b) => b.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
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

interface UseChatOptions {
  onResourceExhausted?: () => void;
}

/**
 * Convert a backend ChatMessage (proto) to the frontend ChatMessage shape.
 * Persona metadata is stored in the proto `metadata` map with keys like
 * "persona_id", "persona_name", "persona_avatar", "persona_accent_color".
 */
function protoToFrontendMessage(msg: ProtoChatMessage): ChatMessage | null {
  const role = msg.role as ChatMessage["role"];
  if (role !== "user" && role !== "assistant" && role !== "system") {
    return null;
  }
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

export function useChat(
  tripId: string | undefined,
  mode: "planning" | "companion" | "selection",
  options?: UseChatOptions,
) {
  const transport = useTransport();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [streamingText, setStreamingText] = useState<string>("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [isLoadingHistory, setIsLoadingHistory] = useState(false);
  const [activePersona, setActivePersona] = useState<ActivePersona | null>(null);
  const [toolActivity, setToolActivity] = useState<ToolActivity | null>(null);
  const [createdTrip, setCreatedTrip] = useState<CreatedTrip | null>(null);
  const [selectedTrip, setSelectedTrip] = useState<SelectedTrip | null>(null);
  const [hasMoreHistory, setHasMoreHistory] = useState(false);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const sessionIdRef = useRef<string>("");
  const activePersonaRef = useRef<ActivePersona | null>(null);
  const historyLoadedRef = useRef<string | null>(null);
  const nextPageTokenRef = useRef<string>("");
  const onResourceExhaustedRef = useRef(options?.onResourceExhausted);
  useEffect(() => {
    onResourceExhaustedRef.current = options?.onResourceExhausted;
  }, [options?.onResourceExhausted]);

  // Load chat history from the backend when a tripId is provided
  useEffect(() => {
    if (!tripId) return;
    // Avoid re-fetching if we already loaded history for this trip
    if (historyLoadedRef.current === tripId) return;

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
          if (converted) {
            loaded.push(converted);
          }
        }

        if (loaded.length > 0) {
          // Use functional updater to merge with any messages sent during load
          setMessages((prev) => {
            if (prev.length === 0) return loaded;
            const existingIds = new Set(prev.map((m) => m.id));
            const newFromHistory = loaded.filter(
              (m) => !existingIds.has(m.id),
            );
            return [...newFromHistory, ...prev];
          });
        }

        // Track pagination for "load more" support
        const nextToken = res.pagination?.nextPageToken ?? "";
        nextPageTokenRef.current = nextToken;
        setHasMoreHistory(nextToken !== "");
        historyLoadedRef.current = tripId;
      } catch (error) {
        // History loading is best-effort; log but don't block chat
        console.error("Failed to load chat history:", error);
      } finally {
        if (!cancelled) {
          setIsLoadingHistory(false);
        }
      }
    };

    void loadHistory();
    return () => {
      cancelled = true;
    };
  }, [tripId, transport]);

  const loadMoreHistory = useCallback(async () => {
    if (!tripId || !nextPageTokenRef.current || isLoadingMore) return;
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
        if (converted) {
          loaded.push(converted);
        }
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
      setIsLoadingMore(false);
    }
  }, [tripId, transport, isLoadingMore]);

  const sendMessage = useCallback(
    async (content: string) => {
      const userMsg: ChatMessage = {
        id: uuid(),
        role: "user",
        content,
      };
      setMessages((prev) => [...prev, userMsg]);
      setIsStreaming(true);
      setStreamingText("");
      setCreatedTrip(null);
      setSelectedTrip(null);

      try {
        const client = createClient(ChatService, transport);
        let fullText = "";

        for await (const event of client.sendMessage({
          sessionId: sessionIdRef.current,
          tripId: tripId ?? "",
          content,
          mode: modeToProto[mode] ?? ChatMode.SELECTION,
        })) {
          const resp = event;
          switch (resp.event.case) {
            case "textDelta":
              fullText += resp.event.value.text;
              setStreamingText(fullText);
              break;

            case "sessionCreated":
              sessionIdRef.current = resp.event.value.sessionId;
              break;

            case "toolCall":
              setToolActivity({
                toolName: resp.event.value.toolName,
                status: "calling",
              });
              break;

            case "toolResult": {
              const toolResult = resp.event.value;
              setToolActivity({
                toolName: toolResult.toolName,
                status: "done",
              });

              // Parse recommend_booking results into recommendation cards
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
                } catch {
                  // Ignore malformed recommendation JSON
                }
              }
              break;
            }

            case "tripCreated": {
              const trip = resp.event.value.trip;
              if (trip) {
                setCreatedTrip({
                  id: trip.id,
                  title: trip.title,
                  description: trip.description,
                });
              }
              break;
            }

            case "tripSelected": {
              const trip = resp.event.value.trip;
              if (trip) {
                setSelectedTrip({
                  id: trip.id,
                  title: trip.title,
                  description: trip.description,
                });
              }
              break;
            }

            case "messageComplete":
              if (resp.event.value.fullContent) {
                fullText = resp.event.value.fullContent;
              }
              setToolActivity(null);
              break;

            case "personaSwitch": {
              const ps = resp.event.value;
              if (ps.newPersona) {
                const newActive = personaToActive(ps.newPersona);
                activePersonaRef.current = newActive;
                setActivePersona(newActive);
              }
              if (ps.handoffMessage) {
                setMessages((prev) => [
                  ...prev,
                  {
                    id: uuid(),
                    role: "system",
                    content: ps.handoffMessage,
                  },
                ]);
              }
              break;
            }

            case "error":
              console.error("Stream error:", resp.event.value.message);
              break;
          }
        }

        if (fullText) {
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
      } catch (error) {
        console.error("Chat error:", error);

        if (error instanceof ConnectError && error.code === Code.ResourceExhausted) {
          onResourceExhaustedRef.current?.();
          setMessages((prev) => [
            ...prev,
            {
              id: uuid(),
              role: "assistant",
              content:
                "You\u2019ve reached your daily message limit. Upgrade to Trip Pro for unlimited messages.",
              isError: true,
            },
          ]);
        } else {
          setMessages((prev) => [
            ...prev,
            {
              id: uuid(),
              role: "assistant",
              content: "Sorry, something went wrong. Please try again.",
              isError: true,
            },
          ]);
        }
      } finally {
        setStreamingText("");
        setIsStreaming(false);
        setToolActivity(null);
      }
    },
    [tripId, mode, transport],
  );

  return {
    messages,
    streamingText,
    isStreaming,
    isLoadingHistory,
    isLoadingMore,
    activePersona,
    toolActivity,
    createdTrip,
    selectedTrip,
    sendMessage,
    hasMoreHistory,
    loadMoreHistory,
  };
}
