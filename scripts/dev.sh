#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

cleanup() {
  if [ -n "${BE_PID:-}" ]; then
    kill "$BE_PID" 2>/dev/null || true
  fi
  if [ -n "${FE_PID:-}" ]; then
    kill "$FE_PID" 2>/dev/null || true
  fi
}

trap cleanup INT TERM EXIT

echo "Starting backend..."
(cd "$ROOT" && go run ./cmd/server) &
BE_PID=$!

echo "Starting frontend..."
(cd "$ROOT" && pnpm --dir web dev) &
FE_PID=$!

echo "Frontend and backend are running. Press Ctrl+C to stop both."
wait
