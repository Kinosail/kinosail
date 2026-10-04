# Native playback response compatibility — 2026-10-04

## Failure analysis before implementation

The reported iPhone player shows 1917, `Playback interrupted`, and
`Could not read the Server’s response. Try again.` The screenshot build is unknown.
The installed build 2913 was built from `7ad46ad1993c3b48d4bb1f5cae1c5fb37faa035c`.
Its current normal launch succeeds; the earlier OS launch rejection is separate.

The exact message is `ClientError.invalidResponse`. Playback responses always
include `policy` in the authoritative Go handler. The installed native decoder's
strict top-level allowlist omits it. Subtitle policy fields are already supported.
An otherwise valid response therefore fails before source selection or AVPlayer.

Failure modes to verify before changing production code:

1. A response emitted by the real local Go handler fails native decoding.
2. Accepting policy must retain strict rejection of unknown fields and duplicates.
3. Policy values must be bounded strings from automatic, direct, or compatible.
4. Omitted policy remains compatible with older responses; malformed values fail.
5. Media identity, same-origin URL and subtitle constraints remain enforced.
6. A rejected response must cause no subsequent media GET.

## Verification design

Run `python3 apps/player/scripts/native-playback-contract/run.py <output-directory>`
on macOS with Xcode, Go, FFmpeg and FFprobe installed. It generates a bounded
fictional MP4, starts the actual Player server on loopback, and compiles the
unchanged production Swift Core sources. The Swift executable reads the populated
library and playback endpoint over HTTP and invokes the production decoder.

The negative mutations protect this concrete contract failure. This is an
isolation exception for rejection paths: a populated UI cannot supply malformed
server fields deterministically. The positive response comes from the real
handler and FFprobe, rather than a hand-written playback schema. The artifact
contains exact revision, commands, fixture and source hashes, and per-case results.
The executable fetches media only after successful response validation.

This verifies the server/native response boundary. It does not certify physical
iPhone or Apple TV decoding, 1917, HDR, Atmos, Siri Remote, or deployed Nox traffic.
No live request or device installation is part of this reproduction.

## Before and after

Before production changes at source7ad46ad19, the actual server response fails
with `invalidResponse`. Removing only `policy` makes it decode. All three valid
policy values fail; no media is fetched. The baseline passes12/16 cases.

The minimal repair accepts and validates optional policy in both source decoding
and the subtitle-policy read. It does not override the validated playback plan.
The same journey passes16/16 cases, then fetches the real fictional media.
Unknown/duplicate fields, malformed policy and subtitle values, wrong media
identity and foreign origins remain rejected. Companion JSON retains each result,
source hashes and exact response checksums; local receipts retain commands.
