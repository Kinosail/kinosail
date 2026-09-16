# Mainstream media format coverage

Research snapshot: 2026-08-27

Implementation baseline: `0fe4815b6da4f3d81f9a5255b6fdb2dd9fcb201c` (`origin/main`)

Scope: Movies, Shows, audiobooks, ebooks, comics, and their playback or reading dependencies

## Decision

Kinosail should support a broad ingest set and a smaller delivery set.

- Ingest means the Server scans, probes, describes, and can transform the source.
- Direct play means the exact container, codecs, profile, level, color, audio, and subtitles work on the client.
- Compatible delivery means Kinosail remuxes or transcodes only the incompatible parts.

This separation is necessary. FFmpeg exposes many demuxers and decoders, while browsers support narrower combinations. FFmpeg requires each build to report its enabled components through `-demuxers`, `-decoders`, and related commands. [FFmpeg formats](https://ffmpeg.org/ffmpeg-formats.html), [FFmpeg codecs](https://ffmpeg.org/ffmpeg-codecs.html)

No credible primary-source dataset measures the formats in all private media libraries. The proposed set is a practical 90% target. Kinosail must verify the target against a representative local inventory before making a coverage claim.

## Current Kinosail baseline

| Area | Current support |
| --- | --- |
| Movie and Show scan | `.avi`, `.m4v`, `.mkv`, `.mov`, `.mp4`, `.webm` |
| Audio scan | `.aac`, `.aif`, `.aiff`, `.flac`, `.m4a`, `.m4b`, `.mp3`, `.ogg`, `.opus`, `.wav`; ordinary audio becomes an audiobook inside an Audiobooks path |
| Book scan | `.cbz`, `.epub`, `.pdf` |
| Sidecar subtitles | `.srt`, `.vtt` |
| Browser delivery | Conservative direct MP4 or WebM combinations; otherwise H.264/AAC fragmented-MP4 HTTP Live Streaming (HLS) |
| Readers | PDF pass-through, ZIP-based CBZ images, and a limited EPUB spine reader |

The scanner list is in [library.go](../../packages/library/library.go). Subtitle discovery is in [scan.go](../../packages/library/scan.go). Playback decisions are in [playback_plan.go](../../apps/player/internal/server/playback_plan.go). Reader behavior is in [reader.go](../../apps/player/internal/server/reader.go).

## Implemented coverage

This change adds the practical formats that fit Kinosail's existing playback and reader boundaries.

- Video discovery now includes 3G2, 3GP, ASF, FLV, MPEG, MPG, M2TS, MTS, OGV, TS, VOB, and WMV.
- Direct capability checks now include AV1 in MP4 or WebM, VP9 in WebM, and AC-3 audio.
- Audiobook discovery now treats MP4 files inside Audiobook paths as audio-only books. It also accepts OGA and WebM audio.
- Photo discovery now accepts AVIF and BMP.
- Comic discovery and reading now accept CBZ, CB7, and CBT. CB7 and CBT use bounded libarchive reads. CBR/RAR remains deferred because the RAR algorithm is proprietary.
- Comic pages accept JPEG, PNG, WebP, GIF, and AVIF.

The change does not claim usable support for DRM-protected AA/AAX, Kindle MOBI/AZW/AZW3, HEIC, or SVG. Those formats need new conversion or security boundaries.

## Priority 0: broad practical coverage

Priority 0 is the proposed 90% target. It includes modern formats and the common legacy inputs that still occur in home libraries.

| Domain | Accept and probe | Delivery rule |
| --- | --- | --- |
| Movie and Show containers | MKV, MP4, M4V, MOV, WebM, AVI, MPEG-TS, MTS, M2TS | Direct play only after an exact capability check. Otherwise remux or transcode. |
| Video codecs | H.264/AVC; HEVC/H.265 Main and Main 10; AV1 Main 8/10-bit; VP9 | Preserve compatible streams. Use H.264/AAC as the widest web fallback. |
| Legacy video codecs | MPEG-2 Video; MPEG-4 Part 2, including Xvid and DivX; VC-1 and WMV3 | Treat these as decode and transcode inputs. Do not promise browser direct play. |
| Video audio | AAC, AC-3, E-AC-3, DTS family, TrueHD, Opus, Vorbis, FLAC, MP3, ALAC, PCM | Preserve a compatible selected track. Otherwise transcode audio without changing video. |
| Subtitles and captions | SRT, WebVTT, ASS/SSA, PGS, VobSub, MP4 timed text, EIA-608/708 | Keep text selectable. Convert text when needed. Burn image subtitles only when required. |
| Audiobook files | MP3; M4B, M4A, and audio-only MP4; FLAC; Ogg Opus; Ogg Vorbis | Support one-file books and ordered file-per-chapter folders. Preserve chapters, cover, language, narrator, and track order. |
| Audiobook codecs | MP3, AAC-LC, HE-AAC, xHE-AAC/USAC, ALAC, FLAC, Opus, Vorbis | Capability-test direct play. Transcode unsupported sources to AAC-LC or Opus. |
| Ebooks | EPUB 2/3, PDF, and DRM-free MOBI, AZW, and AZW3 | Read EPUB/PDF directly. Convert Kindle-family inputs into a safe internal reading representation. Never replace the source. |
| Comics | CBZ/ZIP, CB7/7z, CBT/tar, PDF, fixed-layout EPUB | Decode bounded pages into a cache. Do not modify read-only Library Content. |
| Comic page images | JPEG, PNG, WebP, GIF, AVIF | Serve supported images. Convert only unsupported page encodings to JPEG or PNG. |
| Comic metadata | `ComicInfo.xml` 1.0, 2.0, and recognized 2.1 fields | Preserve page order, cover, double-page, reading direction, language, and creator metadata. |

Jellyfin's official matrix shows why container, video, audio, and subtitle compatibility must be evaluated together. It lists H.264 as the broadest client codec and shows conditional HEVC, AV1, VP9, MKV, and HDR support. [Jellyfin codec support](https://jellyfin.org/docs/general/clients/codec-support/)

The web fallback should remain MP4 or fragmented MP4 with H.264 and AAC. Mozilla's current format guidance calls that combination broadly supported across major browsers. It recommends AV1/Opus WebM where the device supports it. [Mozilla video codec guide](https://developer.mozilla.org/en-US/docs/Web/Media/Guides/Formats/Video_codecs)

The player must query the actual representation. The W3C Media Capabilities API accepts codec, profile, resolution, bitrate, frame rate, and display data. It reports support, expected smoothness, and expected power efficiency. [W3C Media Capabilities](https://www.w3.org/TR/media-capabilities/)

### New format emphasis

The next format work should emphasize these cases:

1. HEVC Main 10 and AV1 Main ingest for modern video libraries.
2. xHE-AAC/USAC decode for newer M4B audiobooks.
3. EPUB 3 fixed layout, right-to-left layout, WebP, Opus, navigation, and media overlays.
4. Bounded archive handling and `ComicInfo.xml` metadata for the open comic containers.
5. PGS, VobSub, and ASS/SSA handling without forcing unrelated transcodes.

The current container pins Jellyfin FFmpeg 7.1.4-3. FFmpeg added native xHE-AAC work during the 7.1 cycle. FFmpeg 8.1 still marks Mps212 support experimental. Kinosail must test real xHE-AAC samples before it claims support. [FFmpeg current release notes](https://ffmpeg.org/index.html), [Audiobookshelf xHE-AAC guidance](https://audiobookshelf.org/docs/faq/server/)

Audiobookshelf identifies MP3, M4B, and Opus as common audiobook choices. It also documents file-per-book and file-per-track layouts. [Audiobookshelf format guidance](https://audiobookshelf.org/docs/faq/server/), [Audiobookshelf book structure](https://audiobookshelf.org/docs/documentation/libraries/book-library/directory-structure/)

M4B is an MP4-family convention, not one codec. Kinosail must probe the contained audio and chapters. ID3 defines `CHAP` and `CTOC` frames for MP3 chapter structures. [ID3 chapter specification](https://id3.org/id3v2-chapters-1.0)

EPUB 3.3 defines a ZIP-based publication with metadata, manifest, spine, navigation, reflowable or fixed layout, and media overlays. Its core media include JPEG, PNG, GIF, SVG, WebP, MP3, AAC-LC in MP4, and Opus in Ogg. [W3C EPUB 3.3](https://www.w3.org/TR/epub-33/)

PDF 2.0 is the current ISO Portable Document Format standard. [ISO 32000-2:2020](https://www.iso.org/standard/75839.html)

CBZ is the open ZIP comic container. CBR is common, but RAR remains proprietary, so Kinosail defers CBR/RAR support until a separate legal review. [Komga supported media](https://komga.org/docs/introduction/), [IANA CBZ registration](https://www.iana.org/assignments/media-types/application/vnd.comicbook%2Bzip), [WinRAR license](https://www.rarlab.com/license.htm)

`ComicInfo.xml` has several deployed schemas. The Anansi Project documents the versions and the page, direction, and double-page fields. Version 2.1 remains a draft. [ComicInfo overview](https://anansi-project.github.io/docs/comicinfo/intro), [ComicInfo 2.1 draft](https://anansi-project.github.io/docs/comicinfo/schemas/v2.1)

## Priority 1: useful compatibility inputs

These formats improve long-tail coverage. They should follow the Priority 0 corpus gate.

| Domain | Formats |
| --- | --- |
| Video containers and disc layouts | MPG, MPEG, VOB, `VIDEO_TS`, BDMV, WMV/ASF, FLV, 3GP |
| Audiobooks | Raw AAC/ADTS, WAV, WMA, and detection of AA/AAX |
| Ebooks | FB2, DjVu, HTML, TXT |
| Comics | CB7/7z and CBT/tar |

Calibre's maintained input plugins cover MOBI/AZW families, EPUB, PDF, FB2, DjVu, HTML, TXT, and comic archives. This is useful implementation evidence, not a market-share measurement. [Calibre input plug-ins](https://github.com/kovidgoyal/calibre/blob/master/src/calibre/customize/builtins.py)

Libarchive is used only for bounded CB7/7z and CBT/tar reads. Its upstream code is BSD-licensed. Kinosail does not enable RAR/CBR discovery or reading. [libarchive](https://github.com/libarchive/libarchive)

## Priority 2: defer

Defer VVC/H.266, LCEVC, MPEG-H, IAMF, APAC, exotic archives, and obsolete ebook formats.

These inputs do not help the first broad-coverage target enough to justify their test and client matrix. VVC can become a transcode-only input after the bundled FFmpeg decoder passes focused samples.

## Security and correctness requirements

Every accepted extension creates an input boundary.

- Verify magic and structure. Do not trust the extension or MIME type.
- Probe media streams before playback. Keep container and codec facts separate.
- Bound source size, archive entries, expanded bytes, compression ratio, nesting, page pixels, metadata, and processing time.
- Reject traversal paths, symlinks, devices, executables, encrypted archives, and external document fetches.
- Isolate PDF and EPUB content. Disable scripts, launch actions, remote references, and unrestricted active content.
- Detect Digital Rights Management (DRM) and encryption. Return an explicit unsupported result.
- Never request, store, or log an Audible activation secret as part of baseline support.
- Keep the original file unchanged. Put derived pages and compatible streams in the bounded cache.

Go's ZIP package exposes 64-bit compressed and uncompressed sizes and reports insecure paths. Those checks are necessary but do not replace aggregate resource limits. [Go `archive/zip`](https://pkg.go.dev/archive/zip)

PDF can contain JavaScript, external references, embedded files, and actions. The registered PDF media type requires consumers to consider these risks. [RFC 8118](https://www.rfc-editor.org/rfc/rfc8118.html)

FFmpeg documents AAX as encrypted M4B that needs an activation secret. This is outside the default trust and credential boundary. [FFmpeg AAX demuxer](https://ffmpeg.org/ffmpeg-formats.html#aax)

## Coverage gate

Build a local, privacy-safe format inventory before implementation. Record only counts by extension, probed container, codec, profile, bit depth, subtitle type, and encryption state. Do not record paths or titles.

Use these two measures:

- Recognition coverage: valid non-DRM items that appear in the correct Library view.
- Usable coverage: valid non-DRM items that complete playback or open in the reader.

Priority 0 passes when a representative corpus reaches at least 95% recognition and 90% usable coverage. Unsupported, corrupt, or encrypted items must show a clear reason. They must not disappear from the scan.

Test each accepted family through scan, probe, versioned API, web adapter, playback or reader, progress, seek or page navigation, and failure recovery. Use malformed, oversized, encrypted, mislabeled, and truncated negative fixtures. Prove that rejected inputs cause no side effects.

Run the final matrix against the exact FFmpeg build in the production container. Static upstream support does not prove that the pinned build contains or correctly handles a decoder.
