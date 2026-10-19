# Demo shell setup: `on <machine>` switches to one simulated machine (own HOME, prompt, demo keys)
# and erases its own command line, so recordings only show the prompt change.
# Usage: source "$DEMO_ENV" server-1 (DEMO_COLOR picks the prompt colour, default cyan)
on() {
  export HOME="$DEMO_ROOT/$1"
  mkdir -p "$HOME/work" && cd "$HOME/work" || return
  export HOP_ACCESS_KEY=DEMOKEY HOP_SECRET_KEY=demo-secret-key SHELL=/bin/bash
  unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY XDG_CONFIG_HOME XDG_CACHE_HOME HOP_CONFIG
  CODE="$(HOME="$DEMO_ROOT/server-1" hop invite --code 2>/dev/null)"
  export CODE
  PS1="\[\e[1;${DEMO_COLOR:-36}m\]$1\[\e[0m\] \[\e[2m\]\$\[\e[0m\] "
  printf '\e[1A\e[2K'
}
on "$1"
