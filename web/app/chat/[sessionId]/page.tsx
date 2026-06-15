"use client";

import Link from "next/link";
import { use, useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import { ChatView } from "@/components/chat/chat-view";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { historyToDisplay, type DisplayMessage } from "@/hooks/use-run";
import { api } from "@/lib/api";
import type { Agent } from "@/lib/types";

type LoadState =
  | { phase: "loading" }
  | { phase: "error"; message: string }
  | { phase: "ready"; agent: Agent; initialMessages: DisplayMessage[] };

export default function ChatPage({
  params,
}: {
  params: Promise<{ sessionId: string }>;
}) {
  const { sessionId } = use(params);
  const router = useRouter();
  // State is keyed by sessionId; navigating between chats resets it during
  // render (React's "adjusting state during render" pattern) so the effect
  // never needs a synchronous setState.
  const [loaded, setLoaded] = useState<{ sid: string; state: LoadState }>({
    sid: sessionId,
    state: { phase: "loading" },
  });
  if (loaded.sid !== sessionId) {
    setLoaded({ sid: sessionId, state: { phase: "loading" } });
  }
  const state = loaded.sid === sessionId ? loaded.state : { phase: "loading" as const };

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const session = await api.getSession(sessionId);
        if (session.kind === "agent_edit") {
          router.replace(`/agents/${session.agent_id}/edit/${session.id}`);
          return;
        }
        const agent = await api.getAgent(session.agent_id);
        if (cancelled) return;
        setLoaded({
          sid: sessionId,
          state: {
            phase: "ready",
            agent,
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
              err instanceof Error ? err.message : "could not load this chat",
          },
        });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [router, sessionId]);

  if (state.phase === "loading") {
    // Skeleton (not a spinner): holds the chat's shape so ready-state
    // content lands without layout shift.
    return (
      <main
        aria-label="Loading chat"
        className="flex flex-1 flex-col overflow-hidden"
      >
        <div className="flex h-14 shrink-0 items-center gap-3 border-b px-4">
          <Skeleton className="size-7 rounded-full" />
          <Skeleton className="h-4 w-36" />
        </div>
        <div className="flex-1 overflow-hidden">
          <div className="mx-auto w-full max-w-2xl space-y-6 px-4 py-6">
            <div className="flex justify-end">
              <Skeleton className="h-10 w-52 rounded-2xl" />
            </div>
            <Skeleton className="h-20 w-full" />
            <div className="flex justify-end">
              <Skeleton className="h-10 w-64 rounded-2xl" />
            </div>
            <Skeleton className="h-14 w-3/4" />
          </div>
        </div>
        <div className="shrink-0 border-t">
          <div className="mx-auto w-full max-w-2xl px-4 py-3">
            <Skeleton className="h-[58px] w-full rounded-xl" />
          </div>
        </div>
      </main>
    );
  }

  if (state.phase === "error") {
    return (
      <main className="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
        <p className="text-sm font-medium">Couldn&apos;t open this chat</p>
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
    <ChatView
      key={sessionId}
      sessionId={sessionId}
      agent={state.agent}
      initialMessages={state.initialMessages}
    />
  );
}
