"use client";

import { useState } from "react";
import { motion } from "motion/react";

import { AgentAvatar } from "@/components/agents/agent-avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { TurnItem } from "@/hooks/use-run";
import type { ApproveDecision } from "@/lib/types";

type ToolItem = Extract<TurnItem, { kind: "tool" }>;

// The create_agent confirm rendered as the agent being born — the same
// visual as the agents-list card — instead of a generic tool prompt.
// You're not approving a function call; you're reviewing the agent.
export function AgentPreviewCard({
  item,
  onDecision,
}: {
  item: ToolItem;
  onDecision: (callId: string, decision: ApproveDecision) => Promise<boolean>;
}) {
  const [submitting, setSubmitting] = useState(false);
  const spec = parseAgentSpec(item.args);

  const decide = async (decision: ApproveDecision) => {
    if (submitting) return;
    setSubmitting(true);
    const accepted = await onDecision(item.callId, decision);
    if (!accepted) setSubmitting(false);
  };

  return (
    <motion.div
      initial={{ opacity: 0, scale: 0.96 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.25, ease: "easeOut" }}
      className="rounded-xl border-2 border-foreground/80 p-4"
    >
      <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
        Review your new agent
      </p>

      <div className="mt-3 flex items-center gap-3">
        <AgentAvatar name={spec.name} className="size-9" />
        <div className="min-w-0">
          <p className="truncate text-sm font-medium">{spec.name}</p>
          {spec.model && (
            <p className="truncate text-xs text-muted-foreground">
              {spec.model}
            </p>
          )}
        </div>
      </div>

      {spec.persona && (
        <p className="mt-3 text-sm text-muted-foreground">{spec.persona}</p>
      )}

      {spec.instructions && (
        <p className="mt-2 line-clamp-3 text-xs text-muted-foreground">
          <span className="font-medium text-foreground">instructions: </span>
          {spec.instructions}
        </p>
      )}

      {spec.tools.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1.5">
          {spec.tools.map((tool) => (
            <Badge key={tool} variant="outline" className="font-mono text-xs">
              {tool}
            </Badge>
          ))}
        </div>
      )}

      <div className="mt-4 flex items-center justify-between">
        <Button size="sm" disabled={submitting} onClick={() => decide("approve")}>
          Create agent
        </Button>
        <Button
          size="sm"
          variant="ghost"
          disabled={submitting}
          onClick={() => decide("deny")}
        >
          Not yet
        </Button>
      </div>
    </motion.div>
  );
}

// Defensive parse: args come from the model via the backend; trust nothing.
function parseAgentSpec(args: Record<string, unknown>) {
  return {
    name: typeof args.name === "string" && args.name ? args.name : "New agent",
    persona: typeof args.persona === "string" ? args.persona : undefined,
    instructions:
      typeof args.instructions === "string" ? args.instructions : undefined,
    model: typeof args.model === "string" ? args.model : undefined,
    tools: Array.isArray(args.tools)
      ? args.tools.filter((t): t is string => typeof t === "string")
      : [],
  };
}
