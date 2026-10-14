# file-transfer — Design (00000-init)

Full detail: `hop/hop — Development Spec.md`. This file records the architecture and flows to implement.

## 1. Bucket layout

```text
<bucket>/<prefix>/
  .hop/workspace.json          # {version, workspace_id, created_at, created_by}
  .hop/devices/<name>.json     # {name, device_id, hostname, os, hop_version, joined_at, last_seen}
  hot/_shared/<name>           # shared shelf (flat)
  hot/<device>/<name>          # device shelf (flat), meta: hop-from, hop-mode
  store/<device>/<path...>     # store tree
```

Local: `~/.config/hop/config.json` (0600, `HOP_CONFIG` overrides it), cache `~/.cache/hop/<workspace_id>/`.

## 2. Package architecture

```text
cmd/hop/main.go ──▶ internal/cli  (Cobra commands: parse args only, no S3)
                        │
                        ├─▶ internal/target    (@ parser → Target{Kind,Device,Path,Dir})
                        ├─▶ internal/complete  (completion funcs) ─▶ internal/cache
                        └─▶ internal/workspace (business: devices, shelves, store ops)
                                 │
                                 ├─▶ internal/store.Storage  (interface)
                                 │      ├─ S3Storage   (aws-sdk-go-v2, manager up/down)
                                 │      └─ FakeStorage (in-memory, tests)
                                 ├─▶ internal/config   (load/save 0600)
                                 ├─▶ internal/joincode (hop1:<base64url json>)
                                 └─▶ internal/ui       (tables, prompts, progress, reltime)
```

Key mapping: shared `n` → `<p>hot/_shared/n`; `@d` `n` → `<p>hot/d/n`; `@d:p` → `<p>store/d/p`.

## 3. Setup flows

```text
[user] hop init
└─ collect inputs (flags → prompt → AWS_* env defaults; secret entered without echo)
   └─ verify: HeadBucket → Put+Delete .hop/.probe-<rand>
      ├─ fail → print the failing step + AWS code, exit 5
      └─ ok → HEAD .hop/workspace.json
         ├─ exists → error "bucket already has a hop workspace…", exit 4
         └─ missing → Put workspace.json (If-None-Match:*)
            └─ register device (see below)
               └─ write config 0600 → offer completion → print "Next: run 'hop invite'"
---
[user] hop invite [--code]
└─ read config → encode {v,b,r,p,e,k,s,w} → base64.RawURLEncoding
   ├─ --code → print the bare code
   └─ default → print the warning + "hop join hop1:…"
---
[user] hop join <code | stdin> [--name] [--force]
└─ strip a leading "hop join ", check the prefix
   ├─ missing/unknown prefix → usage error, exit 2
   └─ hop1 → decode JSON
      └─ local config exists?
         ├─ yes, no --force → error, exit 4
         └─ no / --force → verify credentials → Get workspace.json
            ├─ workspace_id ≠ w → error, exit 1
            └─ match → choose name (--name or prompt with the hostname as default)
               └─ register device → write config → offer completion → "Joined … (N devices)"
---
[internal] register device
└─ Put .hop/devices/<name>.json (If-None-Match:*)
   ├─ 412 → "device name … already used", prompt again if TTY else exit 4
   └─ 200 → write-through cache devices.json
```

## 4. Hot shelf flows

```text
[user] hop push <file>... [@dev|@] [--as n]
└─ target.Parse all args → at most one Device target
   └─ resolve shelf: none → _shared; @dev → dev (unknown → did-you-mean, exit 3); @ → self
      └─ per file: directory → error "push sends single files…"
         └─ Uploader.Put hot/<shelf>/<name> meta{hop-from,hop-mode} → progress bar on stderr
            └─ update cache shelves/<shelf>.json locally (no list call)
---
[user] hop pull [name] [@dev|@] [dest] [--rm] [-f]
└─ no name → TTY: numbered pick list from the shelf; no TTY → exit 2
└─ Head hot/<shelf>/<name>
   ├─ not found on _shared but present on own shelf → hint "hop pull <name> @", exit 3
   ├─ not found → did-you-mean, exit 3
   └─ found → resolve dest (dir → inside; exists without -f → exit 4)
      └─ Downloader → <dest>.hop-tmp
         ├─ error / Ctrl-C → delete tmp, exit 6/130
         └─ ok → rename → chmod hop-mode
            └─ --rm → Delete object → update cache
```

