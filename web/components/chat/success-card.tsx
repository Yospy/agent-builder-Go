"use client";

import { useEffect, useState } from "react";
import { motion } from "motion/react";
import { ArrowRightIcon, CheckIcon } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { useStartChat } from "@/hooks/use-start-chat";
import { api } from "@/lib/api";
import { AGENTS_CHANGED } from "@/lib/events";

// The celebration moment: a create_agent tool_result came back ok. Bridges
// build → use in one click by minting a session with the new agent.
export function SuccessCard({
  agentName,
  resultData,
  disabled = false,
}: {
  agentName: string;
  resultData?: string;
  // Held while the turn is still streaming: clicking would navigate away,
  // abort the run (§9), and drop the builder turn from history.
  disabled?: boolean;
}) {
  const { startChat, startingAgentId } = useStartChat();
  const [resolving, setResolving] = useState(false);

  // The INSERT just happened — make it visible in the sidebar immediately.
  useEffect(() => {
    window.dispatchEvent(new Event(AGENTS_CHANGED));
  }, []);

  const handleStart = async () => {
    if (resolving || startingAgentId) return;
    setResolving(true);
    try {
      const agentId = await resolveAgentId(agentName, resultData);
      if (!agentId) {
        toast.error(`Couldn't find "${agentName}" — try the agents list`);
        return;
      }
      await startChat(agentId, agentName);
    } finally {
      setResolving(false);
    }
  };

  return (
    <motion.div
      initial={{ opacity: 0, scale: 0.9, y: 8 }}
      animate={{ opacity: 1, scale: 1, y: 0 }}
      transition={{ type: "spring", stiffness: 350, damping: 22 }}
      className="flex items-center justify-between gap-3 rounded-xl border bg-foreground p-4 text-background"
    >
      <div className="flex min-w-0 items-center gap-2.5">
        <CheckIcon className="size-4 shrink-0" />
        <p className="truncate text-sm font-medium">{agentName} is live</p>
      </div>
      <Button
        size="sm"
        variant="secondary"
        disabled={disabled || resolving || startingAgentId !== null}
        onClick={handleStart}
      >
        Start chatting
        <ArrowRightIcon className="size-4" />
      </Button>
    </motion.div>
  );
}

// The contract doesn't pin the shape of create_agent's tool_result.data, so
// resolve the new agent's id defensively: parse an id out of the result if
// present, else refresh the list and match by name (newest first).
async function resolveAgentId(
  agentName: string,
  resultData?: string,
): Promise<string | null> {
  if (resultData) {
    try {
      const parsed = JSON.parse(resultData) as Record<string, unknown>;
      const candidate =
        parsed.id ??
        parsed.agent_id ??
        (parsed.agent as Record<string, unknown> | undefined)?.id;
      if (typeof candidate === "string" && candidate) return candidate;
    } catch {
      // data is plain text — fall through to the patterns below
    }
    // Observed backend shape: `created agent agent-<hex> ("Name") with …`
    const match = resultData.match(/\bagent-[A-Za-z0-9]+\b/);
    if (match) return match[0];
  }
  try {
    const agents = await api.listAgents();
    const matches = agents
      .filter((a) => a.name.toLowerCase() === agentName.toLowerCase())
      .sort((a, b) => b.created_at - a.created_at);
    return matches[0]?.id ?? null;
  } catch {
    return null;
  }
}
