"use client";

import Link from "next/link";
import { use, useEffect, useState } from "react";

import { AgentEditChatView } from "@/components/agents/agent-edit-chat-view";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { historyToDisplay, type DisplayMessage } from "@/hooks/use-run";
import { api } from "@/lib/api";
import type { Agent, AgentVersion, Session } from "@/lib/types";

type LoadState =
  | { phase: "loading" }
  | { phase: "error"; message: string }
  | {
      phase: "ready";
      session: Session;
      agent: Agent;
      versions: AgentVersion[];
      initialMessages: DisplayMessage[];
    };

export default function AgentEditPage({
  params,
}: {
  params: Promise<{ agentId: string; sessionId: string }>;
}) {
  const { agentId, sessionId } = use(params);
  const [loaded, setLoaded] = useState<{ sid: string; state: LoadState }>({
    sid: sessionId,
    state: { phase: "loading" },
  });
  if (loaded.sid !== sessionId) {
    setLoaded({ sid: sessionId, state: { phase: "loading" } });
  }
  const state =
    loaded.sid === sessionId ? loaded.state : { phase: "loading" as const };

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [session, agent, versions] = await Promise.all([
          api.getSession(sessionId),
          api.getAgent(agentId),
          api.listAgentVersions(agentId),
        ]);
        if (session.kind !== "agent_edit" || session.agent_id !== agentId) {
          throw new Error("This is not an edit chat for this agent");
        }
        if (cancelled) return;
        setLoaded({
          sid: sessionId,
          state: {
            phase: "ready",
            session,
            agent,
            versions,
            initialMessages: historyToDisplay(session.messages),
          },
        });
      } catch (err) {
        if (cancelled) return;
        setLoaded({
          sid: sessionId,
          state: {
            phase: "error",
            message:
              err instanceof Error
                ? err.message
                : "could not load this edit chat",
          },
        });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [agentId, sessionId]);

  if (state.phase === "loading") {
    return (
      <main
        aria-label="Loading edit chat"
        className="flex flex-1 overflow-hidden"
      >
        <section className="flex min-w-0 flex-1 flex-col overflow-hidden">
          <div className="flex h-14 shrink-0 items-center gap-3 border-b px-4">
            <Skeleton className="size-7 rounded-full" />
            <Skeleton className="h-4 w-36" />
          </div>
          <div className="flex-1 overflow-hidden">
            <div className="mx-auto w-full max-w-2xl space-y-6 px-4 py-6">
              <Skeleton className="ml-auto h-10 w-52 rounded-2xl" />
              <Skeleton className="h-20 w-full" />
              <Skeleton className="ml-auto h-10 w-64 rounded-2xl" />
            </div>
          </div>
          <div className="shrink-0 border-t">
            <div className="mx-auto w-full max-w-2xl px-4 py-3">
              <Skeleton className="h-[58px] w-full rounded-xl" />
            </div>
          </div>
        </section>
        <aside className="hidden w-80 shrink-0 border-l p-4 xl:block">
          <Skeleton className="h-80 rounded-lg" />
        </aside>
      </main>
    );
  }

  if (state.phase === "error") {
    return (
      <main className="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
        <p className="text-sm font-medium">Couldn&apos;t open this edit chat</p>
        <p className="max-w-sm text-sm text-muted-foreground">
          {state.message}
        </p>
        <Button variant="outline" size="sm" asChild>
          <Link href="/">Back to agents</Link>
        </Button>
      </main>
    );
  }

  return (
    <AgentEditChatView
      key={sessionId}
      session={state.session}
      agent={state.agent}
      versions={state.versions}
      initialMessages={state.initialMessages}
    />
  );
}
