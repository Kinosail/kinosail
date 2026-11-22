#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${1:-}"
arch="${2:-}"
destination="${3:-}"
if [[ $# -ne 3 || ${#version} -gt 64 || ! "$version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ || ! "$arch" =~ ^(amd64|arm64)$ || -z "$destination" || "$destination" == *$'\n'* ]]; then
	printf 'usage: %s vMAJOR.MINOR.PATCH amd64|arm64 output-directory\n' "$0" >&2
	exit 2
fi
nfpm="${NFPM:-nfpm}"
command -v "$nfpm" >/dev/null || { printf 'nfpm is required\n' >&2; exit 2; }
label=x64
[[ "$arch" == amd64 ]] || label=arm64
mkdir -p "$destination"
destination="$(cd "$destination" && pwd)"
for format in deb rpm; do
	[[ ! -e "$destination/Kinosail-Linux-$label.$format" ]] || { printf 'package output already exists\n' >&2; exit 1; }
done
stage="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-linux-package.XXXXXX")"
trap 'rm -rf -- "$stage"' EXIT
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -C "$root" -trimpath -ldflags "-s -w -X main.version=$version" -o "$stage/kinosail" ./cmd/kinosail
python3 -I "$root/scripts/fetch-native-runtime.py" "linux-$arch" "$stage/runtime"
cp "$root/packaging/native-installation.json" "$stage/native-installation.json"
cp "$root/LICENSE" "$root/THIRD_PARTY_NOTICES.md" "$stage/"
mkdir "$stage/linux"
cp "$root"/packaging/linux/{kinosail.service,kinosail.defaults,kinosail.env,postinstall.sh,preremove.sh,postremove.sh} "$stage/linux/"
for format in deb rpm; do
	(cd "$stage" && KINOSAIL_ARCH="$arch" KINOSAIL_VERSION="${version#v}" SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-0}" \
		"$nfpm" package --config "$root/packaging/linux/nfpm.yaml" --packager "$format" --target "$stage/package.$format" >/dev/null)
	mv "$stage/package.$format" "$destination/Kinosail-Linux-$label.$format"
done
