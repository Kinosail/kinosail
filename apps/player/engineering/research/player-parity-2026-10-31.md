# Player parity with best-in-class players — 2026-10-31

Goal (from the Owner): the Kinosail player clients must be as good as or better than the best-in-class third-party video players on Apple platforms, and support the major media servers. This note compares Kinosail today with that bar and orders the work. Apple TV and iPhone/iPad come first because iOS and tvOS are the design standard; Android, Android TV and web follow.

Status key: **Have** (shipped and evidenced), **Partial**, **Missing**, **Verify** (code suggests support, but no evidence yet). Evidence paths are relative to the repository root.

## How Kinosail plays media today

Kinosail is server-assisted. The Server probes each file, the client reports its capabilities, and the plan is direct play first, then a lossless remux ("copied HLS", fMP4), then a transcode (`packages/playback/plan.go`, `packages/playback/hls_source.go`). The Apple clients use AVFoundation/AVKit only; they have no software decoder (`apps/player/apps/native/IMPLEMENTATION.md`). Android uses Media3 ExoPlayer without a software decoder extension (`apps/player/apps/android/app/build.gradle.kts`).

Best-in-class players decode on the device with a software decoder when the platform decoder cannot. Kinosail reaches the same files through the Server. The gap is therefore smaller for files on a Kinosail Server and largest for sources without a Kinosail Server (Plex/Jellyfin/Emby direct, network shares).

## The real library (Nox, 2026-10-31)

A random sample of 400 of the 4,263 video files (3,958 MKV, 293 MP4, 12 AVI), probed read-only with ffprobe; only aggregate counts were kept:

| Measure | Share of sampled files |
| --- | --- |
| H.264 video | 90% (no HEVC, HDR or Dolby Vision in the sample) |
| 23.976 fps video | 80% (29.97 fps 7%) |
| DTS-HD audio | 38% |
| E-AC-3 audio | 30% |
| 5.1 or more audio channels | 55% |
| PGS bitmap subtitles | 38% |
| SRT subtitles | 39% |

So, for this household, frame-rate matching, keeping surround sound from DTS-HD, and showing PGS subtitles properly matter more than Dolby Vision today. The plan below reflects that order; Dolby Vision stays in phase 1 so new HDR files are ready.

## Capability matrix

### P1 — Playback compatibility and picture/sound fidelity

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| Direct play of MP4/MOV/HLS with H.264, HEVC (incl. 10-bit), AV1 where the device decodes it | Direct first; capability hints from Apple APIs | Have | `packages/playback/plan.go`; `IMPLEMENTATION.md` Playback row |
| MKV, AVI, other containers | Lossless Server remux to fMP4 HLS when codecs are compatible | Have (Server) | `packages/playback/hls_source.go` |
| VC-1, MPEG-2, VP8, and AV1 on devices without AV1 decode | Server transcode | Partial | Works with a Server; no on-device decoder. See decision D1. |
| Dolby Vision profile 5 and 8.x | Signalled as `dvh1`/`dvhe` in fMP4 HLS | Verify | `packages/isobmff/codecs.go`, `packages/playback/hls_source.go`; needs a real P5/P8 file on Apple TV 4K |
| Dolby Vision profile 7 (dual layer, FEL/MEL) | Base layer used; some conversions rejected ("this Dolby Vision conversion is unsupported") | Partial | `DolbyVisionBaseLayer`, `dolbyVisionNeedsConversion`; target: P7 → P8.1 RPU conversion, else HDR10 base layer |
| HDR10, HLG | Probed and passed through | Verify | `packages/mediaprobe/facts.go` |
| HDR10+ | Probed | Verify | 3 Server references; Apple TV shows HDR10+ only on supported TVs |
| tvOS match frame rate and dynamic range | Not implemented in the app (no `AVDisplayCriteria`) | Missing | Highest-impact Apple TV gap: 24p judder and SDR/HDR mode switching |
| Dolby Digital (AC-3), Dolby Digital Plus (E-AC-3) incl. Atmos (JOC) | Passed through | Verify | `packages/playback/http.go` lists ac3/eac3 |
| TrueHD, DTS, DTS-HD MA, DTS:X | Transcoded to AAC | Partial | `plan.go` uses `aac`; target: E-AC-3 5.1 (7.1 where possible) on Apple TV so surround survives. Lossless/object passthrough is not possible on Apple TV. |
| Fast start | Prepare step, loopback capability, direct-first | Verify | Needs a measured start-time budget (see Quality) |
| Resume (incl. across devices) | Server progress with compare-and-set | Have | `IMPLEMENTATION.md` Progress row |
| Transcode only as last resort; bitrate choice | Direct first; quality choices for downloads | Partial | No user-visible stream quality cap on Apple TV |

