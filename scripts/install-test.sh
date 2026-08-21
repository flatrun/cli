#!/bin/sh

set -eu

version="0.3.0"
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

FLATRUN_VERSION="$version" INSTALL_DIR="${tmp_dir}/bin" sh "$(dirname "$0")/install.sh"
output=$("${tmp_dir}/bin/flatrun" version)

[ "$(printf '%s\n' "$output" | sed -n '1p')" = "$version" ]
printf '%s\n' "$output" | grep '^build_time=' >/dev/null
printf '%s\n' "$output" | grep '^git_commit=' >/dev/null

printf 'CLI installer test passed for release %s\n' "$version"
