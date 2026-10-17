# file-transfer — Requirements (00004-shared-store)

Changes only. `00000-init` … `00003-windows` still apply unless stated here.

## Business intent
Give the workspace one permanent, organized place that belongs to no device: team docs,
shared configs, release artifacts. Hot shelves are for handoffs; per-device stores are tied
to one machine. The shared store fills the gap with the least new surface: it behaves exactly
like a device store, addressed with `@@:`.

## Functional requirements
- FR-S1 `@@:path` addresses the shared store in every store command: `cp`, `ls`, `mv`, `rm`,
  and as the target of `keep` and the source of `push`.
- FR-S2 Every device reads and writes it; no device owns it. It exists without setup.
- FR-S3 `@@` (no colon) is the shared shelf, the same as leaving out `@` in hot commands.
- FR-S4 Same rules as device stores: no overwrite without `-f`, `rm -r` confirmation
  threshold, server-side copy between stores, `..` cannot escape it.
- FR-S5 Messages, errors and JSON refs spell it `@@:path`, never the internal key name.
- FR-S6 `hop status` lists it; its WHERE column uses the @ notation for every row
  (`@@`, `@web-1`, `@@:`, `@web-1:`).
- FR-S7 Tab completion offers `@@` and `@@:` and lists shared-store paths.
- FR-S8 `hop remove --purge` and `hop rename` never touch the shared store.
- FR-S9 README, cheat sheet and the agent skill document it.
- FR-S10 `cp`, `mv` and `rm` say where the files went, with `@` and `@me` resolved to the
  device name (`→ @laptop:`, `from @@:old/`), in text, JSON (`to` / `from`) and the rm prompt,
  so a slip between `@:` and `@@:` is visible.

## Constraints
- No new bucket layout concept: keys live at `store/_shared/…`. `_shared` cannot be a device
  name, so there is no collision, and older hop versions simply ignore the folder.
- No new reserved device name a user could already have (rejected `@shared` for this reason,
  and because PowerShell reads `@shared:x` as a scoped variable and passes an empty string).
- PowerShell needs `'@@:…'` quoted, like every other `@` argument.
