#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${1:-}"
destination="${2:-}"
if [[ $# -ne 2 || ${#version} -gt 64 || ! "$version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ || -z "$destination" || "$destination" == *$'\n'* ]]; then
	printf 'usage: %s vMAJOR.MINOR.PATCH output-directory\n' "$0" >&2
	exit 2
fi
[[ "$(uname -s)" == Darwin ]] || { printf 'build the Mac app on macOS\n' >&2; exit 2; }
mkdir -p "$destination"
destination="$(cd "$destination" && pwd)"
output="$destination/Kinosail-macOS.dmg"
[[ ! -e "$output" ]] || { printf 'package output already exists\n' >&2; exit 1; }
stage="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-macos-package.XXXXXX")"
trap 'rm -rf -- "$stage"' EXIT
app="$stage/image/Kinosail.app"
helpers="$app/Contents/Helpers"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources" "$helpers"
for arch in arm64 amd64; do
	CGO_ENABLED=0 GOOS=darwin GOARCH="$arch" go build -C "$root" -trimpath -ldflags "-s -w -X main.version=$version" -o "$stage/server-$arch" ./cmd/kinosail
	python3 -I "$root/scripts/fetch-native-runtime.py" "macos-$arch" "$stage/runtime-$arch"
done
for arch in arm64 x86_64; do
	xcrun swiftc -O -target "$arch-apple-macos13" -o "$stage/launcher-$arch" "$root/packaging/macos/Launcher.swift"
done
lipo -create -output "$app/Contents/MacOS/Kinosail" "$stage/launcher-arm64" "$stage/launcher-x86_64"
lipo -create -output "$helpers/kinosail" "$stage/server-arm64" "$stage/server-amd64"
for command in ffmpeg ffprobe fpcalc; do
	lipo -create -output "$helpers/$command" "$stage/runtime-arm64/$command" "$stage/runtime-amd64/$command"
done
chmod 0755 "$app/Contents/MacOS/Kinosail" "$helpers"/*
sed "s/KINOSAIL_VERSION/${version#v}/g" "$root/packaging/macos/Info.plist" >"$app/Contents/Info.plist"
plutil -lint "$app/Contents/Info.plist" >/dev/null
icon="$stage/Kinosail.iconset"
mkdir "$icon"
for size in 16 32 128 256 512; do
	sips -z "$size" "$size" "$root/apps/native/Resources/iOS.xcassets/AppIcon.appiconset/AppIcon.png" --out "$icon/icon_${size}x${size}.png" >/dev/null
	double=$((size * 2))
	sips -z "$double" "$double" "$root/apps/native/Resources/iOS.xcassets/AppIcon.appiconset/AppIcon.png" --out "$icon/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil --convert icns --output "$app/Contents/Resources/Kinosail.icns" "$icon"
cp "$root/LICENSE" "$root/THIRD_PARTY_NOTICES.md" "$app/Contents/Resources/"
for binary in "$helpers"/*; do
	codesign --force --sign - --timestamp=none "$binary"
done
codesign --force --sign - --timestamp=none "$app"
codesign --verify --strict --deep "$app"
ln -s /Applications "$stage/image/Applications"
hdiutil create -quiet -volname Kinosail -fs HFS+ -format UDZO -imagekey zlib-level=9 -srcfolder "$stage/image" "$stage/Kinosail.dmg"
hdiutil verify -quiet "$stage/Kinosail.dmg"
mv "$stage/Kinosail.dmg" "$output"
