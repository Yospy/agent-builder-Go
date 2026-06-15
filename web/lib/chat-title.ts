const MAX_TITLE_WORDS = 3;

export function normalizeChatTitle(title: string, fallbackPrompt: string): string {
  const words = titleWords(title);
  if (words.length > 0) return words.slice(0, MAX_TITLE_WORDS).join(" ");
  return fallbackChatTitle(fallbackPrompt);
}

export function fallbackChatTitle(prompt: string): string {
  const words = titleWords(prompt);
  if (words.length === 0) return "New chat";
  return words.slice(0, MAX_TITLE_WORDS).join(" ");
}

export function isCompactChatTitle(title: string): boolean {
  return titleWords(title).length <= MAX_TITLE_WORDS;
}

function titleWords(input: string): string[] {
  return Array.from(input.matchAll(/[\p{L}\p{N}]+/gu), (match) => match[0]);
}
