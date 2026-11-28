# Alloy examples

These are standalone programs, not a shipped Alloy CLI.

- [Alpha](alpha/README.md): minimal server and node using the current public Go SDK.
- [Prototypes](prototypes/README.md): exploratory CLI, event, Codex, Notion, and Browse integrations.

Each Go executable is its own `package main`, but all Go examples remain in the root `alloy` module. There are no separate example Go modules or workspace setup steps. Run `go test ./...` from the repository root to include their tests. The TypeScript examples retain their own package manifests and lockfiles.

Alpha examples read their Alloy client token from the environment and load an optional `.env` from the working directory without overriding exported variables. Copy their `.env.example` templates for local setup. Provider, model, prompt, and client name stay fixed; the only flags are server `-login` and node `-server`. Prototype programs use explicit flags, exported variables, and server-generated connection files.

Never commit credentials or runtime state. Keep upstream credentials in Alloy's managed store and prototype connection files outside the repository.