### P2 — Subtitles

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| Embedded text subtitles (SRT/WebVTT/mov_text) | Native tracks | Have | `IMPLEMENTATION.md` Subtitles row |
| External sidecar subtitles | Delivered as WebVTT | Have | `packages/playback/subtitle_delivery.go` |
| ASS/SSA with styling and positioning | Converted to plain WebVTT text | Partial | Styling lost; target: styled rendering on device |
| PGS, VobSub, DVB bitmap subtitles | OCR to WebVTT text | Partial | `packages/mediaprobe/image_subtitle_ocr.go`; target: show the original bitmaps, keep OCR as fallback |
| Online subtitle search from the player | Not in the player (Kinosail Subtitles has it) | Missing | Wire the player to Kinosail Subtitles |
| Size, color, background, position | System caption style only | Partial | Add in-app style controls |
| Subtitle offset (sync) during playback | None | Missing | No offset control in the app |
| Forced and SDH handling | Forced/SDH roles recognized | Partial | `library.SubtitleFileRole`; automatic forced selection needs verification |
| Memory of audio and subtitle choices | Per title | Partial | `PreferencesAPI.playbackPreferences(itemID:)`; target: per show |
| External captions in Picture in Picture | Not shown | Missing | Documented limit in `IMPLEMENTATION.md` |

### P3 — Player experience

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| Chapters | Server returns chapters | Verify | Need chapter navigation in the transport bar |
| Skip intro, recap, credits | Markers, automatic skip setting | Have | `apps/player/apps/native/DESIGN.md` Video playback |
| Scrubbing thumbnails (trick play) | I-frame playlist when the stream has one | Partial | `EXT-X-I-FRAME` in one Server path; direct play has none |
| Playback speed | AVKit | Have | |
| Audio delay | None | Missing | |
| Sleep timer | Audio only | Partial | Add for video |
| Picture in Picture | iPhone/iPad | Have | |
| AirPlay | Route picker | Have | |
| Siri Remote gestures (click-pad skip, swipe scrub) | AVKit defaults | Verify | Needs a physical remote check |
| Next episode | Starts at once when credits skip; Next Episode action otherwise | Partial | Add a visible countdown with cancel |
| Up Next / Continue watching across devices | Server-backed | Have | |
| Top Shelf | Continue watching, My List, recently added | Have | |

### P4 — Sources ("support the major apps")

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| Kinosail Server | Native | Have | |
| Plex, Jellyfin, Emby | On-device device library with sign-in, discovery, playback, progress and watched reporting | Partial | `Sources/DeviceLibrary`; playback limited to what AVFoundation decodes without a Kinosail Server |
| SMB, NFS, WebDAV shares with metadata matching | None | Missing | Large: needs a share client, scanner and matcher on the device |
| Trakt scrobbling (optional) | None | Missing | |

### P5 — Library

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| Metadata, artwork, collections, filters, search | Server-backed | Have | `IMPLEMENTATION.md` Library row |
| Profiles and parental controls | Viewer Profiles with rating policy | Have | `apps/player/docs/user-guide/profiles.md` |
| Offline downloads | iPhone/iPad with verification | Have | Apple TV streams only, by design |

### P6 — Quality

| Measure | Kinosail today | Status | Note |
| --- | --- | --- | --- |
| Crash-free sessions | Not measured | Missing | Needs MetricKit collection and a target (≥ 99.8%) |
| Time to first frame | Not measured | Missing | Target: direct play ≤ 1.5 s, remux ≤ 3 s on Apple TV HD |
| Memory and thermal on Apple TV HD | Not measured | Missing | Budget per playback mode |

## Decisions needed

- **D1, on-device software decoding.** Best-in-class players bundle a software decoder (typically FFmpeg-based) for VC-1, MPEG-2, unusual audio and styled subtitles. Kinosail is source-available under PolyForm Perimeter. FFmpeg can be used only in an LGPL-only build, shipped as a dynamic framework with notices and relinking materials; GPL builds are not compatible. This is a licensing decision for the Owner. Until then, Kinosail covers these files through the Server. Recommendation: decide after P1–P3, with a measured list of files in the real library that still need a Server transcode.
- **D2, Apple TV lossless audio.** Apple TV cannot pass TrueHD or DTS bitstreams to a receiver. Kinosail will convert to multichannel E-AC-3 instead of AAC. This is a platform limit, not a choice.

## Phase plan

Order chosen by impact on what the Owner watches on Apple TV, then reach:

1. **Phase 1, picture and sound on Apple TV.** Match frame rate and dynamic range (80% of the library is 23.976 fps); E-AC-3 5.1 instead of AAC for DTS-HD and TrueHD (38% of files); then Dolby Vision P5/P8 direct and P7 → P8.1 or HDR10 fallback; a stream quality cap. Verify each against real files from the Nox library.
2. **Phase 2, subtitles.** Original PGS bitmaps instead of OCR text (38% of files); offset control; size/color/background/position; per-show memory of audio and subtitle choices; automatic forced tracks; styled ASS/SSA and original PGS bitmaps; online search through Kinosail Subtitles.
3. **Phase 3, player experience.** Next-episode countdown with cancel; chapters in the transport bar; audio delay; video sleep timer; thumbnails for direct play.
4. **Phase 4, measurement.** MetricKit crash and hang collection, time-to-first-frame and memory budgets, a playback test matrix of real files.
5. **Phase 5, sources.** First-class Plex/Jellyfin/Emby playback (route unsupported files through a Kinosail Server when one is connected), Trakt, then SMB/NFS/WebDAV shares.
6. **Phase 6, other clients.** Bring Android, Android TV and web to the same matrix.

Skipped on purpose: lossless TrueHD/DTS passthrough on Apple TV (platform limit, D2); Google Cast on Apple clients (no native sender SDK, already documented). D1 waits for a licensing decision.
