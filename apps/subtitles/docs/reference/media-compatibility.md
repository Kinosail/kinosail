---
title: Media compatibility
description: Understand scanned media types, browser playback, transcoding, subtitles, and client certification limits.
section: Reference
last_reviewed: 2026-08-28
---

# Media compatibility

Kinosail has two different compatibility questions: whether the scanner can ingest a file, and whether a particular client can play its streams directly.

## Library ingest

The scanner accepts these extensions. Extensions are case-insensitive.

| Library kind | Extensions |
| --- | --- |
| Video | `.3g2`, `.3gp`, `.asf`, `.avi`, `.flv`, `.m2ts`, `.m4v`, `.mkv`, `.mov`, `.mp4`, `.mpeg`, `.mpg`, `.mts`, `.ogv`, `.ts`, `.vob`, `.webm`, `.wmv` |
| Audio | `.aac`, `.aif`, `.aiff`, `.flac`, `.m4a`, `.mp3`, `.oga`, `.ogg`, `.opus`, `.wav` |
| Audiobook | `.m4b`, or audio below a directory named `audiobook` or `audiobooks` |
| Photo | `.avif`, `.bmp`, `.gif`, `.jpeg`, `.jpg`, `.png`, `.webp` |
| Book | `.cbz`, `.epub`, `.pdf` |

Kinosail ignores other extensions. It does not ingest `.cbr` or RAR comics. A `.cbz` file is a ZIP comic archive. A book reader supports PDF, EPUB, and CBZ after the file passes archive and content validation.

The scanner skips symbolic links. It reads optional sidecars beside a media file:

- `.nfo` supplies local title, year, plot, rating, genres, credits, album fields, and supported provider IDs.
- `.vtt` and `.srt` supply external text subtitles when their filename matches the media stem.
- `.lrc` and `.txt` supply lyrics for audio.
- Artwork uses supported image types with names such as `poster`, `folder`, `fanart`, `backdrop`, `thumb`, `clearlogo`, and `logo`, plus matching media suffixes.

For video, names containing `S01E02` group an Episode into a Show and Season. A movie name may include a year such as `Film (2024)` or `Film [2024]`. Local NFO metadata can override the filename title and year.

## Direct playback

Direct playback sends the source representation to the client without video or audio transformation. It depends on the probed source facts, the client capabilities, the Viewer policy, and the network limit.

The bundled browser advertises direct support for these common capabilities:

- containers: MP4, MOV, and WebM;
- video codecs: H.264, VP8, VP9, and AV1;
- audio codecs: AAC, MP3, Opus, and Vorbis;
- text subtitles: WebVTT; and
- SDR video up to 3840 × 2160.

This list describes the bundled browser adapter. A device, browser, Jellyfin client, or network can support a different set. The source file extension alone does not prove direct playback.

## Automatic and Compatibility playback

`Automatic` starts with direct media when possible. It can offer an adaptive HLS fallback when the Viewer may transcode. `Direct` requests only the source path. `Compatibility` requests a server-generated HLS representation. The internal API value is `compatible`.

The playback decision can select:

1. `direct` when the container, codecs, size, bitrate, HDR mode, subtitles, and policy are compatible;
2. `remux` when the container is unsupported but the client accepts a remux;
3. `audio-transcode` when only the audio codec needs conversion;
4. `transcode` when video, size, bitrate, HDR, or subtitle burn-in needs conversion; or
5. `denied` when the Profile cannot play or cannot use transcoding.

Transcoded output uses MP4 with H.264 video and AAC audio. Hardware acceleration is optional and depends on the host and container image. Kinosail can use supported accelerators such as VA-API, QSV, CUDA/NVENC, VideoToolbox, RKMPP, or AMF when the deployment exposes them; the configured accelerator must still pass the Server's capability check.

External and embedded text subtitles can remain a separate text track. Kinosail burns subtitles into video only when the client cannot use the selected text track. Tone mapping can convert unsupported HDR to SDR.

## Reader and non-video behavior

The bundled reader displays PDF documents, EPUB spine chapters, and image pages from CBZ archives. Audio uses the source when the client supports its codec and can use audio conversion when it does not. Photos are browsed as images and are not loaded into the video player.

Offline downloads are prepared files controlled by the Viewer download policy. They do not change Library Content.

## Test a client

Use the item's playback plan from `GET /api/v1/items/{id}/playback`. It reports the selected mode, reason, codecs, subtitle mode, dimensions, and adaptive qualities. Use this response when diagnosing a device instead of guessing from the filename.

Physical devices, real GPUs, network limits, and third-party Jellyfin clients require separate testing. A passing scanner test proves ingest behavior, not direct playback on every client.
