"use client";

import { useRouter } from "next/navigation";
import { useCallback, useState } from "react";
import { toast } from "sonner";

import { api, ApiError } from "@/lib/api";
import { EDIT_SESSIONS_CHANGED } from "@/lib/events";

// Mint a new agent-edit chat and navigate into the edit workspace.
export function useStartAgentEdit() {
  const router = useRouter();
  const [startingEditAgentId, setStartingEditAgentId] = useState<string | null>(
    null,
  );

  const startAgentEdit = useCallback(
    async (agentId: string) => {
      if (startingEditAgentId) return;
      setStartingEditAgentId(agentId);
      try {
        const session = await api.createAgentEditSession(agentId);
        window.dispatchEvent(new Event(EDIT_SESSIONS_CHANGED));
        router.push(`/agents/${agentId}/edit/${session.id}`);
      } catch (err) {
        toast.error(
          err instanceof ApiError
            ? err.message
            : "could not create edit chat",
        );
      } finally {
        setStartingEditAgentId(null);
      }
    },
    [router, startingEditAgentId],
  );

  return { startAgentEdit, startingEditAgentId };
}
