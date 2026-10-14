# file-transfer — Plan (00001-storage-providers)

States: `[ ]` todo · `[~]` in progress · `[x]` done. Carried over: all `00000-init` items are `[x]`.

## P1 — Provider registry
- [x] `store.Provider` + `ProviderManager` (List, Lookup, FromEndpoint, ParseURL)
- [x] Default registry: Bizfly Cloud (first), Amazon S3, Other S3-compatible
- [x] Unit tests for ParseURL / FromEndpoint / order

## P2 — Interactive init
- [x] `Console.Choose` (numbered list with a default)
- [x] `init --provider` and `--url` flags
- [x] Provider prompt first; per-provider questions; region from provider
- [x] Generic key labels ("Access key", "Secret key") for non-AWS providers
- [x] `whoami` shows provider (text + `--json`)
- [x] CLI tests: interactive Bizfly / AWS / Other flows, `--url` only, legacy flags

- [x] Arrow-key menu for `Console.Choose` on a real terminal (↑/↓, j/k, digits, Enter); numbered list elsewhere
- [x] Unit tests for menu key handling and rendering _(plus a real-pty check: ↓ + Enter, typing after the menu)_

## P3 — Provider compatibility
- [x] `ErrNotSupported` sentinel for `NotImplemented`
- [x] Copy fallback to `UploadPartCopy`; empty-object rewrite
- [x] Test shim: 501 CopyObject + UploadPartCopy emulation over gofakes3
- [x] `ResponseChecksumValidation = WhenRequired` for custom endpoints

## P4 — Docs and checks
- [x] README: providers section, interactive init example
- [x] DECISIONS.md entries
- [x] Live check against Bizfly `hop-cli` bucket (trial prefix, cleaned up)
- [x] vet, lint, tests green
- [~] Release v0.2.0 (needs explicit go-ahead to commit, push and tag)
