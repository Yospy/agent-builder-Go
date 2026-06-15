"use client";

import { useLayoutEffect, useRef } from "react";
import { AnimatePresence, motion } from "motion/react";
import { ArrowUpIcon, SquareIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type { RunStatus } from "@/hooks/use-run";
import { MAX_MESSAGE_LENGTH } from "@/lib/types";
import { cn } from "@/lib/utils";

const MAX_TEXTAREA_HEIGHT = 260;

// Draft state lives in ChatView so a rolled-back turn can restore the
// user's message for a painless retry.
export function Composer({
  agentName,
  status,
  value,
  onChange,
  onSend,
  onStop,
}: {
  agentName: string;
  status: RunStatus;
  value: string;
  onChange: (value: string) => void;
  onSend: (text: string) => void;
  onStop: () => void;
}) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const overLimit = value.length > MAX_MESSAGE_LENGTH;
  const canSend = status === "idle" && value.trim().length > 0 && !overLimit;
  const showCounter = overLimit || value.length >= MAX_MESSAGE_LENGTH * 0.9;
  const inputDisabled =
    status === "awaiting_approval" || status === "awaiting_answer";

  useLayoutEffect(() => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    textarea.style.height = "auto";
    textarea.style.height = `${Math.min(
      textarea.scrollHeight,
      MAX_TEXTAREA_HEIGHT,
    )}px`;
  }, [value]);

  const submit = () => {
    if (!canSend) return;
    onSend(value);
  };

  return (
    <footer className="shrink-0 border-t bg-background">
      <div className="mx-auto w-full max-w-4xl px-4 py-4">
        <motion.div
          layout
          transition={{ layout: { duration: 0.18, ease: "easeOut" } }}
          className={cn(
            "flex items-end gap-3 rounded-[28px] border bg-card px-4 py-3 shadow-xs",
            "transition-[border-color,box-shadow,background-color] duration-200 ease-out",
            "focus-within:border-ring focus-within:shadow-[0_0_0_3px_color-mix(in_oklch,var(--ring),transparent_72%)]",
            overLimit && "border-destructive/60",
          )}
        >
          <Textarea
            ref={textareaRef}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={(e) => {
              if (
                e.key === "Enter" &&
                !e.shiftKey &&
                !e.nativeEvent.isComposing
              ) {
                e.preventDefault();
                submit();
              }
            }}
            placeholder={
              status === "awaiting_approval"
                ? "Waiting for your approval above…"
                : status === "awaiting_answer"
                  ? "Answer the question above…"
                : `Message ${agentName}…`
            }
            disabled={inputDisabled}
            aria-label="Message"
            aria-invalid={overLimit}
            rows={1}
            className={cn(
              "scrollbar-none min-h-8 flex-1 resize-none overflow-y-auto overscroll-contain border-0 bg-transparent px-1 py-1 shadow-none",
              "text-[15px] leading-7 transition-[height] duration-200 ease-out",
              "focus-visible:ring-0 md:text-[15px] dark:bg-transparent",
            )}
          />
          {/* Send ⇄ Stop: the run state machine made visible (~150ms crossfade) */}
          <AnimatePresence mode="wait" initial={false}>
            {status === "idle" ? (
              <motion.span
                key="send"
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.8 }}
                transition={{ duration: 0.15 }}
              >
                <Button
                  size="icon"
                  aria-label="Send message"
                  disabled={!canSend}
                  onClick={submit}
                  className="mb-0.5 size-9 rounded-xl"
                >
                  <ArrowUpIcon className="size-4" />
                </Button>
              </motion.span>
            ) : (
              <motion.span
                key="stop"
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.8 }}
                transition={{ duration: 0.15 }}
              >
                <Button
                  size="icon"
                  variant="outline"
                  aria-label="Stop generating"
                  onClick={onStop}
                  className="mb-0.5 size-9 rounded-xl"
                >
                  <SquareIcon className="size-3.5 fill-current" />
                </Button>
              </motion.span>
            )}
          </AnimatePresence>
        </motion.div>
        <AnimatePresence initial={false}>
          {showCounter && (
            <motion.div
              initial={{ opacity: 0, y: -2 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -2 }}
              transition={{ duration: 0.12 }}
              className="mt-1.5 flex justify-end"
            >
              <span
                aria-live="polite"
                className={cn(
                  "text-xs tabular-nums text-muted-foreground",
                  overLimit && "font-medium text-foreground",
                )}
              >
                {value.length.toLocaleString()} /{" "}
                {MAX_MESSAGE_LENGTH.toLocaleString()}
              </span>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </footer>
  );
}
