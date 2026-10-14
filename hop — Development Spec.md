# hop — Development Spec

Oct 13, 2026 · @Quincy

## 1. Overview

`hop` is a single-binary Go CLI that moves files between a person's own Linux servers through one S3 bucket, with no server of its own. One `hop init` plus one pasted `hop join` command per machine is the whole setup.

**Problem.** Copying files between several SSH servers with `scp` means remembering hosts, users and keys, and both machines must be reachable at the same time. Existing tools are either synchronous (croc, magic-wormhole) or heavy to configure (Syncthing, rclone).

**Goals**

- Setup in under a minute per machine: `init` once, then paste one `join` line on every other machine.
- Asynchronous transfers: push now, pull later, even if the sender is offline.
- Short, memorable commands with one consistent `@device` syntax.
- Fast tab completion for commands, devices, hot file names and store paths.
- No backend service: S3 holds both the files and the shared state.

**Non-goals (v1)**

- Multi-user accounts, permissions between people, or sharing with strangers.
- Continuous folder sync or file watching.
- Automatic expiry or cleanup of files.
- Storage providers other than AWS S3 (S3-compatible endpoints are planned, see Future work).
- Windows support (Linux first; macOS should work but is not a v1 target).

## 2. Core concepts

hop has two storage modes with separate verbs: **hot** (`push` / `pull`) for quick handoffs by name, and **store** (`cp` / `ls` / `mv` / `rm`) for organized files with paths.

| Concept | What it is | Overwrite rule | Verbs |
| --- | --- | --- | --- |
| Workspace | One S3 bucket (+ optional prefix) shared by all of a person's devices | n/a | `init`, `invite`, `join` |
| Device | One machine registered in the workspace, identified by a unique name (e.g. `web-1`) | Names are unique; registration fails if taken | `devices`, `rename`, `remove` |
| Shared shelf | One global flat list of hot files | Same name overwrites silently | `push`, `pull`, `hot` (no `@`) |
| Device shelf | A flat list of hot files belonging to one device | Same name overwrites silently | `push`, `pull`, `hot` with `@device` |
| Store | A folder tree per device, with real paths | Never overwrites unless `-f` | `cp`, `ls`, `mv`, `rm` |

**The `@` rule** is the same everywhere:

- No `@` → shared shelf (hot commands only).
- `@device` → that device's shelf (hot) or folder root (store).
- `@` alone → this device (`@` is an alias for `@me`, which also works).
- `@device:path` → a path inside that device's store folder.
- An argument without `@` in a store command is a local path.

All devices are equal peers. Any device can read, write and invite; there is no primary machine after `init`.

## 3. Bucket layout and data model

All state lives under one optional prefix (default empty) in the bucket. Metadata files are small JSON objects; file contents are stored as-is.

```text
<bucket>/<prefix>/
  .hop/
    workspace.json            # created by init
    devices/
      laptop.json             # one per device
      web-1.json
  hot/
    _shared/<name>            # shared shelf (flat)
    <device>/<name>           # device shelves (flat)
  store/
    <device>/<path...>        # store tree per device
```

**Reserved names.** Device names must match `^[a-z0-9][a-z0-9-]{0,31}$`. `_shared` and `me` are reserved and rejected.

**workspace.json**

```json
{
  "version": 1,
  "workspace_id": "7f2c9a1e-...",
  "created_at": "2026-10-13T09:00:00Z",
  "created_by": "laptop"
}
```

**devices/\<name>.json**

```json
{
  "name": "web-1",
  "device_id": "b41e...",
  "hostname": "ip-10-0-1-12",
  "os": "linux/amd64",
  "hop_version": "0.1.0",
  "joined_at": "2026-10-13T09:05:00Z",
  "last_seen": "2026-10-13T11:42:00Z"
}
```

`last_seen` is updated at most once every 5 minutes, after a successful command, to keep write costs down.

