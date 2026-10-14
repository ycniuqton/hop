# file-transfer — Requirements (00001-storage-providers)

Changes only. Everything in `00000-init/01-requirements.md` still applies unless stated here.

## Business intent
Let people run hop on S3-compatible providers other than AWS, starting with **Bizfly Cloud**
(Vietnam), and make `hop init` usable with no flags: every missing value is asked interactively.

## Functional requirements
- FR-P1 `hop init` with no flags on a TTY asks, in order:
  1. **Storage provider** — **Bizfly Cloud first** and the default, then Amazon S3, then
     "Other S3-compatible". On a real terminal it is a menu moved with ↑/↓ (or j/k) and picked
     with Enter; digits jump straight to an item. Elsewhere it is a numbered list.
  2. **Bucket** — for Bizfly, the bucket URL shown in the Bizfly console
     (`https://<bucket>.hcm.ss.bfcplatform.vn`) or just the bucket name.
     Amazon S3 asks bucket name + region; Other asks endpoint URL + bucket name.
  3. **Access key** and **secret key** (secret not echoed; `AWS_*` env vars are defaults).
  4. **Key prefix** (optional) and **device name** (default from hostname), as before.
- FR-P2 New flags `--provider <bizfly|aws|s3>` and `--url <bucket URL>` skip those questions.
  `--url` alone works out provider, endpoint and bucket.
- FR-P3 Existing flags keep working: `--bucket --region --endpoint --prefix --name`.
  Without a TTY and without `--provider`, the provider is Amazon S3 (old behaviour), or
  "Other" when `--endpoint` is given.
- FR-P4 Region is fixed by the provider when it has one (Bizfly: `us-east-1`, which Bizfly
  ignores) and defaults to `us-east-1` for "Other"; only Amazon S3 asks for it.
- FR-P5 Server-side copy must work on providers without `CopyObject` (Bizfly returns
  `501 NotImplemented`): fall back to multipart `UploadPartCopy`; empty objects are rewritten
  empty with the same metadata. No bytes pass through the client.
- FR-P6 `hop whoami` shows the provider name (derived from the endpoint, so joined devices
  show it too without a new join-code field).
- FR-P7 SDK checksum debug lines ("Response has no supported checksum") never reach the
  terminal on non-AWS providers.

## Constraints
- No new dependencies. Join code format (`hop1:`) unchanged.
- Adding a provider = one registry entry (Devlens rules 14/15).
- Credentials still only in `config.json` (0600); never printed.

## Unchanged
Commands, `@` grammar, bucket layout, cache, completion, exit codes, release process.
