# file-transfer — Design (00004-shared-store)

## Idea
The shared area already has a reserved name: the shared shelf is `hot/_shared/`. The shared
store reuses it as if it were a device: `store/_shared/`. Only the spelling layer changes;
every store and shelf code path runs unchanged.

```
 argument        parser (target)            workspace                  S3 key
 ─────────       ────────────────           ──────────                 ──────────────────────
 @web-1:x   ──►  Remote{web-1, x}     ──►   Resolve(web-1): record? ─► store/web-1/x
 @@:x       ──►  Remote{_shared, x}   ──►   Resolve(_shared): always ► store/_shared/x
 @@         ──►  Device{_shared}      ──►   shelf _shared            ► hot/_shared/…
 (no @)     ──►  (no shelf)           ──►   shelf _shared            ► hot/_shared/…
```

## Spelling layer
- `naming.SharedAlias = "@"`: the text after the `@` mark (`@` + `@` = `@@`).
- `naming.Devices.Resolve(alias)` maps `@` → `_shared`; `Devices.Alias(name)` maps back.
- `target.Parser` resolves the alias before device validation (so `_shared` stays rejected
  when typed directly).
- `workspace.RemoteRef` prints `@@:path` through `Devices.Alias`.
- `workspace.Usage.Ref()` builds the status WHERE column: mark + alias + `:` for stores.
- `complete.Completer` adds `@@` / `@@:` to device marks and roots and resolves the alias.

## Existence
`DeviceManager.Resolve` accepts `_shared` without a device record, like `Self`.

## Untouched
`rename` / `remove --purge` work on validated device names; `_shared` and `@` can never be
one, so they cannot reach the shared folder.
