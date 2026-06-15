import type { RunEvent } from "@/lib/types";

// Parses `data: <json>\n\n` SSE frames (§7) from a fetch ReadableStream.
// Tolerates \r\n line endings and multi-line frames; ignores non-data fields.
export async function* parseSSE(
  body: ReadableStream<Uint8Array>,
): AsyncGenerator<RunEvent, void, undefined> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });

      let boundary: number;
      while ((boundary = buffer.search(/\r?\n\r?\n/)) !== -1) {
        const frame = buffer.slice(0, boundary);
        buffer = buffer.slice(boundary).replace(/^\r?\n\r?\n/, "");
        const event = parseFrame(frame);
        if (event) yield event;
      }
    }
    // Flush any multi-byte character buffered in the decoder, then parse a
    // trailing frame without a final blank line (server closed early).
    buffer += decoder.decode();
    const event = parseFrame(buffer);
    if (event) yield event;
  } finally {
    // Cancel (not just release) so an early exit on a terminal event frees
    // the connection deterministically.
    try {
      await reader.cancel();
    } catch {
      // already closed/errored — nothing to free
    }
  }
}

function parseFrame(frame: string): RunEvent | null {
  const dataLines = frame
    .split(/\r?\n/)
    .filter((line) => line.startsWith("data:"))
    .map((line) => line.slice("data:".length).trim());
  if (dataLines.length === 0) return null;
  try {
    return JSON.parse(dataLines.join("\n")) as RunEvent;
  } catch (err) {
    if (process.env.NODE_ENV !== "production") {
      console.warn("Malformed SSE frame skipped", {
        error: err,
        frame: frame.slice(0, 500),
      });
    }
    return null; // malformed frame: skip rather than kill the stream
  }
}
