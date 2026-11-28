import { Codex } from "@openai/codex-sdk";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { codexOptions } from "./options.js";

function requiredEnv(name: string): string {
  const value = process.env[name];

  if (!value) {
    throw new Error(`Missing environment variable: ${name}`);
  }

  return value;
}

const codexHome = requiredEnv("CODEX_HOME");

if (existsSync(join(codexHome, "auth.json"))) {
  throw new Error("Node must not have a copied auth.json");
}

const marker = readFileSync(
  join(process.cwd(), "node-marker.txt"),
  "utf8",
).trim();

console.log(
  "Node has a fresh HOME, no auth.json, and no inherited OpenAI credentials.",
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

const result = await thread.run(
  "Read node-marker.txt in your current working directory using a local command. Reply with only its contents.",
  { signal: AbortSignal.timeout(90000) },
);

const localRead = result.items.some(
  (item) =>
    item.type === "command_execution" &&
    item.exit_code === 0 &&
    item.aggregated_output.includes(marker),
);

if (!localRead || !result.finalResponse.includes(marker)) {
  throw new Error(
    "Thread did not demonstrate a successful node-local file read",
  );
}

console.log("Thread:", thread.id);
console.log("Response:", result.finalResponse);
console.log("Verified: Codex command execution read the node-local marker.");
