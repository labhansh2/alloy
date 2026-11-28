import type { CodexOptions } from "@openai/codex-sdk";

function requiredEnv(name: string): string {
  const value = process.env[name];

  if (!value) {
    throw new Error(`Missing environment variable: ${name}`);
  }

  return value;
}

// Prototype connection configuration only. The launcher selects the Alloy
// server and supplies its temporary node token; the host login stays there.
const baseUrl = requiredEnv("ALLOY_CODEX_URL");

const nodeToken = requiredEnv("ALLOY_NODE_TOKEN");

export const codexOptions: CodexOptions = {
  env: {
    HOME: requiredEnv("HOME"),
    CODEX_HOME: requiredEnv("CODEX_HOME"),
    PATH: requiredEnv("PATH"),
    TMPDIR: requiredEnv("TMPDIR"),
    ALLOY_NODE_TOKEN: nodeToken,
  },

  config: {
    model_provider: "alloy",
    model_providers: {
      alloy: {
        name: "Alloy host auth broker",
        base_url: baseUrl,
        wire_api: "responses",
        env_key: "ALLOY_NODE_TOKEN",
        requires_openai_auth: false,
        supports_websockets: false,
      },
    },
  },
};
