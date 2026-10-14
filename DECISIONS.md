# Decisions

Choices made where the spec is silent or where it had to be adapted. Newest last.

1. **Module path** is `github.com/ycniuqton/hop`, matching the GitHub repo. Go version pinned to `1.24`, the minimum required by aws-sdk-go-v2 at build time.
2. **Command stubs (M1)** live in `internal/cli/stubs.go` as data instead of one file per command. Each command moves to its own file (`init.go`, `push.go`, …) when it is implemented.
3. **Usage errors.** Any error not produced by a command handler (unknown command, bad flag, wrong argument count) exits with code 2. Handler errors without an explicit code exit with 1.
4. **`hop completion`** is our own command (Cobra's default `completion` command is replaced) so that `completion install` can be added in M6.
5. **Devlens coding rules in Go.** "Classes" are structs with methods; "base classes" are interfaces; managers live in `internal/workspace`. See `docs/systems/file-transfer/00000-init/02-design.md` §9.
6. **Domain error package `internal/apperr`** (not listed in spec §8 layout) holds `ExitCode` and the user-facing `Error` type shared by all packages.
7. **`internal/naming`** (not in spec §8 layout) owns device-name and hot-name rules, because `target`, `workspace` and `init` all need them and `target` must not import `workspace`.
8. **Upload library.** `feature/s3/manager` (named by the spec) is marked deprecated upstream in favour of `feature/s3/transfermanager`, which is still pre-1.0 (v0.4.x). v1 keeps `manager`, which is stable; revisit once transfermanager reaches 1.0.
9. **Custom endpoint ⇒ path-style addressing.** When `endpoint` is set (MinIO in tests today, S3-compatible providers later) the S3 client uses path-style URLs. The spec lists a path-style flag as future work; this default is the simplest behaviour that makes MinIO work.
10. **Storage errors** are `*apperr.Error` values (exit code + message as in spec §10) that also wrap the sentinels (`ErrNotFound`, `ErrPreconditionFailed`, plus `ErrAccessDenied` and `ErrNetwork`, which the spec does not list). Callers can use `errors.Is` and still get the right exit code.
11. **Storage manager.** `store.Manager` selects the backend (`s3` default, `memory` for tests) from `Settings.Backend`, following Devlens rules 14/15. Only the `s3` backend is user-facing in v1.
12. **Storage contract tests.** `internal/store/storetest` is one behavior suite run against the fake, against the real S3 client over `gofakes3` (test-only dependency, in-process, no Docker), and later against MinIO. gofakes3 lacks conditional writes, so that one case is skipped there and covered by MinIO.
13. **Copy-source encoding** percent-encodes every key segment, including `+`. Without this, server-side copy of keys containing `+` failed (found by the gofakes3 contract run).
14. **Config adds `workspace_id`.** The completion cache path is `~/.cache/hop/<workspace_id>/` (spec §7), so every device needs the id locally without an S3 call. `init` and `join` write it.
15. **Storage interface additions:** `Download` (parallel ranged GETs through `manager.Downloader`, as spec §8 asks), `CheckBucket` (HeadBucket for init/join verification) and `CanListAllBuckets` (the ListBuckets warning in spec §9).
