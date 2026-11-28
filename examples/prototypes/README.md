# Prototype examples

Exploratory programs built around `internal/prototype/`. They are not the public SDK or a shipped Alloy CLI. Each Go directory is a separate executable package in the root module.

Run the commands below from the repository root. The Codex and Browse launchers resolve their built TypeScript programs relative to that directory.

## Local CLI and event flow

Start the server without upstream integrations to exercise the local protocols without provider login:

```sh
go run ./examples/prototypes/server -codex=false -notion=false -browse=false
```

The server reports its private state directory and connection file paths. Keep these files outside the repository; they contain credentials. In separate terminals, substitute that directory for `/path/to/state`:

```sh
go run ./examples/prototypes/node -connection /path/to/state/cli-node-connection.json
go run ./examples/prototypes/event-worker -connection /path/to/state/worker-node-connection.json
go run ./examples/prototypes/event-emitter -connection /path/to/state/emitter-node-connection.json
```

Start the worker before the emitter. Events are ephemeral, not durable jobs.

## Upstream integration experiments

- `server/`: shared loopback server composing CLI resources, events, Codex, Notion MCP, and Browse MCP.
- `notion-node/`: ordinary MCP client using the server-generated Notion connection file.
- `codex/`: Go launcher for `codexnode/`, a TypeScript Codex SDK program.
- `browse-node/`: Go launcher for `browsenode/`, a TypeScript Codex SDK program using Browse MCP.

Build the TypeScript programs before using their launchers:

```sh
pnpm --dir examples/prototypes/codexnode install --frozen-lockfile
pnpm --dir examples/prototypes/codexnode build
pnpm --dir examples/prototypes/browsenode install --frozen-lockfile
pnpm --dir examples/prototypes/browsenode build
```

Use `go run ./examples/prototypes/server -h` to inspect integration switches and `go run ./examples/prototypes/<name> -h` for each node's flags. Enabling upstream integrations may use host authentication or start authorization/setup; they are not required for the local CLI/event example.

The launchers give nodes fresh home/work directories and a restricted environment, but this is environment separation, **not a sandbox**. The prototype's host-backed credentials and upstream integration policies are separate from the alpha SDK's managed provider authentication. Directory moves do not change account storage or runtime identities.
