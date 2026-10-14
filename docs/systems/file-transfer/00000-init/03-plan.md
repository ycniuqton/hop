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
- [~] Integration: two config dirs against MinIO init + join, duplicate name rejected _(note: passes in-process against the real S3 client over gofakes3 + conditional-write shim; the MinIO run needs Docker — test is written in test/, see M8)_

## M4 — Hot shelves
- [x] `push` (shared / @dev / @, `--as`, dir rejection, meta, progress bar)
- [x] `pull` (dest resolution, `-f`, tmp+rename, mode restore, `--rm`, own-shelf hint, TTY pick list)
- [x] `hot` listing (NAME SIZE FROM AGE, newest first, `--all` grouping)
- [x] `hot rm`, `hot clear` (confirm / `-y`)
- [x] Flat-name validation (`/` rejected)
- [~] SIGINT cancellation + tmp cleanup _(implemented; unit test pending)_
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
- [~] `internal/cache`: devices + shelves files, atomic writes, corrupt → empty, XDG path
- [~] Write-through updates from all mutating/listing commands
- [~] Stale (>30 s) detached `hop __refresh <what>`
- [~] Completion funcs for every row of spec §7 table
- [~] Live store-path completion (1 s / 1000 keys, NoSpace, folders with `/`)
- [~] `completion install` (bash/zsh/fish, idempotent, fpath hint) + `completion <shell>`
- [~] Bash colon patch (`__ltrim_colon_completions`)
- [~] Tests: `hop __complete` per table row; bash-in-Docker `hop ls @web-1:ba`

## M7 — Management & polish
- [ ] `rename` (record + shelf + store move, local config if self)
- [ ] `remove [--purge]` with key-rotation warning
- [ ] `rotate <code>`
- [ ] `status` (counts/bytes per shelf and per device store)
- [ ] Bridges: `keep` (hot→store), `push @dev:path` (store→hot)
- [ ] `--json` output on all list commands
- [ ] Did-you-mean (Levenshtein ≤ 2) for devices and hot names everywhere
- [ ] Exit codes wired end-to-end; error messages match spec §10 examples
- [ ] Tests reproducing spec §10 examples

## M8 — Release
- [ ] `.goreleaser.yaml` (linux/amd64, linux/arm64, darwin/arm64)
- [ ] `install.sh` (`~/.local/bin` or `/usr/local/bin`)
- [ ] README: 60-second quickstart, cheat sheet, security notes (spec §9)
- [ ] Final review of `DECISIONS.md`
