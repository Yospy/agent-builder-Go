"use client";

import { useEffect, useRef } from "react";
import { AnimatePresence, motion } from "motion/react";

import { Message } from "@/components/chat/message";
import { AgentAvatar } from "@/components/agents/agent-avatar";
import { Badge } from "@/components/ui/badge";
import type { DisplayMessage } from "@/hooks/use-run";
import { toolInfo } from "@/lib/tool-catalog";
import {
  BUILDER_AGENT_ID,
  type Agent,
  type AnswerPayload,
  type ApproveDecision,
} from "@/lib/types";

export function MessageList({
  messages,
  agent,
  onDecision,
  onAnswer,
}: {
  messages: DisplayMessage[];
  agent: Agent;
  onDecision: (callId: string, decision: ApproveDecision) => Promise<boolean>;
  onAnswer: (callId: string, answer: AnswerPayload) => Promise<boolean>;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const pinnedRef = useRef(true);

  // Pinned-to-bottom autoscroll: follow the stream until the user scrolls
  // up to read, then stop fighting them; re-pin when they return to bottom.
  useEffect(() => {
    const el = containerRef.current;
    if (el && pinnedRef.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [messages]);

  const handleScroll = () => {
    const el = containerRef.current;
    if (!el) return;
    pinnedRef.current =
      el.scrollHeight - el.scrollTop - el.clientHeight < 40;
  };

  return (
    <div
      ref={containerRef}
      onScroll={handleScroll}
      className="flex-1 overflow-y-auto"
    >
      <div className="mx-auto w-full max-w-2xl space-y-6 px-4 py-6">
        {/* AnimatePresence stays mounted even when a rollback empties the
            chat, so the exit fade fires before the empty state returns.
            Quiet exits: faster than any entrance — don't celebrate a cancel. */}
        <AnimatePresence initial={false}>
          {messages.length === 0 ? (
            <motion.div
              key="empty-state"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ duration: 0.2 }}
            >
              <EmptyState agent={agent} />
            </motion.div>
          ) : (
            messages.map((message) => (
              <motion.div
                key={message.id}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.12 }}
              >
                <Message
                  message={message}
                  onDecision={onDecision}
                  onAnswer={onAnswer}
                />
              </motion.div>
            ))
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}

// Persona as the welcome screen, with the agent's capability list —
// consequential tools honestly flagged "asks first".
function EmptyState({ agent }: { agent: Agent }) {
  const isBuilder = agent.id === BUILDER_AGENT_ID;

  return (
    <div className="flex flex-col items-center gap-3 py-16 text-center">
      <AgentAvatar name={agent.name} className="size-12" filled={isBuilder} />
      <p className="text-base font-medium">{agent.name}</p>
      {agent.persona && (
        <p className="max-w-sm text-sm text-muted-foreground">
          {agent.persona}
        </p>
      )}

      {agent.tools.length > 0 && (
        <div className="mt-4 w-full max-w-sm space-y-1 text-left">
          <p className="px-2 pb-1 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
            {isBuilder ? "How it builds" : "What it can do"}
          </p>
          {agent.tools.map((tool) => {
            const info = toolInfo(tool);
            return (
              <div
                key={tool}
                className="flex items-center gap-2 rounded-md px-2 py-1.5 text-sm"
              >
                <code className="shrink-0 font-mono text-xs font-medium">
                  {tool}
                </code>
                {info.description && (
                  <span className="truncate text-xs text-muted-foreground">
                    {info.description}
                  </span>
                )}
                {info.consequential && (
                  <Badge
                    variant="outline"
                    className="ml-auto shrink-0 text-[10px]"
                  >
                    asks first
                  </Badge>
                )}
              </div>
            );
          })}
        </div>
      )}

      {isBuilder && (
        <p className="mt-2 max-w-sm text-xs text-muted-foreground">
          Describe the agent you want — purpose, personality, what it should
          be able to do. You&apos;ll review before anything is created.
        </p>
      )}
    </div>
  );
}
