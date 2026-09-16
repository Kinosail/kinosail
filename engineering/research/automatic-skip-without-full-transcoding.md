# Automatic skip without whole-file transcoding

Research snapshot: 2026-08-23.

This note asks whether Kinosail can physically omit marked sections so an unaware player receives a shortened presentation, while avoiding a full video/audio transcode. It uses standards, official player/server documentation, original research papers, and the current Kinosail working tree. It does not propose changing marker detection.

## Executive decision

**Yes, Kinosail can avoid transcoding most or all compatible media, but it cannot truthfully call a modified presentation Direct Play of the original file.** Jellyfin defines Direct Play as delivering the file without modification. It calls a container change with untouched audio and video streams a remux; its separate “Direct Stream” category changes audio while retaining video. [Jellyfin playback types](https://jellyfin.org/docs/general/post-install/transcoding/)

Use a three-rung server-side delivery ladder:

1. **Lossless skip remux:** when every cut can be aligned to a safe random-access boundary, copy the original compressed video and audio packets into a shortened fMP4 HLS presentation, rewrite timestamps, and cache it. This uses little CPU and introduces no video generation loss. Advertise it as remux/server-composed playback, not original Direct Play.
2. **Boundary-only smart render:** when exact cuts are required between random-access points, re-encode only the GOPs touching each cut and copy the rest. This is the technically correct way to preserve exact boundaries without encoding the entire title, but it needs a deliberately engineered codec-aware splicer; stock FFmpeg does not make it a safe one-command feature.
3. **Existing full transcode:** retain it only when the source codec, color format, selected tracks, subtitles, or client profile already requires conversion, or when smart rendering cannot produce a compatible splice.

A prepared, immutable chopped derivative may be directly served on later plays. It is reasonable product language to call that “direct playback of an optimized version,” but it is not Direct Play of the original item under Jellyfin’s terminology.

## The constraint: arbitrary frames are not independent

Modern delivery codecs normally use inter-frame prediction. A retained frame after a cut can depend on a removed reference frame. FFmpeg therefore warns that input seeking normally lands on the closest earlier seek point; during stream copy, packets between that seek point and the requested time are preserved. Its segment muxer likewise starts segments on keyframes and says exact split times require matching input keyframes; forcing new keyframes requires transcoding. [FFmpeg seek behavior](https://ffmpeg.org/ffmpeg.html#Main-options), [FFmpeg segment muxer](https://ffmpeg.org/ffmpeg-formats.html#segment_002c-stream_005fsegment_002c-ssegment)

The practical result is:

- packet copy can make an exact cut only where the retained stream can begin independently;
- keyframe-aligned cuts can retain the original encoded samples;
- a frame-exact arbitrary cut needs some re-encoding around the cut, an edit instruction the player understands, or visible boundary error;
- audio must also be cut and retimestamped on valid packet/sample boundaries to keep it synchronized with video.

This is not merely an FFmpeg limitation. Compressed-domain editing research treats missing temporal references and decoder-buffer behavior as the central splice problem. Meng and Chang’s original MPEG work decodes/re-encodes only material outside safe GOP boundaries rather than processing the entire sequence. [Meng and Chang, “Buffer Control Techniques for Compressed-Domain Video Editing”](https://www.ee.columbia.edu/dvmm/publications/96/meng96b.pdf)

## Option comparison

| Technique | Re-encodes video? | Exact arbitrary cuts? | Unaware-player reach | Physically withholds omitted payload? | Honest classification | Decision |
| --- | --- | --- | --- | --- | --- | --- |
| Original file plus seek commands | No | Yes, at playback time | Requires client feature | No | Direct Play | Does not meet the request |
| Keyframe-aligned stream-copy remux | No | Only at safe random-access boundaries | Broad through HLS/fMP4 when the codec is supported | Yes | Remux/server-composed | Build first |
| HLS playlist or byte-range surgery over already prepared CMAF | No | Segment/fragment boundaries only | Broad HLS clients | Yes | Remux/server-composed | Useful cache representation |
| Arbitrary byte ranges over an ordinary MP4/MKV | No | No | Invalid/unreliable | Indeterminate | Neither | Reject |
| MP4 edit lists | No | Can express presentation edits | Not portable for multiple cuts | Usually no | Modified file/remux | Reject for compatibility |
| Matroska ordered chapters | No | Virtual timeline | Matroska-aware players only | No | Modified metadata | Reject for “any player” |
| Browser MSE append windows/timestamp offsets | No | Can be sample-aware | Kinosail-controlled web player only | Potentially | Client-managed | Keep only as a client optimization |
| HLS discontinuities or DASH periods | No if media is already splice-safe | Segment boundaries | Client/profile dependent | Yes | Manifest-composed stream | HLS is useful; DASH adds no needed reach today |
| Boundary-GOP smart rendering | Only boundary GOPs | Yes | Broad if the output is validated | Yes | Mixed transcode/remux | Engineer after lossless remux |
| Cached fully transcoded derivative | Once | Yes | Broadest | Yes | Transcoded derivative | Existing fallback |

## Lossless stream-copy remux is the immediate win

FFmpeg defines stream copy as copying encoded packets without decoding, filtering, or encoding. Its concat demuxer can join compatible streams, but it warns that non-intra codecs can emit extra packets before an `inpoint` and after an `outpoint`, and that timestamps can overlap. This reinforces that a generic `inpoint`/`outpoint` recipe is not enough for frame-exact cuts. [FFmpeg stream copy and concat demuxer](https://ffmpeg.org/ffmpeg-all.html), [FFmpeg concat format](https://ffmpeg.org/ffmpeg-formats.html#concat-1)

For Kinosail, the safe lossless rule should be:

1. Index actual random-access points for the selected video stream.
2. Form source segments at those boundaries and identify segments wholly contained inside each skip marker.
3. Omit only those whole segments. This snaps the effective removed interval **inward**, so Kinosail may leave a short marked prefix or suffix but never deletes ordinary program content outside the marker.
4. Remux kept packets, rewrite decode/presentation timestamps onto the shortened timeline, and align selected audio to the same splice.
5. Probe the finished result before publication.

When a chapter or detector boundary already coincides with a safe access point, the result can be exact with no video re-encoding. Otherwise the imprecision is bounded by source GOP placement. Kinosail should expose the effective omitted ranges in its presentation timeline rather than pretending the requested marker boundaries were used.

### HLS/CMAF is the best transport for this lane

HLS playlists enumerate media segments in playback order. `EXT-X-BYTERANGE` can identify a media segment stored inside a larger resource, and `EXT-X-DISCONTINUITY` is required when the timestamp sequence changes. [RFC 8216 media playlists and byte ranges](https://www.rfc-editor.org/rfc/rfc8216.html#section-4.3.2.2), [RFC 8216 discontinuities](https://www.rfc-editor.org/rfc/rfc8216.html#section-4.3.2.3)

Apple’s CMAF/HLS documentation says CMAF fragments are independently decodable, recommends `EXT-X-INDEPENDENT-SEGMENTS`, permits byte-range segments, and permits `EXT-X-DISCONTINUITY` to concatenate CMAF tracks while resetting presentation timestamps. [Apple CMAF with HLS](https://developer.apple.com/documentation/http-live-streaming/about-the-common-media-application-format-with-http-live-streaming-hls)

There are two important limitations:

- A byte range is not automatically a valid media segment. RFC 8216 requires an fMP4 segment to have the appropriate fragment structure and initialization section. Arbitrary ranges of a normal MP4 or MKV are not a zero-work HLS presentation; ordinary library files must first be fragmented/remuxed. [RFC 8216 fragmented MP4 segments](https://www.rfc-editor.org/rfc/rfc8216.html#section-3.3)
- Apple requires HLS video segments to start with IDR frames and normally requires continuous fragment decode time; encoding breaks must be marked as discontinuities. [Apple HLS authoring requirements](https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices/)

The delivery format has useful reach: Android Media3 officially supports both MPEG-TS and fMP4/CMAF HLS, while Apple defines and natively supports HLS. Android still recommends a continuous media structure, accurate segment durations, and independent segments, so Kinosail must validate splices rather than assume every discontinuity is seamless. [Android Media3 HLS support and authoring guidance](https://developer.android.com/media/media3/exoplayer/hls)

This route is “any normal Kinosail/Jellyfin player that advertises compatible HLS and codecs,” not literally every media player. A raw-file-only client, a profile that rejects HLS, or a client that cannot decode the source codec still needs another representation.

### A virtual progressive file is possible, but deeper

Kinosail could synthesize a shortened MP4 index and map HTTP ranges of that virtual file back to retained packet extents in the source. That could avoid storing duplicate media payload while presenting a seekable progressive file. It is still a remux: Kinosail must build new sample tables or fragments, rewrite offsets and timestamps, implement exact `Content-Length` and range mapping, and begin every retained video run at a safe access point.

This is a deeper and less broadly exercised module than cached fMP4 HLS. It should be considered only after the remux lane proves that storage, rather than CPU, is the actual bottleneck.

## Why metadata-only tricks do not satisfy “any player”

### MP4 edit lists

An ISO BMFF edit list maps movie presentation time to media time, so it can hide pre-roll or describe a virtual edit without changing the encoded samples. Apple documents the edit-list atom as the mapping from movie time to media time. [Apple QuickTime edit-list atom](https://developer.apple.com/documentation/quicktime-file-format/edit_list_atom)

It is not a universal multi-cut solution. The W3C ISO BMFF byte-stream specification requires MSE implementations to support only a single `elst` edit with playback rate one. Multiple edits—the feature needed to remove several interior ranges—are not part of that minimum interoperability contract. [W3C ISO BMFF initialization segments](https://www.w3.org/TR/mse-byte-stream-format-isobmff/#initialization-segments)

Google’s current Media3 Transformer documentation reaches the same product conclusion: MP4 edit-list trimming is experimental, some players ignore its pre-roll position, and the supposedly removed content remains in the file. [Android Media3 edit-list trimming](https://developer.android.com/media/media3/transformer/transformations#mp4-edit-lists)

Edit lists therefore fail both user requirements: unaware-player reliability and physical withholding of the skipped bytes.

### Matroska ordered chapters

Matroska has an elegant native virtual-timeline feature. RFC 9559 specifies ordered chapters that play selected chapter ranges in stored order, allowing an edition to omit unwanted portions without duplicating media. [RFC 9559 ordered chapters](https://www.rfc-editor.org/rfc/rfc9559.html#section-20.1.3)

That behavior is mandatory for a conforming Matroska player, not for MP4/HLS players or every Jellyfin client. The original payload also remains in the Matroska file. It is useful prior art, not a compatible Kinosail delivery contract.

### Browser Media Source Extensions

MSE exposes `appendWindowStart`, `appendWindowEnd`, and `timestampOffset`; a controlled JavaScript player can filter coded frames during append and shift later segments onto a new timeline. [W3C Media Source Extensions](https://www.w3.org/TR/media-source-2/#sourcebuffer)

That is a sound optimization for Kinosail’s web player but does not add functionality to an unaware native client. It also moves correctness back into each player, contrary to the server-composed goal.

### DASH multi-period and HLS discontinuity-only composition

DASH can describe multiple periods and discontinuities, and HLS can concatenate separately timestamped presentations with discontinuities. Both still depend on each kept part beginning at a valid random-access point and on the client handling transitions. DASH-IF’s timing model treats multi-period continuity/connectivity as explicit signaling and a player behavior concern. [DASH-IF timing model](https://dashif.org/Guidelines-TimingModel/)

Kinosail already has an HLS path and Jellyfin clients commonly negotiate it. Adding DASH solely for auto-skip would enlarge the compatibility matrix without removing the codec/GOP constraint. HLS/CMAF is the narrower choice.

## Exact cuts: boundary-only smart rendering

Smart rendering resolves the fundamental conflict by re-encoding only the dependency regions around a cut and copying every unaffected GOP. Original research demonstrates the shape of the solution:

- Meng and Chang describe arbitrary-position MPEG editing in which only frames outside GOP boundaries at the beginning and end are decoded/re-encoded, avoiding full decode/re-encode and its processing cost. [Compressed-domain editing paper](https://www.ee.columbia.edu/dvmm/publications/96/meng96b.pdf)
- Yoneyama, Takishima, and Nakajima demonstrate frame-accurate H.264/AVC cut/splice across I, P, and B pictures using GOP-length modification, bit allocation, and drift management. Their experiment reports much lower quality degradation and processing time than conventional re-encoding, but it uses controlled Baseline-profile material rather than arbitrary modern library files. [H.264/AVC paper](https://www.cecs.uci.edu/~papers/icme05/defevent/papers/cr1717.pdf), [DOI record](https://doi.org/10.1109/ICME.2005.1521667)
- IBM Research describes combining extracted MPEG audio/video segments wholly in the system bitstream domain into a composite file playable through ordinary web interfaces. [IBM compressed-domain system-stream editing](https://research.ibm.com/publications/universal-mpeg-content-access-using-compressed-domain-system-stream-editing-techniques)

The idea remains current but fragile in general-purpose implementations. Android Media3’s trim optimization decodes and re-encodes as little video as possible, then stitches that output to copied input. Google warns that the encoder profile and level must match, that input/output formats may prove incompatible, and that the implementation falls back to a normal full export when it cannot stitch them. The feature is experimental and limited to single-asset MP4. [Android Media3 trim optimization](https://developer.android.com/media/media3/transformer/transformations#optimize-trims), [Transformer builder contract](https://developer.android.com/reference/androidx/media3/transformer/Transformer.Builder#experimentalSetTrimOptimizationEnabled(boolean))

For Kinosail, a production smart-render lane must account for at least:

- H.264/HEVC open versus closed GOPs and leading/trailing references;
- SPS/PPS/VPS, profile, level, pixel format, resolution, color and HDR metadata equivalence;
- B-frame decode versus presentation order and timestamp continuity;
- rate-control and decoder-buffer behavior at splices;
- exact audio boundary, encoder delay, priming, and A/V synchronization;
- selected audio/subtitle tracks and codec-specific bitstream filters;
- a final probe plus multi-player conformance tests before cache publication.

Stock FFmpeg is the right demux/mux/probe foundation, but its documented stream-copy seek and concat behavior do not amount to a general smart renderer. Kinosail should not simulate one by concatenating an encoded boundary clip with copied packets unless it proves bitstream compatibility and fails closed to the existing full-transcode path.

## Recommended Kinosail design

### 1. Add a distinct skip-remux plan

The shared planner should eventually distinguish these outcomes:

```text
direct                 unchanged original file
skip-remux             shortened timeline, copied elementary streams
skip-smart-render      copied body plus encoded boundary GOPs
transcode              encoded video/audio presentation
unavailable            policy or client cannot receive a safe representation
```

This distinction belongs in the versioned playback API and the Jellyfin adapter, not only in the bundled web UI. A `skip-remux` response must not set `SupportsDirectPlay=true` for the original media source. It should expose an HLS server-composed URL and report remux/direct-stream semantics as closely as the Jellyfin contract permits.

Today, `playbackWithAutomaticSkip` unconditionally sets `ForceTranscode` for an eligible timeline, and the FFmpeg `select`/`aselect` filters necessarily enter the encode path. The existing remux branch copies codecs but does not apply omitted ranges. [current planner](../../apps/player/internal/server/playback_timeline.go), [current HLS recipe](../../apps/player/internal/server/hls_plan.go), [current FFmpeg modes](../../apps/player/internal/server/hls.go)

The smallest coherent change is not another client capability flag. It is a server-side packet-remux operation selected by the same shared playback planner that already owns source facts, client support, permissions, markers, and the shortened timeline.

### 2. Prepare and cache immutable presentations

Key the cache by:

```text
source content revision
selected video/audio/subtitle streams
effective marker ranges and marker revision
cut policy and random-access index
output container/segment policy
muxer/encoder build and relevant options
```

Publish only after FFmpeg or the future splicer exits successfully, the result probes with the expected codecs/duration/tracks, every kept HLS segment begins safely, and a playlist/timeline consistency check passes. Use atomic publication and never serve growing partial output as a completed VOD asset.

The first request can start a low-CPU remux job and stream/publish fragments as they finalize, but a ready cached derivative gives later viewers seekable, inexpensive playback. Precompute popular/common marker revisions in background only if measurements justify the storage.

### 3. Preserve one effective timeline

The server must project duration, resume position, progress writes, chapters, subtitles, trickplay, and session state against the **effective** cuts actually used. The current timeline projection is reusable. For lossless remux, feed it the random-access-aligned ranges rather than the requested ranges; for smart render/full transcode, the exact requested ranges can remain.

External subtitle cues that cross a splice must be clipped and shifted. Embedded subtitles can be packet-copied only if their timing and format survive the remux; otherwise serve a projected sidecar or fall back according to the client profile. Trickplay should be generated/projected for the derivative revision so seeking never lands in omitted source time.

### 4. Make fallback behavior explicit

For a source the client could otherwise Direct Play:

1. If all requested cuts are safe and exact at random-access points, use exact `skip-remux`.
2. If keyframe-aligned effective cuts leave only a small bounded amount of marked content and the product accepts that tolerance, use approximate `skip-remux` and expose the effective ranges.
3. If exactness is required and the smart renderer supports the exact codec/profile, use `skip-smart-render`.
4. Otherwise use the existing full transcode when policy permits.
5. If conversion is prohibited, preserve original Direct Play plus marker/client behavior where supported, or report server-side auto-skip unavailable. Do not silently remove unmarked program content to avoid transcoding.

Separately, if the source was not directly decodable by the client before auto-skip, skip-remux cannot make it so. Codec incompatibility, HDR tone mapping, bitrate/resolution limits, and bitmap subtitle burn-in still require the ordinary playback conversion chosen by the planner.

## Acceptance evidence before shipping skip-remux

Automated fixtures should cover H.264 and HEVC with closed and open GOPs, B-frames, long GOPs, AAC/AC-3/E-AC-3 audio, multiple audio tracks, embedded and sidecar subtitles, HDR metadata, variable frame rate, and markers exactly on and between random-access points. Assert:

- no video encoder is invoked in `skip-remux`;
- copied video packet payload hashes match retained source packets;
- the probed shortened duration and effective ranges agree;
- decoded frames around every splice contain no frozen/corrupt/missing-reference run;
- audio remains continuous and synchronized across every splice;
- seeking, resume, chapters, subtitles, trickplay, and progress mapping use the shortened timeline;
- cache identity changes with source or marker revision;
- a failed/incomplete derivative is never advertised.

Then test the same fixtures through Kinosail web playback and the official Jellyfin clients on Safari/iOS, Android Media3, Chromium/Firefox through the web HLS implementation, and representative TV clients. A standards-valid discontinuity is necessary but not sufficient evidence of a seamless user experience.

## Bottom line

Kinosail should replace “auto-skip always forces a full transcode” with **“copy when splice-safe, encode only boundaries when exactness demands it, and fully transcode only for ordinary compatibility or a proven fallback.”**

The first production step is a cached, keyframe-aligned HLS/fMP4 skip-remux lane. It gives the user the important result—an unaware compatible player receives only a shortened presentation—at remux cost and without video quality loss. Exact arbitrary cuts can later use a tested boundary-GOP smart renderer. Neither should be labeled Direct Play of the original file.
