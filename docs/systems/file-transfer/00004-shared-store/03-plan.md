# file-transfer — Plan (00004-shared-store)

States: `[ ]` todo · `[~]` in progress · `[x]` done. Carried over: all earlier items are `[x]`.

## S1 — Spelling and resolution
- [x] `naming.SharedAlias`, `Devices.Alias` / `Devices.Resolve`
- [x] Parser resolves `@@` / `@@:` to `_shared`
- [x] `DeviceManager.Resolve` accepts the shared area
- [x] `RemoteRef` prints `@@:`

## S2 — Surfaces
- [x] `hop status` WHERE column in @ notation
- [x] Completion offers `@@`, `@@:` and shared-store paths

## S3 — Tests
- [x] Parser cases for `@@`, `@@:`, `@@:a/b`
- [x] End-to-end: cp in/out, ls, mv, rm, cross-store copy, push `@@`, keep into `@@:`, errors, status
- [x] Completion expectations updated
- [x] Real binary against `demo/fakes3` with two machines

## S4 — Docs
- [x] README `@` table, cheat sheet, PowerShell note
- [x] Agent skill
- [x] DECISIONS entry

## S5 — Clear destinations
- [x] `cp` / `mv` / `rm` messages, JSON and the rm prompt name the resolved target (`@laptop:`, `@@:docs/`)
- [x] README demo GIFs re-recorded with the new output

## S6 — Release v0.5.0
- [~] Commit and push
- [ ] CI green, tag v0.5.0, verify assets and install
