#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${1:-}"
destination="${2:-}"
if [[ $# -ne 2 || ${#version} -gt 64 || ! "$version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ || -z "$destination" || "$destination" == *$'\n'* ]]; then
	printf 'usage: %s vMAJOR.MINOR.PATCH output-directory\n' "$0" >&2
	exit 2
fi
mkdir -p "$destination"
destination="$(cd "$destination" && pwd)"
temporary="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-native-release.XXXXXX")"
trap 'rm -rf -- "$temporary"' EXIT
release="$temporary/release"
mkdir "$release"
files=(kinosail-native-installation.json kinosail-release.json)
for target in linux-amd64 linux-arm64 macos-amd64 macos-arm64; do
	files+=("kinosail-core-$version-$target.tar.gz")
done
for target in windows-amd64 windows-arm64; do
	files+=("kinosail-core-$version-$target.zip")
done
for file in "${files[@]}"; do
	[[ ! -e "$destination/$file" ]] || { printf 'release output already exists: %s\n' "$file" >&2; exit 1; }
done
contract="$release/kinosail-native-installation.json"
cp "$root/packaging/native-installation.json" "$contract"
python3 "$root/scripts/test-native-contract.py" "$contract"
contract_schema="$(python3 -c 'import json, pathlib, sys; print(json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["schemaVersion"])' "$contract")"
contract_digest="$(shasum -a 256 "$contract" | awk '{print $1}')"
contract_size="$(wc -c <"$contract" | tr -d ' ')"

rows=""
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
	goos="${target%/*}"
	arch="${target#*/}"
	platform="$goos"
	[[ "$platform" != darwin ]] || platform=macos
	format=tar.gz
	executable=kinosail
	[[ "$goos" != windows ]] || { format=zip; executable=kinosail.exe; }
	file="kinosail-core-$version-$platform-$arch.$format"
	stage="$temporary/$platform-$arch"
	mkdir -p "$stage"
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$arch" go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$stage/$executable" "$root/cmd/kinosail"
	python3 "$root/scripts/package-native-archive.py" "$format" "$release/$file" "$stage/$executable" "$contract" "$root/LICENSE" "$root/THIRD_PARTY_NOTICES.md"
	digest="$(shasum -a 256 "$release/$file" | awk '{print $1}')"
	size="$(wc -c <"$release/$file" | tr -d ' ')"
	row="{\"os\":\"$platform\",\"arch\":\"$arch\",\"file\":\"$file\",\"format\":\"$format\",\"sha256\":\"$digest\",\"size\":$size}"
	rows="${rows}${rows:+,}${row}"
done
printf '{"schemaVersion":2,"version":"%s","updateSchema":2,"stateSchema":1,"minimumStateSchema":1,"configurationSchema":1,"minimumConfigurationSchema":1,"runtimeCommands":["ffmpeg","ffprobe","fpcalc"],"installation":{"schemaVersion":%s,"file":"kinosail-native-installation.json","sha256":"%s","size":%s},"artifacts":[%s]}\n' "$version" "$contract_schema" "$contract_digest" "$contract_size" "$rows" >"$release/kinosail-release.json"
for file in "${files[@]}"; do
	mv "$release/$file" "$destination/$file"
done
