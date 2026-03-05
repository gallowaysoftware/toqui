"use client";

import { useState, useRef, useEffect } from "react";
import { MessageBubble } from "./MessageBubble";
import { ChatInput } from "./ChatInput";
import { TypingIndicator } from "./TypingIndicator";
import { useChat } from "@/lib/hooks/useChat";
import type { ActivePersona, CreatedTrip, SelectedTrip } from "@/lib/hooks/useChat";

interface ChatContainerProps {
  tripId?: string;
  mode: "planning" | "companion" | "selection";
  onTripCreated?: (trip: CreatedTrip) => void;
  onTripSelected?: (trip: SelectedTrip) => void;
}

export function ChatContainer({ tripId, mode, onTripCreated, onTripSelected }: ChatContainerProps) {
  const { messages, streamingText, isStreaming, activePersona, toolActivity, createdTrip, selectedTrip, sendMessage } = useChat(tripId, mode);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  useEffect(() => {
    if (autoScroll) {
      messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [messages, streamingText, autoScroll]);

  // Notify parent when a trip is created via chat
  useEffect(() => {
    if (createdTrip && onTripCreated) {
      onTripCreated(createdTrip);
    }
  }, [createdTrip, onTripCreated]);

  // Notify parent when a trip is selected via chat
  useEffect(() => {
    if (selectedTrip && onTripSelected) {
      onTripSelected(selectedTrip);
    }
  }, [selectedTrip, onTripSelected]);

  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const el = e.currentTarget;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 100;
    setAutoScroll(atBottom);
  };

  const emptyPrompt = mode === "selection"
    ? "Hey! Where are we headed? Tell me about your next trip idea."
    : mode === "companion"
      ? "I'm here to help while you travel. What do you need?"
      : "Let's plan your trip. What would you like to figure out?";

  return (
    <div className="flex-1 flex flex-col min-h-0">
      {activePersona && <PersonaBar persona={activePersona} />}

      <div className="flex-1 overflow-y-auto p-4 space-y-4" onScroll={handleScroll}>
        {messages.length === 0 && !isStreaming && (
          <div className="text-center text-gray-400 py-16">
            <p className="text-lg mb-2">{emptyPrompt}</p>
          </div>
        )}

        {messages.map((msg, i) => {
          const prevMsg = messages[i - 1];
          const showBadge = msg.role === "assistant" && (
            !prevMsg ||
            prevMsg.role !== "assistant" ||
            prevMsg.personaId !== msg.personaId
          );

          return (
            <MessageBubble
              key={msg.id}
              message={msg}
              showPersonaBadge={showBadge}
            />
          );
        })}

        {isStreaming && toolActivity && (
          <ToolActivityIndicator toolName={toolActivity.toolName} status={toolActivity.status} />
        )}
        {isStreaming && !streamingText && !toolActivity && <TypingIndicator />}
        {streamingText && (
          <MessageBubble
            message={{
              id: "streaming",
              role: "assistant",
              content: streamingText,
              personaId: activePersona?.id,
              personaName: activePersona?.name,
              personaAvatar: activePersona?.avatarUrl,
              personaAccentColor: activePersona?.accentColor,
            }}
            isStreaming
            showPersonaBadge={false}
          />
        )}

        <div ref={messagesEndRef} />
      </div>

      <ChatInput onSend={sendMessage} disabled={isStreaming} />
    </div>
  );
}

const toolDisplayNames: Record<string, string> = {
  places_search: "Searching places",
  web_search: "Searching the web",
  create_trip: "Creating trip",
  select_trip: "Finding trip",
};

function ToolActivityIndicator({ toolName, status }: { toolName: string; status: "calling" | "done" }) {
  const label = toolDisplayNames[toolName] || `Using ${toolName}`;
  return (
    <div className="flex items-center gap-2 px-4 py-2 text-sm text-gray-500">
      {status === "calling" ? (
        <div className="animate-spin h-3 w-3 border border-gray-400 border-t-transparent rounded-full" />
      ) : (
        <svg className="h-3 w-3 text-green-500" viewBox="0 0 12 12" fill="currentColor">
          <path d="M10.28 2.28a.75.75 0 00-1.06-1.06L4.5 5.94 2.78 4.22a.75.75 0 00-1.06 1.06l2.25 2.25a.75.75 0 001.06 0l5.25-5.25z" />
        </svg>
      )}
      <span>{label}{status === "calling" ? "..." : ""}</span>
    </div>
  );
}

function PersonaBar({ persona }: { persona: ActivePersona }) {
  return (
    <div
      className="flex items-center gap-2 px-4 py-2 bg-white border-b border-gray-200 flex-shrink-0"
    >
      <div
        className="w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold text-white"
        style={{ backgroundColor: persona.accentColor || "#6b7280" }}
      >
        {persona.name[0]}
      </div>
      <span className="text-sm font-medium text-gray-700">{persona.name}</span>
      {persona.specialties.length > 0 && (
        <span className="text-xs text-gray-400">
          {persona.specialties.slice(0, 3).join(" · ")}
        </span>
      )}
    </div>
  );
}
