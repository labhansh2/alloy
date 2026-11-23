#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

go_files() {
  find . -type d \( -name .git -o -name node_modules -o -name dist -o -name vendor \) -prune \
    -o -type f -name '*.go' -print0
}

case "${1:-}" in
  "")
    go_files | xargs -0 gofmt -w
    go tool wsl --fix ./...
    go_files | xargs -0 gofmt -w
    pnpm exec prettier --write --ignore-unknown .
    ;;
  --check)
    unformatted="$(go_files | xargs -0 gofmt -l)"
    if [[ -n "$unformatted" ]]; then
      printf 'Run pnpm format. Unformatted Go files:\n%s\n' "$unformatted" >&2
      exit 1
    fi
    go tool wsl ./...
    pnpm exec prettier --check --ignore-unknown .
    ;;
  *)
    printf 'Usage: bash scripts/format.sh [--check]\n' >&2
    exit 2
    ;;
esac
