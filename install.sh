#!/usr/bin/env sh
# Install ota — a k9s-style TUI for Okta Workforce Identity.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/tedilabs/ota/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/tedilabs/ota/main/install.sh | sh -s -- --version v0.3.0
#   curl -fsSL https://raw.githubusercontent.com/tedilabs/ota/main/install.sh | sh -s -- --bin-dir ~/.local/bin
#
# Detects OS + arch, downloads the matching release tarball from GitHub,
# verifies the SHA256 checksum, and installs the binary into a bin dir
# on $PATH. Falls back to ~/.local/bin when /usr/local/bin isn't writable.

set -eu

OWNER="tedilabs"
REPO="ota"
BIN_NAME="ota"

VERSION=""
BIN_DIR=""

while [ $# -gt 0 ]; do
  case "$1" in
    --version|-v)
      VERSION="$2"
      shift 2
      ;;
    --bin-dir)
      BIN_DIR="$2"
      shift 2
      ;;
    -h|--help)
      sed -n '2,9p' "$0"
      exit 0
      ;;
    *)
      echo "install.sh: unknown flag '$1'" >&2
      exit 1
      ;;
  esac
done

# ---- detect OS / arch -------------------------------------------------------

uname_os() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    darwin) echo "macos" ;;
    linux)  echo "linux" ;;
    *) echo "install.sh: unsupported OS: $os" >&2; exit 1 ;;
  esac
}

uname_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)   echo "amd64" ;;
    aarch64|arm64)  echo "arm64" ;;
    *) echo "install.sh: unsupported arch: $arch" >&2; exit 1 ;;
  esac
}

OS="$(uname_os)"
ARCH="$(uname_arch)"

# ---- resolve version --------------------------------------------------------

if [ -z "$VERSION" ]; then
  # Hit the GitHub Releases API for the latest tag. Falls back to a
  # plain HTTP redirect parse if `jq` isn't available.
  if command -v curl >/dev/null 2>&1; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/$OWNER/$REPO/releases/latest" \
               | sed -n 's/.*"tag_name": *"\(v[^"]*\)".*/\1/p' \
               | head -n1)"
  elif command -v wget >/dev/null 2>&1; then
    VERSION="$(wget -qO- "https://api.github.com/repos/$OWNER/$REPO/releases/latest" \
               | sed -n 's/.*"tag_name": *"\(v[^"]*\)".*/\1/p' \
               | head -n1)"
  else
    echo "install.sh: need curl or wget on PATH" >&2
    exit 1
  fi
fi

if [ -z "$VERSION" ]; then
  echo "install.sh: could not resolve a release version. Pass --version vX.Y.Z explicitly." >&2
  exit 1
fi

# Strip the leading `v` for the archive's version segment (matches the
# GoReleaser archive name template).
VERSION_NUM="${VERSION#v}"

ARCHIVE="${REPO}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$OWNER/$REPO/releases/download/$VERSION/$ARCHIVE"
CHECKSUMS_URL="https://github.com/$OWNER/$REPO/releases/download/$VERSION/checksums.txt"

# ---- pick install dir -------------------------------------------------------

if [ -z "$BIN_DIR" ]; then
  if [ -w /usr/local/bin ] 2>/dev/null; then
    BIN_DIR=/usr/local/bin
  else
    BIN_DIR="$HOME/.local/bin"
    mkdir -p "$BIN_DIR"
  fi
fi

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "install.sh: warning — $BIN_DIR is not on \$PATH. Add it to your shell profile." >&2 ;;
esac

# ---- download + verify ------------------------------------------------------

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "==> Downloading $ARCHIVE"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL"             -o "$TMP/$ARCHIVE"
  curl -fsSL "$CHECKSUMS_URL"   -o "$TMP/checksums.txt"
else
  wget -qO "$TMP/$ARCHIVE"      "$URL"
  wget -qO "$TMP/checksums.txt" "$CHECKSUMS_URL"
fi

echo "==> Verifying checksum"
EXPECTED="$(grep " $ARCHIVE\$" "$TMP/checksums.txt" | awk '{print $1}')"
if [ -z "$EXPECTED" ]; then
  echo "install.sh: archive '$ARCHIVE' not listed in checksums.txt" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "$TMP/$ARCHIVE" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(shasum -a 256 "$TMP/$ARCHIVE" | awk '{print $1}')"
else
  echo "install.sh: need sha256sum or shasum on PATH" >&2
  exit 1
fi

if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "install.sh: checksum mismatch for $ARCHIVE" >&2
  echo "  expected: $EXPECTED" >&2
  echo "  actual:   $ACTUAL"   >&2
  exit 1
fi

# ---- install ----------------------------------------------------------------

echo "==> Extracting"
tar -xzf "$TMP/$ARCHIVE" -C "$TMP"

echo "==> Installing to $BIN_DIR/$BIN_NAME"
install -m 0755 "$TMP/$BIN_NAME" "$BIN_DIR/$BIN_NAME"

echo ""
echo "ota $VERSION installed at $BIN_DIR/$BIN_NAME"
"$BIN_DIR/$BIN_NAME" --version || true
