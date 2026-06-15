import type {
  Agent,
  AgentEditSession,
  AgentVersion,
  AnswerPayload,
  ApproveDecision,
  CreatedSession,
  Session,
} from "@/lib/types";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function parseErrorMessage(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string };
    if (body.error) return body.error;
  } catch {
    // non-JSON error body; fall through
  }
  return `request failed with status ${res.status}`;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });
  if (!res.ok) {
    throw new ApiError(res.status, await parseErrorMessage(res));
  }
  return (await res.json()) as T;
}

async function requestNoContent(path: string, init?: RequestInit): Promise<void> {
  const res = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });
  if (!res.ok) {
    throw new ApiError(res.status, await parseErrorMessage(res));
  }
}

export const api = {
  listAgents: () => request<Agent[]>("/api/agents"),

  getAgent: (id: string) =>
    request<Agent>(`/api/agents/${encodeURIComponent(id)}`),

  listAgentVersions: (agentId: string) =>
    request<AgentVersion[]>(
      `/api/agents/${encodeURIComponent(agentId)}/versions`,
    ),

  createSession: (agentId: string) =>
    request<CreatedSession>("/api/sessions", {
      method: "POST",
      body: JSON.stringify({ agent_id: agentId }),
    }),

  createAgentEditSession: (agentId: string) =>
    request<AgentEditSession>(
      `/api/agents/${encodeURIComponent(agentId)}/edit-sessions`,
      {
        method: "POST",
        body: JSON.stringify({}),
      },
    ),

  listAgentEditSessions: () =>
    request<AgentEditSession[]>("/api/agent-edit-sessions"),

  getSession: (id: string) =>
    request<Session>(`/api/sessions/${encodeURIComponent(id)}`),

  updateSessionTitle: (id: string, title: string) =>
    request<Session>(`/api/sessions/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify({ title }),
    }),

  approve: (sessionId: string, callId: string, decision: ApproveDecision) =>
    request<{ ok: true }>(
      `/api/sessions/${encodeURIComponent(sessionId)}/approve`,
      {
        method: "POST",
        body: JSON.stringify({ call_id: callId, decision }),
      },
    ),

  answer: (sessionId: string, callId: string, answer: AnswerPayload) =>
    request<{ ok: true }>(
      `/api/sessions/${encodeURIComponent(sessionId)}/answer`,
      {
        method: "POST",
        body: JSON.stringify({
          call_id: callId,
          option_id: answer.optionId ?? "",
          custom_text: answer.customText ?? "",
        }),
      },
    ),

  deleteAgent: (id: string) =>
    requestNoContent(`/api/agents/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),

  deleteSession: (id: string) =>
    requestNoContent(`/api/sessions/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),

  summarizeChatTitle: (prompt: string) =>
    request<{ title: string }>("/api/chat-title", {
      method: "POST",
      body: JSON.stringify({ prompt }),
    }),

  // The run endpoint streams SSE; callers consume the body via lib/sse.ts.
  // Returns the raw Response so the caller owns error handling + abort.
  run: (sessionId: string, message: string, signal: AbortSignal) =>
    fetch(`/api/sessions/${encodeURIComponent(sessionId)}/run`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ message }),
      signal,
    }),
};

export { parseErrorMessage };
