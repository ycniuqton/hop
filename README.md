# hop

Move files between your own machines through one S3 bucket.

```bash
laptop$ hop push dump.sql @web-1          # hand a file to web-1
web-1$  hop pull dump.sql @               # pick it up

laptop$ hop cp -r ./logs @web-1:logs/     # or keep organized files with real paths
web-1$  hop ls @:logs/
```

There is no server, daemon or account: every machine talks straight to the bucket with the same IAM keys.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/ycniuqton/hop/main/install.sh | sh
```

This installs to `~/.local/bin`. Pipe to `sudo sh` to install to `/usr/local/bin` instead. Builds are available for Linux (amd64, arm64), macOS (Apple Silicon) and Windows (amd64, arm64).

**Windows** (PowerShell):

```powershell
irm https://raw.githubusercontent.com/ycniuqton/hop/main/install.ps1 | iex
```

This installs `hop.exe` to `%LOCALAPPDATA%\Programs\hop` and adds that folder to your PATH; open a new terminal afterwards. WSL users can use the Linux command above instead.

The installer reads a few environment variables:

| Variable | Effect |
| --- | --- |
| `HOP_VERSION` | Install a specific tag, for example `HOP_VERSION=v0.1.0`. |
| `HOP_INSTALL_DIR` | Install into a different folder. |

Keep it current, or remove it:

```bash
hop update            # replace this binary with the latest release (checksum-verified)
hop update --check    # only say whether a newer release exists
hop uninstall         # remove the binary, completions, skill, cache and config (asks first)
```

`hop uninstall` never touches the bucket. To drop the device from the workspace, run `hop remove <name>` on another machine. Use `--keep-config` to keep the credentials.

You can also build from source with Go 1.26 or later:

```bash
go install github.com/ycniuqton/hop/cmd/hop@latest
```

## 60-second quickstart

**1. Make a bucket and an access key** at your storage provider: Bizfly Cloud, Amazon S3, or any S3-compatible service. If you use Amazon S3, hop prints a minimal IAM policy for you:

```bash
hop init --print-policy --bucket my-hop
```

**2. Create the workspace on the first machine.** Run `hop init` with no flags and answer the questions:

```text
$ hop init
Storage provider:
  1) Bizfly Cloud
  2) Amazon S3
  3) Other S3-compatible
Choose [1]:
Bucket URL or name: https://hop-cli.hcm.ss.bfcplatform.vn
Access key: YOUR_ACCESS_KEY
Secret key:
Key prefix (optional):
Device name [laptop]:
✔ Created hop workspace on Bizfly Cloud in hop-cli/ as "laptop".
Next: run 'hop invite' to add another machine.
```

The secret key is never echoed. If `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` are set, they are used as the defaults.

You can skip the questions with flags:

```bash
hop init --url https://hop-cli.hcm.ss.bfcplatform.vn --name laptop       # Bizfly Cloud
hop init --provider aws --bucket my-hop --region eu-west-1 --name laptop # Amazon S3
hop init --endpoint http://minio.local:9000 --bucket my-hop --name lab   # any S3-compatible
```

**3. Add each other machine.** On the first machine, run:

```bash
hop invite
```

It prints a `hop join hop1:…` line. Then on the new machine, either paste the code when prompted (which keeps it out of shell history):

```bash
hop join --name web-1
```

or pass it directly:

```bash
hop join hop1:… --name web-1
```

**4. Move files:**

```bash
hop push dump.sql @web-1        # laptop → web-1's shelf
hop pull dump.sql @             # on web-1: take it from my shelf
```

**5. Turn on tab completion:**

```bash
hop completion install          # then open a new shell
```

## Two ways to store files

| Mode | Use it for | Names | Overwrites | Commands |
| --- | --- | --- | --- | --- |
| **Hot shelves** | Quick handoffs | Flat names | The same name replaces the old file | `push`, `pull`, `hot`, `keep` |
| **Store** | Organized files | A folder tree per device | Only with `-f` | `cp`, `ls`, `mv`, `rm` |

**The `@` rule** is the same in every command:

| You type | It means |
| --- | --- |
| no `@` | The shared shelf (hot commands). In store commands, a local path. |
| `@web-1` | web-1's shelf (hot commands) or web-1's folder root (store commands). |
| `@` or `@me` | This machine. |
| `@web-1:backups/db.sql` | A path inside web-1's store. |

## Cheat sheet

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

# bridges
hop keep dump.sql @:backups/      # hot → store (server-side copy)
hop push @web-1:backups/db.sql    # store → shared shelf (server-side copy)

# devices
hop rename web-1 web-2
hop remove old-box --purge
hop status
```

## Output and scripting

| Flag | Effect |
| --- | --- |
| `--json` | Prints one JSON document on stdout: the list for list commands, a result object for action commands. Errors go to stderr as JSON. |
| `-q` | Prints nothing except errors. |
| `-v` | Logs every S3 call to stderr. The secret key is never printed. |
| `--no-color` | Plain output. Setting `NO_COLOR` does the same. |

Without a terminal, hop never prompts:
- a missing required value is an error that names the flag to pass;
- a confirmation needs `-y`.

Exit codes:

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | General error |
| 2 | Usage error |
| 3 | Not found |
| 4 | Already exists or conflict |
| 5 | S3 auth or permission error |
| 6 | Network error or timeout |
| 130 | Interrupted (Ctrl-C) |

