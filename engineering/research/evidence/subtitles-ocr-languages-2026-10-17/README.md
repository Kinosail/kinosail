# Subtitles app OCR language download evidence (2026-10-17)

This note records automatic image subtitle conversion in the Subtitles container. It covers a language whose OCR data is not bundled, so the app must download it from Debian on first use.

## Revision and environment

- Source tree: `a66c13a1` on `feat/subtitles-auto-ocr`. The Subtitles binary was built on the host with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath ./cmd/kinosail`.
- Container: the runtime stage of `apps/subtitles/Containerfile` at that tree. Image ID `sha256:b7e4e27f0ee4099ca82618f98c4082771b5bef908d7a17033307e20875273668`.
- Local build changes, not committed:
  - Docker Hub rate limits forced `mirror.gcr.io/library/debian:trixie-slim`.
  - The sandbox proxy certificate was trusted so that TLS verified.
  - The whisper.cpp stage was left out; it is not used by OCR.
- Pinned language data: Debian 13 `main/binary-all/Packages.xz` from the `InRelease` file dated 2026-09-12. `gpgv` reported "Good signature from Debian Stable Release Key (13/trixie)". The Packages.xz SHA-256 `55bef3400f6614ac8bfdd05922310d9ff456e458d0bf9a0a201e6d9359144108` matched `InRelease`.

## Test data

- `Arrival.mkv`: a 12-second test video plus `apps/subtitles/internal/server/testdata/image-subtitles-eng.sup`, tagged `language=vie`. SHA-256 `1fc4a320a984b4e27483fceb7d98f44bada5795c90ab4aa1646e21592fc86f58`.
- The PGS text is English. Vietnamese was chosen because its OCR data is not bundled, so a download must happen.

## Run

```sh
docker run -d --name subocr -v "$PWD/media:/media" -v "$PWD/cache:/cache" -v "$PWD/config:/config" -v "$PWD/backups:/backups" \
  -e KINOSAIL_TLS_ENABLED=false -e KINOSAIL_SUBTITLE_LANGUAGE=vi -e KINOSAIL_BACKUP_KEY=<local test key> kinosail-sub-ocr:local
```

No subtitle provider was configured.

## Results

- The app logged `OCR language data installed` (`language=vie`, `bytes=531275`, `duration_ms=135`). The file at `/cache/tessdata/vie.traineddata` has SHA-256 `79df64caf7bcfb2a27df5042ecb6121e196eada34da774956995747636d5bfa1` and mode 0600.
- It then logged `image subtitle converted to text` (`duration_ms=693`) and `automatic subtitle maintenance completed` (`attempted=1`, `added=1`).
- It wrote `/media/Arrival.vi.srt`, saved here as [`container-Arrival.vi.srt`](container-Arrival.vi.srt). SHA-256 `bae28ff455e3aae201bd832e2ded16e55b96a2aed62e526a933673f43da63f30`. The cue times match the source exactly. The text has two small errors ("jumbps", "dogl") because English text was read with Vietnamese OCR data.
- The logs contained no paths, URLs, or subtitle text.
- The first container run failed with `failure=verification`. The app read the extracted file with the private file reader, which rejects the package's 0644 mode. Commit `a66c13a1` fixed this, and the run above passed.
