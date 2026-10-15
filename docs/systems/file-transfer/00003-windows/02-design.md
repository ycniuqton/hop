# file-transfer — Design (00003-windows)

Changes only. Platform differences live in `_windows.go` / `_unix.go` files behind build tags;
business code calls one function and never checks `runtime.GOOS`.

## 1. Platform seams

| Seam | Unix | Windows |
| --- | --- | --- |
| `cli` background refresh spawn | `Setsid` | `CREATE_NEW_PROCESS_GROUP \| DETACHED_PROCESS` |
| `config` mode check | refuse `mode & 0077` | skipped |
| `selfinstall.Binary.swap` | rename new over old | old → `hop.exe.old`, new → `hop.exe` (rollback on failure) |
| `selfinstall` binary removal | `os.Remove` | rename to `.old` → detached `cmd /c` deletes it after exit |
| `ui` terminal setup | none | enable `ENABLE_VIRTUAL_TERMINAL_PROCESSING`; failure → no colour, no menu |
| `complete` default shell | none (`$SHELL`) | `powershell` |

## 2. Release archives

```text
selfinstall.Releases.Fetch(tag, goos, goarch)
└─ Archives[goos] (default tar.gz)
   ├─ linux, darwin → hop_<os>_<arch>.tar.gz → tarFormat.extract("hop")
   └─ windows       → hop_<os>_<arch>.zip    → zipFormat.extract("hop.exe")
└─ checksum check as before
```

## 3. Self-update on Windows

```text
hop update (hop.exe running)
└─ write hop.exe.new-* next to hop.exe
└─ rename hop.exe → hop.exe.old        (allowed while running)
   └─ fails → error, nothing changed
└─ rename new → hop.exe
   └─ fails → rename hop.exe.old back → error
---
next hop run (any command)
└─ selfinstall.CleanStale → delete hop.exe.old if present (ignore errors)
```

## 4. install.ps1

```text
irm …/install.ps1 | iex
└─ arch: PROCESSOR_ARCHITECTURE AMD64 → amd64, ARM64 → arm64, else error
└─ download hop_windows_<arch>.zip + checksums.txt (latest or HOP_VERSION, or HOP_DOWNLOAD_URL)
└─ Get-FileHash SHA256 must match → else error, nothing installed
└─ Expand-Archive → copy hop.exe to %LOCALAPPDATA%\Programs\hop (or HOP_INSTALL_DIR)
└─ folder not in user PATH → add it (User scope) → "open a new terminal"
```

## 5. CI
A `windows` job on `windows-latest`: `go vet`, `go test ./...` (no `-race`), and an
`install.ps1` run against locally built zip assets served over HTTP. Tests that rely on Unix
modes, symlinks or bash skip themselves on Windows.
