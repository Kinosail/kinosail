#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${1:-}"
stage="${2:-}"
if [[ $# -ne 2 || ${#version} -gt 64 || ! "$version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ || -z "$stage" || "$stage" == *$'\n'* ]]; then
	printf 'usage: %s vMAJOR.MINOR.PATCH stage-directory\n' "$0" >&2
	exit 2
fi
[[ ! -e "$stage" ]] || { printf 'stage directory already exists\n' >&2; exit 1; }
mkdir -p "$stage/server"
python=python3
case "$(uname -s)" in MINGW* | MSYS* | CYGWIN*) python=python ;; esac
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -C "$root" -trimpath -ldflags "-s -w -X main.version=$version" -o "$stage/server/kinosail-server.exe" ./cmd/kinosail
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -C "$root" -trimpath -ldflags "-s -w -H=windowsgui" -o "$stage/Kinosail.exe" ./cmd/kinosail-launcher
"$python" -I "$root/scripts/fetch-native-runtime.py" windows-amd64 "$stage/runtime"
mv "$stage/runtime/ffmpeg.exe" "$stage/runtime/ffprobe.exe" "$stage/runtime/fpcalc.exe" "$stage/server/"
rmdir "$stage/runtime"
cp "$root/packaging/windows/Kinosail.ico" "$root/packaging/windows/Kinosail.wxs" "$root/LICENSE" "$root/THIRD_PARTY_NOTICES.md" "$stage/"
printf '%s\n' "$version" >"$stage/version"
