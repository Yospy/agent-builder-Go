import type { NextConfig } from "next";

const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  experimental: {
    // The dev proxy kills upstream connections after 30s by default, which
    // aborts SSE runs paused on a confirm gate (no bytes flow while waiting
    // for human approval). 10 minutes covers any realistic decision.
    proxyTimeout: 600_000,
  },
  async rewrites() {
    return {
      // Filesystem App Router API routes must win first. In particular,
      // /api/sessions/:id/run has a dedicated streaming route so paused SSE
      // question flows are not handled by the generic rewrite proxy.
      fallback: [
        {
          source: "/api/:path*",
          destination: `${BACKEND_URL}/api/:path*`,
        },
      ],
    };
  },
};

export default nextConfig;
