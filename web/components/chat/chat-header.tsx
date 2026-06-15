import Link from "next/link";
import { ArrowLeftIcon, Edit3Icon, HammerIcon } from "lucide-react";

import { AgentAvatar } from "@/components/agents/agent-avatar";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { toolInfo } from "@/lib/tool-catalog";
import { BUILDER_AGENT_ID, type Agent } from "@/lib/types";
import { cn } from "@/lib/utils";

const MAX_VISIBLE_TOOLS = 4;

export function ChatHeader({
  agent,
  mode = "chat",
}: {
  agent: Agent;
  mode?: "chat" | "edit";
}) {
  const isBuilder = agent.id === BUILDER_AGENT_ID;
  const isEdit = mode === "edit";
  const visibleTools = agent.tools.slice(0, MAX_VISIBLE_TOOLS);
  const hiddenCount = agent.tools.length - visibleTools.length;

  return (
    <header
      className={cn(
        "flex h-14 shrink-0 items-center gap-3 border-b bg-background px-4 text-foreground",
        isBuilder && "border-border",
      )}
    >
      <Link
        href="/"
        aria-label="Back to agents"
        className={cn(
          "rounded-md p-1.5 focus-visible:outline-2 focus-visible:outline-ring",
          "text-muted-foreground hover:bg-accent hover:text-foreground",
        )}
      >
        <ArrowLeftIcon className="size-4" />
      </Link>
      <AgentAvatar
        name={agent.name}
        className="size-7"
        filled={isBuilder}
      />
      <div className="flex min-w-0 items-baseline gap-2">
        <span className="truncate text-sm font-medium">{agent.name}</span>
        {isBuilder && (
          <span className="hidden items-center gap-1 text-xs text-muted-foreground sm:flex">
            <HammerIcon className="size-3" />
            designing a new agent
          </span>
        )}
        {isEdit && (
          <span className="hidden items-center gap-1 text-xs text-muted-foreground sm:flex">
            <Edit3Icon className="size-3" />
            editing agent
          </span>
        )}
      </div>
      <div className="ml-auto hidden flex-wrap items-center gap-1.5 sm:flex">
        {visibleTools.map((tool) => {
          const info = toolInfo(tool);
          const tag = (
            <span
              // Focusable so the tooltip opens on keyboard focus too.
              tabIndex={info.description ? 0 : undefined}
              className="inline-flex h-7 items-center rounded-md border border-border bg-muted/35 px-2.5 font-mono text-xs text-muted-foreground focus-visible:outline-2 focus-visible:outline-ring"
            >
              {tool}
            </span>
          );
          return info.description ? (
            <Tooltip key={tool}>
              <TooltipTrigger asChild>{tag}</TooltipTrigger>
              <TooltipContent>
                {info.description}
                {info.consequential ? " — asks first" : ""}
              </TooltipContent>
            </Tooltip>
          ) : (
            <span key={tool}>{tag}</span>
          );
        })}
        {hiddenCount > 0 && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span
                tabIndex={0}
                className="inline-flex h-7 items-center rounded-md border border-border bg-muted/35 px-2.5 text-xs text-muted-foreground focus-visible:outline-2 focus-visible:outline-ring"
              >
                +{hiddenCount}
              </span>
            </TooltipTrigger>
            <TooltipContent>
              {agent.tools.slice(MAX_VISIBLE_TOOLS).join(", ")}
            </TooltipContent>
          </Tooltip>
        )}
      </div>
    </header>
  );
}