**Object metadata on hot files.** Every hot object carries S3 user metadata `x-amz-meta-hop-from` (sender device name) and `x-amz-meta-hop-mode` (original file mode, e.g. `0644`). `hop hot` reads size and LastModified from the listing, and the sender via HeadObject only for the rows displayed.

**Folders.** S3 has no folders. A folder is a key prefix ending in `/`. Listing uses `ListObjectsV2` with `Delimiter: "/"`. Empty folders are not represented.

**Local config** on each device lives at `~/.config/hop/config.json` with mode `0600`:

```json
{
  "version": 1,
  "device": "web-1",
  "bucket": "my-hop",
  "region": "ap-southeast-1",
  "prefix": "",
  "endpoint": "",
  "access_key_id": "AKIA...",
  "secret_access_key": "..."
}
```

`HOP_CONFIG` overrides the path. The cache lives at `~/.cache/hop/` (see section 7).

## 4. Setup flow: init, invite, join

The first device runs `hop init` with S3 credentials; every later device joins by pasting one base64 code printed by `hop invite` on any existing device.

### hop init

1. Collect bucket, region, access key, secret key, optional prefix and device name. Use flags if given, otherwise prompt interactively. Read the secret with no echo. Fall back to `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` / `AWS_REGION` env vars as prompt defaults.
2. Default device name = short hostname, lowercased and sanitized to the name regex.
3. Verify credentials: `HeadBucket`, then a test `PutObject` + `DeleteObject` on `.hop/.probe-<random>`. On failure, print which step failed and the AWS error code.
4. If `.hop/workspace.json` already exists, stop with: `bucket already has a hop workspace — run 'hop invite' on an existing device and use 'hop join' instead`. `--force` is not offered in v1.
5. Write `workspace.json` with `If-None-Match: *`.
6. Register the device (see Device registration below).
7. Write local config with mode `0600`.
8. Offer tab completion: `Enable tab completion for bash? [Y/n]` (shell detected from `$SHELL`).
9. Print a success line and the hint `Next: run 'hop invite' to add another machine.`

`hop init --print-policy` prints a minimal IAM policy for the bucket (s3:ListBucket on the bucket; s3:GetObject, PutObject, DeleteObject on `<bucket>/<prefix>*`) and exits.

### hop invite

Prints a ready-to-paste command:

```text
$ hop invite
Run this on the new machine (it contains your S3 keys — treat it like a password):

  hop join aG9wMTp7ImIiOiJteS1ob3AiLCJyIjoi...
```

`hop invite --code` prints only the code, for scripts.

### Join code format

```text
hop1:<base64url(JSON)>
```

The JSON payload uses short keys to keep the code short:

```json
{
  "v": 1,
  "b": "my-hop",
  "r": "ap-southeast-1",
  "p": "",
  "e": "",
  "k": "AKIA...",
  "s": "secret...",
  "w": "7f2c9a1e-..."
}
```

Use unpadded base64url (`base64.RawURLEncoding`) so the code has no `+`, `/` or `=` characters that break copy-paste or shells. The `hop1:` prefix lets future versions change the format; the decoder rejects unknown prefixes with a clear message.

### hop join

```text
hop join <code> [--name web-1]
hop join --name web-1 < code.txt     # read code from stdin, keeps it out of shell history
```

1. Decode and validate the code. Also accept a code pasted together with a leading 'hop join ', and fail loudly on a missing or unknown `hop1:` prefix.
2. Verify credentials (same checks as init) and that `.hop/workspace.json` exists and its `workspace_id` matches `w`.
3. Pick the name: `--name`, else prompt with hostname as default.
4. Register the device, write local config, offer tab completion.
5. Print `Joined workspace as "web-1" (3 devices).`

If the machine already has a config, refuse unless `--force`, which replaces it.

### Device registration

Write `.hop/devices/<name>.json` with `PutObject` and `If-None-Match: *` (S3 conditional write). A `412 Precondition Failed` means the name is taken: print `device name "web-1" is already used — pick another with --name` and, in interactive mode, re-prompt.

