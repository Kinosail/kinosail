#!/usr/bin/env bash
# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

base="${1:-https://127.0.0.1:38127}"
fixture="$(mktemp -d)"
trap 'rm -rf -- "${fixture:?}"' EXIT

curl --fail --silent --insecure --cookie-jar "$fixture/cookies" --data "name=Owner&password=test-instance-password&code=$(./scripts/test-instance.sh totp)" "$base/login" --output /dev/null

library=""
for _ in {1..80}; do
  library="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/api/v1/library")"
  [[ "$library" == *'Example metadata from the local generated TMDB fixture.'* ]] && break
  sleep 0.25
done

for expected in \
  '"kind":"video"' \
  '"kind":"audio"' \
  '"kind":"audiobook"' \
  '"kind":"book"' \
  '"kind":"photo"' \
  'Example Movie' \
  'Example Show' \
  'Example Track One' \
  'Example Audiobook' \
  'Example PDF Book' \
  'Example EPUB Book' \
  'Example Comic' \
  '"container":"EPUB"' \
  '"container":"CBZ"' \
  'Example Photo One' \
  'Example metadata from the local generated TMDB fixture.'; do
  grep -Fq "$expected" <<<"$library"
done

for pair in \
  'movies:Example Movie' \
  'shows:Example Show' \
  'music:Example Album' \
  'audiobooks:Example Audiobook' \
  'books:Example PDF Book' \
  'photos:Example Photo One'; do
  view="${pair%%:*}" expected="${pair#*:}"
  page="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/?view=$view")"
  grep -Fq "$expected" <<<"$page"
done

home="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/?view=movies")"
movie_id="$(sed -n 's|.*href="/item/\([a-f0-9]*\)".*Example Movie.*|\1|p' <<<"$home" | head -1)"
[[ -n "$movie_id" ]]
grep -Fq '<video' <<<"$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/watch/$movie_id")"
curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/art/$movie_id" --output "$fixture/poster.jpg"
[[ -s "$fixture/poster.jpg" ]]

show="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/?view=shows")"
show_id="$(sed -n 's|.*href="/show/\([a-f0-9]*\)".*|\1|p' <<<"$show" | head -1)"
episode="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/show/$show_id")"
episode_id="$(sed -n 's|.*href="/watch/\([a-f0-9]*\)".*|\1|p' <<<"$episode" | head -1)"
grep -Fq '<video' <<<"$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/watch/$episode_id")"

music="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/?view=music")"
album_id="$(sed -n 's|.*href="/album/\([a-f0-9]*\)".*|\1|p' <<<"$music" | head -1)"
album="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/album/$album_id")"
music_id="$(sed -n 's|.*href="/watch/\([a-f0-9]*\)".*|\1|p' <<<"$album" | head -1)"
grep -Fq '<audio' <<<"$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/watch/$music_id")"

audiobook="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/?view=audiobooks")"
audiobook_id="$(sed -n 's|.*href="/watch/\([a-f0-9]*\)".*|\1|p' <<<"$audiobook" | head -1)"
grep -Fq 'Example Chapter One' <<<"$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/watch/$audiobook_id")"

books="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/?view=books")"
book_id="$(sed -n 's|.*href="/book/\([a-f0-9]*\)".*|\1|p' <<<"$books" | head -1)"
grep -Fq 'iframe class="book-reader"' <<<"$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/read/$book_id")"
curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/download/$book_id" --output "$fixture/book.pdf"
[[ -s "$fixture/book.pdf" ]]

epub_id="$(sed -n 's/.*"id":"\([a-f0-9]*\)","kind":"book","title":"Example EPUB Book".*/\1/p' <<<"$library")"
comic_id="$(sed -n 's/.*"id":"\([a-f0-9]*\)","kind":"book","title":"Example Comic".*/\1/p' <<<"$library")"
[[ -n "$epub_id" && -n "$comic_id" ]]
grep -Fq 'iframe class="book-reader"' <<<"$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/read/$epub_id")"
comic="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/read/$comic_id")"
grep -Fq 'class="reader-pages"' <<<"$comic"
grep -Fq 'Example Photo One.jpg' <<<"$comic"
grep -Fq 'Example Photo Two.jpg' <<<"$comic"

grep -Fq 'This product uses the TMDB API but is not endorsed or certified by TMDB.' <<<"$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/")"
printf 'Verified Movies, Shows, Music, Audiobooks, PDF, EPUB, CBZ, Photos, playback, API inventory, and local TMDB enrichment at %s\n' "$base"
