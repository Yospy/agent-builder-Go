"use client";

import { usePathname, useRouter } from "next/navigation";
import Link from "next/link";
import { useEffect, useState, useSyncExternalStore } from "react";
import { AnimatePresence, motion } from "motion/react";
import { useTheme } from "next-themes";
import {
  BrainCircuitIcon,
  CommandIcon,
  Edit3Icon,
  MessageSquareIcon,
  MoonIcon,
  PlusIcon,
  SunIcon,
  Trash2Icon,
} from "lucide-react";
import { toast } from "sonner";

import { DeleteConfirmPopover } from "@/components/delete-confirm-popover";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useStartChat } from "@/hooks/use-start-chat";
import { useStartAgentEdit } from "@/hooks/use-start-agent-edit";
import { api, ApiError } from "@/lib/api";
import {
  fallbackChatTitle,
  isCompactChatTitle,
  normalizeChatTitle,
} from "@/lib/chat-title";
import {
  getRecentChats,
  getRecentChatsServerSnapshot,
  removeRecentChat,
  subscribeRecentChats,
  touchRecentChat,
  type RecentChat,
} from "@/lib/recent-chats";
import { AGENTS_CHANGED, EDIT_SESSIONS_CHANGED } from "@/lib/events";
import { BUILDER_AGENT_ID, type Agent, type AgentEditSession } from "@/lib/types";
import { cn } from "@/lib/utils";

const compactingChatTitles = new Set<string>();

