#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  printf 'usage: %s OUTPUT_DIRECTORY\n' "$0" >&2
  exit 2
fi

output="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
engine="${CONTAINER_ENGINE:-}"
image="${KINOSAIL_TEST_IMAGE:-localhost/kinosail-subtitles:dev}"
if [[ -z "$engine" ]]; then
  if command -v podman >/dev/null; then engine=podman; else engine=docker; fi
fi

mkdir -p \
  "$output/Movies" \
  "$output/Shows/Example Show/Season 01" \
  "$output/Music/Example Album" \
  "$output/Audiobooks" \
  "$output/Books" \
  "$output/Photos/Geometry" \
  "$output/TMDB/api/search" \
  "$output/TMDB/api/movie" \
  "$output/TMDB/api/tv/202/season/1/episode" \
  "$output/TMDB/images"

cp testdata/CC0-1.0.txt "$output/CC0-1.0.txt"

cat >"$output/Shows/Example Show/Season 01/Example Show S01E01 Example Episode One.nfo" <<'EOF'
<episodedetails><title>Example Episode One</title><plot>An original generated episode used to test season and episode browsing.</plot><aired>2026-01-01</aired></episodedetails>
EOF
cat >"$output/Shows/Example Show/Season 01/Example Show S01E02 Example Episode Two.nfo" <<'EOF'
<episodedetails><title>Example Episode Two</title><plot>A second original generated episode used to test next-Episode behavior.</plot><aired>2026-01-02</aired></episodedetails>
EOF
cat >"$output/Movies/Example Movie.vtt" <<'EOF'
WEBVTT

00:00:00.000 --> 00:00:03.000
Example generated CC0 subtitle fixture.
EOF
cat >"$output/Music/Example Album/Example Track One.lrc" <<'EOF'
[00:00.00]Example non-melodic generated test tone
[00:04.00]CC0 fixture generated locally
EOF
cat >"$output/Audiobooks/chapters.ffmeta" <<'EOF'
;FFMETADATA1
title=Example Audiobook
artist=Kinosail Test Lab
[CHAPTER]
TIMEBASE=1/1000
START=0
END=6000
title=Example Chapter One
[CHAPTER]
TIMEBASE=1/1000
START=6000
END=12000
title=Example Chapter Two
EOF

uid="$(id -u)" gid="$(id -g)"
# shellcheck disable=SC2016,SC2026 # The script is intentionally expanded inside the container.
"$engine" run --rm --user "$uid:$gid" --entrypoint /bin/sh --volume "$output:/output" "$image" -c '
set -eu
video() {
  frequency="$1"; destination="$2"; color="$3"
  ffmpeg -hide_banner -loglevel error -y \
    -f lavfi -i "color=c=${color}:s=640x360:r=24,geq=r=X/W*255:g=Y/H*255:b=(X+Y)/4,scroll=horizontal=0.01" \
    -f lavfi -i "sine=frequency=${frequency}:sample_rate=48000:duration=12" -t 12 -shortest \
    -c:v libx264 -preset veryfast -crf 28 -pix_fmt yuv420p -c:a aac -b:a 96k -movflags +faststart "$destination"
}
video 220 "/output/Movies/Example Movie.mp4" 0x18324a
video 180 "/output/Movies/Arrival.mp4" 0x274e5f
video 260 "/output/Movies/Beta.mp4" 0x3c6a4d
video 520 "/output/Movies/Gamma.mp4" 0x6c4a2f
cat >"/output/Movies/Arrival.nfo" <<'EOF'
<movie><title>Arrival</title><year>2016</year></movie>
EOF
cat >"/output/Movies/Beta.nfo" <<'EOF'
<movie><title>Beta</title><year>2026</year></movie>
EOF
cat >"/output/Movies/Gamma.nfo" <<'EOF'
<movie><title>Gamma</title><year>2026</year></movie>
EOF
video 330 "/output/Shows/Example Show/Season 01/Example Show S01E01 Example Episode One.mp4" 0x214f3b
video 440 "/output/Shows/Example Show/Season 01/Example Show S01E02 Example Episode Two.mp4" 0x5b315f
ffmpeg -hide_banner -loglevel error -y -f lavfi -i "sine=frequency=523.25:sample_rate=48000:duration=12" \
  -c:a aac -b:a 128k -metadata title="Example Track One" -metadata artist="Kinosail Test Lab" -metadata album="Example Album" -metadata track="1/2" \
  "/output/Music/Example Album/Example Track One.m4a"
ffmpeg -hide_banner -loglevel error -y -f lavfi -i "sine=frequency=659.25:sample_rate=48000:duration=12" \
  -c:a aac -b:a 128k -metadata title="Example Track Two" -metadata artist="Kinosail Test Lab" -metadata album="Example Album" -metadata track="2/2" \
  "/output/Music/Example Album/Example Track Two.m4a"
ffmpeg -hide_banner -loglevel error -y -f lavfi -i "sine=frequency=196:sample_rate=48000:duration=12" -i /output/Audiobooks/chapters.ffmeta \
  -map 0:a -map_metadata 1 -c:a aac -b:a 96k "/output/Audiobooks/Example Audiobook.m4b"
