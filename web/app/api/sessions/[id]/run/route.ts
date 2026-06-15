const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8080";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function POST(
  request: Request,
  context: { params: Promise<{ id: string }> },
) {
  const { id } = await context.params;
  const body = await request.text();
  const upstream = await fetch(
    `${BACKEND_URL}/api/sessions/${encodeURIComponent(id)}/run`,
    {
      method: "POST",
      headers: {
        "Content-Type":
          request.headers.get("content-type") ?? "application/json",
        Accept: "text/event-stream",
      },
      body,
      cache: "no-store",
      signal: request.signal,
    },
  );

  const headers = new Headers();
  headers.set(
    "Content-Type",
    upstream.headers.get("content-type") ?? "text/event-stream",
  );
  headers.set("Cache-Control", "no-cache, no-transform");
  headers.set("X-Accel-Buffering", "no");
  headers.set("X-Agent-Builder-Route", "next-run-stream");

  return new Response(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers,
  });
}
