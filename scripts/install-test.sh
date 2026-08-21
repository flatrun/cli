#!/bin/sh

set -eu

version="0.3.0"
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM
install_dir="${INSTALL_DIR:-${tmp_dir}/bin}"

FLATRUN_VERSION="$version" INSTALL_DIR="$install_dir" sh "$(dirname "$0")/install.sh"
output=$(PATH="${install_dir}:$PATH" flatrun version)

[ "$(printf '%s\n' "$output" | sed -n '1p')" = "$version" ]
printf '%s\n' "$output" | grep '^build_time=' >/dev/null
printf '%s\n' "$output" | grep '^git_commit=' >/dev/null

printf 'CLI installer test passed for release %s\n' "$version"
