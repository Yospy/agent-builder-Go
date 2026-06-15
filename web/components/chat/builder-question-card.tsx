"use client";

import { type KeyboardEvent, useMemo, useState } from "react";
import { motion } from "motion/react";
import { CheckIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type { TurnItem } from "@/hooks/use-run";
import type { AnswerPayload } from "@/lib/types";
import { cn } from "@/lib/utils";

type QuestionItem = Extract<TurnItem, { kind: "question" }>;

export function BuilderQuestionCard({
  item,
  onAnswer,
}: {
  item: QuestionItem;
  onAnswer: (callId: string, answer: AnswerPayload) => Promise<boolean>;
}) {
  const [selected, setSelected] = useState<string>(
    item.options[0]?.id ?? "custom",
  );
  const [customText, setCustomText] = useState("");

  const progressText = useMemo(() => {
    if (item.progress?.current && item.progress?.total) {
      return `${item.progress.current} / ${item.progress.total}`;
    }
    return null;
  }, [item.progress]);

  const customSelected = selected === "custom";
  const choiceIds = useMemo(
    () => [
      ...item.options.map((option) => option.id),
      ...(item.allowCustom ? ["custom"] : []),
    ],
    [item.allowCustom, item.options],
  );
  const canSubmit =
    item.status !== "submitting" &&
    (customSelected ? customText.trim().length > 0 : selected.length > 0);

  const submit = async () => {
    if (!canSubmit) return;
    const payload: AnswerPayload = customSelected
      ? { customText: customText.trim() }
      : { optionId: selected };
    await onAnswer(item.callId, payload);
  };

  const progressPercent =
    item.progress?.current && item.progress?.total
      ? Math.min(100, (item.progress.current / item.progress.total) * 100)
      : null;
  const legendId = `question-legend-${item.callId}`;

  if (item.status === "answered") {
    return (
      <div className="w-full rounded-lg border bg-card px-3.5 py-3 text-sm">
        <p className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
          {item.field.replaceAll("_", " ")}
        </p>
        <p className="mt-1 text-sm font-medium">{item.answer ?? "Answered"}</p>
      </div>
    );
  }

  const focusChoice = (group: HTMLElement | null, id: string) => {
    if (!group) return;
    const choices = group.querySelectorAll<HTMLButtonElement>("[data-choice-id]");
    const nextChoice = Array.from(choices).find(
      (choice) => choice.dataset.choiceId === id,
    );
    nextChoice?.focus();
  };

  const selectAndFocus = (id: string, group: HTMLElement | null) => {
    setSelected(id);
    focusChoice(group, id);
  };

  const moveSelection = (
    currentId: string,
    direction: 1 | -1,
    group: HTMLElement | null,
  ) => {
    const currentIndex = choiceIds.indexOf(currentId);
    if (currentIndex === -1 || choiceIds.length === 0) return;
    const nextIndex =
      (currentIndex + direction + choiceIds.length) % choiceIds.length;
    selectAndFocus(choiceIds[nextIndex], group);
  };

  const handleChoiceKeyDown = (
    event: KeyboardEvent<HTMLButtonElement>,
    currentId: string,
  ) => {
    if (item.status === "submitting") return;
    const group = event.currentTarget.closest(
      "[role='radiogroup']",
    ) as HTMLElement | null;

    switch (event.key) {
      case "ArrowDown":
      case "ArrowRight":
        event.preventDefault();
        moveSelection(currentId, 1, group);
        break;
      case "ArrowUp":
      case "ArrowLeft":
        event.preventDefault();
        moveSelection(currentId, -1, group);
        break;
      case "Home":
        event.preventDefault();
        if (choiceIds[0]) {
          selectAndFocus(choiceIds[0], group);
        }
        break;
      case "End":
        event.preventDefault();
        if (choiceIds[choiceIds.length - 1]) {
          selectAndFocus(choiceIds[choiceIds.length - 1], group);
        }
        break;
      case " ":
      case "Enter":
        event.preventDefault();
        setSelected(currentId);
        break;
    }
  };

  return (
    <motion.div
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.22, ease: "easeOut" }}
      className="rounded-xl border-2 border-foreground/80 bg-card p-4 shadow-sm"
    >
      <div className="flex items-center justify-between gap-4">
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
          {item.progress?.label ?? "Agent setup"}
        </p>
        {progressText && (
          <p className="shrink-0 text-xs tabular-nums text-muted-foreground">
            {progressText}
          </p>
        )}
      </div>

      {progressPercent !== null && (
        <div className="mt-2 h-1 overflow-hidden rounded-full bg-muted">
          <div
            className="h-full rounded-full bg-foreground transition-[width]"
            style={{ width: `${progressPercent}%` }}
          />
        </div>
      )}

      <fieldset className="mt-4 space-y-2">
        <legend
          id={legendId}
          className="mb-3 text-base font-semibold leading-snug"
        >
          {item.question}
        </legend>

        <div
          role="radiogroup"
          aria-labelledby={legendId}
          className="space-y-2"
        >
          {item.options.map((option, index) => {
            const checked = selected === option.id;
            return (
              <motion.button
                key={option.id}
                type="button"
                role="radio"
                aria-checked={checked}
                data-choice-id={option.id}
                disabled={item.status === "submitting"}
                tabIndex={checked ? 0 : -1}
                initial={{ opacity: 0, y: 4 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.18, delay: index * 0.035 }}
                onClick={() => setSelected(option.id)}
                onKeyDown={(event) => handleChoiceKeyDown(event, option.id)}
                className={cn(
                  "group flex min-h-14 w-full cursor-pointer gap-3 rounded-lg border p-3 text-left transition-all hover:-translate-y-px hover:bg-muted/50",
                  "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/30 focus-visible:outline-none",
                  "disabled:cursor-not-allowed disabled:opacity-60",
                  checked && "border-foreground bg-muted/70",
                )}
              >
                <span
                  aria-hidden
                  className={cn(
                    "mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full border transition-colors",
                    checked
                      ? "border-foreground bg-foreground text-background"
                      : "border-muted-foreground/50 text-transparent",
                  )}
                >
                  <CheckIcon className="size-3" />
                </span>
                <span className="min-w-0">
                  <span className="block text-sm font-medium">
                    {option.label}
                  </span>
                  {option.description && (
                    <span className="mt-0.5 block text-xs leading-relaxed text-muted-foreground">
                      {option.description}
                    </span>
                  )}
                </span>
              </motion.button>
            );
          })}

          {item.allowCustom && (
            <div className="pt-1">
              <button
                type="button"
                role="radio"
                aria-checked={customSelected}
                data-choice-id="custom"
                disabled={item.status === "submitting"}
                tabIndex={customSelected ? 0 : -1}
                onClick={() => setSelected("custom")}
                onKeyDown={(event) => handleChoiceKeyDown(event, "custom")}
                className={cn(
                  "flex w-full cursor-pointer items-center gap-3 rounded-lg border p-3 text-left text-sm font-medium transition-all hover:-translate-y-px hover:bg-muted/50",
                  "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/30 focus-visible:outline-none",
                  "disabled:cursor-not-allowed disabled:opacity-60",
                  customSelected && "border-foreground bg-muted/70",
                )}
              >
                <span
                  aria-hidden
                  className={cn(
                    "flex size-5 shrink-0 items-center justify-center rounded-full border transition-colors",
                    customSelected
                      ? "border-foreground bg-foreground text-background"
                      : "border-muted-foreground/50 text-transparent",
                  )}
                >
                  <CheckIcon className="size-3" />
                </span>
                Custom answer
              </button>

              {customSelected && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: "auto" }}
                  transition={{ duration: 0.18 }}
                  className="overflow-hidden"
                >
                  <label
                    htmlFor={`custom-${item.callId}`}
                    className="mt-3 block text-xs font-medium text-muted-foreground"
                  >
                    Custom answer
                  </label>
                  <Textarea
                    id={`custom-${item.callId}`}
                    value={customText}
                    disabled={item.status === "submitting"}
                    onChange={(event) => setCustomText(event.target.value)}
                    placeholder={item.customPlaceholder ?? "Type your answer"}
                    rows={3}
                    className="mt-1 min-h-20"
                  />
                </motion.div>
              )}
            </div>
          )}
        </div>
      </fieldset>

      {item.status === "error" && (
        <p className="mt-3 text-xs font-medium text-foreground" role="alert">
          That answer could not be saved. Check the selection and try again.
        </p>
      )}

      <div className="mt-4 flex justify-end">
        <Button
          size="sm"
          disabled={!canSubmit}
          onClick={submit}
          aria-label="Continue with selected answer"
        >
          {item.status === "submitting" ? "Saving…" : "Continue"}
        </Button>
      </div>
    </motion.div>
  );
}
