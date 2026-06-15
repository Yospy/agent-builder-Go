"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import { api, ApiError, parseErrorMessage } from "@/lib/api";
import { parseSSE } from "@/lib/sse";
import type {
  AnswerPayload,
  ApproveDecision,
  ChatMessage,
  QuestionOption,
  QuestionProgress,
  RunEvent,
} from "@/lib/types";

export type ToolStatus = "running" | "ok" | "error" | "confirm" | "denied";
export type QuestionStatus = "pending" | "submitting" | "answered" | "error";

export type TurnItem =
  | { kind: "text"; text: string }
  | { kind: "status"; message: string }
  | {
      kind: "question";
      callId: string;
      field: string;
      question: string;
      options: QuestionOption[];
      allowCustom: boolean;
      customPlaceholder?: string;
      progress?: QuestionProgress;
      status: QuestionStatus;
      answer?: string;
    }
  | {
      kind: "tool";
      callId: string;
      name: string;
      args: Record<string, unknown>;
      status: ToolStatus;
      data?: string;
    };

export type DisplayMessage =
  | { id: string; role: "user"; content: string }
  | { id: string; role: "assistant"; items: TurnItem[]; streaming: boolean };

export type RunStatus =
  | "idle"
  | "streaming"
  | "awaiting_approval"
  | "awaiting_answer";

let nextId = 0;
const mintId = () => `msg-${++nextId}`;

// Rebuild display turns from the backend's normalized OpenAI history:
// everything between two user messages is one assistant turn; tool_calls
// become tool rows and role:"tool" results attach to them by call id.
export function historyToDisplay(history: ChatMessage[]): DisplayMessage[] {
  const out: DisplayMessage[] = [];
  let turn: Extract<DisplayMessage, { role: "assistant" }> | null = null;

  for (const m of history) {
    switch (m.role) {
      case "user":
        turn = null;
        out.push({ id: mintId(), role: "user", content: m.content ?? "" });
        break;
      case "assistant": {
        if (!turn) {
          turn = {
            id: mintId(),
            role: "assistant",
            items: [],
            streaming: false,
          };
          out.push(turn);
        }
        if (m.content) {
          turn.items.push({ kind: "text", text: m.content });
        }
        for (const call of m.tool_calls ?? []) {
          if (call.name === "ask_user_question") {
            turn.items.push(questionFromArgs(call.id, call.args ?? {}));
          } else {
            turn.items.push({
              kind: "tool",
              callId: call.id,
              name: call.name,
              args: call.args ?? {},
              // History doesn't record ok/error per call; completed is all
              // we can honestly claim for a persisted turn.
              status: "ok",
            });
          }
        }
        break;
      }
      case "tool": {
        const item = turn?.items.find(
          (i) =>
            (i.kind === "tool" || i.kind === "question") &&
            i.callId === m.tool_call_id,
        );
        if (item && item.kind === "tool") {
          item.data = m.content;
        } else if (item && item.kind === "question") {
          item.status = "answered";
          item.answer = parseQuestionAnswer(m.content);
        }
        break;
      }
      case "system":
        break; // never rendered
    }
  }
  return out;
}

interface UseRunOptions {
  sessionId: string;
  initialMessages: DisplayMessage[];
  initialStatusMessage?: string;
  // Called once per committed turn (the `done` event), e.g. to update
  // the recent-chats title or refresh the agents list after create_agent.
  onTurnCommitted?: (userMessage: string) => void;
  // Called whenever an uncommitted turn is rolled back (§9), with the user's
  // message so the composer can restore it for a painless retry.
  onRollback?: (userMessage: string) => void;
}

