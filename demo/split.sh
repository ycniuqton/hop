#!/usr/bin/env bash
# Opens the two-machine demo screen: tmux with server-1 (left) and server-2 (right).
# "ready" sets both machines up first and gives server-1 sample files; "fresh" leaves them empty.
# Usage (inside a tape): "$DEMO_DIR/split.sh" ready|fresh
set -euo pipefail
export HOP_ACCESS_KEY=DEMOKEY HOP_SECRET_KEY=demo-secret-key
quiet() { HOME="$DEMO_ROOT/$1" hop "${@:2}" </dev/null >/dev/null 2>&1; }

mkdir -p "$DEMO_ROOT/server-1/work" "$DEMO_ROOT/server-2/work"
if [ "${1:-ready}" = ready ]; then
  quiet server-1 init --endpoint http://localhost:9000 --bucket hop-demo --name server-1
  quiet server-2 join "$(HOME="$DEMO_ROOT/server-1" hop invite --code)" --name server-2
  cd "$DEMO_ROOT/server-1/work"
  head -c 3145728 /dev/urandom > dump.sql
  mkdir -p logs docs
  for d in 12 13 14; do seq 1 4000 > "logs/app-10-$d.log"; done
  printf '# Deploy guide\n\n1. hop pull release.tar\n2. ./install.sh\n' > docs/deploy.md
  printf '# On-call\n\nPage the owner, then check hop status.\n' > docs/oncall.md
fi

pane() { echo "DEMO_MACHINE=$1 DEMO_COLOR=$2 bash --rcfile '$DEMO_DIR/pane.rc' -i"; }
t() { tmux -L "$DEMO_TMUX" -f "$DEMO_DIR/tmux.conf" "$@"; }
t new-session -d -s demo -x "$(tput cols)" -y "$(tput lines)" "$(pane server-1 36)"
t split-window -h -t demo "$(pane server-2 35)"
t select-pane -t demo:0.0 -T server-1
t select-pane -t demo:0.1 -T server-2
t select-pane -t demo:0.0
exec tmux -L "$DEMO_TMUX" attach -t demo
