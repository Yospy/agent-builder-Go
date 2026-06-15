"use client";

import { useMemo, useState } from "react";
import type React from "react";
import { FileTextIcon, WrenchIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { Agent, AgentVersion } from "@/lib/types";

export function AgentContextPanel({
  agent,
  versions,
}: {
  agent: Agent;
  versions: AgentVersion[];
}) {
  const [tab, setTab] = useState<"config" | "versions">("config");
  const tools = agent.tools ?? [];
  const sources = agent.sources ?? [];
  const promptPreview = useMemo(() => {
    const text = agent.instructions?.trim() || agent.persona?.trim() || "";
    return text.length > 420 ? `${text.slice(0, 420)}...` : text;
  }, [agent.instructions, agent.persona]);

  return (
    <aside className="hidden w-80 shrink-0 border-l bg-background/95 xl:block">
      <div className="sticky top-0 h-full max-h-screen overflow-y-auto p-4">
        <div className="rounded-lg border bg-card p-3">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold">{agent.name}</p>
              <p className="truncate font-mono text-xs text-muted-foreground">
                {agent.model}
              </p>
            </div>
          </div>

          <div className="mt-3 grid grid-cols-2 gap-1 rounded-md bg-muted p-1">
            <Button
              type="button"
              size="sm"
              variant={tab === "config" ? "secondary" : "ghost"}
              className="h-7 rounded"
              onClick={() => setTab("config")}
            >
              Config
            </Button>
            <Button
              type="button"
              size="sm"
              variant={tab === "versions" ? "secondary" : "ghost"}
              className="h-7 rounded"
              onClick={() => setTab("versions")}
            >
              Versions
            </Button>
          </div>

          {tab === "config" ? (
            <div className="mt-4 space-y-4">
              <PanelSection icon={<WrenchIcon className="size-3.5" />} title="Tools">
                {tools.length > 0 ? (
                  <div className="flex flex-wrap gap-1.5">
                    {tools.map((tool) => (
                      <Badge
                        key={tool}
                        variant="outline"
                        className="rounded-md font-mono text-[11px]"
                      >
                        {tool}
                      </Badge>
                    ))}
                  </div>
                ) : (
                  <EmptyText>No tools</EmptyText>
                )}
              </PanelSection>

              <PanelSection
                icon={<FileTextIcon className="size-3.5" />}
                title="Sources"
              >
                {sources.length > 0 ? (
                  <div className="space-y-1">
                    {sources.map((source) => (
                      <p
                        key={source}
                        className="truncate rounded-md border bg-muted/30 px-2 py-1.5 text-xs text-muted-foreground"
                        title={source}
                      >
                        {source}
                      </p>
                    ))}
                  </div>
                ) : (
                  <EmptyText>No sources yet</EmptyText>
                )}
              </PanelSection>

              <PanelSection title="Prompt preview">
                {promptPreview ? (
                  <p className="max-h-48 overflow-y-auto whitespace-pre-wrap rounded-md border bg-muted/30 p-2 text-xs leading-5 text-muted-foreground">
                    {promptPreview}
                  </p>
                ) : (
                  <EmptyText>No prompt text</EmptyText>
                )}
              </PanelSection>
            </div>
          ) : (
            <div className="mt-4 space-y-2">
              {versions.length === 0 ? (
                <EmptyText>No applied versions yet</EmptyText>
              ) : (
                versions.map((version) => (
                  <div
                    key={version.id}
                    className="rounded-md border bg-muted/20 p-2 text-xs"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-medium">
                        Version {version.version_number}
                      </span>
                      <span className="text-muted-foreground">
                        {formatDate(version.created_at)}
                      </span>
                    </div>
                    <p className="mt-1 text-muted-foreground">
                      {version.change_summary || "Agent updated"}
                    </p>
                  </div>
                ))
              )}
            </div>
          )}
        </div>
      </div>
    </aside>
  );
}

function PanelSection({
  icon,
  title,
  children,
}: {
  icon?: React.ReactNode;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section>
      <div className="mb-2 flex items-center gap-1.5 text-[11px] font-medium uppercase text-muted-foreground">
        {icon}
        <span>{title}</span>
      </div>
      {children}
    </section>
  );
}

function EmptyText({ children }: { children: React.ReactNode }) {
  return <p className="text-xs text-muted-foreground">{children}</p>;
}

function formatDate(seconds: number): string {
  if (!seconds) return "";
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(new Date(seconds * 1000));
}