export function AppSidebar() {
  const pathname = usePathname();
  const { startChat, startingAgentId } = useStartChat();
  const { startAgentEdit, startingEditAgentId } = useStartAgentEdit();
  const [agents, setAgents] = useState<Agent[] | null>(null);
  const [editSessions, setEditSessions] = useState<AgentEditSession[] | null>(
    null,
  );
  const [agentsError, setAgentsError] = useState(false);
  const [editSessionsError, setEditSessionsError] = useState(false);
  const recentChats = useSyncExternalStore(
    subscribeRecentChats,
    getRecentChats,
    getRecentChatsServerSnapshot,
  );

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      api
        .listAgents()
        .then((list) => {
          if (cancelled) return;
          setAgents(list);
          setAgentsError(false);
        })
        .catch(() => {
          if (cancelled) return;
          setAgents([]);
          setAgentsError(true);
        });
    };
    load();
    window.addEventListener(AGENTS_CHANGED, load);
    return () => {
      cancelled = true;
      window.removeEventListener(AGENTS_CHANGED, load);
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      api
        .listAgentEditSessions()
        .then((list) => {
          if (cancelled) return;
          setEditSessions(list);
          setEditSessionsError(false);
        })
        .catch(() => {
          if (cancelled) return;
          setEditSessions([]);
          setEditSessionsError(true);
        });
    };
    load();
    window.addEventListener(EDIT_SESSIONS_CHANGED, load);
    window.addEventListener(AGENTS_CHANGED, load);
    return () => {
      cancelled = true;
      window.removeEventListener(EDIT_SESSIONS_CHANGED, load);
      window.removeEventListener(AGENTS_CHANGED, load);
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    for (const chat of recentChats) {
      if (
        chat.title === "New chat" ||
        isCompactChatTitle(chat.title) ||
        compactingChatTitles.has(chat.sessionId)
      ) {
        continue;
      }

      compactingChatTitles.add(chat.sessionId);
      void api
        .summarizeChatTitle(chat.title)
        .then(({ title }) => {
          if (cancelled) return;
          touchRecentChat(chat.sessionId, {
            title: normalizeChatTitle(title, chat.title),
          });
        })
        .catch(() => {
          if (cancelled) return;
          touchRecentChat(chat.sessionId, {
            title: fallbackChatTitle(chat.title),
          });
        })
        .finally(() => {
          compactingChatTitles.delete(chat.sessionId);
        });
    }
    return () => {
      cancelled = true;
    };
  }, [recentChats]);

  const activeSessionId = pathname.startsWith("/chat/")
    ? pathname.slice("/chat/".length)
    : null;
  const activeEditSessionId = pathname.includes("/edit/")
    ? pathname.split("/edit/")[1] ?? null
    : null;
  const visibleAgents =
    agents?.filter((agent) => agent.id !== BUILDER_AGENT_ID) ?? [];
  const editSessionsByAgent = new Map<string, AgentEditSession[]>();
  for (const session of editSessions ?? []) {
    const existing = editSessionsByAgent.get(session.agent_id) ?? [];
    existing.push(session);
    editSessionsByAgent.set(session.agent_id, existing);
  }

  return (
    <aside className="hidden h-full w-72 shrink-0 flex-col border-r bg-[linear-gradient(180deg,var(--sidebar),color-mix(in_oklch,var(--sidebar),var(--foreground)_3%))] text-sidebar-foreground md:flex">
      <div className="flex h-16 shrink-0 items-center gap-2 border-b px-4">
        <Link
          href="/"
          className="flex min-w-0 items-center gap-2 rounded-lg focus-visible:outline-2 focus-visible:outline-ring"
        >
          <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground shadow-sm">
            <BrainCircuitIcon className="size-5" />
          </span>
          <span className="min-w-0">
            <span className="block truncate text-sm font-semibold">
              Agent Builder
            </span>
            <span className="block text-[11px] leading-none text-muted-foreground">
              Workshop
            </span>
          </span>
        </Link>
      </div>

      <div className="border-b p-3">
        <Button
          className="h-10 w-full justify-start gap-2 rounded-lg"
          disabled={startingAgentId !== null}
          onClick={() =>
            startChat(BUILDER_AGENT_ID, "Agent Builder", {
              addToRecentChats: false,
            })
          }
        >
          <PlusIcon className="size-4" />
          New agent
        </Button>
      </div>

      <ScrollArea className="flex-1 px-2">
        <SectionLabel>Agents</SectionLabel>
        {agents === null ? (
          <div className="space-y-1 px-1 py-1">
            <Skeleton className="h-8 w-full rounded-md" />
            <Skeleton className="h-8 w-full rounded-md" />
            <Skeleton className="h-8 w-full rounded-md" />
          </div>
        ) : agentsError ? (
          <p className="px-2 py-1.5 text-xs text-muted-foreground">
            Backend unreachable
          </p>
        ) : visibleAgents.length === 0 ? (
          <p className="px-2 py-1.5 text-xs text-muted-foreground">
            No agents yet
          </p>
        ) : (
          <nav className="flex flex-col gap-2">
            {/* New agents slide in: the INSERT made visible. */}
            <AnimatePresence initial={false}>
              {visibleAgents.map((agent) => (
                <motion.div
                  key={agent.id}
                  initial={{ opacity: 0, x: -8 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ duration: 0.2, ease: "easeOut" }}
                  className="rounded-lg"
                >
                  <div className="px-2 py-1.5 text-xs font-medium text-muted-foreground">
                    <span className="block truncate">{agent.name}</span>
                  </div>
                  <button
                    type="button"
                    disabled={startingEditAgentId !== null}
                    onClick={() => startAgentEdit(agent.id)}
                    className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent focus-visible:outline-2 focus-visible:outline-ring disabled:text-muted-foreground"
                  >
                    <Edit3Icon className="size-3.5 shrink-0" />
                    <span className="block truncate">
                      {startingEditAgentId === agent.id
                        ? "Opening edit chat"
                        : "Edit Agent"}
                    </span>
                  </button>
                  {(editSessionsByAgent.get(agent.id) ?? []).map((session) => (
                    <EditSessionRow
                      key={session.id}
                      agentId={agent.id}
                      session={session}
                      active={session.id === activeEditSessionId}
                    />
                  ))}
                </motion.div>
              ))}
            </AnimatePresence>
          </nav>
        )}
        {editSessionsError && (
          <p className="px-2 py-1.5 text-xs text-muted-foreground">
            Edit chats unavailable
          </p>
        )}

        {recentChats.length > 0 && (
          <>
            <SectionLabel>Chats</SectionLabel>
            <nav className="flex flex-col gap-1 pb-3">
              {recentChats.map((chat) => (
                <ChatRow
                  key={chat.sessionId}
                  chat={chat}
                  active={chat.sessionId === activeSessionId}
                />
              ))}
            </nav>
          </>
        )}
      </ScrollArea>

      <div className="flex h-14 shrink-0 items-center justify-between gap-3 border-t px-3">
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 truncate text-xs font-medium">
            <CommandIcon className="size-3" />
            Local workspace
          </p>
          <p className="truncate text-[11px] text-muted-foreground">:8080</p>
        </div>
        <ThemeToggle />
      </div>
    </aside>
  );
}