## 5. Command reference

Every command below is in v1. Global flags on all commands: `-q/--quiet`, `--json` (machine output for list commands), `--no-color`, `-v/--verbose` (log S3 calls).

### Setup and devices

| Command | Behavior |
| --- | --- |
| `hop init [--bucket --region --prefix --name --print-policy]` | Create workspace, register this device (section 4) |
| `hop invite [--code]` | Print the join command or bare code |
| `hop join <code> [--name] [--force]` | Join a workspace; code may come from stdin |
| `hop devices` | Table: NAME, LAST SEEN (relative), HOSTNAME; current device marked `(you)` |
| `hop whoami` | Device name, bucket, region, prefix |
| `hop rename <old> <new>` | Copy device record + move its hot shelf and store folder to the new name; update local config if renaming self |
| `hop remove <device> [--purge]` | Delete the device record (asks to confirm). `--purge` also deletes its shelf and store folder. Prints the warning that keys must be rotated for a real revoke |
| `hop rotate <code>` | Replace this device's credentials with those in a new join code for the same workspace |
| `hop completion install` / `hop completion <bash\|zsh\|fish>` | Install completion, or print the script |
| `hop status` | Object count and total bytes for hot (shared + each shelf) and store (per device) |

### Hot shelves

```bash
hop push <file>... [@device|@] [--as <name>]
hop pull <name> [@device|@] [<local-dest>] [--rm] [-f]
hop hot [@device|@] [--all]
hop hot rm <name>... [@device|@]
hop hot clear [@device|@]        # asks to confirm; -y skips
```

- `push` uses the local file's base name unless `--as` is given. `--as` is only valid with one file. Directories are rejected with: `push sends single files — use 'hop cp -r' for folders`.
- Target shelf: no `@` → `_shared`; `@device` → that device; `@` → self. Pushing to an unknown device fails with a did-you-mean suggestion.
- `pull` defaults to the shared shelf. Local destination defaults to `./<name>`; if it is a directory, write inside it. Refuse to overwrite an existing local file unless `-f`.
- If `pull <name>` misses on the shared shelf but `<name>` exists on the caller's own shelf, the error adds: `did you mean: hop pull <name> @`.
- `pull --rm` deletes the object only after the download is fully written and renamed into place.
- Restore the original file mode from `hop-mode` metadata on pull.
- `hop hot` lists NAME, SIZE, FROM, AGE, newest first. `--all` groups by shelf.

### Store

```bash
hop cp [-r] [-f] <src>... <dst>
hop ls [-R] [-l] [@device:path|@:path]
hop mv <src> <dst>
hop rm [-r] <@device:path>...
```

`cp` direction is decided by which side has `@`:

| Source | Destination | Operation |
| --- | --- | --- |
| local | `@dev:path` | upload |
| `@dev:path` | local | download |
| `@dev:path` | `@dev2:path` | server-side `CopyObject` (no download) |
| local | local | error: `use the regular cp for local copies` |

Destination rules (same as `cp`/`scp`):

- Ends with `/`, or is a device root (`@web-1:`, `@:`) → copy into it, keep the source base name.
- Otherwise → exact destination name (rename). With several sources, the destination must be a folder.
- Existing destination object → error `exists: @web-1:backups/db.sql (use -f to overwrite)`.
- `-r` required for local directories and remote prefixes; it preserves relative paths.

`ls` with no argument lists `@:`. Default output shows folders first (with trailing `/`), then files. `-l` adds size and modified time. `-R` lists recursively. `mv` is copy-then-delete and accepts remote-to-remote only in v1. `rm -r` on a prefix asks to confirm when it would delete more than 10 objects (`-y` skips).

### Bridges between modes

```bash
hop keep <name> [@device|@] <@dev:path>    # hot → store (server-side copy)
hop push <@dev:path> [@device|@]           # store → hot (server-side copy)
```

