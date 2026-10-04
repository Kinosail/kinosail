# Apple compatible-playback failure analysis

## Observed public failure

The supplied iPhone Safari screenshot shows Raya and the Last Dragon (2021)
with `Playback could not start. Try Play again.`, `Retry playback`, and
`Playback interrupted`. The screenshot was read as pixels, not just OCR.
Its SHA256 is `8d7d7d8554880913aae4edd3b23149f2bf8336fde60ed2a91993897f56e2d95f`.
Last reported production revision is `9d1b158e1a49fd523fb61a3603faafcfa45c1aeb`.
Investigation starts on fetched main `5ae30d9dc`, with no production changes.

The first message is the Apple Play-promise rejection handler. The interruption
state is a compatible-playback recovery state. Neither establishes the root
cause, HTTP outcome, codec, or device decoder failure by itself.

## Failure modes to distinguish before implementation

- Native HLS transport rejects a playlist/fragment because authentication,
  source identity, cache recipe, gateway reachability, or encoding failed.
- The compatible plan copies video that Safari cannot decode, or advertises an
  incompatible codec/profile/HDR/sample-entry combination.
- Asynchronous codec negotiation changes the source after a Play gesture and
  aborts its pending promise; stale rejection pauses the replacement stream.
- A recoverable native media error retries the same unsupported source without
  selecting an allowed working rendition.
- Fullscreen or Play lacks a current gesture, or a stale callback overrides a
  later explicit pause, fresh gesture, navigation, or source generation.
- Resume/timeline conversion seeks outside available native HLS ranges.
- UI listeners or stale immutable assets disagree with the server revision.

## Verification boundaries and required evidence

Correlate bounded title-specific codec/plan data, safe encoder/request failures,
and current-session playback error/rejection events. Reproduce the identified
failure at its public browser seam before changing production behavior. Keep
authentication, TLS validation, Direct First policy, and active-playback priority.
Do not export movie bytes, credentials, raw URLs, or raw logs. Use synthetic media
for regression work. Retain revision/command/data/environment/result receipts.

After a fix, rerun the reproduction, required gates, and a recorded selection of
varied real titles through the authorized UI owner. Separate simulated Apple API,
hosted browser, production transport, and physical iPhone evidence. A few passing
titles do not prove universal playback support.

## First production correlation and reproduced lifecycle defect

The deployment owner reported a healthy exact `9d1b158e` Player. Raya is MKV,
H.264 High, 1920x804, 8-bit SDR, 23.976 fps, with DTS-HD MA English 7.1 audio
at 48 kHz. The native HLS recipe was audio-transcode with no accelerator.
The matching attempt logged HLS start at 01:48:09.307 UTC, a Play rejection
and media error 4 after about 20 seconds, then playlist readiness timeout and
HTTP 503 after 30 seconds. No completion/failure appeared in the bounded window.

`HLS transcode started` precedes workload acquisition and process launch, so
these logs do not distinguish queue wait, input/encoding delay, or readiness
rejection. Cold master publication requires init plus one physical segment;
it does not wait for the speculative eight-second window. Additional safe
workload/process/cache-state evidence is required before fixing that delay.

Separately, the test-first Apple lifecycle reproduction showed an obsolete
Play `AbortError` pausing a newer successful Play and reporting a failure after
explicit fullscreen dismissal. Request/source ownership now ignores those
obsolete rejections while retaining current failures and fresh-gesture policy.
The Apple rejection label was absent from safe diagnostics; its named error
classes are now preserved, while unknown classes and extra fields remain redacted.

Before production edits, the newer-Play journey failed; the corrected immediate
dismissal-feedback check failed separately. Named Apple diagnostic positives
also failed. After the repair, all 11 focused Apple launch/intent/pause journeys
and the focused TraceHTTP success/privacy cases passed. These use a headless
browser and simulated media/Apple APIs; they do not prove Raya playback works.
Same-URL reload without a new intent remains outside the ownership regression.
