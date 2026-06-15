"use client";

import { useState } from "react";
import { motion } from "motion/react";
import { ShieldAlertIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { TurnItem } from "@/hooks/use-run";
import type { ApproveDecision } from "@/lib/types";

type ToolItem = Extract<TurnItem, { kind: "tool" }>;

// Inline confirmation dialog (deliberately not a modal): the surrounding
// conversation is the context for the decision. Describes what will run;
// the safe and consequential actions sit far apart.
export function ConfirmCard({
  item,
  onDecision,
}: {
  item: ToolItem;
  onDecision: (callId: string, decision: ApproveDecision) => Promise<boolean>;
}) {
  const [submitting, setSubmitting] = useState(false);
  const isAgentUpdate = item.name === "update_agent";

  const decide = async (decision: ApproveDecision) => {
    if (submitting) return;
    setSubmitting(true);
    const accepted = await onDecision(item.callId, decision);
    // On failure the card must stay actionable — it's the only way out.
    if (!accepted) setSubmitting(false);
  };

  return (
    <motion.div
      initial={{ opacity: 0, scale: 0.96 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.25, ease: "easeOut" }}
      className="rounded-lg border-2 border-foreground/80 p-4 text-sm">
      <div className="flex items-center gap-2">
        <ShieldAlertIcon className="size-4" />
        <p className="font-medium">
          {isAgentUpdate ? "Apply agent changes?" : "Approval needed"}
        </p>
      </div>
      <p className="mt-2 text-muted-foreground">
        {isAgentUpdate ? (
          "This will update the live agent configuration."
        ) : (
          <>
            The agent wants to run{" "}
            <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs font-medium text-foreground">
              {item.name}
            </code>{" "}
            — this changes something outside the conversation.
          </>
        )}
      </p>
      {Object.keys(item.args).length > 0 && (
        isAgentUpdate ? (
          <div className="mt-3 space-y-1 rounded-md bg-muted p-2 text-xs">
            {Object.entries(item.args)
              .filter(([, value]) => value !== null && value !== undefined)
              .map(([key, value]) => (
                <div key={key} className="grid grid-cols-[6rem_minmax(0,1fr)] gap-2">
                  <span className="font-mono text-muted-foreground">{key}</span>
                  <span className="min-w-0 break-words">
                    {Array.isArray(value)
                      ? value.join(", ") || "[]"
                      : String(value)}
                  </span>
                </div>
              ))}
          </div>
        ) : (
          <pre className="mt-3 max-h-48 overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted p-2 font-mono text-xs">
            {JSON.stringify(item.args, null, 2)}
          </pre>
        )
      )}
      <div className="mt-4 flex items-center justify-between">
        <Button
          size="sm"
          disabled={submitting}
          onClick={() => decide("approve")}
        >
          {isAgentUpdate ? "Apply changes" : "Approve"}
        </Button>
        <Button
          size="sm"
          variant="ghost"
          disabled={submitting}
          onClick={() => decide("deny")}
        >
          {isAgentUpdate ? "Cancel" : "Deny"}
        </Button>
      </div>
    </motion.div>
  );
}
