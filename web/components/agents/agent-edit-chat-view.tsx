"use client";

import { useCallback, useEffect, useState } from "react";

import { AgentContextPanel } from "@/components/agents/agent-context-panel";
import { ChatHeader } from "@/components/chat/chat-header";
import { Composer } from "@/components/chat/composer";
import { MessageList } from "@/components/chat/message-list";
import { useRun, type DisplayMessage } from "@/hooks/use-run";
import { api } from "@/lib/api";
import { fallbackChatTitle, normalizeChatTitle } from "@/lib/chat-title";
import { AGENTS_CHANGED, EDIT_SESSIONS_CHANGED } from "@/lib/events";
import type { Agent, AgentVersion, Session } from "@/lib/types";

const namingEditChats = new Set<string>();

export function AgentEditChatView({
  session,
  agent: initialAgent,
  versions: initialVersions,
  initialMessages,
}: {
  session: Session;
  agent: Agent;
  versions: AgentVersion[];
  initialMessages: DisplayMessage[];
}) {
  const [draft, setDraft] = useState("");
  const [agent, setAgent] = useState(initialAgent);
  const [versions, setVersions] = useState(initialVersions);
  const [title, setTitle] = useState(session.title);

  const refreshAgentContext = useCallback(async () => {
    const [freshAgent, freshVersions] = await Promise.all([
      api.getAgent(agent.id),
      api.listAgentVersions(agent.id),
    ]);
    setAgent(freshAgent);
    setVersions(freshVersions);
    window.dispatchEvent(new Event(AGENTS_CHANGED));
  }, [agent.id]);

  const nameEditChatFromPrompt = useCallback(
    (prompt: string) => {
      if (title !== "New chat" || namingEditChats.has(session.id)) return;
      namingEditChats.add(session.id);
      void api
        .summarizeChatTitle(prompt)
        .then(({ title: summarized }) =>
          api.updateSessionTitle(
            session.id,
            normalizeChatTitle(summarized, prompt),
          ),
        )
        .catch(() =>
          api.updateSessionTitle(session.id, fallbackChatTitle(prompt)),
        )
        .then((updated) => {
          setTitle(updated.title);
          window.dispatchEvent(new Event(EDIT_SESSIONS_CHANGED));
        })
        .finally(() => {
          namingEditChats.delete(session.id);
        });
    },
    [session.id, title],
  );

  useEffect(() => {
    const firstUserMessage = initialMessages.find(
      (message): message is Extract<DisplayMessage, { role: "user" }> =>
        message.role === "user",
    );
    if (firstUserMessage) {
      nameEditChatFromPrompt(firstUserMessage.content);
    }
  }, [initialMessages, nameEditChatFromPrompt]);

  const onTurnCommitted = useCallback(
    (userMessage: string) => {
      if (title === "New chat") {
        nameEditChatFromPrompt(userMessage);
      } else {
        window.dispatchEvent(new Event(EDIT_SESSIONS_CHANGED));
      }
      void refreshAgentContext();
    },
    [nameEditChatFromPrompt, refreshAgentContext, title],
  );

  const onRollback = useCallback((userMessage: string) => {
    setDraft((current) => (current.length > 0 ? current : userMessage));
  }, []);

  const { messages, status, send, stop, approve, answer } = useRun({
    sessionId: session.id,
    initialMessages,
    initialStatusMessage: "Preparing edit",
    onTurnCommitted,
    onRollback,
  });

  const handleSend = useCallback(
    (text: string) => {
      nameEditChatFromPrompt(text.trim());
      setDraft("");
      void send(text);
    },
    [nameEditChatFromPrompt, send],
  );

  return (
    <main className="flex h-full flex-1 overflow-hidden">
      <section className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <ChatHeader agent={agent} mode="edit" />
        <MessageList
          messages={messages}
          agent={agent}
          onDecision={approve}
          onAnswer={answer}
        />
        <Composer
          agentName={agent.name}
          status={status}
          value={draft}
          onChange={setDraft}
          onSend={handleSend}
          onStop={stop}
        />
      </section>
      <AgentContextPanel agent={agent} versions={versions} />
    </main>
  );
}
