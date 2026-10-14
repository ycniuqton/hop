# file-transfer — Requirements (00002-agent-support)

Changes only. `00000-init` and `00001-storage-providers` still apply unless stated here.

## Business intent
Coding agents (Claude Code, Codex, Cursor, …) should be able to set up and use hop on their own:
no prompts they cannot answer, output they can parse, and a skill that teaches them the tool.

## Functional requirements
- FR-A1 One-line, non-interactive init without secrets in argv:
  `HOP_ACCESS_KEY=… HOP_SECRET_KEY=… hop init --url <bucket URL> --name <device>`.
  `HOP_ACCESS_KEY` / `HOP_SECRET_KEY` win over `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`.
- FR-A2 `hop init --secret-stdin` reads the secret key from the first line of stdin.
- FR-A3 With `--json`, a failure prints exactly one JSON object on stderr:
  `{"error": "...", "hint": "...", "code": 3, "kind": "not_found"}`; stdout stays empty.
- FR-A4 With `--json`, every action command prints exactly one JSON object on stdout describing
  what it did: init, join, rename, remove, rotate, push, pull, keep, hot rm, hot clear, cp, mv,
  rm, completion install, skill install. Each has an `"action"` field.
- FR-A5 `hop skill install` writes a Claude Code skill to `~/.claude/skills/hop/SKILL.md`
  (`$CLAUDE_CONFIG_DIR` respected); `--project` writes `./.claude/skills/hop/SKILL.md`.
  Idempotent; prints the path.
- FR-A6 `hop skill print` prints the same guidance as plain Markdown (no front matter) for
  `AGENTS.md` and other agents.
- FR-A7 The skill teaches: when to use hop, hot vs store, the `@` grammar, setup one-liners,
  `--json` and exit codes, and safety rules (never echo keys or join codes, no `-y`/`-f` unless
  the user asked, pipe join codes over ssh instead of pasting them).
- FR-A8 README gets an "Using hop with AI agents" section.
- FR-A9 `hop update` replaces the running binary with the latest GitHub release for this OS/CPU,
  verified against `checksums.txt`. `--check` only reports; `--version vX.Y.Z` pins; `--force`
  reinstalls even when current or newer. Up to date → says so, exit 0. No write access → error
  with a sudo / reinstall hint. Supports `--json`.
- FR-A10 `hop uninstall` removes this machine's hop footprint: binary, completion scripts, the
  user-scope agent skill, the cache and the config (credentials). It lists what it will delete
  and asks; without a TTY it needs `-y`. `--keep-config` keeps `config.json`. It never touches
  the bucket and tells the user to run `hop remove <name>` from another device.

## Constraints
- No new dependencies; the skill is embedded in the binary so it always matches its version.
- Human output is unchanged when `--json` is not given.
- Secrets never appear in JSON output, errors or the skill.

## Unchanged
Commands' behaviour, flags (except the new ones above), storage, providers, release process.