function EditSessionRow({
  agentId,
  session,
  active,
}: {
  agentId: string;
  session: AgentEditSession;
  active: boolean;
}) {
  const router = useRouter();

  const removeFromList = () => {
    window.dispatchEvent(new Event(EDIT_SESSIONS_CHANGED));
    if (active) router.replace("/");
  };

  const deleteEditSession = async (): Promise<boolean> => {
    try {
      await api.deleteSession(session.id);
      removeFromList();
      return true;
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        removeFromList();
        return true;
      }
      if (err instanceof ApiError && err.status === 409) {
        toast("Run in flight. Retry after it finishes.");
        return false;
      }
      if (err instanceof ApiError && err.status === 405) {
        toast.error(
          "Delete is not available on the running backend. Restart the Go server.",
        );
        return false;
      }
      toast.error(
        err instanceof Error ? err.message : "could not delete edit chat",
      );
      return false;
    }
  };

  return (
    <div
      className={cn(
        "group/edit-chat ml-4 grid h-8 grid-cols-[minmax(0,1fr)_1.75rem] items-center rounded-md text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent focus-within:bg-sidebar-accent",
        active && "bg-sidebar-accent",
      )}
    >
      <Link
        href={`/agents/${agentId}/edit/${session.id}`}
        title={session.title}
        className="flex h-8 min-w-0 items-center gap-2 rounded-md pl-2 pr-1 focus-visible:outline-2 focus-visible:outline-ring"
      >
        <MessageSquareIcon className="size-3.5 shrink-0 text-muted-foreground" />
        <span className="block min-w-0 truncate">{session.title}</span>
      </Link>
      <DeleteConfirmPopover
        title="Delete this edit chat?"
        description="This permanently deletes this edit chat. Applied agent config changes stay on the agent."
        confirmLabel="Delete chat"
        onConfirm={deleteEditSession}
      >
        <Button
          size="icon-xs"
          variant="ghost"
          aria-label={`Delete ${session.title}`}
          className="mr-1 text-muted-foreground opacity-100 hover:bg-destructive/15 hover:text-destructive focus-visible:text-destructive"
        >
          <Trash2Icon className="size-3.5" />
        </Button>
      </DeleteConfirmPopover>
    </div>
  );
}

function ChatRow({ chat, active }: { chat: RecentChat; active: boolean }) {
  const router = useRouter();

  const removeLocally = () => {
    removeRecentChat(chat.sessionId);
    if (active) router.replace("/");
  };

  const deleteChat = async (): Promise<boolean> => {
    try {
      await api.deleteSession(chat.sessionId);
      removeLocally();
      return true;
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        removeLocally();
        return true;
      }
      if (err instanceof ApiError && err.status === 409) {
        toast("Run in flight. Retry after it finishes.");
        return false;
      }
      if (err instanceof ApiError && err.status === 405) {
        toast.error(
          "Delete is not available on the running backend. Restart the Go server.",
        );
        return false;
      } else {
        toast.error(err instanceof Error ? err.message : "could not delete chat");
      }
      return false;
    }
  };

  return (
    <div
      className={cn(
        "group/chat-row grid h-8 grid-cols-[minmax(0,1fr)_2rem] items-center rounded-md text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent focus-within:bg-sidebar-accent",
        active && "bg-sidebar-accent",
      )}
    >
      <Link
        href={`/chat/${chat.sessionId}`}
        title={chat.title}
        className="flex h-8 min-w-0 items-center rounded-md pl-2 pr-1 focus-visible:outline-2 focus-visible:outline-ring"
      >
        <span className="block min-w-0 truncate">{chat.title}</span>
      </Link>
      <DeleteConfirmPopover
        title="Delete this chat?"
        description="This permanently deletes this chat from live state. The agent and audit logs are not deleted."
        confirmLabel="Delete chat"
        onConfirm={deleteChat}
      >
        <Button
          size="icon-xs"
          variant="ghost"
          aria-label={`Delete ${chat.title}`}
          className="mr-1 text-muted-foreground opacity-100 hover:bg-destructive/15 hover:text-destructive focus-visible:text-destructive"
        >
          <Trash2Icon className="size-3.5" />
        </Button>
      </DeleteConfirmPopover>
    </div>
  );
}

function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme();

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          size="icon-sm"
          variant="outline"
          aria-label="Toggle theme"
          onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
        >
          <SunIcon className="size-3.5 dark:hidden" />
          <MoonIcon className="hidden size-3.5 dark:block" />
        </Button>
      </TooltipTrigger>
      <TooltipContent>Toggle theme</TooltipContent>
    </Tooltip>
  );
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <p className="px-2 pb-2 pt-5 text-[11px] font-medium uppercase tracking-wider text-muted-foreground first:pt-3">
      {children}
    </p>
  );
}
