"use client";

import { useState } from "react";
import {
  CheckIcon,
  ChevronDownIcon,
  Loader2Icon,
  MinusIcon,
  XIcon,
} from "lucide-react";

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import type { TurnItem } from "@/hooks/use-run";
import { cn } from "@/lib/utils";

type ToolItem = Extract<TurnItem, { kind: "tool" }>;

const STATUS_LABEL: Record<ToolItem["status"], string> = {
  running: "running…",
  ok: "done",
  error: "failed",
  confirm: "awaiting approval",
  denied: "denied",
};

function StatusIcon({ status }: { status: ToolItem["status"] }) {
  switch (status) {
    case "running":
      return <Loader2Icon className="size-3.5 animate-spin" />;
    case "ok":
      return <CheckIcon className="size-3.5" />;
    case "error":
      return <XIcon className="size-3.5" />;
    case "denied":
      return <MinusIcon className="size-3.5" />;
    case "confirm":
      return <Loader2Icon className="size-3.5 animate-spin" />;
  }
}

// Progressive disclosure: collapsed = name + status; expanded = args + result.
export function ToolCallRow({ item }: { item: ToolItem }) {
  const [open, setOpen] = useState(false);
  const hasDetails =
    Object.keys(item.args).length > 0 || (item.data?.length ?? 0) > 0;

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <div className="rounded-lg border bg-muted/40 text-sm">
        <CollapsibleTrigger
          disabled={!hasDetails}
          className={cn(
            "flex w-full items-center gap-2 px-3 py-2 text-left focus-visible:outline-2 focus-visible:outline-ring",
            hasDetails && "cursor-pointer",
          )}
        >
          <StatusIcon status={item.status} />
          <code className="font-mono text-xs font-medium">{item.name}</code>
          <span className="text-xs text-muted-foreground">
            {STATUS_LABEL[item.status]}
          </span>
          {hasDetails && (
            <ChevronDownIcon
              className={cn(
                "ml-auto size-3.5 text-muted-foreground transition-transform",
                open && "rotate-180",
              )}
            />
          )}
        </CollapsibleTrigger>
        <CollapsibleContent>
          <div className="space-y-2 border-t px-3 py-2">
            {Object.keys(item.args).length > 0 && (
              <DetailBlock label="args">
                {JSON.stringify(item.args, null, 2)}
              </DetailBlock>
            )}
            {item.data && <DetailBlock label="result">{item.data}</DetailBlock>}
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>
  );
}

function DetailBlock({
  label,
  children,
}: {
  label: string;
  children: string;
}) {
  return (
    <div>
      <p className="mb-1 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </p>
      <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted p-2 font-mono text-xs">
        {children}
      </pre>
    </div>
  );
}