### Cheat sheet

```bash
# setup
hop init
hop invite
hop join aG9w... --name web-1
hop devices

# hot: names only
hop push dump.sql                 # → shared
hop push dump.sql @db-1           # → db-1's shelf
hop push dump.sql @               # → my shelf
hop pull dump.sql                 # from shared
hop pull dump.sql @               # from my shelf
hop pull dump.sql @web-1 --rm
hop hot / hop hot @ / hop hot --all

# store: paths
hop cp db.sql @:backups/
hop cp db.sql @web-1:backups/2026-10.sql
hop cp -r ./logs @web-1:logs/
hop cp @web-1:backups/2026-10.sql .
hop cp @web-1:backups/2026-10.sql @db-1:restore/
hop ls @web-1:backups/
hop mv @web-1:a.sql @web-1:old/a.sql
hop rm -r @web-1:old/
```

## 6. Argument parsing and path rules

One parser in `internal/target` turns every positional argument into a typed target; commands never parse `@` themselves.

```go
type Kind int
const (
    Local Kind = iota   // ./file, /abs/path, file
    Device              // @web-1, @, @me     (no colon)
    Remote              // @web-1:path, @:path
)

type Target struct {
    Kind   Kind
    Device string // resolved name; "" for Local
    Path   string // cleaned, no leading "/"; "" = device root
    Dir    bool   // ends with "/" or is a device root
}
```

**Rules**

1. An argument starting with `@` is a device or remote target. Anything else is local. A local file literally named `@x` must be written `./@x`.
2. `@` and `@me` resolve to the configured device name.
3. Split on the first `:` only. `@web-1:a:b.txt` is path `a:b.txt` on `web-1`.
4. Clean remote paths with `path.Clean`, strip leading `/`, and reject any path that escapes with `..`.
5. Hot commands accept **at most one** `Device` target, in **any position**. `hop pull dump.sql @web-1` and `hop pull @web-1 dump.sql` are identical. This is required for good completion (section 7).
6. Hot commands reject `Remote` targets except in the bridge forms (`hop push @dev:path`).
7. Store commands reject bare `Device` targets with a hint: `did you mean @web-1: (with a colon)?`.
8. Device names in targets are validated against the device list (cache first, S3 on a cache miss). Unknown names produce `unknown device "web1" — did you mean web-1?` using Levenshtein distance ≤ 2.

**Key mapping**

| Target | S3 key |
| --- | --- |
| shared shelf, name `n` | `<prefix>hot/_shared/n` |
| `@dev` shelf, name `n` | `<prefix>hot/dev/n` |
| `@dev:p` | `<prefix>store/dev/p` |

Hot names must not contain `/`; reject them with `hot names are flat — use 'hop cp' for paths`.

## 7. Tab completion and local cache

Completion uses Cobra's built-in dynamic completion (`ValidArgsFunction` + `RegisterFlagCompletionFunc`) and must answer in under 50 ms by reading only the local cache, never S3 (except store paths, below).

### What completes where

| Typed so far | Suggestions | Source |
| --- | --- | --- |
| `hop pu<TAB>` | `push pull` | command tree |
| `hop push <TAB>` | local files | shell default (`ShellCompDirectiveDefault`) |
| any hot command, word starting with `@` | `@` plus `@<device>` for every device | cache: devices |
| `hop pull <TAB>`, `hop hot rm <TAB>` (no `@` on the line) | names on shared shelf | cache: shelf `_shared` |
| same, with `@web-1` anywhere on the line | names on web-1's shelf | cache: shelf `web-1` |
| `hop cp @web-1:<TAB>`, `hop ls`, `hop rm`, `hop mv` | `@web-1:backups/`, `@web-1:notes.txt` | live `ListObjectsV2` with delimiter, one level |
| `hop cp db.sql <TAB>` | local files plus `@<device>:` | shell default + cache |
| `hop rename <TAB>`, `hop remove <TAB>` | device names (no `@`) | cache: devices |

