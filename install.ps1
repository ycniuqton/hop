# Installs hop.exe from GitHub Releases on Windows.
#
#   irm https://raw.githubusercontent.com/ycniuqton/hop/main/install.ps1 | iex
#
# Environment: HOP_VERSION (tag such as v0.4.0, default latest), HOP_INSTALL_DIR, HOP_REPO,
# HOP_DOWNLOAD_URL (a mirror holding the release assets).
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# Fail throws instead of calling exit, which would close the window under `irm | iex`.
function Fail($message) {
    throw "error: $message"
}

function Get-HopArch {
    $arch = $env:PROCESSOR_ARCHITEW6432
    if (-not $arch) { $arch = $env:PROCESSOR_ARCHITECTURE }
    switch ($arch) {
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
        default { Fail "unsupported CPU '$arch' - hop ships for amd64 and arm64" }
    }
}

function Get-HopBaseUrl($repo, $version) {
    if ($env:HOP_DOWNLOAD_URL) { return $env:HOP_DOWNLOAD_URL.TrimEnd('/') }
    if ($version -eq 'latest') { return "https://github.com/$repo/releases/latest/download" }
    return "https://github.com/$repo/releases/download/$version"
}

function Test-HopChecksum($sumsFile, $asset, $file) {
    $line = Get-Content $sumsFile | Where-Object { $_ -match "\s$([regex]::Escape($asset))$" } | Select-Object -First 1
    if (-not $line) { Fail "no checksum for $asset in checksums.txt" }
    $expected = ($line -split '\s+')[0].ToLower()
    $actual = (Get-FileHash -Algorithm SHA256 -Path $file).Hash.ToLower()
    if ($expected -ne $actual) { Fail "checksum mismatch for $asset" }
}

function Add-HopToPath($dir) {
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $parts = @($userPath -split ';' | Where-Object { $_ })
    if ($parts -contains $dir) { return $false }
    [Environment]::SetEnvironmentVariable('Path', (($parts + $dir) -join ';'), 'User')
    $env:Path = "$env:Path;$dir"
    return $true
}

function Install-Hop {
    $repo = if ($env:HOP_REPO) { $env:HOP_REPO } else { 'ycniuqton/hop' }
    $version = if ($env:HOP_VERSION) { $env:HOP_VERSION } else { 'latest' }
    $dir = if ($env:HOP_INSTALL_DIR) { $env:HOP_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\hop' }
    $asset = "hop_windows_$(Get-HopArch).zip"
    $base = Get-HopBaseUrl $repo $version

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("hop-install-" + [Guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "Downloading $asset ($version)..."
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile (Join-Path $tmp $asset)
            Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')
        } catch {
            Fail "download failed from $base - $($_.Exception.Message)"
        }
        Test-HopChecksum (Join-Path $tmp 'checksums.txt') $asset (Join-Path $tmp $asset)
        Expand-Archive -Path (Join-Path $tmp $asset) -DestinationPath (Join-Path $tmp 'x') -Force
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
        $target = Join-Path $dir 'hop.exe'
        Remove-Item -Force "$target.old" -ErrorAction SilentlyContinue
        if (Test-Path $target) { Move-Item $target "$target.old" }
        Copy-Item (Join-Path $tmp 'x\hop.exe') $target
        Write-Host "Installed $(& $target --version) to $target"
        if (Add-HopToPath $dir) { Write-Host "Added $dir to your PATH - open a new terminal to use 'hop'." }
        Write-Host "Next: run 'hop init' (first machine) or 'hop join <code>' (others)."
    } finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }
}

Install-Hop