export function useRun({
  sessionId,
  initialMessages,
  initialStatusMessage,
  onTurnCommitted,
  onRollback,
}: UseRunOptions) {
  const [messages, setMessages] = useState<DisplayMessage[]>(initialMessages);
  const [status, setStatus] = useState<RunStatus>("idle");
  const abortRef = useRef<AbortController | null>(null);
  // Where the in-flight turn began, for §9 rollback (server never commits
  // an aborted turn, so the UI must not keep it either).
  const rollbackIndexRef = useRef<number>(initialMessages.length);

  const updateCurrentTurn = useCallback(
    (update: (items: TurnItem[]) => TurnItem[]) => {
      setMessages((prev) => {
        const last = prev[prev.length - 1];
        if (!last || last.role !== "assistant") return prev;
        return [...prev.slice(0, -1), { ...last, items: update(last.items) }];
      });
    },
    [],
  );

  const finalizeTurn = useCallback(() => {
    setMessages((prev) => {
      const last = prev[prev.length - 1];
      if (!last || last.role !== "assistant") return prev;
      return [...prev.slice(0, -1), { ...last, streaming: false }];
    });
  }, []);

  const rollbackTurn = useCallback(() => {
    setMessages((prev) => prev.slice(0, rollbackIndexRef.current));
  }, []);

  const handleEvent = useCallback(
    (event: RunEvent, userMessage: string): "open" | "closed" => {
      switch (event.type) {
        case "status":
          updateCurrentTurn((items) => [
            ...items.filter((item) => item.kind !== "status"),
            { kind: "status", message: event.message },
          ]);
          return "open";

        case "llm_text":
          updateCurrentTurn((items) => {
            const last = items[items.length - 1];
            if (last?.kind === "text") {
              return [
                ...items.slice(0, -1),
                { kind: "text", text: last.text + event.text },
              ];
            }
            return [...items, { kind: "text", text: event.text }];
          });
          return "open";

        case "tool_use":
          updateCurrentTurn((items) => [
            ...items,
            {
              kind: "tool",
              callId: event.call_id,
              name: event.name,
              args: event.args,
              status: "running",
            },
          ]);
          return "open";

        case "confirm":
          // The backend emits tool_use AND confirm for the same call_id
          // (observed in integration): upgrade the existing row instead of
          // duplicating it; only push when confirm arrives alone.
          updateCurrentTurn((items) => {
            const existing = items.some(
              (i) => i.kind === "tool" && i.callId === event.call_id,
            );
            if (existing) {
              return items.map((item) =>
                item.kind === "tool" && item.callId === event.call_id
                  ? { ...item, status: "confirm" as const }
                  : item,
              );
            }
            return [
              ...items,
              {
                kind: "tool",
                callId: event.call_id,
                name: event.name,
                args: event.args,
                status: "confirm",
              },
            ];
          });
          setStatus("awaiting_approval");
          return "open";

        case "user_question":
          updateCurrentTurn((items) => [
            ...items.filter((item) => item.kind !== "status"),
            {
              kind: "question",
              callId: event.call_id,
              field: event.field,
              question: event.question,
              options: event.options,
              allowCustom: event.allow_custom,
              customPlaceholder: event.custom_placeholder,
              progress: event.progress,
              status: "pending",
            },
          ]);
          setStatus("awaiting_answer");
          return "open";

        case "tool_result":
          updateCurrentTurn((items) =>
            items.map((item) => {
              if (item.kind === "question" && item.callId === event.call_id) {
                return {
                  ...item,
                  status: event.ok ? "answered" : "error",
                  answer: parseQuestionAnswer(event.data),
                };
              }
              if (item.kind === "tool" && item.callId === event.call_id) {
                return {
                  ...item,
                  // A denied call's result echoes the denial (§8) — keep it
                  // labeled as the user's decision, not a tool failure.
                  status:
                    item.status === "denied"
                      ? "denied"
                      : event.ok
                        ? "ok"
                        : "error",
                  data: event.data,
                };
              }
              return item;
            }),
          );
          return "open";

        case "done":
          updateCurrentTurn((items) => {
            const withoutStatus = items.filter(
              (item) => item.kind !== "status",
            );
            // `done.text` is the authoritative final answer (§7). If the tail
            // of the turn is streamed text it's the same content — replace it;
            // otherwise (text → tool_use → tool_result → done) append it.
            const last = withoutStatus[withoutStatus.length - 1];
            if (last?.kind === "text") {
              return [
                ...withoutStatus.slice(0, -1),
                { kind: "text", text: event.text || last.text },
              ];
            }
            if (event.text) {
              return [...withoutStatus, { kind: "text", text: event.text }];
            }
            return withoutStatus;
          });
          finalizeTurn();
          setStatus("idle");
          onTurnCommitted?.(userMessage);
          return "closed";

        case "error":
          // The server does not persist a turn that ended in error (§11:
          // "history stays clean"), so the UI must roll the in-flight turn
          // back to match the DB and surface the error out-of-band — otherwise
          // the user message + partial turn vanish on the next reload.
          rollbackTurn();
          setStatus("idle");
          toast.error(event.message);
          onRollback?.(userMessage);
          return "closed";

        case "aborted":
          rollbackTurn();
          setStatus("idle");
          onRollback?.(userMessage);
          return "closed";
      }
    },
    [updateCurrentTurn, finalizeTurn, rollbackTurn, onTurnCommitted, onRollback],
  );

  const send = useCallback(
    async (text: string) => {
      const message = text.trim();
      if (!message || abortRef.current) return;

      setMessages((prev) => {
        rollbackIndexRef.current = prev.length;
        const initialItems: TurnItem[] = initialStatusMessage
          ? [{ kind: "status", message: initialStatusMessage }]
          : [];
        return [
          ...prev,
          { id: mintId(), role: "user", content: message },
          {
            id: mintId(),
            role: "assistant",
            items: initialItems,
            streaming: true,
          },
        ];
      });
      setStatus("streaming");

      const controller = new AbortController();
      abortRef.current = controller;

      try {
        const res = await api.run(sessionId, message, controller.signal);

        if (!res.ok || !res.body) {
          const errorMessage = await parseErrorMessage(res);
          rollbackTurn();
          setStatus("idle");
          onRollback?.(message);
          if (res.status === 409) {
            toast("A run is already in flight for this session");
          } else {
            toast.error(errorMessage);
          }
          return;
        }

        let terminated = false;
        for await (const event of parseSSE(res.body)) {
          if (handleEvent(event, message) === "closed") {
            terminated = true;
            break;
          }
        }
        if (!terminated) {
          // Stream ended without done/error/aborted — connection dropped.
          // The server treats a disconnect as abort (§9) and never commits
          // the turn, so the UI must drop it too.
          rollbackTurn();
          setStatus("idle");
          onRollback?.(message);
          toast.error("Connection lost — the turn was not saved");
        }
      } catch (err) {
        if (controller.signal.aborted) {
          // User hit Stop (or navigated away): §9 abort path.
          rollbackTurn();
        } else {
          rollbackTurn();
          toast.error(
            err instanceof Error ? err.message : "the request failed",
          );
        }
        onRollback?.(message);
        setStatus("idle");
      } finally {
        abortRef.current = null;
      }
    },
    [sessionId, initialStatusMessage, handleEvent, rollbackTurn, onRollback],
  );

  const stop = useCallback(() => {
    abortRef.current?.abort();
  }, []);

  // Navigating away mid-run must disconnect: the server treats the drop as
  // abort (§9) and releases the per-session in-flight lock (§11). Without
  // this, a paused confirm would hold the session's 409 lock forever.
  useEffect(() => {
    return () => {
      abortRef.current?.abort();
    };
  }, []);

  // Returns whether the decision was accepted, so the confirm card can
  // re-enable its buttons on failure instead of locking up.
  const approve = useCallback(
    async (callId: string, decision: ApproveDecision): Promise<boolean> => {
      try {
        await api.approve(sessionId, callId, decision);
        updateCurrentTurn((items) =>
          items.map((item) =>
            item.kind === "tool" && item.callId === callId
              ? {
                  ...item,
                  status: decision === "approve" ? "running" : "denied",
                }
              : item,
          ),
        );
        // Continuation flows on the original /run stream (§8).
        setStatus("streaming");
        return true;
      } catch (err) {
        toast.error(
          err instanceof ApiError ? err.message : "approval request failed",
        );
        return false;
      }
    },
    [sessionId, updateCurrentTurn],
  );

  const answer = useCallback(
    async (callId: string, payload: AnswerPayload): Promise<boolean> => {
      updateCurrentTurn((items) =>
        items.map((item) =>
          item.kind === "question" && item.callId === callId
            ? { ...item, status: "submitting" }
            : item,
        ),
      );
      try {
        await api.answer(sessionId, callId, payload);
        // Continuation flows on the original /run stream.
        setStatus("streaming");
        return true;
      } catch (err) {
        updateCurrentTurn((items) =>
          items.map((item) =>
            item.kind === "question" && item.callId === callId
              ? { ...item, status: "error" }
              : item,
          ),
        );
        setStatus("awaiting_answer");
        toast.error(
          err instanceof ApiError ? err.message : "answer request failed",
        );
        return false;
      }
    },
    [sessionId, updateCurrentTurn],
  );

  return { messages, status, send, stop, approve, answer };
}