## 5. Store flows

```text
[user] hop cp [-r] [-f] <src>... <dst>
└─ parse targets; a bare Device target → hint "did you mean @dev: ?", exit 2
   └─ choose the direction from which side has @
      ├─ local → local   → error "use the regular cp", exit 2
      ├─ local → remote  → walk (needs -r for dirs) → upload ×8 concurrently
      ├─ remote → local  → List prefix (-r) → download ×8 → refuse paths outside dest
      └─ remote → remote → Storage.Copy (server-side, multipart above 5 GB)
   └─ per object: destination key from the rules (trailing / or root → keep the base name)
      ├─ exists && !-f → "exists: @dev:path (use -f to overwrite)", exit 4
      └─ else → transfer
---
[user] hop rm [-r] @dev:path...
└─ list the keys under the prefix (-r)
   ├─ >10 objects and no -y → TTY confirm / no TTY → exit 2
   └─ Delete in batches of 1000
---
[user] hop keep <name> [@dev] <@dev:path>     → Copy hot/… → store/…
[user] hop push <@dev:path> [@dev]            → Copy store/… → hot/…
```

## 6. Completion & cache

```text
[shell TAB] hop __complete …
└─ which kind of word is being completed?
   ├─ command          → Cobra command tree
   ├─ word starts "@"  → cache devices.json → "@", "@<dev>"
   ├─ hot name         → cache shelves/<shelf>.json (shelf from any @ on the line)
   ├─ "@dev:" prefix   → live List(delim "/", 1000 keys, 1 s timeout) → NoSpace; timeout → empty
   └─ device arg       → cache devices.json (no @)
   └─ cache file older than 30 s → answer from it + start detached `hop __refresh <what>` [async]
```

- Cache writes: temp file + rename. A corrupt file is deleted and treated as empty.
- Bash: patch the generated script with `__ltrim_colon_completions`.

## 7. Storage interface

`Put(key, r, size, {IfNoneMatch, Meta})`, `Get`, `Head`, `Delete(keys...)`, `Copy(src,dst)`,
`List(prefix, delim, limit) → (objs, dirs)`. Sentinel errors are `ErrNotFound` and `ErrPreconditionFailed`.
The S3 implementation maps AWS errors to these errors and to exit codes 3/4/5/6.

## 8. Errors & exit codes
A domain error type carries `{ExitCode, Message, Hint}`. The CLI root turns it into the exit status.
Codes: 0 ok, 1 general, 2 usage, 3 not found, 4 conflict, 5 auth, 6 network, 130 interrupted.

## 9. Mapping Devlens coding rules onto Go
- "Class / domain class" → a Go struct with methods. "Base class" → an interface plus a shared embedded struct where useful.
- Extensible domains, each with an interface and a Manager:
  - Storage backends: `store.Storage`, chosen by config (`endpoint`) through a registry. No switch on concrete types.
  - Shell completion installers: `ShellInstaller` interface (bash/zsh/fish), with a registry keyed by a `Shell` enum.
  - Transfer directions: `TransferStrategy` interface (upload/download/copy), selected by a `Direction` enum.
  - Output renderers: `Renderer` (table/json), selected by flags.
- `workspace` exposes managers: `DeviceManager`, `ShelfManager`, `StoreManager`. The `cli` package talks only to these.
- Constants are grouped in domain types: `ExitCode`, `Kind`, `Shell`, `ShelfName`, `Limits{MultipartSize…}`.
- Functions have at most 50 lines and at most 3 nesting levels. Comments go only at the top of a declaration.
- If a rule conflicts with the spec or with idiomatic Go → spec wins (Rule 0); record it in `DECISIONS.md`.