For store paths, return folders with a trailing `/` and use `ShellCompDirectiveNoSpace` so the user can keep tabbing deeper. Cap the live list call at 1 second and 1000 keys; on timeout, return nothing rather than hang the shell.

### Cache

Location: `~/.cache/hop/<workspace_id>/` (respect `XDG_CACHE_HOME`).

```text
devices.json          # [{name, last_seen}], fetched_at
shelves/_shared.json  # [{name, size, modified}], fetched_at
shelves/web-1.json
```

- **Write-through:** every command that lists or changes devices or shelves rewrites the matching cache file. `push`/`hot rm`/`pull --rm` update the cache locally without an extra list call.
- **Stale refresh:** if a cache file read during completion is older than 30 s, completion still answers from it, then spawns a detached `hop __refresh <what>` child process and returns immediately.
- Write files atomically (temp file + rename). A corrupt cache file is deleted and treated as empty.

### Installation

`hop completion install` detects the shell from `$SHELL` and writes:

- bash: `~/.local/share/bash-completion/completions/hop`
- zsh: `~/.zsh/completions/_hop`, and prints the `fpath` line to add if that directory is not on `fpath`
- fish: `~/.config/fish/completions/hop.fish`

It is idempotent and prints `open a new shell to use it`.

### Bash colon gotcha

Bash splits words on `:` (`COMP_WORDBREAKS`), which breaks `@web-1:path`. Patch the generated bash script so it calls the bash-completion helpers `__ltrim_colon_completions` on the current word, or wraps candidates accordingly. Write an integration test that completes `hop ls @web-1:ba` in bash and expects `@web-1:backups/`.

## 8. Go architecture

hop is one static Go binary (`CGO_ENABLED=0`). Commands are thin Cobra wrappers; all logic sits in `internal/` packages behind a small storage interface so tests can run against a fake.

### Libraries

| Need | Library |
| --- | --- |
| CLI + completion | `github.com/spf13/cobra` |
| S3 | `github.com/aws/aws-sdk-go-v2` (`config`, `credentials`, `service/s3`, `feature/s3/manager` for multipart) |
| Prompts / secret input | `golang.org/x/term` (keep prompts hand-written; no heavy TUI lib in v1) |
| Progress bars | `github.com/schollz/progressbar/v3` |
| Tests | standard `testing` + `github.com/stretchr/testify`; MinIO via `testcontainers-go` for integration |

Use the minimum Go version supported by the current AWS SDK v2 at build time, pinned in `go.mod`.

### Project layout

```text
hop/
  cmd/hop/main.go              # calls cli.Execute()
  internal/
    cli/                       # one file per command: init.go, push.go, cp.go ...
    config/                    # load/save ~/.config/hop/config.json (0600)
    joincode/                  # encode/decode hop1:<base64url(json)>
    target/                    # @-argument parser (section 6)
    store/                     # Storage interface + S3 implementation
      storage.go
      s3.go
      fake.go                  # in-memory, for unit tests
    workspace/                 # devices, shelves, store ops on top of Storage
    cache/                     # completion cache (section 7)
    complete/                  # completion functions used by cli/
    ui/                        # tables, colors, relative time, prompts, progress
  scripts/e2e.sh
  .goreleaser.yaml
  Makefile
  README.md
```

### Storage interface

```go
type Object struct {
    Key      string
    Size     int64
    Modified time.Time
    Meta     map[string]string
}

type Storage interface {
    Put(ctx context.Context, key string, r io.Reader, size int64, opt PutOptions) error // opt.IfNoneMatch, opt.Meta
    Get(ctx context.Context, key string) (io.ReadCloser, Object, error)
    Head(ctx context.Context, key string) (Object, error)
    Delete(ctx context.Context, keys ...string) error            // batches of 1000
    Copy(ctx context.Context, src, dst string) error             // server-side; multipart copy above 5 GB
    List(ctx context.Context, prefix, delimiter string, limit int) (objs []Object, dirs []string, err error)
}

var ErrNotFound = errors.New("not found")
var ErrPreconditionFailed = errors.New("precondition failed")
```