function questionFromArgs(
  callId: string,
  args: Record<string, unknown>,
): Extract<TurnItem, { kind: "question" }> {
  return {
    kind: "question",
    callId,
    field: typeof args.field === "string" ? args.field : "unknown",
    question:
      typeof args.question === "string" ? args.question : "Question answered",
    options: parseQuestionOptions(args.options),
    allowCustom: args.allow_custom === true,
    customPlaceholder:
      typeof args.custom_placeholder === "string"
        ? args.custom_placeholder
        : undefined,
    progress: parseProgress(args),
    status: "pending",
  };
}

function parseQuestionOptions(value: unknown): QuestionOption[] {
  if (!Array.isArray(value)) return [];
  const options: QuestionOption[] = [];
  for (const item of value) {
    if (!item || typeof item !== "object") continue;
    const record = item as Record<string, unknown>;
    if (
      typeof record.id !== "string" ||
      typeof record.label !== "string" ||
      typeof record.value !== "string"
    ) {
      continue;
    }
    options.push({
      id: record.id,
      label: record.label,
      value: record.value,
      description:
        typeof record.description === "string"
          ? record.description
          : undefined,
    });
  }
  return options;
}

function parseProgress(args: Record<string, unknown>): QuestionProgress | undefined {
  const label =
    typeof args.progress_label === "string" ? args.progress_label : undefined;
  const current =
    typeof args.progress_current === "number"
      ? args.progress_current
      : undefined;
  const total =
    typeof args.progress_total === "number" ? args.progress_total : undefined;
  if (!label && current === undefined && total === undefined) return undefined;
  return { label, current, total };
}

function parseQuestionAnswer(data?: string): string | undefined {
  if (!data) return undefined;
  try {
    const parsed = JSON.parse(data) as { answer?: unknown; value?: unknown };
    if (typeof parsed.answer === "string" && parsed.answer) {
      return parsed.answer;
    }
    if (typeof parsed.value === "string" && parsed.value) {
      return parsed.value;
    }
  } catch {
    // Plain text fallback for defensive history rendering.
  }
  return data;
}
