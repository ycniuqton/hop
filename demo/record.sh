#!/usr/bin/env bash
# Renders the README demo GIFs from demo/*.tape. Needs vhs, ttyd, ffmpeg and tmux on PATH
# (https://github.com/charmbracelet/vhs). Run from the repo root: ./demo/record.sh [tape...]
set -euo pipefail
cd "$(dirname "$0")/.."

work="$(mktemp -d)"
export DEMO_TMUX=hopdemo TMUX_TMPDIR="$work" DEMO_DIR="$PWD/demo" DEMO_ENV="$PWD/demo/env.sh"
unset TMUX
trap 'tmux -L "$DEMO_TMUX" kill-server 2>/dev/null || true; kill "${server:-0}" 2>/dev/null || true; rm -rf "$work"' EXIT
CGO_ENABLED=0 go build -ldflags "-X main.version=$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')" -o "$work/bin/hop" ./cmd/hop
go build -o "$work/fakes3" ./demo/fakes3

tapes=("$@")
[ ${#tapes[@]} -gt 0 ] || tapes=(demo/*.tape)
for tape in "${tapes[@]}"; do
  rm -rf "$work/machines" && mkdir -p "$work/machines"
  "$work/fakes3" -addr 127.0.0.1:9000 >/dev/null 2>&1 &
  server=$!
  sleep 1
  echo "recording $tape"
  DEMO_ROOT="$work/machines" PATH="$work/bin:$PATH" vhs "$tape"
  tmux -L "$DEMO_TMUX" kill-server 2>/dev/null || true
  kill "$server"
  wait "$server" 2>/dev/null || true
done
