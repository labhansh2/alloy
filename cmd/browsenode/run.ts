import { Codex, type McpToolCallItem } from "@openai/codex-sdk";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { codexOptions } from "./options.js";

const codeHome = process.env.CODEX_HOME;

if (!codeHome || existsSync(join(codeHome, "auth.json"))) {
  throw new Error(
    "The browser node must run with a fresh CODEX_HOME and no copied auth.json",
  );
}

const instructions = readFileSync(
  new URL("../instructions.md", import.meta.url),
  "utf8",
);

const codex = new Codex(codexOptions);

const thread = codex.startThread({
  model: process.env.ALLOY_CODEX_MODEL || "gpt-6.1-sol",
  workingDirectory: process.cwd(),
  skipGitRepoCheck: true,
  sandboxMode: "read-only",
  approvalPolicy: "never",
  webSearchMode: "disabled",
});

console.log("Running node-local instructions with server-hosted Browse MCP.");

const result = await thread.run(instructions, {
  signal: AbortSignal.timeout(180000),
  outputSchema: {
    type: "object",
    properties: {
      pages: {
        type: "array",
        items: {
          type: "object",
          properties: {
            url: { type: "string" },
            title: { type: "string" },
            heading: { type: ["string", "null"] },
          },
          required: ["url", "title", "heading"],
          additionalProperties: false,
        },
      },
      summary: { type: "string" },
    },
    required: ["pages", "summary"],
    additionalProperties: false,
  },
});

const calls = result.items.filter(
  (item): item is McpToolCallItem =>
    item.type === "mcp_tool_call" &&
    item.server === "browse" &&
    item.tool === "execute" &&
    item.status === "completed",
);

const evidence = calls.map((item) => JSON.stringify(item.result)).join("\n");

if (
  calls.length === 0 ||
  !evidence.includes("example.com") ||
  !evidence.includes("iana.org/domains/reserved")
) {
  throw new Error(
    "The thread did not return Browse MCP evidence for both public pages",
  );
}

const report = JSON.parse(result.finalResponse) as {
  pages: { url: string; title: string; heading: string | null }[];
  summary: string;
};

if (
  report.pages.length !== 2 ||
  !report.pages.some((page) => /^https:\/\/example\.com\/?$/.test(page.url)) ||
  !report.pages.some(
    (page) => page.url === "https://www.iana.org/domains/reserved",
  ) ||
  report.pages.some(
    (page) =>
      !page.title.trim() || (page.heading !== null && !page.heading.trim()),
  )
) {
  throw new Error("Unexpected or incomplete public-page report");
}

console.log("Thread:", thread.id);
console.log("Completed Browse MCP calls:", calls.length);
console.log(JSON.stringify(report, null, 2));
