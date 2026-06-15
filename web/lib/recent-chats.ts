// The contract has no list-sessions endpoint, so the sidebar's CHATS section
// is backed by localStorage: we remember every session this browser minted.
// Exposed as an external store (subscribe/snapshot) for useSyncExternalStore.

export interface RecentChat {
  sessionId: string;
  agentId: string;
  agentName: string;
  title: string;
  updatedAt: number;
}

const STORAGE_KEY = "agent-builder:recent-chats";
const MAX_RECENT_CHATS = 30;
const CHANGED_EVENT = "agent-builder:recent-chats-changed";

const EMPTY: RecentChat[] = [];
let cache: RecentChat[] | null = null;

function read(): RecentChat[] {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return EMPTY;
    const parsed = JSON.parse(raw) as RecentChat[];
    return Array.isArray(parsed) ? parsed : EMPTY;
  } catch {
    return EMPTY;
  }
}

export function getRecentChats(): RecentChat[] {
  if (typeof window === "undefined") return EMPTY;
  if (cache === null) cache = read();
  return cache;
}

export function getRecentChatsServerSnapshot(): RecentChat[] {
  return EMPTY;
}

export function subscribeRecentChats(onChange: () => void): () => void {
  window.addEventListener(CHANGED_EVENT, onChange);
  return () => window.removeEventListener(CHANGED_EVENT, onChange);
}

export function upsertRecentChat(chat: RecentChat): void {
  if (typeof window === "undefined") return;
  const rest = getRecentChats().filter((c) => c.sessionId !== chat.sessionId);
  cache = [chat, ...rest].slice(0, MAX_RECENT_CHATS);
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(cache));
  } catch {
    // storage full/unavailable — recent chats are a convenience, not state
  }
  window.dispatchEvent(new Event(CHANGED_EVENT));
}

export function touchRecentChat(
  sessionId: string,
  patch: Partial<Pick<RecentChat, "title" | "updatedAt">>,
): void {
  const existing = getRecentChats().find((c) => c.sessionId === sessionId);
  if (!existing) return;
  upsertRecentChat({ ...existing, ...patch, updatedAt: Date.now() });
}

function writeRecentChats(chats: RecentChat[]): void {
  if (typeof window === "undefined") return;
  cache = chats.slice(0, MAX_RECENT_CHATS);
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(cache));
  } catch {
    // storage full/unavailable — recent chats are a convenience, not state
  }
  window.dispatchEvent(new Event(CHANGED_EVENT));
}

export function removeRecentChat(sessionId: string): void {
  if (typeof window === "undefined") return;
  writeRecentChats(getRecentChats().filter((c) => c.sessionId !== sessionId));
}

export function removeRecentChatsForAgent(agentId: string): void {
  if (typeof window === "undefined") return;
  writeRecentChats(getRecentChats().filter((c) => c.agentId !== agentId));
}
