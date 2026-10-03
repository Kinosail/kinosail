# Startup cache boundary security followup

## Failure analysis recorded before production changes

Main `5fc3b76d4b9b1cd9775ce9d913ad2a4f894d1e2c` failed its CodeQL findings gate.
Go analysis `1886892443` reports 29 high `go/path-injection` alerts, numbered 70–98.
Its 116 paths carry preparation JSON `source` through the codec or burn field into cache filesystem operations.
The parser already rejects unknown values, but retains the original request strings after validation.
Convert accepted fields into server-owned constants before they enter an HLS recipe.
Retain the existing scoped startup cache roots and security gates.

The repair must preserve these properties:

- Reject unknown, unsupported, automatic, mixed-case, path-bearing, encoded, and control-character enum values before cache work.
- Preserve the supported codec catalog: H.264, HEVC, AV1, and VP9. Never map an unknown value to H.264.
- Preserve `none`, `text`, `image`, and `external` burn modes and their exact existing tokens.
- Preserve the default H.264 legacy cache identity and every accepted recipe's canonical token.
- Preserve resume offsets, subtitle identities, rendition paths, source/settings versions, and original stream initialization.
- Rejected authenticated preparation requests must leave the cache directory inventory unchanged.
- Preserve authentication, authorization, playback adoption, cancellation, and workload bounds.
- Require fresh CodeQL and findings-policy success on the followup PR and fetched main. Do not dismiss alerts or alter the gate.

The codec lookup mirrors the supported catalog. A pre-change regression checks that every supported catalog entry remains accepted.
Existing recipe regressions cover legacy cache identity, timeline and effects fields. Public synthetic-media E2E covers rejection and stream continuity.
Actual burn-in media remains an environment boundary: this host's FFmpeg has no subtitles filter.

## Evidence

The pre-change supported-catalog/cache-identity and invalid-enum regressions passed (0.310 seconds).
The repaired playback package passed (1.092 seconds), including its real loopback HTTP diagnostics.
All 50 CI contract tests, workflow validation, actionlint, source caps, and diff checks passed.
Independent source review classified all 116 flows and found no actionable issue in the enum repair.

The public startup journey now checks 21 invalid codec/burn requests and unchanged cache inventory.
Player's hosted Chromium job runs the existing serial synthetic-media journey against a fresh source build.
It records the actual host platform and retains only safe receipts, resources, checksums, and measurements.
Authentication/configuration files, backups, raw browser traces, and logs are excluded from that upload.

Host free space fell below 1 GiB. The parent explicitly held further local Player builds and app verification.
The prior startup media artifact remains preserved. No fresh local media or app-gate success is claimed.
Exact-source hosted public E2E, CodeQL, findings policy, and fetched-main verification remain pending.
