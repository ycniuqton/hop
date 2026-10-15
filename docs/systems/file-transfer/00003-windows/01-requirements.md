# file-transfer — Requirements (00003-windows)

Changes only. `00000-init` … `00002-agent-support` still apply unless stated here.

## Business intent
Run hop natively on Windows (amd64, arm64) with the same commands and behaviour as Linux and
macOS, installed with one PowerShell line. WSL keeps working with the Linux build.

## Functional requirements
- FR-W1 Releases include `hop_windows_amd64.zip` and `hop_windows_arm64.zip` (containing
  `hop.exe`), listed in the same `checksums.txt`.
- FR-W2 `install.ps1`: `irm https://raw.githubusercontent.com/ycniuqton/hop/main/install.ps1 | iex`
  installs `hop.exe` into `%LOCALAPPDATA%\Programs\hop`, verifies the checksum and adds that
  folder to the user PATH. Honours `HOP_VERSION`, `HOP_INSTALL_DIR`, `HOP_DOWNLOAD_URL`.
- FR-W3 hop compiles and its tests pass on Windows (CI runs them on `windows-latest`).
- FR-W4 The config permission check (refuse group/world-readable files) is Unix-only; on
  Windows the file lives in the user's private profile.
- FR-W5 `hop update` works while `hop.exe` is running: the old file is renamed aside, the new one
  put in place, and the leftover is deleted on the next run.
- FR-W6 `hop uninstall` removes `hop.exe` even though it is running (renamed aside and deleted
  by a short-lived helper after hop exits).
- FR-W7 Tab completion for PowerShell: `hop completion powershell` and `hop completion install`
  (default shell on Windows when `$SHELL` is unset) with a hint to dot-source it from `$PROFILE`.
- FR-W8 The ↑/↓ menu and colours work in Windows Terminal and the classic console (virtual
  terminal mode is switched on); if the console refuses, hop falls back to the numbered list
  and plain text.
- FR-W9 A local destination ending in `\` counts as a folder, like one ending in `/`.
- FR-W10 README "Windows" section, including: quote arguments starting with `@` in PowerShell
  (`'@web-1'`), because PowerShell treats a leading `@` as its own syntax; cmd.exe needs nothing.

## Constraints
- No new direct dependencies beyond `golang.org/x/sys` (already in the module graph).
- Same config/cache locations as other platforms (`%USERPROFILE%\.config\hop`, `\.cache\hop`).
- Linux/macOS behaviour unchanged.

## Unchanged
Commands, flags, storage layout, providers, join codes, agent skill, JSON output.
