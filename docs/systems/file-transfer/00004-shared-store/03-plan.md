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
- [x] Commit and push
- [x] CI green, tag v0.5.0, verify assets and install

## S7 — Two-machine README demos (added after v0.5.0)
- [x] tmux split screen inside VHS: panes titled server-1 / server-2 (`demo/split.sh`, `demo/tmux.conf`, `demo/pane.rc`)
- [x] Scenes: hot push/pull, device store, global space `@@:`, setup (init → invite → paste code → join)
- [x] Replace `demo/usage.tape` with the three scenes; README "See it" section
- [x] Fix folder-copy examples (`hop cp -r ./logs @web-1:` → `@web-1:logs/…`) in README, `cp --help`, skill
- [x] No real hostname in the GIFs (`hop whoami` instead of `hop devices`)
