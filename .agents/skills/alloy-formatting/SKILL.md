---
name: alloy-formatting
description: Formats the entire Alloy repository after any code change and before finishing a task.
---

- Separate functions, declarations, and logical sections with blank lines; avoid dense one-liners. Keep related assignments and error checks together.
- From the repo root: `pnpm install --frozen-lockfile` if needed, then `pnpm format` (Go: gofmt + wsl whitespace fixes; TS/JSON/YAML/Markdown: Prettier).
- Before finishing: `pnpm format:check`. Reformat after further edits. Never format dependencies, generated output, or private docs.