### Transfers

- Uploads go through `manager.Uploader` (multipart above 16 MB, part size 16 MB, concurrency 4).
- Downloads go through `manager.Downloader` into `<dest>.hop-tmp`, then `os.Rename` on success; delete the temp file on error or Ctrl-C.
- Show a progress bar on stderr for transfers larger than 1 MB when stderr is a TTY; never with `-q` or `--json`.
- `cp -r` runs up to 8 file transfers concurrently.
- Handle `SIGINT` via `signal.NotifyContext` so in-flight requests are cancelled cleanly.

### Build and release

- `make build`, `make test`, `make e2e`, `make lint` (golangci-lint).
- GoReleaser builds `linux/amd64`, `linux/arm64`, `darwin/arm64` and publishes GitHub release archives.
- Install script: `curl -fsSL <repo>/install.sh | sh` downloads the right binary into `~/.local/bin` (or `/usr/local/bin` with sudo).
- Version string injected with `-ldflags "-X main.version=..."`; `hop --version` prints it.

## 9. Security model

Every device holds the same S3 keys, so the security boundary is the IAM user behind those keys. hop's job is to make that boundary small and to say plainly what it cannot protect.

**What hop does**

- Recommends a dedicated IAM user scoped to one bucket and prefix (`hop init --print-policy`), and warns at `init` if the keys can list other buckets (`ListBuckets` succeeds).
- Stores credentials only in `~/.config/hop/config.json` with mode `0600`; refuses to run and prints a fix if the file is group- or world-readable.
- Supports `hop join` reading the code from stdin so it stays out of shell history.
- Labels the invite output: `contains your S3 keys — treat it like a password`.
- Never prints the secret key, including in `-v` logs and error messages.
- Rejects remote paths containing `..` and local writes outside the destination directory during `cp -r` downloads.

**What hop does not do (documented in README)**

- `hop remove` is a soft removal. A removed device still has working keys until they are rotated in IAM. The real revoke procedure is: create new keys, run `hop invite` with them on one device, then `hop rotate <code>` on every remaining device, then delete the old keys in IAM.
- Files are stored unencrypted at the application level (S3 server-side encryption still applies). Client-side encryption is future work.
- Anyone with the keys can read and delete every device's files.

## 10. UX conventions

Output is short, human-first on a TTY and stable for scripts with `--json`; every error says what went wrong and the exact command that fixes it.

**Output**

- Success lines start with `✔` (plain `ok:` with `--no-color` or when `NO_COLOR` is set). Errors go to stderr prefixed `error:`.
- Tables have no borders, aligned columns, header in dim text. Sizes are human-readable (`12 MB`), times relative (`2m ago`).
- `--json` prints one JSON array or object to stdout and nothing else.
- Interactive prompts appear only when stdin is a TTY. Without a TTY, a missing required value is an error, and confirmations require `-y`.
- Running `hop pull` with no name on a TTY shows a numbered pick list of the shelf; without a TTY it errors.

**Error messages (examples to match)**

```text
error: not configured — run 'hop init' or 'hop join <code>'
error: "dmp.sql" not on shared shelf — did you mean dump.sql?
error: "dump.sql" not on shared shelf — did you mean: hop pull dump.sql @
error: unknown device "web1" — did you mean web-1?
error: exists: @web-1:backups/db.sql (use -f to overwrite)
error: S3 access denied on PutObject my-hop/hot/_shared/x — check the IAM policy ('hop init --print-policy')
```

**Exit codes**

