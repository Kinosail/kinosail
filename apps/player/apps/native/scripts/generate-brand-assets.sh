#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_dir="$root/assets/source"
scratch="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-assets.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT

render() {
  local source="$1" output="$2" width="$3" height="$4" temporary
  temporary="$scratch/render.png"
  sips -s format png "$source" --out "$temporary" >/dev/null
  sips --resampleHeightWidth "$height" "$width" "$temporary" --out "$output" >/dev/null
  unlink "$temporary"
}

# Render directly into the active Swift asset catalogs.
ios="$root/Resources/iOS.xcassets/AppIcon.appiconset/AppIcon.png"
icon="$scratch/icon.png"
render "$source_dir/icon.svg" "$icon" 1024 1024
ffmpeg -v error -y -i "$icon" -pix_fmt rgb24 "$ios"
catalog="$root/Resources/tvOS.xcassets/AppIcon.brandassets"
for size in 400x240 800x480 1280x768; do
  stack="App Icon.imagestack"
  [[ "$size" == 1280x768 ]] && stack="App Icon - App Store.imagestack"
  for layer in Front Back; do
    source="tv-foreground.svg"
    [[ "$layer" == Back ]] && source="tv-background.svg"
    render "$source_dir/$source" "$catalog/$stack/$layer.imagestacklayer/Content.imageset/icon-$size.png" "${size%x*}" "${size#*x}"
  done
done
for size in 1920x720 3840x1440 2320x720 4640x1440; do
  set="Top Shelf Image.imageset"
  source="top-shelf.svg"
  if [[ "$size" == 2320x720 || "$size" == 4640x1440 ]]; then
    set="Top Shelf Image Wide.imageset"
    source="top-shelf-wide.svg"
  fi
  render "$source_dir/$source" "$catalog/$set/icon-$size.png" "${size%x*}" "${size#*x}"
done
