# Bounded startup preparation: implementation and evidence

PR: https://github.com/Kinosail/kinosail/pull/452

Production revision reviewed independently: `42d7679f9e8b5eb5c8898213f11437423d77f018`.
Full media acceptance revision: `848130ba3e437c1811b9d2bd7d1d77589c313c65`.
Changes between these revisions only stabilize queue admission and preserve E2E measurements.

## Behavior

Idle browsing can prepare the first eight seconds of the actual compatible stream at the saved resume offset.
Preparation reuses the existing validated recipe, source, subtitle, settings and cache identity.
Playback adopts matching work and keeps the same init, encoder binding, timestamps and segment numbering.
Missing later segments use the existing seekable cache continuation path.

The authenticated preparation API rejects unrelated sources, query strings and invalid recipes before encoding.
It rechecks viewer access before queued work starts. The browser respects Direct First and decoder evidence.
Direct-compatible content receives metadata and bounded range reads instead of video conversion.
The capability helper preserves Apple's native HLS choice and selects Chrome's supported MSE adapter.
Current Play/Pause intent and saved resume survive delayed source attachment.
Content-hashed theme URLs deliver preparation code to browsers with old immutable assets.

Preparation has one worker, three queued entries, a 30-second queue lifetime and a 12-second work deadline.
It waits for idle encoding capacity and yields to playback, direct responses and active playback observations.
Matching playback marks the encoder adopted before speculative cancellation can stop it.
Speculative encoding uses background scheduling, bounded thread settings and an input read rate of four.
The scheduler reserves cache headroom under the existing configured cache limit.
It limits unused preparation data to 256 MiB and monitors each window against a 64 MiB budget.
Monitoring is periodic; the byte budget is not an exact filesystem quota.
New cache reads and marker writes use `os.Root` under the configured cache directory.
Existing cache eviction policy remains authoritative.

## Repeatable media acceptance

Command: `GOMAXPROCS=2 python3 apps/player/scripts/test-startup-local.py`.
Artifact ID: `.verification/startup/20261003T200402Z`.
The runner records revision, clean diff hash, binary/media hashes, commands, fixture metadata and environment.
It retains browser traces, server logs, decoded continuation media, metrics and `SHA256SUMS` privately.
Configuration and backups contain disposable credentials and are excluded from the checksum projection.

The fixture is 64 seconds of moving 720p/24 fps HEVC Main 10, PQ, BT.2020 and EAC3 audio.
A separate H.264/AAC MP4 provides the Direct First control.
The test uses real Chrome on macOS ARM64 and supported authenticated HTTP loopback.
No movie data, production deployment, container images or TLS bypass were used locally.

The measurement starts before the visibility assertion and automated click on the watch link.
It ends at the second video-frame callback with advancing media time in the new document.
It includes Playwright click overhead, navigation and attachment. Document-relative timings are retained separately.

| Journey | Click to moving frame | Cache evidence |
| --- | ---: | --- |
| Cold compatible playback | 1,407.8 ms | cold master requests |
| Prepared compatible playback | 278.0 ms | warm master requests |
| Direct First MP4 | 175.3 ms | direct media; no HLS cache directory |
| Adoption of running preparation | 304.4 ms | active encoder adopted; init unchanged |

The paired cold/prepared result improves by 80.3%, or 1.130 seconds, in this run.
These two journeys copy HEVC video and convert EAC3 audio to AAC in the compatible stream.
Full software H.264 conversion and continuation passed separately; their startup latency was not separately benchmarked.
Idle preparation took 1.828 seconds before the warm click; that cost is separate from playback startup.
This single paired run is not a latency distribution or a production performance guarantee.
Earlier document-navigation experiments are diagnostic evidence and are not included in these click measurements.

The complete journey passed in 59.3 seconds. It verified:

- Saved resume at 12.3 seconds, playback beyond 25 seconds, stable init hashes and no media error.
- Caption cues, a seek past 42 seconds, continued moving media and retained position after reload.
- Adoption followed by playback beyond 14 seconds without replacing init data.
- Competing preparation stays queued while actual playback is active.
- Three distinct pending recipes are accepted, a fourth receives 429, and DELETE releases pending work.
- Unauthenticated and malformed requests leave cache directories unchanged.
- Source and transcoder settings changes invalidate prepared cache identity.
- Software video conversion can be cancelled, prepared again and continued with the original init.
- Six fragments spanning the prepared window decode with the original init using FFmpeg `-xerror`.
- New startup lifecycle logging is structured; the rejected query token is absent from server logs.
- A browser retaining the old immutable theme URL receives the new content-hashed bundle.

Final whole-run resource receipt: 276 samples, one FFmpeg encoder, one speculative encoder,
260.3% peak aggregate sampled process CPU and 688,400 KiB peak aggregate RSS.
The browser's earlier snapshot contains 222 samples and slightly lower peaks; the final receipt is authoritative.
Final cache size was 31,551,741 bytes. These observations establish this fixture's resource use, not a universal CPU ceiling.

## Other verification and boundaries

The delayed-adapter Pause E2E reproduced the defect before its repair and passed afterward.
The repaired run passed 19 Apple launch, touch, native seek and playback-intent browser checks.
Artifact ID: `.verification/startup-intent-after`.
Required hosted CI passed on the full acceptance revision:
https://github.com/Kinosail/kinosail/actions/runs/37150196496
This includes Player/Subtitles Go, populated Chromium, production container, lint and security checks.
Task-related CodeQL path findings were repaired with rooted cache access; the findings policy passed.

The host FFmpeg lacks the subtitles filter, so burn-in subtitle-version invalidation was explicitly not run.
Ordinary external caption rendering passed. Physical Apple devices, Nox, production networks,
long completed caches, full-size 4K movie workloads, and permission revocation during queued work remain separate verification boundaries.
Synthetic codec similarity does not establish equivalent movie performance.