| Code | Meaning |
| --- | --- |
| 0 | success |
| 1 | general error |
| 2 | usage error (bad flags or arguments) |
| 3 | not found (file, device, path) |
| 4 | already exists / conflict |
| 5 | auth or permission error from S3 |
| 6 | network error or timeout |
| 130 | interrupted (Ctrl-C) |

## 11. Testing

Unit tests run against the in-memory fake storage; integration tests run the real binary against MinIO in Docker, so no AWS account is needed in CI.

- **Unit:** `target` parser (every row of the section 6 rules, including `@` in any position and `..` rejection), `joincode` round-trip and bad inputs, destination rules for `cp`, cache read/write/staleness, did-you-mean suggestions.
- **Workspace logic on the fake:** init twice fails, name collision returns `ErrPreconditionFailed`, push/pull/overwrite on shared and device shelves, `pull --rm`, `cp` refusing to overwrite, `rm -r` confirmation threshold, `rename` moving shelf and store.
- **Integration (MinIO via testcontainers-go):** two config directories simulate two devices. Script: device A `init` → `invite` → device B `join` → A `push x @B` → B `pull x @` → byte-for-byte compare → B `cp -r` a folder → A `ls -R` → remote-to-remote `cp` → `rm`. Include a 50 MB file to exercise multipart.
- **Completion:** run `hop __complete ...` directly and assert candidates for each row of the section 7 table; one bash-in-Docker test for the colon case.
- **CI:** GitHub Actions on push: `go vet`, golangci-lint, unit tests with `-race`, integration tests.

## 12. Build milestones

Build in this order; each milestone ends with its tests passing and a working binary, so the tool is usable from milestone 3 onward.

1. **Skeleton.** Go module, Cobra root with global flags, `--version`, Makefile, golangci-lint, CI. Done when `hop --help` lists every command as a stub.
2. **Foundations.** `config`, `joincode`, `target` parser, `store` interface with S3 and fake implementations. Done when unit tests for sections 3, 4 (code format) and 6 pass.
3. **Setup.** `init` (with `--print-policy`), `invite`, `join`, `whoami`, `devices`. Done when two config dirs against MinIO can init and join, and a duplicate name is rejected.
4. **Hot shelves.** `push`, `pull`, `hot`, `hot rm`, `hot clear`, with shared, device and `@` shelves, progress bars and mode restore. Done when the integration script's hot steps pass.
5. **Store.** `cp` (all directions, `-r`, `-f`), `ls` (`-l`, `-R`), `mv`, `rm` (`-r`). Done when the integration script's store steps pass, including the 50 MB multipart file.
6. **Completion and cache.** Cache package, write-through updates, `__refresh`, completion functions, `completion install`, bash colon fix. Done when every row of the section 7 table is covered by a test.
7. **Management and polish.** `rename`, `remove [--purge]`, `rotate`, `status`, `keep`, store→hot bridge, `--json` on list commands, did-you-mean everywhere, exit codes. Done when section 10's examples are reproduced in tests.
8. **Release.** GoReleaser config, install script, README with a 60-second quickstart, the cheat sheet and the security notes from section 9.

**Instructions for the implementing agent**

- Treat this document as the source of truth; if something is ambiguous, choose the simplest behavior consistent with the `@` rule and note the choice in `DECISIONS.md`.
- Do not add commands, flags or dependencies beyond this spec without noting them in `DECISIONS.md`.
- Keep `internal/cli` free of S3 calls; it parses arguments and calls `workspace`.

## 13. Future work

These are out of scope for v1 but the design leaves room for them; do not build them yet.

- **S3-compatible providers:** Cloudflare R2, MinIO, Backblaze B2, Wasabi via the existing `endpoint` field (path-style addressing flag).
- **Client-side encryption:** a workspace key carried in the join code; files encrypted before upload.
- **Optional expiry for hot shelves** (`hop push --ttl 7d`) using S3 lifecycle rules.
- **`hop watch`:** wait for a file to appear on a shelf and pull it automatically.
- **Resumable transfers** for very large files on unstable links.
