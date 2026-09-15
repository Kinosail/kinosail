# Playback startup and recovery implementation

Date: 2026-09-08. Scope: Player web, Player API, and native Player clients.

This implements source-level improvements from the playback architecture review. It does not certify startup targets, codec/HDR coverage, battery efficiency, or a competitive ranking. `.gates-disabled` remains present: no test suites, builds, device benchmarks, or browser gates were run for this work.

## Delivered behavior

- Native playback resolves saved audio-processing requirements and known incompatible Apple containers before constructing a player. MP4 downloads use their actual MIME type instead of inheriting an original MKV container. Unknown formats retain the platform path.
- An explicit decoder error may open the local original-file engine once. Network, authorization, cancellation, and unknown errors do not automatically change engines. The recovery panel explains access/connection failures without offering a decoder switch or conversion. Unknown/decode failures offer local playback and explicit Server compatibility separately.
- Reading local progress runs alongside item/source requests. Live playback preferences do not wait for local cache reads or writes. Successful Server preferences are not replaced with defaults after a storage-write failure.
- Native recovery negotiation queries hardware codec support and HDR eligibility. Android checks installed decoders; Apple checks VideoToolbox. The API validates optional `hdrFormats` and `maxAudioChannels` before probing media. Original delivery, viewer permissions, and conversion authorization are unchanged. A native source selected from the original URL is labeled direct even if the Server preferred a rendition.
- Web compatibility loads hls.js alongside capability negotiation. Detection has a 1.5-second budget; adapter loading has a five-second budget. Invalid, oversized, stale, and cross-origin negotiation responses cannot replace the approved stream. This work starts only when compatible playback is selected.
- The Apple original-file gateway uses one ephemeral URLSession per authorized source so range requests can share connections. Cancelled ranges release capacity immediately. Four-transfer limits, backpressure, credential isolation, redirect rejection, and no persistent media cache remain in place.
- Browser traces distinguish the first presented frame from advancing presentation and ignore stale seek frames. Native samples record first-frame and first-timeline-progress separately, completed/incomplete app-initiated seeks, and buffering observations. Missing timing values remain null. App-controlled local replays reset track state and start a fresh measurement without reopening the source gateway.

Codec-family detection is a negotiation hint, not proof of every profile, bit depth, resolution, Dolby Vision variant, or audio output route. Android recovery remains stereo because installed decoders and connected outputs do not establish the active media route. Dolby Vision and encoded passthrough are not inferred from HDR10/HLG or channel counts. These distinctions follow the platform contracts: [Android decoder capabilities](https://developer.android.com/reference/android/media/MediaCodecInfo), [Apple HDR eligibility](https://developer.apple.com/documentation/avfoundation/avplayer/eligibleforhdrplayback).

## Collecting comparable evidence

Export the native `playbackMetrics()` result from a development session after closing the playback attempts. The session history retains 100 attempts, and each attempt retains its latest 128 completed app-initiated seek timings; collect batches before those bounds are reached. Do not concatenate duplicate exports. Wrap batches from one matched fixture/device/network/cache/revision cohort as follows:

```json
{
  "cohort": {
    "device": "device-model",
    "os": "os-version",
    "network": "lan",
    "outputRoute": "internal-speaker",
    "cache": "warm",
    "fixtureSha256": "replace-with-the-64-character-fixture-sha256",
    "appRevision": "revision",
    "serverRevision": "revision"
  },
  "samples": []
}
```

This is a shape example; empty samples and the placeholder hash are intentionally not valid measurements. Once gates are enabled, summarize a populated file with `python3 scripts/summarize_playback.py /path/to/cohort.json` from `apps/player`. The tool rejects unknown fields, duplicate JSON keys, missing/invalid values, oversized input, and inconsistent durations/counts before emitting a report. It includes failures and missing first frames alongside distributions. Under 200 observations for a metric, p95 is flagged as screening only; 200 samples are still not a substitute for confidence intervals or repeated sessions.

Native timeline progress is not seek-to-picture, audible-output, or A/V-sync proof. Expo's native controls can perform seeks outside the app's callbacks. Native buffering observations cannot resolve every pause-during-buffering edge case exposed by the platform wrapper. Use device instrumentation for all native-control seeks, frame drops, A/V sync, energy, thermal behavior, HDR presentation, AirPlay, PiP, and display/audio route changes. Browser frame traces and native timeline measurements must not be pooled as the same metric.

## Remaining experiments

Retain current buffer defaults and two-second fMP4 HLS packaging until the existing populated startup, seeking, and bandwidth suites plus real devices establish a better setting. Compare original bytes and output fidelity, cold and warm paths, LAN/WAN latency and loss, rapid scrubbing, and 30-minute sustained sessions. Use the existing `instant-playback.spec.ts`, `playback-startup.spec.ts`, `player-seeking.spec.ts`, and `playback-bandwidth.spec.ts` and record the exact fixture hash, device/OS, engine, application and Server revisions, output route, network condition, and cache state for each run. Do not claim energy or codec certification from mocks or simulator execution.

For transport, compare HTTP/2 with HTTP/3 only where the actual client, origin/proxy, and connection metrics prove the negotiated protocol. URLSession connection reuse is delivered; an HTTP/3 server rollout is not. QUIC can change transport behavior, but does not remove media probing, decode, buffering, or rendering costs. Media over QUIC remains a separate live/interactive experiment; this change does not add a bespoke wire protocol. [QUIC specification](https://www.rfc-editor.org/info/rfc9000/), [MoQ working-group scope](https://datatracker.ietf.org/doc/charter-ietf-moq/).

## Verification ownership and commands

The native owner runs the focused engine/capability/preferences/measurement and playback-selection tests, the native suite, the Swift package tests, and Apple/Android builds after gates are enabled. New native modules require rebuilding the app; a JavaScript refresh is insufficient. The API owner runs `go test ./packages/playback ./apps/player/internal/server`; the web owner runs the affected Playwright fixtures and populated playback suites. Run `python3 -m unittest discover -s scripts -p test_summarize_playback.py` from `apps/player` for the report tool, then the affected app's `make verify-changed` and root `make max-loc` as required by repository policy. These are future verification instructions, not passed checks.
