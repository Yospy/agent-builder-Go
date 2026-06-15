"use client";

import { useRouter } from "next/navigation";
import { useCallback, useState } from "react";
import { toast } from "sonner";

import { api, ApiError } from "@/lib/api";
import { upsertRecentChat } from "@/lib/recent-chats";

interface StartChatOptions {
  addToRecentChats?: boolean;
}

// Mint a session for an agent and navigate into the chat. Used by the
// agents list, the sidebar, and the "+ New Agent" CTA (agent_id "builder").
export function useStartChat() {
  const router = useRouter();
  const [startingAgentId, setStartingAgentId] = useState<string | null>(null);

  const startChat = useCallback(
    async (
      agentId: string,
      agentName: string,
      options: StartChatOptions = {},
    ) => {
      if (startingAgentId) return;
      setStartingAgentId(agentId);
      try {
        const { session_id } = await api.createSession(agentId);
        if (options.addToRecentChats !== false) {
          upsertRecentChat({
            sessionId: session_id,
            agentId,
            agentName,
            title: "New chat",
            updatedAt: Date.now(),
          });
        }
        router.push(`/chat/${session_id}`);
        setStartingAgentId(null);
      } catch (err) {
        toast.error(
          err instanceof ApiError
            ? err.message
            : "could not reach the backend on :8080",
        );
        setStartingAgentId(null);
      }
    },
    [router, startingAgentId],
  );

  return { startChat, startingAgentId };
}
