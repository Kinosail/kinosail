# Image subtitle OCR evidence (2026-10-17)

This note records how PGS image subtitles became a WebVTT text track inside the Player container.

## Revision and environment

- Source revision: `a7a81b55` on `fix/image-subtitle-ocr`.
- Container: the runtime stage of `apps/player/Containerfile` at that revision. The Player binary was built on the host with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath ./cmd/kinosail`.
- Local build changes: Docker Hub rate limits forced `mirror.gcr.io/library/debian:trixie-slim`. The sandbox proxy certificate was trusted during the build so that the jellyfin-ffmpeg download could verify TLS. Neither change is committed.
- Image ID: `sha256:a5cd63ba8b337c31f2f99269669c59236b9f18668a7b71a4df4f69d2faade991`.
- Tools in the image: jellyfin-ffmpeg 8.1.3-1 and Tesseract 5.5.0. `tesseract --list-langs` reports ces, dan, deu, ell, eng, fin, fra, hun, ita, nld, nor, osd, pol, por, ron, rus, spa, swe, tur, and ukr.

## Test data

| File | SHA-256 |
| --- | --- |
| `apps/player/internal/server/testdata/image-subtitles-eng.sup` | `cedd869ebdfad84761cb8ec9734239ffe81e24ce3f68a442c67651d4d61aac47` |
| `apps/player/internal/server/testdata/image-subtitles-deu.sup` | `65b9fa12f39c37a95000c495bbeae7a7748a050751fb284c23f651b4a9bd7716` |
| `apps/player/internal/server/testdata/image-subtitles.py` | `ff782b69b238cf812870f11475a17e8a29ecf2e565fb509d4b45ce18322d7ded` |
| `Film.mkv` (generated, not committed) | `bc53a04f12fc0b1ad5349daed72d7c4ee94ba7342092bc09e5d6869288a87034` |

`image-subtitles.py` writes the `.sup` files from cue lines on standard input, using DejaVu Sans at 44 px on a 1280 × 720 canvas. `Film.mkv` was made with:

```sh
ffmpeg -v error -f lavfi -i testsrc2=size=1280x720:rate=24:duration=12 -f lavfi -i sine=duration=12 \
  -i image-subtitles-eng.sup -i image-subtitles-deu.sup -map 0 -map 1 -map 2 -map 3 \
  -c:v libx264 -preset ultrafast -c:a aac -c:s copy -copyts \
  -metadata:s:s:0 language=eng -metadata:s:s:1 language=deu Film.mkv
```

## Run

```sh
docker run -d --name ocrtest -v "$PWD/media:/media:ro" -v "$PWD/cache:/cache" -v "$PWD/config:/config" \
  -e KINOSAIL_TLS_ENABLED=false kinosail-ocr:local
```

## Results

- The English PGS track (stream 2) became `cache/embedded-subtitles/<item>/<version>/2.vtt` about 1 second after the first Library scan. The file is [`container-converted.vtt`](container-converted.vtt), SHA-256 `e640a426338ee04edaf3b57ad487fa83e81be8e4d0d3e75068d414e9df675293`.
- All three cues match the source text and timing exactly.
- The German track was not converted, because German is not the preferred subtitle language.
- No `.ocr-*` workspace remained in the cache.
- The container logged `image subtitle text recognition is available` with `languages=19`, then `image subtitles converted to text` with `streams=1` and `duration_ms=980`. The logs contained no paths, URLs, or subtitle text.

## Image size

Same local runtime build, with and without the Tesseract packages:

| Image | Uncompressed | `docker save | gzip -6` |
| --- | --- | --- |
| Without Tesseract | 1,114,076,489 bytes | 277,976,707 bytes |
| With Tesseract and 19 languages | 1,301,595,594 bytes | 335,677,770 bytes |

The OCR packages add about 188 MB uncompressed and 58 MB compressed.
