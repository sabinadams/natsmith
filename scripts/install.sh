#!/usr/bin/env sh
# Install natsmith from GitHub Releases.
# Usage: curl -fsSL https://sabinadams.github.io/natsmith/install.sh | sh

set -e

REPO="sabinadams/natsmith"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

err() {
  echo "install.sh: $*" >&2
  exit 1
}

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) err "unsupported OS: $(uname -s) (use GitHub Releases or Homebrew on macOS)" ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) err "unsupported architecture: $(uname -m)" ;;
esac

if ! command -v curl >/dev/null 2>&1; then
  err "curl is required"
fi

api="https://api.github.com/repos/${REPO}/releases/latest"
json="$(curl -fsSL -H "Accept: application/vnd.github+json" "$api")" ||
  err "could not fetch latest release from GitHub"

tag="$(printf '%s\n' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"v\([^"]*\)".*/\1/p')"
[ -n "$tag" ] || err "could not parse latest release tag"

version="$tag"
archive="natsmith_${version}_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/v${version}/${archive}"
checksums_url="https://github.com/${REPO}/releases/download/v${version}/checksums.txt"

tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t natsmith)"
trap 'rm -rf "$tmpdir"' EXIT INT TERM

curl -fsSL "$url" -o "${tmpdir}/${archive}" ||
  err "could not download ${url}"

if curl -fsSL "$checksums_url" -o "${tmpdir}/checksums.txt" 2>/dev/null; then
  expected="$(grep " ${archive}$" "${tmpdir}/checksums.txt" | awk '{print $1}')"
  if [ -n "$expected" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      actual="$(sha256sum "${tmpdir}/${archive}" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
      actual="$(shasum -a 256 "${tmpdir}/${archive}" | awk '{print $1}')"
    else
      actual=""
    fi
    [ -z "$actual" ] || [ "$actual" = "$expected" ] ||
      err "checksum mismatch for ${archive}"
  fi
fi

tar xzf "${tmpdir}/${archive}" -C "$tmpdir" ||
  err "could not extract ${archive}"

bin="${tmpdir}/natsmith"
[ -f "$bin" ] || err "archive did not contain natsmith binary"

if [ -w "$INSTALL_DIR" ] 2>/dev/null; then
  mv "$bin" "${INSTALL_DIR}/natsmith"
  chmod 755 "${INSTALL_DIR}/natsmith"
else
  if ! command -v sudo >/dev/null 2>&1; then
    err "cannot write to ${INSTALL_DIR}; set INSTALL_DIR to a writable directory"
  fi
  sudo mkdir -p "$INSTALL_DIR"
  sudo mv "$bin" "${INSTALL_DIR}/natsmith"
  sudo chmod 755 "${INSTALL_DIR}/natsmith"
fi

echo "Installed natsmith v${version} to ${INSTALL_DIR}/natsmith"
"${INSTALL_DIR}/natsmith" -h >/dev/null 2>&1 ||
  err "natsmith installed but failed to run; is ${INSTALL_DIR} on your PATH?"

echo "Run: natsmith migrate kv -h"
