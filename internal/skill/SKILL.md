---
name: hop
description: Move files between the user's own machines (laptop, servers) through their S3 bucket with the `hop` CLI. Use when the user asks to send, hand off, copy, back up or fetch files between machines, list what is stored for a machine, or set up hop on a new machine.
---

# hop — files between your machines through one bucket

`hop` is installed as a CLI. Every machine ("device") in a workspace talks to the same
bucket (Bizfly Cloud, Amazon S3 or any S3-compatible service). There is no server: a file
sent from one machine is picked up on another by running hop *there*.

## Ground rules for agents

- Always add `--json` and read the result. A success prints one JSON object (or list) on
  stdout. A failure prints `{"error","hint","code","kind"}` on stderr and exits non-zero.
- Never print, echo, log or paste access keys, secret keys or join codes. A join code
  (`hop1:…`) **contains the keys**.
- Never put a secret in a command argument. Use `HOP_ACCESS_KEY` / `HOP_SECRET_KEY` env vars
  or `--secret-stdin`.
- Do not add `-y` (skip confirmation) or `-f` (overwrite) unless the user asked to delete or
  overwrite. Without them hop refuses safely, and you can report that.
- hop never prompts when stdin is not a terminal. A missing value is an error that names the
  flag or env var to set.

## Is it set up here?

```bash
hop whoami --json        # exit 1 + kind "general": not configured on this machine
hop devices --json       # machines in the workspace: [{name, hostname, last_seen}]
```

## Two ways to keep files

| Use | Mode | Names | Overwrite |
| --- | --- | --- | --- |
| Quick handoff ("send this to web-1") | **hot shelf**: `push` / `pull` / `hot` | flat names | same name replaces |
| Organized files, backups, folders | **store**: `cp` / `ls` / `mv` / `rm` | real paths per device | only with `-f` |

## The `@` rule (same in every command)

- no `@` → the shared shelf (hot commands); a local path (store commands)
- `@web-1` → web-1's shelf (hot) · `@` or `@me` → this machine
- `@web-1:backups/db.sql` → a path in web-1's store · `@:` → this machine's store root
- `@@:docs/x.md` → the shared store, one tree every machine reads and writes · `@@` → the shared shelf
- Folders end with `/`. A destination ending in `/` keeps the source name.

## Recipes

```bash
# hand a file to another machine (it must then run: hop pull dump.sql @)
hop push dump.sql @web-1 --json

# pick up what was sent to this machine, then delete it from the shelf
hop hot @ --json
hop pull dump.sql @ ./dump.sql --rm --json

# back up a folder into this machine's store, then list it
hop cp -r ./logs @:logs/ --json
hop ls -R @:logs/ --json

# fetch from another machine's store; copy between stores without downloading
hop cp @web-1:backups/db.sql ./db.sql --json
hop cp @web-1:backups/db.sql @db-1:restore/ --json

# files every machine should find in one place (not tied to a device)
hop cp ./guide.md @@:docs/ --json
hop ls -R @@: --json

# hot ↔ store bridges
hop keep dump.sql @:backups/ --json            # shared shelf → my store
hop push @web-1:backups/db.sql @db-1 --json    # web-1's store → db-1's shelf

hop status --json                               # sizes per shelf and store
```

## Setting up a machine (only when the user asks)

First machine, non-interactive. The user supplies the bucket URL and keys; read them from the
environment, never inline:

```bash
HOP_ACCESS_KEY="$KEY" HOP_SECRET_KEY="$SECRET" \
  hop init --url https://my-bucket.hcm.ss.bfcplatform.vn --name laptop --json
# Amazon S3: hop init --provider aws --bucket my-hop --region eu-west-1 --name laptop --json
# other S3:  hop init --endpoint https://s3.example.com --bucket my-hop --name lab --json
```

Another machine: move the join code over ssh so it never appears in the chat or in history:

```bash
hop invite --code | ssh web-1 'hop join --name web-1 --json'
```

If there is no ssh access, ask the user to run `hop invite` and `hop join` themselves.

## Exit codes (`kind` in JSON errors)

0 ok · 1 general · 2 usage · 3 not_found · 4 conflict (exists; needs `-f`) · 5 auth ·
6 network · 130 interrupted. On `not_found`, read `hint`: it often holds the right name
("did you mean …?") or the right command.

## Other commands

`hop rename <old> <new>`, `hop remove <device> [--purge]`, `hop rotate <code>`,
`hop hot rm <name>`, `hop hot clear`, `hop mv`, `hop rm [-r]`. Run `hop <command> --help`
for the flags.

Keeping hop itself current: `hop update --check --json` reports whether a newer release exists.
Run `hop update` only when the user agrees, then `hop skill install` to refresh this skill.
Never run `hop uninstall` unless the user explicitly asks: it deletes this machine's credentials.
