# Indexed MP4 copied AAC recovery transfer

Scope: H264 IDR-indexed remux, retained MP4 source, selected AAC LC stereo 48 kHz with exact 1/48000 packet grid. This is a new producer certificate, not approval of other codecs, arbitrary resumes, MKV, native playback or the fourteen diagnostic holds.

Historical R12/R13/R15 and R16 failures remain immutable. R16 normal-initial plus packet-derived shift proves one fixed CLI mechanism; actual Server/cache/reopen proof is still required.

## Failure matrix written before implementation

- Scoped video preroll must retain AAC packets while excluding earlier video keys.
- First canonical AAC packet must uniquely match a bounded source window by SHA256 and integer PTS. Missing or ambiguous payload identity fails closed.
- Source/canonical AAC codec, profile, selected track, sample rate, channels and timebase must agree. No float or unknown packet clocks are admitted.
- Edited and raw canonical first packet must have equal payload and duration, with equal PTS/DTS edit deltas; raw PTS zero and a bounded edit must explain the actual initial seek.
- Initial edited video clock must be zero; a shared video clock cannot supply an audio origin.
- Actual microsecond seek and mux values require integer rescaling. Fractional carry, negative values, overflow and invalid key DTS must reject or correct exactly.
- Source identity, policy, rooted init/first assets and actual canonical generation must stay unchanged through one inherited two-second lease and owned probe settlement.
- A Version1 cache for an affected indexed request must not be returned as Version2-ready. Other unindexed cold caches, HEVC and encoded-audio generations retain existing behavior.
- Version2 timeline and certificate must agree; missing/modified/ambiguous metadata, rewritten init, stale generation and cancellation cannot publish readiness or overwrite canonical bytes.
- Public refill4 must preserve complete source AAC payloads including 935–938, every edited packet clock, every video payload/frame, canonical init, full native PCM and EOF.
- Public reopen, zero refill, missing-first regeneration and interrupted unindexed cold controls must preserve source/canonical assets and resource limits.
- Browser/native color, phase, audible samples, source offset and all 50 simulator receipts remain separate acceptance boundaries.

## Retained isolation rationale

Public journeys cannot deterministically inject ambiguous packet metadata, integer overflow, a one-tick fractional rescale carry, a metadata inode replacement or certificate version mismatch. Focused tests protect these failures; their success is not public media acceptance. Actual authenticated Server and hosted pinned FFmpeg proof remain mandatory before delivery.
