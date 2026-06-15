"use client";

import { useState } from "react";
import { CheckIcon, CopyIcon } from "lucide-react";

import { AgentPreviewCard } from "@/components/chat/agent-preview-card";
import { BuilderQuestionCard } from "@/components/chat/builder-question-card";
import { ConfirmCard } from "@/components/chat/confirm-card";
import { MarkdownText } from "@/components/chat/markdown-text";
import { SuccessCard } from "@/components/chat/success-card";
import { ToolCallRow } from "@/components/chat/tool-call-row";
import { Button } from "@/components/ui/button";
import type { DisplayMessage, TurnItem } from "@/hooks/use-run";
import type { AnswerPayload, ApproveDecision } from "@/lib/types";

type StatusItem = Extract<TurnItem, { kind: "status" }>;

export function Message({
  message,
  onDecision,
  onAnswer,
}: {
  message: DisplayMessage;
  onDecision: (callId: string, decision: ApproveDecision) => Promise<boolean>;
  onAnswer: (callId: string, answer: AnswerPayload) => Promise<boolean>;
}) {
  if (message.role === "user") {
    return (
      <div className="flex justify-end">
        <div className="max-w-[80%] whitespace-pre-wrap break-words rounded-2xl bg-muted px-4 py-2.5 text-sm">
          {message.content}
        </div>
      </div>
    );
  }

  const statusItems = message.items.filter(
    (item): item is StatusItem => item.kind === "status",
  );
  const contentItems = message.items.filter(
    (item): item is Exclude<TurnItem, StatusItem> => item.kind !== "status",
  );
  const answerText = contentItems
    .filter(
      (item): item is Extract<TurnItem, { kind: "text" }> =>
        item.kind === "text",
    )
    .map((item) => item.text)
    .join("\n\n")
    .trim();

  return (
    <div className="space-y-3">
      {statusItems.length > 0 && (
        <CurrentActivityStatus item={statusItems[statusItems.length - 1]} />
      )}
      {contentItems.map((item, index) => {
        switch (item.kind) {
          case "text":
            return (
              <div key={index} className="relative">
                <MarkdownText text={item.text} />
                {message.streaming && index === contentItems.length - 1 && (
                  <span
                    aria-hidden
                    className="ml-0.5 inline-block h-4 w-[2px] translate-y-[2px] animate-pulse bg-foreground motion-reduce:animate-none"
                  />
                )}
              </div>
            );
          case "question":
            return (
              <BuilderQuestionCard
                key={item.callId}
                item={item}
                onAnswer={onAnswer}
              />
            );
          case "tool": {
            if (item.status === "confirm") {
              // The create_agent confirm is the agent's birth: render the
              // spec as an agent card, not a generic tool prompt.
              return item.name === "create_agent" ? (
                <AgentPreviewCard
                  key={item.callId}
                  item={item}
                  onDecision={onDecision}
                />
              ) : (
                <ConfirmCard
                  key={item.callId}
                  item={item}
                  onDecision={onDecision}
                />
              );
            }
            const createdAgentName =
              item.name === "create_agent" && item.status === "ok"
                ? typeof item.args.name === "string" && item.args.name
                  ? item.args.name
                  : "New agent" // same fallback as the preview card
                : null;
            return (
              <div key={item.callId} className="space-y-3">
                <ToolCallRow item={item} />
                {createdAgentName && (
                  <SuccessCard
                    agentName={createdAgentName}
                    resultData={item.data}
                    // Navigating away mid-stream would abort the turn (§9)
                    // and drop it from the builder's history — hold the CTA
                    // until the turn commits.
                    disabled={message.streaming}
                  />
                )}
              </div>
            );
          }
        }
      })}
      {!message.streaming && answerText && (
        <CopyAnswerButton text={answerText} />
      )}
      {message.streaming &&
        contentItems.length === 0 &&
        statusItems.length === 0 && (
          <div className="flex items-center gap-1.5" aria-label="Thinking">
            <ThinkingDot delay="0ms" />
            <ThinkingDot delay="150ms" />
            <ThinkingDot delay="300ms" />
          </div>
        )}
    </div>
  );
}

function CopyAnswerButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1200);
    } catch {
      setCopied(false);
    }
  };

  return (
    <div className="pt-1">
      <Button
        type="button"
        variant="ghost"
        size="icon-xs"
        onClick={copy}
        aria-label={copied ? "Copied answer" : "Copy answer"}
        title={copied ? "Copied" : "Copy answer"}
        className="text-muted-foreground hover:text-foreground"
      >
        {copied ? <CheckIcon /> : <CopyIcon />}
      </Button>
    </div>
  );
}

function CurrentActivityStatus({ item }: { item: StatusItem }) {
  return (
    <div
      className="flex w-fit max-w-[min(28rem,100%)] items-center gap-3 rounded-full border bg-card/80 px-3.5 py-2 shadow-sm"
      aria-live="polite"
      aria-label={item.message}
    >
      <span className="relative flex size-2.5 shrink-0">
        <span className="absolute inline-flex size-full animate-ping rounded-full bg-foreground/35 motion-reduce:animate-none" />
        <span className="relative inline-flex size-2.5 rounded-full bg-foreground" />
      </span>
      <p className="truncate text-sm font-medium leading-none">
        {item.message}
      </p>
    </div>
  );
}

function ThinkingDot({ delay }: { delay: string }) {
  return (
    <span
      aria-hidden
      style={{ animationDelay: delay }}
      className="size-1.5 animate-pulse rounded-full bg-muted-foreground motion-reduce:animate-none"
    />
  );
}
