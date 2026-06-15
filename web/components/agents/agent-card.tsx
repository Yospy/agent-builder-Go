"use client";

import { Loader2Icon, Trash2Icon } from "lucide-react";

import { AgentAvatar } from "@/components/agents/agent-avatar";
import { DeleteConfirmPopover } from "@/components/delete-confirm-popover";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { BUILDER_AGENT_ID, type Agent } from "@/lib/types";

const MAX_VISIBLE_TOOLS = 4;

export function AgentCard({
  agent,
  onStartChat,
  onStartEdit,
  onDelete,
  starting,
  startingEdit,
}: {
  agent: Agent;
  onStartChat: (agentId: string, agentName: string) => void;
  onStartEdit: (agentId: string) => void;
  onDelete?: (agent: Agent) => Promise<boolean>;
  starting: boolean;
  startingEdit: boolean;
}) {
  const visibleTools = agent.tools.slice(0, MAX_VISIBLE_TOOLS);
  const hiddenCount = agent.tools.length - visibleTools.length;
  const canDelete = agent.id !== BUILDER_AGENT_ID && Boolean(onDelete);

  return (
    <Card className="h-full min-h-52 gap-0 rounded-lg p-0 transition-colors hover:bg-accent/20">
      <div className="flex flex-1 flex-col">
        <div className="flex items-start gap-3 px-4 pb-3 pt-4">
        <AgentAvatar
          name={agent.name}
          className="size-10"
          filled={agent.id === BUILDER_AGENT_ID}
        />
        <div className="min-w-0 flex-1">
          <p className="truncate text-base font-semibold leading-6">
            {agent.name}
          </p>
          <p className="truncate font-mono text-xs leading-5 text-muted-foreground">
            {agent.model}
          </p>
        </div>
        {starting && (
          <Loader2Icon className="size-4 animate-spin text-muted-foreground" />
        )}
        </div>

        <div className="px-4">
        {agent.persona ? (
          <p className="line-clamp-2 text-sm leading-5 text-muted-foreground">
            {agent.persona}
          </p>
        ) : (
          <p className="text-sm leading-5 text-muted-foreground">
            No persona set
          </p>
        )}
        </div>

        <div className="px-4 py-4">
        {agent.tools.length > 0 ? (
          <div className="flex flex-wrap gap-1.5">
            {visibleTools.map((tool) => (
              <Badge
                key={tool}
                variant="outline"
                className="h-6 rounded-md px-2 font-mono text-[11px] text-muted-foreground"
              >
                {tool}
              </Badge>
            ))}
            {hiddenCount > 0 && (
              <Badge
                variant="outline"
                className="h-6 rounded-md px-2 text-[11px] text-muted-foreground"
              >
                +{hiddenCount}
              </Badge>
            )}
          </div>
        ) : (
          <span className="text-xs text-muted-foreground">No tools</span>
        )}
        </div>
      </div>

      <div className="mt-auto flex h-14 items-center gap-2 border-t bg-muted/20 px-4">
        <Button
          size="sm"
          variant="outline"
          className="h-9 flex-1 justify-center"
          disabled={starting || startingEdit}
          onClick={() => onStartChat(agent.id, agent.name)}
        >
          {starting ? (
            <Loader2Icon className="size-3.5 animate-spin" />
          ) : null}
          Chat
        </Button>
        <Button
          size="sm"
          variant="outline"
          className="h-9 flex-1 justify-center"
          disabled={starting || startingEdit}
          onClick={() => onStartEdit(agent.id)}
        >
          {startingEdit ? (
            <Loader2Icon className="size-3.5 animate-spin" />
          ) : null}
          Edit Agent
        </Button>

        {canDelete && onDelete && (
          <DeleteConfirmPopover
            title={`Delete ${agent.name}?`}
            description="This permanently deletes the agent and all chats for it. Audit logs on disk are left alone."
            confirmLabel="Delete agent"
            disabled={starting}
            onConfirm={() => onDelete(agent)}
          >
            <Button
              size="icon-sm"
              variant="destructive"
              aria-label={`Delete ${agent.name}`}
              disabled={starting}
            >
              <Trash2Icon className="size-3.5" />
            </Button>
          </DeleteConfirmPopover>
        )}
      </div>
    </Card>
  );
}
