# file-transfer — Design (00002-agent-support)

Changes only; earlier versions' design is unchanged.

## 1. Credentials for non-interactive init

```text
hop init --url U --name N [--secret-stdin]
└─ access key  = flag-less: HOP_ACCESS_KEY → AWS_ACCESS_KEY_ID → prompt (TTY) → usage error
└─ secret key
   ├─ --secret-stdin → first stdin line (empty → usage error)
   └─ HOP_SECRET_KEY → AWS_SECRET_ACCESS_KEY → hidden prompt (TTY) → usage error
      hint: "set HOP_SECRET_KEY or use --secret-stdin"
```

## 2. JSON results and errors

```text
any command --json
└─ success
   ├─ list command → the list / object (unchanged)
   └─ action command → Console.Done(Outcome{Action, Data}) → one JSON object on stdout
                       (text mode: the usual "✔ …" line from the same call)
└─ failure → App.reportError
   ├─ --json → {"error","hint","code","kind"} on stderr
   └─ text   → "error: msg — hint" on stderr (unchanged)
```

- `ui.Outcome{Action string; Data map[string]any}` — the action name is a domain constant
  (`ui.Actions`); data keys are snake_case.
- `apperr.ExitCode.Kind()` names each code: ok, general, usage, not_found, conflict, auth,
  network, interrupted. Errors that are not `*apperr.Error` report code 2 / usage (Cobra).
- `push` with several files reports one object with a `files` array.

## 3. Agent skill (`internal/skill`)

```text
hop skill install [--project]          hop skill print
└─ Targets registry                    └─ embedded SKILL.md minus front matter → stdout
   ├─ user    → $CLAUDE_CONFIG_DIR|~/.claude/skills/hop/SKILL.md
   └─ project → ./.claude/skills/hop/SKILL.md
   → atomic write, same bytes each time → "installed <path>"
```

- `SKILL.md` lives in `internal/skill/` and is compiled in with `go:embed`.
- A test checks that every `hop <command>` mentioned in the skill exists in the command tree,
  so the skill cannot drift from the CLI.

## 4. Self-management (`internal/selfinstall`)

```text
hop update [--check] [--version vX] [--force]
└─ Releases.Latest → GET github.com/<repo>/releases/latest (no redirect) → tag from Location
   (no API call, so no rate limit)
└─ compare with this build (numeric x.y.z; "-dev" sorts before its release)
   ├─ same, no --force      → "already up to date" (exit 0)
   ├─ this build is newer   → "newer than latest; use --force" (exit 0)
   └─ --check               → report only
└─ download hop_<os>_<arch>.tar.gz + checksums.txt → sha256 must match
└─ extract "hop" → temp file next to the real executable (symlinks resolved) → chmod 755 → rename
   └─ permission denied → error, hint: sudo or the curl installer
---
hop uninstall [-y] [--keep-config]
└─ Footprint.Items: binary, completion (bash/zsh/fish paths), skill (user scope), cache dir, config
   → keep only paths that exist
└─ none → "nothing to remove"
└─ list them → confirm (TTY) or require -y
└─ delete each (cache dir recursively) → report removed + failed
└─ note: "this device is still listed in the workspace — run 'hop remove <name>' on another device"
```

- The release source is `Releases{Base, Repo}`, `https://github.com` + `ycniuqton/hop`, with env
  overrides `HOP_RELEASE_BASE` / `HOP_REPO` (mirrors, tests), as in `install.sh`.
- `Env.Executable` (default `os.Executable`) lets tests point update/uninstall at a temp file.
