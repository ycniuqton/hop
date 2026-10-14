#!/bin/sh
# Installs the hop binary from GitHub Releases.
#
#   curl -fsSL https://raw.githubusercontent.com/ycniuqton/hop/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/ycniuqton/hop/main/install.sh | sudo sh   # → /usr/local/bin
#
# Environment: HOP_VERSION (tag such as v0.1.0, default latest), HOP_INSTALL_DIR, HOP_REPO,
# HOP_DOWNLOAD_URL (a mirror holding the release assets).
set -eu

REPO="${HOP_REPO:-ycniuqton/hop}"
VERSION="${HOP_VERSION:-latest}"
BINARY="hop"

fail() {
	echo "error: $*" >&2
	exit 1
}

detect_os() {
	case "$(uname -s)" in
	Linux) echo linux ;;
	Darwin) echo darwin ;;
	*) fail "unsupported OS $(uname -s) — hop ships for Linux and macOS" ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo amd64 ;;
	aarch64 | arm64) echo arm64 ;;
	*) fail "unsupported CPU $(uname -m) — hop ships for amd64 and arm64" ;;
	esac
}

install_dir() {
	if [ -n "${HOP_INSTALL_DIR:-}" ]; then
		echo "$HOP_INSTALL_DIR"
	elif [ "$(id -u)" -eq 0 ]; then
		echo /usr/local/bin
	else
		echo "$HOME/.local/bin"
	fi
}

download() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -q -O "$2" "$1"
	else
		fail "need curl or wget"
	fi
}

verify() {
	expected="$(grep " $2\$" "$1" | cut -d ' ' -f 1)"
	[ -n "$expected" ] || fail "no checksum for $2 in checksums.txt"
	if command -v sha256sum >/dev/null 2>&1; then
		actual="$(sha256sum "$3" | cut -d ' ' -f 1)"
	elif command -v shasum >/dev/null 2>&1; then
		actual="$(shasum -a 256 "$3" | cut -d ' ' -f 1)"
	else
		echo "warning: no sha256sum or shasum — skipping checksum check" >&2
		return 0
	fi
	[ "$expected" = "$actual" ] || fail "checksum mismatch for $2"
}

main() {
	os="$(detect_os)"
	arch="$(detect_arch)"
	[ "$os/$arch" != "darwin/amd64" ] || fail "Intel Macs are not supported — build from source with 'go install github.com/$REPO/cmd/hop@latest'"
	archive="${BINARY}_${os}_${arch}.tar.gz"
	if [ -n "${HOP_DOWNLOAD_URL:-}" ]; then
		base="$HOP_DOWNLOAD_URL"
	elif [ "$VERSION" = latest ]; then
		base="https://github.com/$REPO/releases/latest/download"
	else
		base="https://github.com/$REPO/releases/download/$VERSION"
	fi

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT INT TERM
	echo "Downloading $archive ($VERSION)..."
	download "$base/$archive" "$tmp/$archive" || fail "download failed: $base/$archive"
	download "$base/checksums.txt" "$tmp/checksums.txt" || fail "download failed: $base/checksums.txt"
	verify "$tmp/checksums.txt" "$archive" "$tmp/$archive"
	tar -xzf "$tmp/$archive" -C "$tmp" "$BINARY"

	dir="$(install_dir)"
	mkdir -p "$dir"
	install -m 0755 "$tmp/$BINARY" "$dir/$BINARY" 2>/dev/null || {
		cp "$tmp/$BINARY" "$dir/$BINARY" && chmod 0755 "$dir/$BINARY"
	} || fail "cannot write $dir — re-run with sudo or set HOP_INSTALL_DIR"
	echo "Installed $("$dir/$BINARY" --version) to $dir/$BINARY"

	case ":$PATH:" in
	*":$dir:"*) ;;
	*) echo "note: $dir is not on your PATH — add: export PATH=\"$dir:\$PATH\"" ;;
	esac
	echo "Next: run 'hop init' (first machine) or 'hop join <code>' (others)."
}

main "$@"
