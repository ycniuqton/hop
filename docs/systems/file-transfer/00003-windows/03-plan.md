# file-transfer — Plan (00003-windows)

States: `[ ]` todo · `[~]` in progress · `[x]` done. Carried over: all earlier items are `[x]`.

## W1 — Compile and core behaviour
- [x] Detached refresh spawn per platform
- [x] Config mode check Unix-only
- [x] Local `\` destination counts as a folder
- [x] Cross-compile + vet for windows/amd64 and windows/arm64

## W2 — Terminal
- [x] Enable virtual terminal processing on Windows; fall back to numbered list / no colour

## W3 — Self-management
- [x] Archive formats registry (tar.gz, zip) for `hop update`
- [x] Windows swap (rename aside + rollback) and `CleanStale` on start
- [x] Windows uninstall of the running `hop.exe`

## W4 — Completion
- [x] PowerShell installer + default shell on Windows + `$PROFILE` hint

## W5 — Release and install
- [x] GoReleaser: windows amd64/arm64 zip
- [x] `install.ps1` _(run under PowerShell 7 on Linux up to executing hop.exe; full run in the windows CI job)_
- [x] CI windows job (tests + install.ps1 against local assets) _(green on windows-latest)_

## W6 — Tests and docs
- [x] Tests: zip extraction, swap + rollback, CleanStale, PowerShell installer, `\` folder dest; skip Unix-only tests on Windows
- [x] README "Windows" section (PowerShell `@` quoting)
- [x] DECISIONS.md entries
- [x] vet, lint (linux + windows), tests, rule check green
- [x] Release v0.4.0 (needs explicit go-ahead; tag only after the windows CI job passes) _(windows CI green incl. install/update/uninstall; released)_

## D — README demo (added after v0.4.0)
- [x] VHS tapes `demo/setup.tape` (init menu + join) and `demo/usage.tape` (push/pull, cp, ls, status)
- [x] `demo/fakes3` local S3 server (conditional writes, ListBuckets denied) + `demo/record.sh` + `make demo`
- [x] GIFs in `assets/`, embedded in README
- [x] Fix: coloured table headers no longer skew column alignment (found in the recording)
