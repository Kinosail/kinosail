#!/usr/bin/env bash
set -euo pipefail

stage="${1:-}"
destination="${2:-}"
if [[ $# -ne 2 || ! -f "$stage/version" || -z "$destination" || "$destination" == *$'\n'* ]]; then
	printf 'usage: %s stage-directory output-directory\n' "$0" >&2
	exit 2
fi
version="$(cat "$stage/version")"
[[ ${#version} -le 64 && "$version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || { printf 'invalid staged version\n' >&2; exit 2; }
command -v wix >/dev/null || { printf 'the WiX Toolset command wix is required\n' >&2; exit 2; }
mkdir -p "$destination"
stage="$(cd "$stage" && pwd)"
destination="$(cd "$destination" && pwd)"
output="$destination/Kinosail-Windows-x64.msi"
[[ ! -e "$output" ]] || { printf 'package output already exists\n' >&2; exit 1; }
native() {
	if command -v cygpath >/dev/null; then cygpath -w "$1"; else printf '%s' "$1"; fi
}
build="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-windows-package.XXXXXX")"
trap 'rm -rf -- "$build"' EXIT
wix build -arch x64 -ext WixToolset.Util.wixext -d "Version=${version#v}" -d "Stage=$(native "$stage")" \
	-o "$(native "$build/Kinosail-Windows-x64.msi")" "$(native "$stage/Kinosail.wxs")"
mv "$build/Kinosail-Windows-x64.msi" "$output"
