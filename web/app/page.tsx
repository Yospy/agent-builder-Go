"use client";

import { useEffect, useState } from "react";
import { motion } from "motion/react";
import { BotIcon, CircleDashedIcon, RefreshCwIcon } from "lucide-react";
import { toast } from "sonner";

import { AgentCard } from "@/components/agents/agent-card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useStartAgentEdit } from "@/hooks/use-start-agent-edit";
import { useStartChat } from "@/hooks/use-start-chat";
import { api, ApiError } from "@/lib/api";
import { AGENTS_CHANGED } from "@/lib/events";
import { removeRecentChatsForAgent } from "@/lib/recent-chats";
import { BUILDER_AGENT_ID, type Agent } from "@/lib/types";

type LoadState =
  | { phase: "loading" }
  | { phase: "error"; message: string }
  | { phase: "ready"; agents: Agent[] };

export default function AgentsPage() {
  const [state, setState] = useState<LoadState>({ phase: "loading" });
  const [attempt, setAttempt] = useState(0);
  const { startChat, startingAgentId } = useStartChat();
  const { startAgentEdit, startingEditAgentId } = useStartAgentEdit();

  useEffect(() => {
    let cancelled = false;
    api
      .listAgents()
      .then((agents) => {
        if (!cancelled) setState({ phase: "ready", agents });
      })
      .catch((err) => {
        if (!cancelled)
          setState({
            phase: "error",
            message:
              err instanceof Error ? err.message : "could not load agents",
          });
      });
    return () => {
      cancelled = true;
    };
  }, [attempt]);

  const retry = () => {
    setState({ phase: "loading" });
    setAttempt((n) => n + 1);
  };

  const removeAgentLocally = (agentId: string) => {
    setState((current) =>
      current.phase === "ready"
        ? {
            phase: "ready",
            agents: current.agents.filter((agent) => agent.id !== agentId),
          }
        : current,
    );
    removeRecentChatsForAgent(agentId);
    window.dispatchEvent(new Event(AGENTS_CHANGED));
  };

  const deleteAgent = async (agent: Agent): Promise<boolean> => {
    if (agent.id === BUILDER_AGENT_ID) return false;
    try {
      await api.deleteAgent(agent.id);
      removeAgentLocally(agent.id);
      return true;
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        removeAgentLocally(agent.id);
        return true;
      }
      if (err instanceof ApiError && err.status === 409) {
        toast("Run in flight. Retry after it finishes.");
        return false;
      }
      if (err instanceof ApiError && err.status === 405) {
        toast.error(
          "Delete is not available on the running backend. Restart the Go server.",
        );
        return false;
      }
      toast.error(err instanceof Error ? err.message : "could not delete agent");
      return false;
    }
  };

  const countLabel =
    state.phase === "ready"
      ? `${state.agents.filter((agent) => agent.id !== BUILDER_AGENT_ID).length} built`
      : "Local registry";
  const agentsForRegistry =
    state.phase === "ready"
      ? state.agents.filter((agent) => agent.id !== BUILDER_AGENT_ID)
      : [];
  const builtCount = agentsForRegistry.length;

  return (
    <main className="flex flex-1 flex-col overflow-hidden bg-background">
      <header className="flex min-h-14 shrink-0 items-center border-b bg-background px-5">
        <div className="min-w-0">
          <div className="flex min-w-0 items-baseline gap-2">
            <h1 className="truncate text-sm font-semibold">Agent registry</h1>
            <span className="hidden text-xs text-muted-foreground sm:inline">
              {countLabel}
            </span>
          </div>
          <p className="truncate text-xs leading-5 text-muted-foreground">
            User-created agents appear here.
          </p>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-6xl px-4 py-5 md:px-6">
          {state.phase === "loading" && (
            <div className="grid auto-rows-fr grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
              <Skeleton className="h-64 rounded-lg" />
              <Skeleton className="h-64 rounded-lg" />
              <Skeleton className="h-64 rounded-lg" />
            </div>
          )}

          {state.phase === "error" && (
            <div className="flex min-h-[320px] flex-col items-center justify-center gap-3 rounded-lg border border-dashed px-6 text-center">
              <CircleDashedIcon className="size-8 text-muted-foreground" />
              <p className="text-sm font-medium">Can&apos;t reach the backend</p>
              <p className="max-w-sm text-sm text-muted-foreground">
                {state.message}. Make sure the Go service is running on port
                8080, then retry.
              </p>
              <Button variant="outline" size="sm" onClick={retry}>
                <RefreshCwIcon className="size-4" />
                Retry
              </Button>
            </div>
          )}

          {state.phase === "ready" && agentsForRegistry.length === 0 && (
            <div className="flex min-h-[320px] flex-col items-center justify-center gap-3 rounded-lg border border-dashed px-6 text-center">
              <BotIcon className="size-8 text-muted-foreground" />
              <p className="text-sm font-medium">No agents yet</p>
              <p className="max-w-sm text-sm text-muted-foreground">
                Use the sidebar action to create one.
              </p>
            </div>
          )}

          {state.phase === "ready" && agentsForRegistry.length > 0 && (
            <div className="space-y-4">
              <div className="flex min-h-12 items-center justify-between gap-3 border-b pb-3">
                <div className="min-w-0">
                  <h2 className="truncate text-sm font-semibold">
                    Your agents
                  </h2>
                  <p className="text-xs text-muted-foreground">
                    {builtCount} built locally
                  </p>
                </div>
              </div>

              <motion.div
                className="grid auto-rows-fr grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3"
                initial="hidden"
                animate="show"
                variants={{
                  hidden: {},
                  show: { transition: { staggerChildren: 0.04 } },
                }}
              >
                {agentsForRegistry.map((agent) => (
                  <motion.div
                    key={agent.id}
                    className="h-full"
                    variants={{
                      hidden: { opacity: 0, y: 8 },
                      show: {
                        opacity: 1,
                        y: 0,
                        transition: { duration: 0.25, ease: "easeOut" },
                      },
                    }}
                  >
                    <AgentCard
                      agent={agent}
                      onStartChat={startChat}
                      onStartEdit={startAgentEdit}
                      onDelete={deleteAgent}
                      starting={startingAgentId === agent.id}
                      startingEdit={startingEditAgentId === agent.id}
                    />
                  </motion.div>
                ))}
              </motion.div>
            </div>
          )}
        </div>
      </div>
    </main>
  );
}
