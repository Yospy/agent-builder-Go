// Mirrors the locked contract in docs/00-CONTEXT.md (§7 events, §12 endpoints).

export interface Agent {
  id: string;
  name: string;
  persona?: string;
  instructions?: string;
  model: string;
  tools: string[];
  sources: string[];
  created_at: number;
  updated_at: number;
}

// Session history is the backend's normalized OpenAI message log (observed in
// integration): plain user/assistant text, plus assistant tool_calls entries
// and role:"tool" results keyed by tool_call_id.
export interface ChatMessage {
  role: "user" | "assistant" | "tool" | "system";
  content?: string;
  tool_calls?: HistoryToolCall[];
  tool_call_id?: string;
  name?: string;
}

export interface HistoryToolCall {
  id: string;
  name: string;
  args: Record<string, unknown>;
}

export interface Session {
  id: string;
  agent_id: string;
  kind: "normal" | "agent_edit";
  title: string;
  messages: ChatMessage[];
  created_at: number;
  updated_at: number;
}

export interface CreatedSession {
  session_id: string;
  agent_id: string;
}

export interface AgentEditSession {
  id: string;
  agent_id: string;
  kind: "agent_edit";
  title: string;
  messages: ChatMessage[];
  created_at: number;
  updated_at: number;
}

export interface AgentVersion {
  id: string;
  agent_id: string;
  session_id: string;
  version_number: number;
  snapshot_json: string;
  change_summary: string;
  created_at: number;
}

export type ApproveDecision = "approve" | "deny";

export interface QuestionOption {
  id: string;
  label: string;
  description?: string;
  value: string;
}

export interface QuestionProgress {
  label?: string;
  current?: number;
  total?: number;
}

export interface AnswerPayload {
  optionId?: string;
  customText?: string;
}

// SSE event union (§7). `seq` orders events within one invocation.
export type RunEvent =
  | { type: "llm_text"; seq: number; text: string }
  | { type: "status"; seq: number; message: string }
  | {
      type: "tool_use";
      seq: number;
      call_id: string;
      name: string;
      args: Record<string, unknown>;
    }
  | {
      type: "tool_result";
      seq: number;
      call_id: string;
      name: string;
      ok: boolean;
      data: string;
    }
  | {
      type: "confirm";
      seq: number;
      call_id: string;
      name: string;
      args: Record<string, unknown>;
    }
  | {
      type: "user_question";
      seq: number;
      call_id: string;
      name: string;
      field: string;
      question: string;
      options: QuestionOption[];
      allow_custom: boolean;
      custom_placeholder?: string;
      progress?: QuestionProgress;
    }
  | { type: "done"; seq: number; text: string }
  | { type: "error"; seq: number; message: string }
  | { type: "aborted"; seq: number };

export const MAX_MESSAGE_LENGTH = 10_000; // mirrors the server-side validator

export const BUILDER_AGENT_ID = "builder";
