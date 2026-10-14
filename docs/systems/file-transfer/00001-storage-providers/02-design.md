# file-transfer — Design (00001-storage-providers)

Changes only; the rest of `00000-init/02-design.md` is unchanged.

## 1. Provider registry (`internal/store/provider.go`)

```text
Provider            { ID, Title, Endpoint, Region, BucketHost }
ProviderManager     List() · Lookup(id) · FromEndpoint(url) · ParseURL(url) → (Provider, bucket)
Providers (default) bizfly  "Bizfly Cloud"         https://hcm.ss.bfcplatform.vn  us-east-1
                    aws     "Amazon S3"            (SDK default)                  asked
                    s3      "Other S3-compatible"  asked                          us-east-1
```

- Order of `List()` is the order shown at the prompt; Bizfly is first and the default.
- `ParseURL` accepts virtual-hosted (`https://hop-cli.hcm.ss.bfcplatform.vn`) and path-style
  (`https://hcm.ss.bfcplatform.vn/hop-cli`) URLs. A host matching no known provider becomes
  provider `s3` with that endpoint.
- Config is unchanged: the provider is derived from `endpoint` (`FromEndpoint`), so `join`,
  `rotate` and old configs need nothing new.

## 2. Interactive init

```text
hop init [flags]
└─ config already exists → error exit 4 (unchanged)
└─ pick provider
   ├─ --url given → ParseURL → provider + endpoint + bucket
   ├─ --provider given → Lookup
   ├─ --endpoint given → Other S3-compatible
   ├─ TTY → numbered list, default 1 = Bizfly Cloud
   └─ no TTY → Amazon S3
└─ ask what the provider still needs
   ├─ Bizfly → "Bucket URL or name" → ParseURL or plain name
   ├─ Amazon S3 → "S3 bucket", "AWS region"
   └─ Other → "Endpoint URL", "Bucket", region = us-east-1
└─ ask access key, secret key (env defaults), prefix (TTY only), device name
└─ verify bucket → create workspace → register device → save config (unchanged)
```

## 2b. Arrow-key menu (`internal/ui/menu.go`)

```text
Console.Choose(title, items, def)
└─ no TTY on stdin → def
└─ stdin and stderr are terminals → raw mode → Menu.Run
   ├─ ↑ / k → previous (wraps)      ├─ ↓ / j → next (wraps)
   ├─ 1..n → select that item       ├─ Enter → select current
   └─ Ctrl-C / Ctrl-D → interrupted (exit 130)
   → redraw items in place; on select collapse to "title answer"
└─ otherwise → numbered list, "Choose [n]:" (piped input, tests)
```

Keys are read one byte at a time (no buffering), so text typed for the next prompt is not lost.

## 3. Copy fallback (`internal/store/s3_copy.go`)

```text
Storage.Copy(src, dst)
└─ Head(src)
└─ size ≤ 5 GB and CopyObject not known to be missing
   ├─ CopyObject ok → done
   └─ 501 NotImplemented → remember for this client → multipart path
└─ multipart path
   ├─ size = 0 → PutObject empty body + source metadata
   └─ size > 0 → CreateMultipartUpload(meta) → UploadPartCopy × n → Complete
                 error → AbortMultipartUpload
```

- `NotImplemented` maps to the new sentinel `store.ErrNotSupported` (exit 1, message names the
  operation and the provider gap) so the fallback uses `errors.Is`, not string matching.
- The "remembered" flag is per process; every command is a new process, so the cost is one
  extra failed request per command that copies.

## 4. SDK settings for custom endpoints
When `endpoint` is set, the S3 client uses `ResponseChecksumValidation = WhenRequired`,
which stops the SDK's per-GET debug line on servers that return no checksum headers.
AWS keeps the SDK defaults.

## 5. Tests
- Provider registry: list order, lookup, FromEndpoint, ParseURL (virtual-hosted, path-style,
  bare name, unknown host, junk).
- Init: piped TTY answers drive the full prompt flow for Bizfly/AWS/Other; `--url` only;
  old flag form unchanged.
- Copy fallback: gofakes3 behind a shim that answers `CopyObject` with 501 and serves
  `UploadPartCopy` as a ranged read + UploadPart; contract suite + `cp`/`mv`/`keep`/empty file.
