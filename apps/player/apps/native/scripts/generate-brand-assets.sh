#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_dir="$root/assets/source"
images="$root/assets/images"
tv="$root/assets/tv_icons"
mkdir -p "$images" "$tv"

render() {
  local source="$1" output="$2" width="$3" height="$4" temporary
  temporary="$(mktemp -t kinosail-asset).png"
  sips -s format png "$source" --out "$temporary" >/dev/null
  sips --resampleHeightWidth "$height" "$width" "$temporary" --out "$output" >/dev/null
  unlink "$temporary"
}

render "$source_dir/icon.svg" "$images/icon.png" 1024 1024
render "$source_dir/icon.svg" "$images/splash-icon.png" 512 512
render "$source_dir/icon.svg" "$images/favicon.png" 64 64
render "$source_dir/tv-banner.svg" "$tv/icon-1280x768.png" 1280 768
render "$source_dir/tv-banner.svg" "$tv/icon-800x480.png" 800 480
render "$source_dir/tv-banner.svg" "$tv/icon-400x240.png" 400 240
render "$source_dir/top-shelf.svg" "$tv/icon-1920x720.png" 1920 720
render "$source_dir/top-shelf.svg" "$tv/icon-3840x1440.png" 3840 1440
render "$source_dir/top-shelf-wide.svg" "$tv/icon-2320x720.png" 2320 720
render "$source_dir/top-shelf-wide.svg" "$tv/icon-4640x1440.png" 4640 1440

# Active Swift asset catalogs share the authored sources with the reference exports.
ios="$root/Resources/iOS.xcassets/AppIcon.appiconset/AppIcon.png"
ffmpeg -v error -y -i "$images/icon.png" -pix_fmt rgb24 "$ios"
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
  [[ "$size" == 2320x720 || "$size" == 4640x1440 ]] && set="Top Shelf Image Wide.imageset"
  cp "$tv/icon-$size.png" "$catalog/$set/icon-$size.png"
done
