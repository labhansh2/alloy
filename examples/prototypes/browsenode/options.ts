import type { CodexOptions } from "@openai/codex-sdk";

function requiredEnv(name: string): string {
  const value = process.env[name];

  if (!value) {
    throw new Error(`Missing environment variable: ${name}`);
  }

  return value;
}

// Only native SDK configuration. Server-owned Browse pairing and Codex auth
// are not available in this node's clean environment.
export const codexOptions: CodexOptions = {
  env: {
    HOME: requiredEnv("HOME"),
    CODEX_HOME: requiredEnv("CODEX_HOME"),
    PATH: requiredEnv("PATH"),
    TMPDIR: requiredEnv("TMPDIR"),
    ALLOY_CODEX_TOKEN: requiredEnv("ALLOY_CODEX_TOKEN"),
    ALLOY_BROWSE_TOKEN: requiredEnv("ALLOY_BROWSE_TOKEN"),
  },

  config: {
    model_provider: "alloy",
    model_providers: {
      alloy: {
        name: "Alloy host auth broker",
        base_url: requiredEnv("ALLOY_CODEX_URL"),
        wire_api: "responses",
        env_key: "ALLOY_CODEX_TOKEN",
        requires_openai_auth: false,
        supports_websockets: false,
      },
    },

    mcp_servers: {
      browse: {
        url: requiredEnv("ALLOY_BROWSE_URL"),
        bearer_token_env_var: "ALLOY_BROWSE_TOKEN",
        enabled_tools: ["execute"],
        tools: {
          execute: { approval_mode: "approve" },
        },
        startup_timeout_sec: 20,
        tool_timeout_sec: 65,
      },
    },
  },
};
