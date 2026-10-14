# file-transfer — Requirements (00000-init)

Source: `hop/hop — Development Spec.md` (Oct 13, 2026). The spec is the source of truth; this file summarizes it.

## Business intent
`hop` is a single-binary Go CLI that moves files between one person's own Linux servers
through a single S3 bucket, with no backend service. It replaces `scp` juggling (hosts, users,
keys, both machines online at the same time) with asynchronous push/pull and organized storage.

## Goals
- Setup under one minute per machine: `hop init` once, then paste one `hop join` line per machine.
- Asynchronous transfers: push now, pull later, even if the sender is offline.
- Short commands with one consistent `@device` syntax.
- Tab completion for commands, devices, hot names and store paths, under 50 ms (local cache).
- S3 holds both the files and all shared state.

## Non-goals (v1)
- Multi-user accounts, sharing with strangers, permissions between people.
- Continuous sync or file watching. Automatic expiry or cleanup.
- Providers other than AWS S3. Windows (macOS may work but is not a target).

## Functional requirements
### Workspace & devices
- FR-1 `init`: collect bucket/region/keys/prefix/name (flags, prompt, or AWS env defaults); verify
  credentials (HeadBucket + probe Put/Delete); refuse if a workspace already exists; write
  `workspace.json` and the device record with `If-None-Match: *`; save config with mode 0600; offer completion.
- FR-2 `init --print-policy` prints a minimal IAM policy for the bucket and prefix.
- FR-3 `invite [--code]` prints `hop join hop1:<base64url(json)>` with a warning that it contains secrets.
- FR-4 `join <code> [--name] [--force]`: code from an argument or stdin; validate the prefix and workspace_id;
  a taken name → 412 → clear error and a new prompt.
- FR-5 `devices`, `whoami`, `rename`, `remove [--purge]`, `rotate <code>`, `status`.
- FR-6 Device names match `^[a-z0-9][a-z0-9-]{0,31}$`; `_shared` and `me` are reserved.
- FR-7 `last_seen` is written at most once every 5 minutes, after a successful command.

### Hot shelves (flat, overwrite silently)
- FR-8 `push <file>... [@dev|@] [--as]`: no `@` → shared shelf; `@dev` → that device; `@` → self.
  Directories are rejected. `--as` works with one file only. Metadata `hop-from` and `hop-mode` are stored.
- FR-9 `pull <name> [@dev|@] [dest] [--rm] [-f]`: no local overwrite without `-f`; restore the file mode;
  `--rm` deletes only after the atomic rename; a pick list on a TTY when no name is given.
- FR-10 `hot [@dev|@] [--all]`, `hot rm`, `hot clear` (asks to confirm, `-y` skips).
- FR-11 Hot names must not contain `/`.

### Store (path tree per device, never overwrites without `-f`)
- FR-12 `cp [-r] [-f] <src>... <dst>`: upload, download, or server-side remote→remote copy; local→local is an error.
  Destination rules follow cp/scp (trailing `/` or device root → copy into it).
- FR-13 `ls [-R] [-l]` (default `@:`, folders first), `mv` (remote→remote only),
  `rm [-r]` (asks to confirm above 10 objects).
- FR-14 Bridges: `keep <name> [@dev] <@dev:path>` (hot→store), `push <@dev:path> [@dev]` (store→hot).

### Argument parsing
- FR-15 One parser turns every argument into Local / Device / Remote. It splits on the first `:`,
  cleans paths, rejects `..`, accepts a Device target in any position (hot commands, at most one),
  and suggests close device names (Levenshtein ≤ 2).

### Completion & cache
- FR-16 Cobra dynamic completion reads only the local cache. The exception is store paths: a live list
  capped at 1 s and 1000 keys.
- FR-17 Cache under `$XDG_CACHE_HOME/hop/<workspace_id>/`, written through on every change, written atomically;
  older than 30 s → answer from the cache and start a detached `hop __refresh`.
- FR-18 `completion install` for bash, zsh and fish, safe to run again; fix the bash colon word-break problem.

### UX
- FR-19 Global flags `-q`, `--json`, `--no-color`, `-v`. Prompts only on a TTY; without a TTY, confirmations need `-y`.
- FR-20 Errors name the fix; exit codes 0/1/2/3/4/5/6/130 as defined in spec §10.

## Non-functional / constraints
- Static binary (`CGO_ENABLED=0`). Libraries: cobra, aws-sdk-go-v2, x/term, progressbar/v3, testify, testcontainers-go.
- Multipart above 16 MB (16 MB parts, concurrency 4). Downloads go to `.hop-tmp` and then rename. `cp -r` runs up to 8 transfers at once.
- Ctrl-C cancels cleanly (`signal.NotifyContext`). Progress bar on stderr only for TTY transfers over 1 MB.
- Security: config must be 0600 or hop refuses to run; the secret is never printed (including `-v`); warn when the keys can
  list other buckets; refuse local writes outside the destination during `cp -r`.
- `internal/cli` makes no S3 calls. Every deviation from the spec is recorded in `DECISIONS.md`.
- Targets: linux/amd64, linux/arm64, darwin/arm64 via GoReleaser.

## Open points
- Devlens coding rules (classes, Managers, base classes) must be mapped onto Go idioms. See design §9;
  confirm with the user if a conflict comes up (Rule 0).
