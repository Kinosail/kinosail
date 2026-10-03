# Startup preparation: failures and acceptance criteria

Written before implementation. Base: PR 449, 455749bcccf4b5506cb4735994e27cc9ff25e690.

Preparation must generate the actual playback stream, never a preview clip.
The public interface is an authenticated, bounded POST to an item's playback preparation route.
Continue Watching and focused title links may request preparation during idle browsing.
Direct First remains authoritative: direct-compatible content receives metadata and bounded range reads only.

## Failure modes

- Unauthenticated, restricted-library, stream-disabled or transcode-disabled viewers cause expensive work.
- Malformed paths, recipes, offsets or bodies cause work before rejection.
- Preparation encodes a different resume offset, rendition, codec, HDR mode, subtitle, audio track or effect.
- Changed media, subtitles, settings or cache versions reuse stale fragments.
- Cancellation removes cache while a real viewer uses it, or leaves partial output marked complete.
- An encoder continuation replaces init data or changes timestamps, keyframes or segment numbers.
- Background work consumes encoding capacity needed for real playback.
- Browser focus changes create an unbounded queue or preparation encodes whole titles/library content.
- A direct-compatible file is transcoded without compatibility or decode evidence.
- Cache cleanup crosses configured roots, removes active streams or bypasses existing permissions/policy.
- Disk/CPU use grows without limits, or an unsuccessful preparation prevents normal cold playback.
- URLs, paths, tokens or credentials enter new metrics/artifacts.

## Verification planned before code

Use a repeatable host-only HTTP loopback E2E runner with synthetic moving HEVC Main 10/PQ/EAC3 media and direct H.264/AAC controls. No movie export, Nox mutation, container images or TLS bypass.

Record revision/diff/binary hashes, exact commands, fixture metadata, environment, browser trace, server logs and checksums. Measure cold and warm intent-to-first-moving-frame separately from preparation time. Decode fragments spanning the prepared window and lazy continuation with the original init. Verify resume, seek, reload, captions, cancellation, adoption and competing playback. Reject invalid and unauthenticated requests with no cache side effects. Verify source/settings/subtitle invalidation and queue/process/disk bounds.

Physical iPhone, Nox hardware and production networks are separate boundaries; synthetic codec similarity is not equivalent workload proof. Do not claim instant or universal startup.

Independent review found a delayed-attachment failure before its repair:
an explicit Pause can be overridden by the original autoplay attributes.
The browser E2E at `9c81a65b4` reproduces the failure with a delayed adapter,
keyboard activation of the real Pause control, and replacement decoder metadata.
Current play intent must remain authoritative through attachment.
