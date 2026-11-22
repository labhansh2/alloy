# Codex in a clean-environment node

This experiment runs OpenAI's `@openai/codex-sdk` inside a separate process with
a fresh HOME, CODEX_HOME, working directory, and an explicitly minimal environment.
The SDK launches its bundled Codex CLI locally in that node environment.

Alloy runs a loopback HTTP server with the existing node routes and an additional
`POST /codex/responses` auth-broker route on the same port. The node uses a custom
Codex model provider pointing at Alloy. Alloy forwards the request to Codex's
ChatGPT backend and injects the host login there. The node receives only a random,
per-experiment Alloy token, never the host's ChatGPT access or refresh tokens.

## Run

From the repository root, with a working ChatGPT Codex login on the host:

```sh
pnpm --dir cmd/codexnode install --frozen-lockfile
pnpm --dir cmd/codexnode build
go run ./cmd/prototype-codex
```

The node source is `cmd/codexnode/run.ts`. The Go launcher runs its compiled
`dist/run.js`; generated output is ignored by Git. Use
`pnpm --dir cmd/codexnode typecheck` to check the TypeScript without emitting files.

Connection configuration lives separately in `cmd/codexnode/options.ts` as a
plain, typed `CodexOptions` object. The node uses the native SDK directly:

```ts
import { Codex } from "@openai/codex-sdk";
import { codexOptions } from "./options.js";

const codex = new Codex(codexOptions);
const thread = codex.startThread({ /* node-local thread settings */ });
const result = await thread.run("Summarize this project");
```

There is no Alloy SDK or thread wrapper. The launcher explicitly supplies the
broker URL and temporary node token through environment variables; the options
object passes only the required environment to the node-local Codex subprocess.
It does not discover accounts or fall back to another authentication source.

The proof currently defaults to `gpt-6.1-sol`, the model configured on the host
used to verify this experiment. Set `ALLOY_CODEX_MODEL` to another supported model
if needed. Login is read from `$CODEX_HOME/auth.json`, or `~/.codex/auth.json`.
Credentials are not printed, copied, or committed. Keychain-only login is not
implemented.

The launcher creates a random marker file in the node workspace. The thread must
execute a successful node-local command to read it, then return its contents.
The script checks the command event and final answer. This proves local execution
rather than merely asking a host-side Codex thread to respond.

The temporary workspace is printed and retained for inspection, including the
node's own Codex session files. It contains no copied host login. The HTTP server
is stopped when the experiment exits.

## Limits

- This is environment separation, **not a security sandbox**. Both processes still
  run as the same OS user; a malicious node could access host files. Use a container
  or separate OS identity for actual isolation.
- The auth proxy uses the ChatGPT backend experimentally. It is not a stable public
  API contract, and login expiry, refresh, revocation and account policy need proper
  handling before this becomes a product capability. The proxy reads the access
  token once; if it expires, refresh through the official host login and rerun.
- The node token grants model usage through this proxy and must itself be treated
  as a credential. There are no per-node model allowlists or usage quotas yet.
- This route is HTTP/SSE rather than a resource call over the node WebSocket. It
  shares Alloy's listener so Codex can use its ordinary provider HTTP interface.
- No host Codex SDK process executes the thread: the host only proxies model traffic.
  The SDK dependency is installed separately for the node, not shared as a live
  object across processes.
- This proof does not integrate automatic login discovery, Docker, or a general
  harness abstraction. Do not expose the prototype server publicly.

Verified locally: a fresh node HOME with no `auth.json` completed a thread and read
its own workspace marker using the host's existing ChatGPT login through Alloy.
