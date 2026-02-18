import type { NextConfig } from "next";
import fs from "fs";
import path from "path";

// Load .env from project root (parent dir) for local dev.
// In Docker, env vars are passed via docker-compose environment section.
const parentEnv = path.resolve(__dirname, "..", ".env");
if (fs.existsSync(parentEnv)) {
  for (const line of fs.readFileSync(parentEnv, "utf-8").split("\n")) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const eqIdx = trimmed.indexOf("=");
    if (eqIdx === -1) continue;
    const key = trimmed.slice(0, eqIdx).trim();
    const val = trimmed.slice(eqIdx + 1).trim();
    if (!process.env[key]) process.env[key] = val;
  }
}

const apiUrl = process.env.API_URL || "http://localhost:3001";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${apiUrl}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
