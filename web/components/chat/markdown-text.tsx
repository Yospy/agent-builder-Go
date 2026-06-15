"use client";

import type { ReactNode } from "react";

type Block =
  | { kind: "paragraph"; text: string }
  | { kind: "heading"; level: 2 | 3 | 4; text: string }
  | { kind: "ul"; items: string[] }
  | { kind: "ol"; items: string[] }
  | { kind: "quote"; text: string }
  | { kind: "code"; code: string; language?: string };

export function MarkdownText({ text }: { text: string }) {
  const blocks = parseBlocks(text);

  return (
    <div className="space-y-3 break-words text-sm leading-relaxed">
      {blocks.map((block, index) => {
        switch (block.kind) {
          case "heading": {
            const Tag = `h${block.level}` as "h2" | "h3" | "h4";
            return (
              <Tag key={index} className="font-semibold leading-snug">
                {renderInline(block.text)}
              </Tag>
            );
          }
          case "paragraph":
            return (
              <p key={index} className="whitespace-pre-wrap">
                {renderInline(block.text)}
              </p>
            );
          case "ul":
            return (
              <ul key={index} className="list-disc space-y-1 pl-5">
                {block.items.map((item, itemIndex) => (
                  <li key={itemIndex}>{renderInline(item)}</li>
                ))}
              </ul>
            );
          case "ol":
            return (
              <ol key={index} className="list-decimal space-y-1 pl-5">
                {block.items.map((item, itemIndex) => (
                  <li key={itemIndex}>{renderInline(item)}</li>
                ))}
              </ol>
            );
          case "quote":
            return (
              <blockquote
                key={index}
                className="border-l-2 pl-3 text-muted-foreground"
              >
                {renderInline(block.text)}
              </blockquote>
            );
          case "code":
            return (
              <pre
                key={index}
                className="overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs leading-relaxed"
              >
                <code>{block.code}</code>
              </pre>
            );
        }
      })}
    </div>
  );
}

function parseBlocks(markdown: string): Block[] {
  const lines = markdown.replace(/\r\n?/g, "\n").split("\n");
  const blocks: Block[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];
    if (line.trim() === "") {
      i += 1;
      continue;
    }

    const fence = line.match(/^```(\S*)\s*$/);
    if (fence) {
      const code: string[] = [];
      i += 1;
      while (i < lines.length && !lines[i].startsWith("```")) {
        code.push(lines[i]);
        i += 1;
      }
      if (i < lines.length) i += 1;
      blocks.push({
        kind: "code",
        code: code.join("\n"),
        language: fence[1] || undefined,
      });
      continue;
    }

    const heading = line.match(/^(#{1,3})\s+(.+)$/);
    if (heading) {
      blocks.push({
        kind: "heading",
        level: (heading[1].length + 1) as 2 | 3 | 4,
        text: heading[2],
      });
      i += 1;
      continue;
    }

    const quote = line.match(/^>\s?(.*)$/);
    if (quote) {
      const parts = [quote[1]];
      i += 1;
      while (i < lines.length) {
        const next = lines[i].match(/^>\s?(.*)$/);
        if (!next) break;
        parts.push(next[1]);
        i += 1;
      }
      blocks.push({ kind: "quote", text: parts.join(" ") });
      continue;
    }

    if (/^\s*[-*]\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length) {
        const item = lines[i].match(/^\s*[-*]\s+(.+)$/);
        if (!item) break;
        items.push(item[1]);
        i += 1;
      }
      blocks.push({ kind: "ul", items });
      continue;
    }

    if (/^\s*\d+\.\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length) {
        const item = lines[i].match(/^\s*\d+\.\s+(.+)$/);
        if (!item) break;
        items.push(item[1]);
        i += 1;
      }
      blocks.push({ kind: "ol", items });
      continue;
    }

    const paragraph: string[] = [line.trim()];
    i += 1;
    while (
      i < lines.length &&
      lines[i].trim() !== "" &&
      !isBlockStart(lines[i])
    ) {
      paragraph.push(lines[i].trim());
      i += 1;
    }
    blocks.push({ kind: "paragraph", text: paragraph.join(" ") });
  }

  return blocks;
}

function isBlockStart(line: string) {
  return (
    /^```/.test(line) ||
    /^#{1,3}\s+/.test(line) ||
    /^>\s?/.test(line) ||
    /^\s*[-*]\s+/.test(line) ||
    /^\s*\d+\.\s+/.test(line)
  );
}

function renderInline(text: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  let rest = text;
  let key = 0;

  while (rest.length > 0) {
    const match = findNextInline(rest);
    if (!match) {
      nodes.push(rest);
      break;
    }

    if (match.index > 0) {
      nodes.push(rest.slice(0, match.index));
    }

    nodes.push(renderToken(match, key++));
    rest = rest.slice(match.index + match.raw.length);
  }

  return nodes;
}

type InlineMatch =
  | { type: "code"; index: number; raw: string; text: string }
  | { type: "bold"; index: number; raw: string; text: string }
  | { type: "italic"; index: number; raw: string; text: string }
  | { type: "link"; index: number; raw: string; text: string; href: string };

function findNextInline(text: string): InlineMatch | null {
  const patterns: InlineMatch[] = [];
  const code = /`([^`]+)`/.exec(text);
  if (code?.index !== undefined) {
    patterns.push({
      type: "code",
      index: code.index,
      raw: code[0],
      text: code[1],
    });
  }

  const bold = /\*\*([^*]+)\*\*/.exec(text);
  if (bold?.index !== undefined) {
    patterns.push({
      type: "bold",
      index: bold.index,
      raw: bold[0],
      text: bold[1],
    });
  }

  const link = /\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/.exec(text);
  if (link?.index !== undefined) {
    patterns.push({
      type: "link",
      index: link.index,
      raw: link[0],
      text: link[1],
      href: link[2],
    });
  }

  const italic = /(?<!\*)\*([^*]+)\*(?!\*)/.exec(text);
  if (italic?.index !== undefined) {
    patterns.push({
      type: "italic",
      index: italic.index,
      raw: italic[0],
      text: italic[1],
    });
  }

  return patterns.sort((a, b) => a.index - b.index)[0] ?? null;
}

function renderToken(token: InlineMatch, key: number): ReactNode {
  switch (token.type) {
    case "code":
      return (
        <code key={key} className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
          {token.text}
        </code>
      );
    case "bold":
      return <strong key={key}>{renderInline(token.text)}</strong>;
    case "italic":
      return <em key={key}>{renderInline(token.text)}</em>;
    case "link":
      return (
        <a
          key={key}
          href={token.href}
          target="_blank"
          rel="noreferrer"
          className="underline underline-offset-2"
        >
          {renderInline(token.text)}
        </a>
      );
  }
}