## Windows notes

- **In PowerShell, quote arguments that start with `@`.** PowerShell treats a leading `@` as its own syntax and rejects the line before hop runs. cmd.exe and WSL need nothing.

  ```powershell
  hop push dump.sql '@web-1'
  hop cp db.sql '@:backups/'
  hop ls '@web-1:backups/'
  ```

  Tab completion (`hop completion install`) inserts the escaping for you.
- Paths inside the bucket always use `/` (`'@:backups/2026/db.sql'`). Local paths can use either `\` or `/`.
- Config and cache live in `%USERPROFILE%\.config\hop` and `%USERPROFILE%\.cache\hop`, the same layout as on Linux and macOS.
- `hop completion install` writes `%LOCALAPPDATA%\hop\completion.ps1` and prints the line to add to your `$PROFILE`.
- `hop update` and `hop uninstall` work while `hop.exe` is running: the old file is moved aside and deleted afterwards.

## Using hop with AI agents

hop is built so coding agents (Claude Code, Codex, Cursor, …) can use it without help:

- **It never prompts without a terminal.** A missing value is an error that names the flag or environment variable to set.
- **`--json` everywhere.** List commands print their data, and action commands (`push`, `pull`, `cp`, `init`, …) print one object describing what they did. Failures print one object on stderr:

  ```json
  {"error": "unknown device \"web1\"", "hint": "did you mean web-1?", "code": 3, "kind": "not_found"}
  ```

- **One-line setup with no secret in the command:**

  ```bash
  HOP_ACCESS_KEY=… HOP_SECRET_KEY=… hop init --url https://hop-cli.hcm.ss.bfcplatform.vn --name web-1 --json
  hop init --url … --name web-1 --secret-stdin < secret.txt         # or read the secret from stdin
  hop invite --code | ssh web-1 'hop join --name web-1 --json'       # add a machine; the code never hits the screen
  ```

  `HOP_ACCESS_KEY` and `HOP_SECRET_KEY` take precedence over `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`.

**Teach your agent hop** with the skill that ships inside the binary. It always matches your hop version:

```bash
hop skill install              # Claude Code, all projects: ~/.claude/skills/hop/SKILL.md
hop skill install --project    # Claude Code, this repo only: ./.claude/skills/hop/SKILL.md
hop skill print >> AGENTS.md   # Codex, Cursor and other agents that read AGENTS.md
```

The skill covers when to use hot shelves and when to use the store, the `@` rule, recipes, exit codes, and safety rules for agents. For example, it tells them never to echo keys or join codes, and to use `-y` or `-f` only when you asked.

## Security

Every device holds **the same S3 keys**. The real security boundary is the IAM user behind those keys, so keep that user small.

**What hop does**

- Recommends a dedicated IAM user limited to one bucket (`hop init --print-policy`). It warns during `init` if the keys can list other buckets.
- Stores credentials only in `~/.config/hop/config.json` with mode `0600`. It refuses to run if that file is readable by your group or by other users, and tells you the `chmod` that fixes it.
- `hop join` can read the code from stdin, so it stays out of your shell history.
- The `hop invite` output is labelled **contains your S3 keys — treat it like a password**.
- Never prints the secret key: not in `-v` logs, not in error messages.
- Rejects remote paths containing `..`. During `cp -r` downloads it refuses to write outside the destination folder.

**What hop does not do**

- **`hop remove` is a soft removal.** A removed device's keys keep working until you rotate them in IAM. To really revoke a device:
  1. Create new access keys for the IAM user.
  2. On one remaining device, run `hop invite` with the new keys exported.
  3. Run `hop rotate <code>` on every remaining device.
  4. Delete the old keys in IAM.
- **Files are not encrypted by hop.** S3 server-side encryption still applies. Client-side encryption is future work.
- **Anyone with the keys can read and delete every device's files.**

## Storage providers

| Provider | `--provider` | What `hop init` asks |
| --- | --- | --- |
| **Bizfly Cloud** (default) | `bizfly` | Bucket URL (`https://<bucket>.hcm.ss.bfcplatform.vn`) or bucket name, then the keys |
| Amazon S3 | `aws` | Bucket, region, then the keys |
| Other S3-compatible (MinIO, …) | `s3` | Endpoint URL, bucket, then the keys |

`hop whoami` shows which provider a device uses. Machines added with `hop join` take the provider from the join code, so nothing needs to be chosen there.

The provider must support conditional writes (`If-None-Match: *`). hop relies on them so two devices can never register the same name. Bizfly Cloud, Amazon S3 and current MinIO all support them.

Some providers, including Bizfly Cloud, don't implement plain server-side copy (`CopyObject`). On those, hop switches to multipart copy (`UploadPartCopy`). Either way the copy stays inside the provider: `cp @a:x @b:y`, `mv`, `keep` and `rename` never download the data.

## Development

```bash
make build      # bin/hop
make test       # go vet + unit tests (-race)
make e2e        # real binary against MinIO in Docker (testcontainers)
make lint       # golangci-lint
```

Design notes and the build plan are in [`docs/systems/file-transfer/`](docs/systems/file-transfer/00000-init/). Choices the spec left open are recorded in [`DECISIONS.md`](DECISIONS.md).

To make a release, tag and push:

```bash
git tag v0.1.0 && git push origin v0.1.0
```

GitHub Actions then runs GoReleaser, which publishes the archives that `install.sh` downloads.
