#!/bin/bash
set -euo pipefail
S="$(cd "$(dirname "$0")" && pwd)"
M="${1:?usage: make-media.sh <output directory>}"
mkdir -p "$M/movies/Big Buck Bunny (2008)" "$M/movies/Sintel (2010)" "$M/movies/Tears of Steel (2012)" "$M/shows/Pioneer One (2010)/Season 01"
cd "$M"
q() { ffmpeg -loglevel error -y "$@"; }
q -f lavfi -i testsrc2=size=1280x720:rate=24:duration=90 -f lavfi -i sine=frequency=440:duration=90 \
  -c:v libx264 -preset ultrafast -crf 30 -pix_fmt yuv420p -c:a aac -movflags +faststart "movies/Big Buck Bunny (2008)/Big Buck Bunny (2008).mp4"
printf '1\n00:00:01,000 --> 00:00:40,000\nKinosail test subtitle\n\n2\n00:00:41,000 --> 00:01:25,000\nSecond test subtitle\n' \
  >"movies/Big Buck Bunny (2008)/Big Buck Bunny (2008).en.srt"
q -f lavfi -i testsrc2=size=1920x1080:rate=24:duration=90 -f lavfi -i sine=frequency=550:duration=90 -i "$S/sub.ass" \
  -map 0 -map 1 -map 2 -c:v libx265 -preset ultrafast -crf 34 -c:a ac3 -c:s ass "movies/Sintel (2010)/Sintel (2010).mkv"
q -f lavfi -i testsrc=size=1280x720:rate=24:duration=90 -f lavfi -i sine=frequency=660:duration=90 \
  -c:v mpeg4 -q:v 8 -c:a libmp3lame "movies/Tears of Steel (2012)/Tears of Steel (2012).avi"
for e in 1 2 3; do
  q -f lavfi -i testsrc2=size=1280x720:rate=24:duration=20 -f lavfi -i "sine=frequency=$((300 + e * 50)):duration=20" \
    -c:v libx264 -preset ultrafast -crf 32 -pix_fmt yuv420p -c:a aac "shows/Pioneer One (2010)/Season 01/Pioneer One - S01E0$e.mkv"
done
find . -type f