ffmpeg -hide_banner -loglevel error -y -f lavfi -i "color=c=0x1f6f78:s=900x600,geq=r=X/W*180:g=Y/H*210:b=(X+Y)/(W+H)*255" \
  -frames:v 1 "/output/Photos/Geometry/Example Photo One.jpg"
ffmpeg -hide_banner -loglevel error -y -f lavfi -i "color=c=0x733c58:s=900x600,geq=r=Y/H*220:g=X/W*160:b=(W-X)/W*220" \
  -frames:v 1 "/output/Photos/Geometry/Example Photo Two.jpg"
cp "/output/Photos/Geometry/Example Photo One.jpg" "/output/TMDB/images/provider-poster.jpg"
cp "/output/Photos/Geometry/Example Photo Two.jpg" "/output/TMDB/images/episode-still.jpg"
cp "/output/Photos/Geometry/Example Photo Two.jpg" "/output/Movies/Example Movie-fanart.jpg"
cp "/output/Photos/Geometry/Example Photo One.jpg" "/output/Shows/Example Show/fanart.jpg"
'

make_pdf() {
  local file="$1" stream xref offset object
  stream=$'BT\n/F1 24 Tf\n72 720 Td\n(Example PDF Book) Tj\n/F1 12 Tf\n0 -48 Td\n(A wholly original example document generated for the Kinosail reader.) Tj\n0 -24 Td\n(No quotation, character, story, or artwork is taken from another work.) Tj\nET\n'
  printf '%%PDF-1.4\n' >"$file"
  local -a offsets=(0)
  for object in \
    '1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj' \
    '2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj' \
    '3 0 obj << /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >> endobj' \
    '4 0 obj << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> endobj'; do
    offsets+=("$(wc -c <"$file" | tr -d ' ')")
    printf '%s\n' "$object" >>"$file"
  done
  offsets+=("$(wc -c <"$file" | tr -d ' ')")
  printf '5 0 obj << /Length %d >> stream\n%sendstream\nendobj\n' "$(printf '%s' "$stream" | wc -c | tr -d ' ')" "$stream" >>"$file"
  xref="$(wc -c <"$file" | tr -d ' ')"
  printf 'xref\n0 6\n0000000000 65535 f \n' >>"$file"
  for offset in "${offsets[@]:1}"; do printf '%010d 00000 n \n' "$offset" >>"$file"; done
  printf 'trailer << /Size 6 /Root 1 0 R >>\nstartxref\n%s\n%%%%EOF\n' "$xref" >>"$file"
}
make_pdf "$output/Books/Example PDF Book.pdf"

epub_source="$output/Books/.epub-source"
mkdir -p "$epub_source/META-INF" "$epub_source/OEBPS"
printf 'application/epub+zip' >"$epub_source/mimetype"
cat >"$epub_source/META-INF/container.xml" <<'EOF'
<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>
EOF
cat >"$epub_source/OEBPS/content.opf" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<package version="3.0" xmlns="http://www.idpf.org/2007/opf" unique-identifier="book-id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="book-id">kinosail-example-epub</dc:identifier><dc:title>Example EPUB Book</dc:title><dc:language>en</dc:language></metadata><manifest><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="chapter"/></spine></package>
EOF
cat >"$epub_source/OEBPS/chapter.xhtml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" lang="en"><head><title>Example Chapter</title></head><body><h1>Example Chapter</h1><p>An original generated chapter written to test the Kinosail EPUB reader.</p></body></html>
EOF
(
  cd "$epub_source"
  zip -X0 ../Example\ EPUB\ Book.epub mimetype >/dev/null
  zip -Xr9 ../Example\ EPUB\ Book.epub META-INF OEBPS >/dev/null
)
rm -rf -- "$epub_source"

zip -Xj "$output/Books/Example Comic.cbz" \
  "$output/Photos/Geometry/Example Photo One.jpg" \
  "$output/Photos/Geometry/Example Photo Two.jpg" >/dev/null

cat >"$output/TMDB/api/search/movie" <<'EOF'
{"results":[{"id":101,"title":"Example Movie","overview":"Example metadata from the local generated TMDB fixture.","release_date":"2026-01-01","poster_path":"/provider-poster.jpg"}]}
EOF
cat >"$output/TMDB/api/movie/101" <<'EOF'
{"belongs_to_collection":{"name":"Example Collection"}}
EOF
cat >"$output/TMDB/api/search/tv" <<'EOF'
{"results":[{"id":202,"name":"Example Show","overview":"An original generated episodic fixture.","first_air_date":"2026-01-01","poster_path":"/provider-poster.jpg"}]}
EOF
cat >"$output/TMDB/api/tv/202/season/1/episode/1" <<'EOF'
{"name":"Example Episode One","overview":"Example metadata for the first generated episode.","air_date":"2026-01-01","still_path":"/episode-still.jpg"}
EOF
cat >"$output/TMDB/api/tv/202/season/1/episode/2" <<'EOF'
{"name":"Example Episode Two","overview":"Example metadata for the second generated episode.","air_date":"2026-01-02","still_path":"/episode-still.jpg"}
EOF

find "$output" -type d -exec chmod a+rx {} +
find "$output" -type f -exec chmod a+r {} +
