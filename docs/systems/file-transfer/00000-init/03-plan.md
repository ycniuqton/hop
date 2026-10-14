# file-transfer — Plan (00000-init)

States: `[ ]` todo · `[~]` in progress · `[x]` done. Each milestone ends with green tests and a working binary.

## M1 — Skeleton
- [x] Init Go module `hop`, pin the Go version supported by aws-sdk-go-v2
- [x] `cmd/hop/main.go` + Cobra root with global flags `-q --json --no-color -v`
- [x] `--version` via `-ldflags "-X main.version=..."`
- [x] Stub every v1 command so that `hop --help` lists them
- [x] Makefile targets: build, test, e2e, lint _(note: `make` not installed locally; targets verified by running the go commands directly)_
- [x] golangci-lint config + GitHub Actions (vet, lint, `-race` unit, integration) _(note: lint and `-race` not runnable locally — no golangci-lint/gcc; enforced in CI)_
- [x] Create `DECISIONS.md`

## M2 — Foundations
- [x] `internal/config`: load/save 0600, `HOP_CONFIG`, refuse group/world-readable files
- [x] `internal/joincode`: encode/decode `hop1:` RawURLEncoding, strip "hop join ", reject unknown prefixes
- [x] `internal/target`: Local/Device/Remote parser, first-colon split, path.Clean, `..` rejection
- [x] Device name validation + reserved names (`_shared`, `me`)
- [x] `internal/store`: `Storage` interface, sentinel errors, backend registry
- [x] S3 implementation (Put IfNoneMatch, Get, Head, Delete batched, Copy multipart >5 GB, List) _(note: multipart copy >5 GB is written but untested — too large for unit tests; add a lowered-threshold MinIO test in M5)_
- [x] In-memory fake implementation
- [x] Domain error type + `ExitCode` enum
- [x] Unit tests: target (every rule), joincode round-trip/bad input, config perms _(also: storage contract suite run on fake + S3 client via gofakes3)_

## M3 — Setup
- [x] `init` (inputs, credential verify, workspace existence, conditional write, config, completion offer)
- [x] `init --print-policy` + ListBuckets warning
- [x] Device registration with 412 handling and re-prompt
- [x] `invite` / `invite --code`
- [x] `join` (arg or stdin, workspace_id check, `--force`)
- [x] `whoami`, `devices` (relative last seen, `(you)` marker)
- [x] `last_seen` throttled update (≤ once / 5 min)
- [x] Workspace tests on the fake (init twice fails, name collision)
- [x] Integration: two config dirs against MinIO init + join, duplicate name rejected _(note: `test/integration_test.go` passes in CI against `bitnamilegacy/minio` — official image no longer on Docker Hub, DECISIONS #24; no Docker locally, verified there via `HOP_S3_ENDPOINT`)_

## M4 — Hot shelves
- [x] `push` (shared / @dev / @, `--as`, dir rejection, meta, progress bar)
- [x] `pull` (dest resolution, `-f`, tmp+rename, mode restore, `--rm`, own-shelf hint, TTY pick list)
- [x] `hot` listing (NAME SIZE FROM AGE, newest first, `--all` grouping)
- [x] `hot rm`, `hot clear` (confirm / `-y`)
- [x] Flat-name validation (`/` rejected)
- [x] SIGINT cancellation + tmp cleanup _(tests: interrupted download leaves no `.hop-tmp`; cancelled run exits 130 quietly)_
- [x] Tests: push/pull/overwrite on shared and device shelves, `pull --rm`
- [x] Integration: hot steps of the e2e script

## M5 — Store
- [x] `cp` direction strategies (upload / download / remote copy / local-local error)
- [x] Destination rules (trailing `/`, device root, rename, multi-source needs folder)
- [x] `-r` walk with relative paths, 8-way concurrency, path-escape guard on download
- [x] `-f` overwrite check
- [x] `ls` (`@:` default, folders first, `-l`, `-R`)
- [x] `mv` (remote→remote, copy then delete)
- [x] `rm` / `rm -r` with >10 confirm threshold
- [x] Multipart uploader config (16 MB parts, concurrency 4)
- [x] Tests: cp refuses overwrite, rm -r threshold
- [x] Integration: store steps incl. 50 MB multipart file _(run in-process over gofakes3; MinIO run pending Docker)_

## M6 — Completion & cache
- [x] `internal/cache`: devices + shelves files, atomic writes, corrupt → empty, XDG path
- [x] Write-through updates from all mutating/listing commands
- [x] Stale (>30 s) detached `hop __refresh <what>`
- [x] Completion funcs for every row of spec §7 table
- [x] Live store-path completion (1 s / 1000 keys, NoSpace, folders with `/`)
- [x] `completion install` (bash/zsh/fish, idempotent, fpath hint) + `completion <shell>`
- [x] Bash colon patch (`__ltrim_colon_completions`) _(note: Cobra's V2 script already trims via `__hop_handle_special_char`; no patch needed — DECISIONS #22)_
- [x] Tests: `hop __complete` per table row; bash-in-Docker `hop ls @web-1:ba` _(note: bash test runs on the host with bash-completion instead of Docker — DECISIONS #22)_

## M7 — Management & polish
- [x] `rename` (record + shelf + store move, local config if self)
- [x] `remove [--purge]` with key-rotation warning
- [x] `rotate <code>`
- [x] `status` (counts/bytes per shelf and per device store)
- [x] Bridges: `keep` (hot→store), `push @dev:path` (store→hot)
- [x] `--json` output on all list commands
- [x] Did-you-mean (Levenshtein ≤ 2) for devices and hot names everywhere
- [x] Exit codes wired end-to-end; error messages match spec §10 examples _(also added the `✔` / `ok:` success marker — DECISIONS #21)_
- [x] Tests reproducing spec §10 examples

## M8 — Release
- [x] `.goreleaser.yaml` (linux/amd64, linux/arm64, darwin/arm64) _(validated with `goreleaser check` and a local snapshot build)_
- [x] `install.sh` (`~/.local/bin` or `/usr/local/bin`) _(tested under dash against the snapshot archives, incl. checksum mismatch)_
- [x] README: 60-second quickstart, cheat sheet, security notes (spec §9)
- [x] Final review of `DECISIONS.md`
- [x] Release workflow: GoReleaser on `v*` tag push (`.github/workflows/release.yml`)
- [x] MinIO integration test with testcontainers (`test/`, build tag `integration`)
- [x] golangci-lint clean locally (`std-error-handling` preset; SA1019 excluded for `feature/s3/manager` — DECISIONS #8)
- [ ] Tag `v0.1.0` and confirm the release + `curl … install.sh | sh` on a real machine
