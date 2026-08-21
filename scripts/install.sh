#!/bin/sh

set -eu

REPOSITORY="flatrun/cli"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
RELEASE_API_URL="${FLATRUN_RELEASE_API_URL:-https://api.github.com/repos/${REPOSITORY}/releases/latest}"
RELEASE_BASE_URL="${FLATRUN_RELEASE_BASE_URL:-https://github.com/${REPOSITORY}/releases/download}"

fail() {
    printf 'FlatRun CLI installation failed: %s\n' "$1" >&2
    exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"

if [ -z "${FLATRUN_VERSION:-}" ]; then
    FLATRUN_VERSION=$(curl -fsSL "$RELEASE_API_URL" | sed -n 's/.*"tag_name": *"v\([^"]*\)".*/\1/p' | head -n 1)
fi
[ -n "$FLATRUN_VERSION" ] || fail "could not determine the latest release"
case "$FLATRUN_VERSION" in
    *[!0-9A-Za-z.+-]*) fail "release version contains unsupported characters" ;;
esac

case "$(uname -s)" in
    Linux) os="linux" ;;
    Darwin) os="darwin" ;;
    *) fail "supported operating systems are Linux and macOS" ;;
esac

case "$(uname -m)" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) fail "supported architectures are amd64 and arm64" ;;
esac

archive="flatrun-${FLATRUN_VERSION}-${os}-${arch}.tar.gz"
binary="flatrun-${FLATRUN_VERSION}-${os}-${arch}"
release_url="${RELEASE_BASE_URL}/v${FLATRUN_VERSION}"
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

curl -fsSL "${release_url}/${archive}" -o "${tmp_dir}/${archive}"
curl -fsSL "${release_url}/checksums.txt" -o "${tmp_dir}/checksums.txt"

expected=$(awk -v name="$archive" '$2 == name { print $1 }' "${tmp_dir}/checksums.txt")
[ -n "$expected" ] || fail "release checksum is missing"
if command -v sha256sum >/dev/null 2>&1; then
    calculated=$(sha256sum "${tmp_dir}/${archive}" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
    calculated=$(shasum -a 256 "${tmp_dir}/${archive}" | awk '{ print $1 }')
else
    fail "sha256sum or shasum is required"
fi
[ "$calculated" = "$expected" ] || fail "checksum verification failed"

tar -xzf "${tmp_dir}/${archive}" -C "$tmp_dir" "$binary"
mkdir -p "$INSTALL_DIR"
install -m 0755 "${tmp_dir}/${binary}" "${INSTALL_DIR}/flatrun"

printf 'FlatRun CLI %s installed at %s/flatrun\n' "$FLATRUN_VERSION" "$INSTALL_DIR"
