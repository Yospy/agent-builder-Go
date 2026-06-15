"use client";

import { useCallback, useEffect, useState } from "react";

import { ChatHeader } from "@/components/chat/chat-header";
import { Composer } from "@/components/chat/composer";
import { MessageList } from "@/components/chat/message-list";
import { useRun, type DisplayMessage } from "@/hooks/use-run";
import { api } from "@/lib/api";
import { fallbackChatTitle, normalizeChatTitle } from "@/lib/chat-title";
import { AGENTS_CHANGED } from "@/lib/events";
import { getRecentChats, touchRecentChat } from "@/lib/recent-chats";
import { BUILDER_AGENT_ID, type Agent } from "@/lib/types";

const namingChatTitles = new Set<string>();

export function ChatView({
  sessionId,
  agent,
  initialMessages,
}: {
  sessionId: string;
  agent: Agent;
  initialMessages: DisplayMessage[];
}) {
  const [draft, setDraft] = useState("");

  const nameChatFromPrompt = useCallback(
    (prompt: string) => {
      const chat = getRecentChats().find((c) => c.sessionId === sessionId);
      if (!chat || chat.title !== "New chat" || namingChatTitles.has(sessionId)) {
        return;
      }

      namingChatTitles.add(sessionId);
      void api
        .summarizeChatTitle(prompt)
        .then(({ title }) => {
          touchRecentChat(sessionId, {
            title: normalizeChatTitle(title, prompt),
          });
        })
        .catch(() => {
          touchRecentChat(sessionId, {
            title: fallbackChatTitle(prompt),
          });
        })
        .finally(() => {
          namingChatTitles.delete(sessionId);
        });
    },
    [sessionId],
  );

  useEffect(() => {
    const firstUserMessage = initialMessages.find(
      (message): message is Extract<DisplayMessage, { role: "user" }> =>
        message.role === "user",
    );
    if (firstUserMessage) {
      nameChatFromPrompt(firstUserMessage.content);
    }
  }, [initialMessages, nameChatFromPrompt]);

  const onTurnCommitted = useCallback(
    (userMessage: string) => {
      // First committed turn names the chat in the sidebar.
      const chat = getRecentChats().find((c) => c.sessionId === sessionId);
      if (chat && chat.title === "New chat") {
        nameChatFromPrompt(userMessage);
      } else {
        touchRecentChat(sessionId, {});
      }
      // A builder turn may have INSERTed an agent row — refresh the list.
      if (agent.id === BUILDER_AGENT_ID) {
        window.dispatchEvent(new Event(AGENTS_CHANGED));
      }
    },
    [sessionId, agent.id, nameChatFromPrompt],
  );

  // A rolled-back turn (§9) restores the message so retry is one keypress —
  // unless the user already started typing something new.
  const onRollback = useCallback((userMessage: string) => {
    setDraft((current) => (current.length > 0 ? current : userMessage));
  }, []);

  const { messages, status, send, stop, approve, answer } = useRun({
    sessionId,
    initialMessages,
    initialStatusMessage:
      agent.id === BUILDER_AGENT_ID
        ? "Preparing agent brief"
        : "Preparing response",
    onTurnCommitted,
    onRollback,
  });

  const handleSend = useCallback(
    (text: string) => {
      nameChatFromPrompt(text.trim());
      setDraft("");
      void send(text);
    },
    [send, nameChatFromPrompt],
  );

  return (
    <main className="flex h-full flex-1 flex-col overflow-hidden">
      <ChatHeader agent={agent} />
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
    </main>
  );
}
