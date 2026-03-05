import Markdown from "react-markdown";
import type { ChatMessage } from "@/lib/hooks/useChat";

interface MessageBubbleProps {
  message: ChatMessage;
  isStreaming?: boolean;
  showPersonaBadge?: boolean;
}

export function MessageBubble({ message, isStreaming, showPersonaBadge }: MessageBubbleProps) {
  const { role, content } = message;

  if (role === "system") {
    return (
      <div className="flex justify-center">
        <p className="text-xs text-gray-500 bg-gray-100 rounded-full px-4 py-1.5 max-w-[80%] text-center">
          {content}
        </p>
      </div>
    );
  }

  const isUser = role === "user";
  const accentColor = message.personaAccentColor;

  return (
    <div className={`flex ${isUser ? "justify-end" : "justify-start"}`}>
      <div className="max-w-[80%]">
        {!isUser && showPersonaBadge && message.personaName && (
          <div className="flex items-center gap-1.5 mb-1 ml-1">
            {message.personaAvatar && (
              <div
                className="w-5 h-5 rounded-full bg-gray-200 flex items-center justify-center text-[10px] font-bold text-white"
                style={accentColor ? { backgroundColor: accentColor } : undefined}
              >
                {message.personaName[0]}
              </div>
            )}
            <span className="text-xs font-medium text-gray-500">{message.personaName}</span>
          </div>
        )}
        <div
          className={`rounded-2xl px-4 py-3 ${
            isUser
              ? "bg-blue-600 text-white rounded-br-sm"
              : "bg-white border border-gray-200 text-gray-800 rounded-bl-sm"
          }`}
          style={!isUser && accentColor ? { borderLeftColor: accentColor, borderLeftWidth: 3 } : undefined}
        >
          {isUser ? (
            <p className="whitespace-pre-wrap text-sm leading-relaxed">{content}</p>
          ) : (
            <div className="text-sm leading-relaxed prose prose-sm prose-gray max-w-none prose-p:my-1.5 prose-ul:my-1.5 prose-ol:my-1.5 prose-li:my-0.5 prose-headings:my-2 prose-pre:my-2 prose-a:text-blue-600">
              <Markdown>{content}</Markdown>
              {isStreaming && <span className="inline-block w-1.5 h-4 bg-gray-400 ml-0.5 animate-pulse align-text-bottom" />}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
