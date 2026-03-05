"use client";

import { ChatContainer } from "@/components/chat/ChatContainer";

export default function CompanionPage() {
  return (
    <div className="h-screen flex flex-col">
      <header className="bg-white border-b border-gray-200 px-4 py-3 flex-shrink-0">
        <h1 className="text-lg font-semibold">Travel Companion</h1>
      </header>
      <ChatContainer tripId="" mode="companion" />
    </div>
  );
}
