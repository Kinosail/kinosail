# Modern adaptive-bitrate VOD streaming for Kinosail

Research snapshot: 2026-08-23.

This note turns current HLS specifications, browser/player behavior, FFmpeg capabilities, and original ABR research into a production direction for Kinosail. The scope is on-demand video from one owner-hosted Kinosail Server container.

## Executive decision

Build an **adaptive compatibility lane**, not a replacement for every direct play:

1. Keep **Original** direct play when the client can decode the source and the viewer explicitly asks for it, and keep it as the zero-transcode fast path for confidently local playback.
2. Make **Auto** use one source-aware HLS multivariant presentation whenever the path may be constrained. Its top rendition should preserve the source by copy/remux only when that produces codec-compatible HLS with boundaries aligned to the encoded ladder; otherwise encode the top rung too. Lower H.264/AAC renditions provide graceful fallback.
3. Let the maintained hls.js controller make web ABR decisions. Configure automatic bandwidth testing, player-size and sustained-frame-drop caps, and expose a manual `Auto / Original / <resolution>` menu. Do not build a custom BOLA, MPC, or learned controller yet.
4. Treat the ladder and manifest as measured data, not labels pasted over encoder targets: cap the ladder at the source's resolution, frame rate, and useful bitrate; align every rendition at the same independently decodable boundaries; and publish measured peak/average bandwidth plus complete codec/display attributes.
5. Tune from private, local-only quality-of-experience measurements and a repeatable network-trace test matrix. Never send playback telemetry off the Server.

This direction reuses the current playback plan and fMP4 HLS seams rather than creating a second media system. It also preserves Kinosail's API-driven monolith: the versioned playback operation must describe the selected lane and quality options, while the web adapter presents those same choices.

## Current Kinosail baseline

Kinosail already has the important skeleton:

- playback planning distinguishes direct, remux, audio-transcode, and video-transcode work and already carries source dimensions, bitrate constraints, HDR state, and selected tracks ([`playback_plan.go`](../../internal/server/playback_plan.go));
- compatibility playback emits fMP4 HLS with four-second segments and two H.264/AAC targets, 1080p/6 Mbit/s and 720p/2.5 Mbit/s ([`hls.go`](../../internal/server/hls.go));
- the generated multivariant playlist currently declares only `BANDWIDTH` and `AVERAGE-BANDWIDTH` and identifies renditions as `high`/`low` ([`hls_playlist.go`](../../internal/server/hls_playlist.go)); and
- web playback instantiates hls.js 1.7.1 with its defaults and has no manual quality control ([`player.js`](../../internal/server/static/player.js)).

The gaps are therefore narrow but material: only two coarse rungs; unconditional 1920/1280 targets rather than a source-aware ladder; incomplete manifest attributes; no guaranteed closed, aligned GOP contract; default hls.js startup rather than an explicit bandwidth-test policy; no viewport/decode-pressure cap; and no quality UI or ABR validation harness.

## What the standards require

### Segments and switching

Every video segment should begin with an IDR, and Apple currently recommends IDRs every two seconds, a six-second target duration, accurate `EXTINF` durations, and matching segment boundaries across every audio/video variant and rendition. The key interoperability property is alignment: a client must be able to select another rendition at the same media time without a gap, overlap, or dependency on the prior rendition. Apple says VOD playlists must be immutable `EXT-X-PLAYLIST-TYPE:VOD` presentations and recommends `EXT-X-INDEPENDENT-SEGMENTS` when the segment starts are independently decodable. [Apple HLS Authoring Specification, video, segment, playlist, and multivariant requirements](https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices)

FFmpeg's HLS muxer cuts at the next key frame after `hls_time`; its own documentation says encoding should use a closed GOP sized to the segment constraint. Its `independent_segments` flag is correct only when every video segment is guaranteed to begin with a key frame, while `split_by_time` may cut on non-key frames and can make seeking worse. Use closed GOPs and explicit common keyframe times; do not use `split_by_time`. [FFmpeg HLS muxer documentation](https://ffmpeg.org/ffmpeg-formats.html#hls-2)

