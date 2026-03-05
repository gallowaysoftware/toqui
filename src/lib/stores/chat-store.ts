import { create } from "zustand";

interface ChatMessage {
  id: string;
  role: string;
  content: string;
  createdAt: Date;
}

interface ChatState {
  messages: ChatMessage[];
  streamingText: string;
  isStreaming: boolean;
  sessionId: string | null;
  addMessage: (msg: ChatMessage) => void;
  setStreamingText: (text: string) => void;
  setIsStreaming: (streaming: boolean) => void;
  setSessionId: (id: string) => void;
  clearMessages: () => void;
}

export const useChatStore = create<ChatState>((set) => ({
  messages: [],
  streamingText: "",
  isStreaming: false,
  sessionId: null,
  addMessage: (msg) => set((state) => ({ messages: [...state.messages, msg] })),
  setStreamingText: (text) => set({ streamingText: text }),
  setIsStreaming: (streaming) => set({ isStreaming: streaming }),
  setSessionId: (id) => set({ sessionId: id }),
  clearMessages: () => set({ messages: [], streamingText: "", sessionId: null }),
}));
