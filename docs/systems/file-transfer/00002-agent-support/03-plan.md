# file-transfer — Plan (00002-agent-support)

States: `[ ]` todo · `[~]` in progress · `[x]` done. Carried over: all `00000-init` and
`00001-storage-providers` items are `[x]`.

## A1 — Agent-safe init
- [x] `HOP_ACCESS_KEY` / `HOP_SECRET_KEY` (preferred over `AWS_*`)
- [x] `init --secret-stdin`
- [x] Missing-key errors name the env vars and `--secret-stdin`
- [x] Tests: env precedence, stdin secret, empty stdin, secret never in output

## A2 — JSON everywhere
- [x] `apperr.ExitCode.Kind()`
- [x] JSON error object on stderr with `--json`
- [x] `ui.Outcome` + `Console.Done`; action names as constants
- [x] Action commands report through `Done` (init, join, rename, remove, rotate, push, pull, keep, hot rm, hot clear, cp, mv, rm, completion install)
- [x] Tests: one JSON object per action, JSON errors, text output unchanged

## A3 — Skill
- [x] `internal/skill/SKILL.md` (front matter + guidance)
- [x] `skill.Targets` registry (user, project) + install/print
- [x] `hop skill install [--project]`, `hop skill print`
- [x] Tests: install path + idempotent, `$CLAUDE_CONFIG_DIR`, print strips front matter, commands in skill exist

## A5 — Update and uninstall
- [x] `selfinstall.Releases`: latest tag via redirect, asset + checksum download, tar.gz extract
- [x] Version compare (x.y.z, "-dev" before release)
- [x] Atomic self-replace next to the resolved executable; permission hint
- [x] `hop update [--check] [--version] [--force]` with `--json`
- [x] `selfinstall.Footprint` + `hop uninstall [-y] [--keep-config]` with `--json`
- [x] Tests: fake release server (redirect, assets, bad checksum), up-to-date/newer/check/pin, uninstall lists + confirm + removes, keep-config _(plus a real self-update 0.1.0 → 0.2.0 from GitHub)_
- [x] README + DECISIONS + skill mention

## A4 — Docs and checks
- [x] README: "Using hop with AI agents"
- [x] DECISIONS.md entries
- [x] vet, lint, tests, rule check green
- [~] Release v0.3.0 (needs explicit go-ahead to commit, push and tag)