**Kinosail decision:** use a common two-second closed-GOP/IDR cadence inside the current four-second VOD segment as the initial product hypothesis. Every segment then starts on every second aligned IDR, while the intermediate IDR supports seeking and Apple's current recommendation. Four-second segments give faster ABR feedback and bound the delay before a manual switch, but differ from Apple's six-second `SHOULD`; promote six seconds if the cross-browser/device matrix shows no meaningful quality-switch benefit from four. Do not shorten further for VOD without measurements: Low-Latency HLS parts solve a live-latency problem Kinosail does not have.

### Multivariant playlist truthfulness

RFC 8216 defines `BANDWIDTH` as the peak segment bitrate and warns that an inaccurate value can cause stalls or prevent playback; `AVERAGE-BANDWIDTH` describes average segment bitrate. It recommends `CODECS`, `RESOLUTION`, and `FRAME-RATE` for video variants, and requires `CODECS` to name every format present when the attribute is supplied. [RFC 8216, `EXT-X-STREAM-INF`](https://www.rfc-editor.org/rfc/rfc8216.html#section-4.3.4.2)

Apple's current authoring rules are stricter: VOD measured average and peak segment rates must each be within 10% of their declared attribute; `AVERAGE-BANDWIDTH` and video `FRAME-RATE` are required; `VIDEO-RANGE` is required unless every rendition is SDR; and `BANDWIDTH` must include the largest playable video+audio combination. Apple also recommends separate streams for alternative or multichannel audio and at least two encodes at the highest available resolution for HD catalogs. [Apple HLS Authoring Specification](https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices)

The May 2026 HLS 2nd Edition draft is work in progress, not an IETF standard, but it records current protocol direction: switchable variants must present matching content, timestamps, discontinuity sequences, and target durations, and should carry the same encoded audio bitstream to avoid audible glitches. [HTTP Live Streaming 2nd Edition, draft 22 §§6.2.4–6.2.5](https://datatracker.ietf.org/doc/draft-pantos-hls-rfc8216bis/)

**Kinosail decision:** every HLS recipe must publish, from probe/encode results:

- `BANDWIDTH` measured from the largest segment-time window, including the selected audio rendition;
- `AVERAGE-BANDWIDTH` measured over the complete VOD representation;
- exact `CODECS`, `RESOLUTION`, and `FRAME-RATE`;
- `VIDEO-RANGE=SDR` or the correct HDR range when the presentation is not uniformly SDR;
- `AUDIO` and `SUBTITLES` group references when tracks are separate, otherwise `CLOSED-CAPTIONS=NONE` when true; and
- `EXT-X-INDEPENDENT-SEGMENTS` only after the encode/remux output proves the condition.

Encode the selected audio once as a separate AAC rendition and reference it from every compatible video variant. If the first increment keeps audio muxed into every variant, use byte-identical AAC settings and one common rate across all variants; the current 160/128 kbit/s split creates avoidable switch risk and duplicated work.

Do not infer these declarations from names such as `high` and `low`. Measure finalized segments, and reject publication if timestamps, codecs, dimensions, or declared rates fail validation.

Publish media atomically. FFmpeg's `hls_flags temp_file` writes segments and playlists to temporary files and renames them when complete, preventing a client from fetching a partially written fragment. [FFmpeg HLS muxer documentation](https://ffmpeg.org/ffmpeg-formats.html#hls-2)

## Source-aware rendition ladder

Apple's current H.264 example ladder spans 234p/145 kbit/s, 360p/365 kbit/s, 432p/730 and 1100 kbit/s, 540p/2 Mbit/s, 720p/3 and 4.5 Mbit/s, and 1080p/6 and 7.8 Mbit/s. Apple explicitly calls these initial targets and says to evaluate them against the actual content and encoding workflow. The same guidance preserves the natural VOD frame rate and original VOD color space, and advises against using a higher codec level than the content requires. [Apple HLS Authoring Specification](https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices)

Kinosail should use those figures as safe starting points, not precompute every point. Build the non-dominated subset of this H.264/AAC baseline:

| User label | Maximum 16:9 frame | Initial average video target | When to include |
| --- | ---: | ---: | --- |
| 360p | 640×360 | 365 kbit/s | Constrained remote safety rung for any source above 360p. |
| 432p | 768×432 | 1.1 Mbit/s | Bridge between 360p and 540p; omit for sources at or below 360p. |
| 540p | 960×540 | 2.0 Mbit/s | Default-quality neighborhood and a useful phone/tablet rung. |
| 720p | 1280×720 | 3.0 Mbit/s | Include only when the source is at least 720p. |
| 1080p | 1920×1080 | 6.0 Mbit/s | Include only when the source is at least 1080p and this is lower than a useful source-preserving top rung. |
| Original | Source dimensions | Measured source rate | Include in the adaptive set only when copy/remux produces a compatible rendition in the same codec family with the ladder's exact switch boundaries; always remain available as an explicit direct lane when the client supports it. |

The implementation rules matter more than the exact table:

- Never upscale, increase frame rate, convert SDR to HDR, or encode a target whose average rate is at/above the source rate unless a codec/container/subtitle transformation is independently required.
- Preserve aspect ratio and sample aspect ratio; round encoded dimensions to codec-compatible even values.
- Cap every target by the playback plan's client/profile/path maximum bitrate after reserving audio overhead.
- Keep adjacent useful rungs close enough that a single downshift can rescue playback. Treat a ratio near 2× as a starting test hypothesis, not a standard; content analysis may remove a rung that adds no visible quality.
- Keep the same codec family across one automatically switchable set. Apple's guidance says each codec stream should offer all anticipated bandwidths so a client is not required to switch codecs. An H.264/AAC fallback family is the broadest first implementation.
- If two targets collapse to the same dimensions/rate after source and client caps, emit one rendition.
- Name/display options by measured vertical resolution and, when useful, frame rate (`1080p`, `1080p60`), not internal directory names.

Netflix's original per-title work showed why a fixed ladder is only a baseline: different source complexity produces different rate-quality curves. Its later Dynamic Optimizer encodes and evaluates multiple resolution/quality choices per shot against VMAF, then chooses an efficient perceptual trajectory; Netflix reports that the analysis required roughly an order of magnitude more computation and over two orders of magnitude more encode units than its prior multi-minute chunks. [Netflix, “Optimized shot-based encodes: Now Streaming!”](https://netflixtechblog.com/optimized-shot-based-encodes-now-streaming-4b9464204830)

**Now:** source-aware pruning, capped VBR targets, and post-encode measurement. **Later:** optional background per-title analysis using sampled encodes and VMAF to remove dominated rungs. **Defer:** per-shot Dynamic Optimizer-style encoding until Kinosail has durable background scheduling, cache quotas, and evidence that the owner wants to spend that much compute for storage/bandwidth savings.

## ABR control: use the maintained hybrid already present

hls.js 1.x does considerably more than “pick bitrate below the last download speed.” Its default controller maintains fast and slow EWMA throughput estimates; discounts bandwidth more aggressively when switching up; considers forward-buffer time and estimated fragment load time; permits bounded starvation when no stall-free choice exists; and can abort an in-flight fragment for a lower level when finishing it would exhaust the buffer. Its current VOD defaults use EWMA half-lives of 3 and 9 seconds, a 500 kbit/s initial estimate, 0.95 stay/down and 0.7 up-switch bandwidth factors, and four-second maximum startup/starvation delays. [hls.js ABR controller source](https://github.com/video-dev/hls.js/blob/master/src/controller/abr-controller.ts), [hls.js configuration source](https://github.com/video-dev/hls.js/blob/master/src/config.ts)

That is already a practical throughput+buffer hybrid. The research alternatives are useful design evidence, but not a reason to fork the player:

- BOLA derives quality selection from buffer occupancy with a provable long-run utility bound and needs no bandwidth prediction. Its simplicity and rebuffer resistance support keeping buffer state in any future controller. [Spiteri, Urgaonkar, and Sitaraman, “BOLA”](https://arxiv.org/abs/1601.06748)
- RobustMPC explicitly optimizes quality, rebuffering, and switching over a short horizon using throughput and buffer state. The original evaluation found that ordinary MPC became worse than buffer-based control when prediction error grew beyond about 25%, while its robust variant protected against error by optimizing worst-case QoE. [Yin et al., “A Control-Theoretic Approach for Dynamic Adaptive Video Streaming over HTTP”](https://www.cs.cmu.edu/~xia/resources/Documents/Yin_sigcomm15.pdf)
- Pensieve learned a policy from network traces and reported 12–25% average QoE gains in its evaluated conditions, but it requires a representative simulator/training corpus and a carefully chosen reward. [Mao, Netravali, and Alizadeh, “Neural Adaptive Video Streaming with Pensieve”](https://web.mit.edu/pensieve/content/pensieve-sigcomm17.pdf)
- Fugu's large randomized Internet deployment found sophisticated and learned control hard to make better than a simple buffer-based scheme in the real world; its successful design limited learning to a short-lived, checkable transmission-time predictor feeding classical MPC and trained it in situ. [Yan et al., “Learning in situ: a randomized experiment in video streaming”](https://www.usenix.org/conference/nsdi20/presentation/yan)

For a private server with few clients and no representative global trace corpus, a custom learned policy has high validation cost and little evidence base. Keep hls.js upgradeable as a vendored dependency and configure documented public APIs only.

### Recommended hls.js policy

Instantiate the current player with:

- `startLevel: -1` and `testBandwidth: true`, so it downloads one lowest-level fragment to establish a path estimate rather than starting from the first playlist entry;
- `capLevelToPlayerSize: true`, so Auto does not spend bytes decoding pixels the displayed player cannot use;
- `capLevelOnFPSDrop: true`, so sustained decoder pressure lowers and caps quality; and
- `abrMaxWithRealBitrate: true` as defense in depth while Kinosail transitions from nominal to measured manifest rates.

The hls.js API specifies that the bandwidth test requires `startLevel=-1`; it also documents viewport capping, FPS-drop capping, the ABR safety factors, and conservative quality-switch controls. [hls.js API](https://github.com/video-dev/hls.js/blob/master/docs/API.md)

Do not override the EWMA constants or buffer limits until network-trace measurements prove a problem. Do not use the Network Information API as the primary estimate: its report is not on the W3C standards track, can be unavailable, can describe the first hop rather than the end-to-end media path, and carries fingerprinting/location-inference concerns. At most, treat `saveData` as a user-intent hint where available; measured segment delivery remains authoritative. [WICG Network Information API](https://wicg.github.io/netinfo/)

Use the Media Capabilities API to exclude an optional codec/resolution/profile that the user agent reports unsupported or unlikely to be smooth/power-efficient. These answers describe decode ability, not network capacity, and are advisory. [W3C Media Capabilities](https://www.w3.org/TR/media-capabilities/)

## Startup and on-demand preparation

The ideal startup sequence is:

1. The shared playback operation selects direct Original or adaptive Auto from client capability, viewer policy, connection intent, and explicit user choice.
2. Auto prioritizes the smallest safety rendition, then starts every intended rung under the same durable recipe/cache key.
3. Publish the stable master only after every intended rendition has its initialization section, playlist, and first independently decodable segment. hls.js ordinarily loads the multivariant playlist once, so a partial master would strand that session on the subset available at first fetch.
4. hls.js downloads one small lowest-rung segment as its bandwidth test, and playback begins without waiting for the complete title to encode. Preparing the remainder stays bounded by the existing transcode scheduler.

Do not make a viewer wait for every rendition to finish before returning a playable master, but do wait for the first playable unit of every rendition the master promises. Do not advertise a rendition whose initialization section and first media segment are unavailable. Later sessions can reuse the same complete ladder from cache.

Adding three more encodes naively would multiply CPU/GPU work and contend with playback. Preserve the one-container installation by keeping resource control inside the server:

- reuse one source probe and, when the configured FFmpeg/hardware path supports it reliably, one decode/filter graph feeding several encoders;
- admit only a bounded number of encodes, prioritize the safety/startup rung, and let ready-file serving and direct play bypass the encoder queue;
- cache by immutable source version plus complete recipe, encoder identity/options, ladder version, audio/subtitle choice, and color transform;
- stop preparing unused higher rungs after a grace period when every interested session ends; and
- expose preparation separately from buffering so “Server preparing compatible stream” never looks like a bad network.

## Quality control experience

The player menu should show:

- **Auto (currently 720p)** — default for adaptive playback;
- **Original** — exact source/direct lane when supported, with an honest warning if the current path may buffer; and
- one entry for every actually advertised rendition, ordered by resolution and frame rate.

On hls.js, build the menu from `hls.levels` after `MANIFEST_PARSED`, update the Auto subtitle on `LEVEL_SWITCHED`, and use `hls.nextLevel = index` for a prompt switch that avoids flushing the whole active buffer. Setting `currentLevel` flushes the buffer and can visibly interrupt playback; setting `loadLevel` is conservative but may not become visible until the existing buffer drains. Setting any of these to `-1` restores Auto. [hls.js quality-switch API](https://github.com/video-dev/hls.js/blob/master/docs/API.md#quality-switch-control-api)

Persist only the device-local preference, expressed as `auto`, `original`, or a maximum height/frame-rate target rather than a level index. Level indices and available rungs change per title. If a manual level errors, explain the fallback and return to Auto; do not trap the viewer in a retry loop. Keyboard, screen-reader, touch, and television/remote focus states need the same labels and current-selection feedback.

Native HLS implementations remain responsible for their own automatic ABR. Only promise manual levels on a client where Kinosail can control rendition selection through a supported API. Feature-detect the menu; do not fake control over a native player.

## Local-only QoE telemetry

An incredible experience must be measurable, but a private media server should not create a viewing-history export. Keep a short-retention, local-only per-playback diagnostic stream with coarse fields:

- time from play intent to first rendered frame;
- server preparation wait separately from manifest, segment, decode, and user-gesture wait;
- rebuffer count and duration, fatal/recovered error categories, and time-to-recovery;
- anonymized rung height/rate, time at each rung, automatic/manual switch count, and estimated bandwidth bucket;
- segment duration/bytes/load/TTFB and buffer-seconds buckets;
- dropped-frame ratio and decode-cap activation; and
- transcode queue time, encode speed, cache hit, fallback encoder, and recipe version.

Do not record title, item ID, source path, playlist/segment URL, token, IP address, profile name, or raw fine-grained network trace in the QoE series. Keep session identifiers random and short-lived. Aggregate before durable retention, let an Owner inspect/delete the diagnostics, and never forward them off the Server. If richer traces are needed for a user-authorized bug report, make capture explicit, bounded, previewable, and local-export-only.

This local data is enough to decide whether startup is slow because encoding is late, the path estimate is wrong, the ladder has a gap, the decoder is overloaded, or the user forced an unsustainable level. It is not enough to train Pensieve/Fugu responsibly, which is another reason to defer learned ABR.

## Validation gates

### Encoder and manifest tests

Use synthetic and tiny licensed fixtures spanning 360p/720p/1080p/2160p, 23.976/30/60 fps, portrait and non-square pixels, SDR/HDR, low/high source bitrate, interlaced input, missing audio, alternate audio, text/image subtitles, and odd dimensions. For every generated presentation, assert:

- no emitted rendition exceeds source dimensions/frame rate or the playback-plan bitrate cap;
- no duplicate/dominated rung survives source-aware pruning;
- every variant's segment count, start PTS, duration, and boundary times align within one source-frame duration;
- every video segment begins with an IDR and closed-GOP output, and `EXT-X-INDEPENDENT-SEGMENTS` appears only then;
- the fMP4 initialization segment and media fragments probe successfully;
- declared codecs, resolution, frame rate, color range, average bandwidth, and peak bandwidth equal measured output within Apple's VOD tolerances;
- the master is stable and never advertises an unavailable file; and
- hardware encoders and software fallback honor the same public ladder/manifest contract.

Run Apple's `mediastreamvalidator` where available and retain its report as a test artifact; use `ffprobe`-based assertions everywhere. Apple explicitly supplies `mediastreamvalidator` for HLS conformance, but a validator pass does not replace cross-player playback. [Apple HLS tools and validation guidance](https://developer.apple.com/documentation/http-live-streaming/using-apple-s-http-live-streaming-hls-tools)

### Player and network tests

Automate Chromium, Firefox, WebKit/native HLS where supported, and real iOS/Android/television smoke tests. Replay deterministic bandwidth/latency/loss steps that cover:

- cold start at 250 kbit/s, 1, 3, 8, and 25 Mbit/s;
- sudden bandwidth collapse and recovery;
- periodic oscillation around each rung boundary;
- high RTT with adequate bandwidth;
- brief outage, timeout, retry, and server restart;
- player resize, fullscreen, device-pixel-ratio changes, background/foreground, and sustained FPS drops;
- seek before full preparation, seek across a quality change, audio/subtitle change, watch-room synchronization, and resume position; and
- Auto→manual→Auto, unavailable remembered level, manual-level error fallback, and Original fallback.

Gate on user outcomes, not just HTTP success: startup time, zero avoidable stalls, downshift before buffer exhaustion, bounded oscillation, quality recovery after sustained good bandwidth, no A/V discontinuity, correct selected tracks, and no duplicate transcodes. Compare these metrics to the current two-rung player under the identical traces.

Actual-device, large-file, long-duration, constrained-network, hardware-encoder, and source-replacement tests remain separate release boundaries even after the focused suite passes.

## Recommended implementation order

1. **Make output correct:** introduce a source-aware rendition-plan value behind the existing playback operation; generate a pruned H.264/AAC ladder; force common closed GOP/IDR boundaries; measure output; and publish complete truthful manifests. Add API and web-adapter coverage.
2. **Make Auto excellent:** configure hls.js automatic startup testing plus viewport/FPS caps, order/publish a small safety rung first, keep a stable per-session master, and separate preparation from buffering status.
3. **Give control:** add the accessible Auto/current, Original, and rendition menu using public hls.js switching APIs and device-local semantic preference.
4. **Prove adaptation:** add deterministic trace-driven browser tests, FFmpeg/manifest structural checks, Apple validation where available, and a real-device matrix.
5. **Tune privately:** add local-only coarse QoE diagnostics and adjust the ladder/startup policy only from evidence.
6. **Optimize later:** optional sampled VMAF per-title pruning, additional codec families selected through Media Capabilities, and more sophisticated control only after representative measurements show the maintained hls.js controller is the limiting factor.

## Advanced techniques to defer

| Technique | Why it is not the next Kinosail increment |
| --- | --- |
| Custom BOLA or RobustMPC | Strong research foundations, but hls.js already combines bandwidth, buffer, load-time, and emergency-abandon signals. A fork creates a long-lived player-validation burden before a measured failure exists. |
| Pensieve/Fugu-style learned ABR | Kinosail lacks representative in-situ data at sufficient scale; learning from a few households risks overfitting and conflicts with the privacy boundary. Fugu itself is evidence for caution. |
| Per-shot VMAF optimization | Excellent compression frontier, but far more analysis/encode work than a private single-server default can justify. Start with source-aware and later sampled per-title pruning. |
| Parallel H.264 + HEVC + AV1 ladders | Multiplies encode/cache cost and client combinations. Add a complete second codec family only when hardware, Media Capabilities, native-HLS, and actual-device tests prove it worthwhile. Never require an in-session codec switch. |
| Low-Latency HLS/CMAF parts | Added requests, state, and validation solve glass-to-glass latency, not VOD startup from a local source. |
| Content Steering/CDN logic | Kinosail has one owner-hosted media origin and no relay/cache tier. There is no alternate delivery pathway to steer among. |

The advanced experience for Kinosail is not the most exotic controller. It is an honest, source-aware ladder; seamless independently decodable switches; a conservative startup; clear Auto/manual control; bounded owner-hosted resource use; and evidence that it remains smooth on the viewers' real paths.
