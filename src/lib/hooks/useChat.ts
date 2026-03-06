"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import { createClient, Code, ConnectError } from "@connectrpc/connect";
import { useTransport } from "@/components/providers/GrpcProvider";
import { ChatService, ChatMode } from "@/gen/toqui/v1/chat_pb";
import type { SendMessageResponse } from "@/gen/toqui/v1/chat_pb";
import type { Persona } from "@/gen/toqui/v1/persona_pb";

export interface ChatMessage {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
  personaId?: string;
  personaName?: string;
  personaAvatar?: string;
  personaAccentColor?: string;
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

export function useChat(tripId: string | undefined, mode: "planning" | "companion" | "selection", options?: UseChatOptions) {
  const transport = useTransport();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [streamingText, setStreamingText] = useState<string>("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [activePersona, setActivePersona] = useState<ActivePersona | null>(null);
  const [toolActivity, setToolActivity] = useState<ToolActivity | null>(null);
  const [createdTrip, setCreatedTrip] = useState<CreatedTrip | null>(null);
  const [selectedTrip, setSelectedTrip] = useState<SelectedTrip | null>(null);
  const sessionIdRef = useRef<string>("");
  const activePersonaRef = useRef<ActivePersona | null>(null);
  const onResourceExhaustedRef = useRef(options?.onResourceExhausted);
  useEffect(() => {
    onResourceExhaustedRef.current = options?.onResourceExhausted;
  }, [options?.onResourceExhausted]);

  const sendMessage = useCallback(
    async (content: string) => {
      const userMsg: ChatMessage = {
        id: crypto.randomUUID(),
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
          const resp = event as SendMessageResponse;
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

            case "toolResult":
              setToolActivity({
                toolName: resp.event.value.toolName,
                status: "done",
              });
              break;

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
                    id: crypto.randomUUID(),
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
              id: crypto.randomUUID(),
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
              id: crypto.randomUUID(),
              role: "assistant",
              content: "You\u2019ve reached your daily message limit. Upgrade to Trip Pro for unlimited messages.",
            },
          ]);
        } else {
          setMessages((prev) => [
            ...prev,
            {
              id: crypto.randomUUID(),
              role: "assistant",
              content: "Sorry, something went wrong. Please try again.",
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

  return { messages, streamingText, isStreaming, activePersona, toolActivity, createdTrip, selectedTrip, sendMessage };
}
