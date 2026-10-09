# ARM publication pull/scan repair preparation

## Initial unreferenced preparation record

This unreferenced preparation uses main eabd600167bba53b0fe7fab2b3afba1901b9da1c.
No existing checkout/ref is changed and no competing change is published.
The existing owner's identity/release remains unresolved.

Main CI37552317158 and earlier37547993270 reproduce image initialization
failure before the same immutable ARM candidate finishes pulling. Runtime
tests and both AMD64 verifications pass; promotion and tags are skipped.

Before writing tests or repair code, the failure cases are:

- Scanning starts before Docker has the immutable candidate locally.
- Pull fails but runtime/scan or digest export proceeds.
- A mutable tag, wrong repository/app, malformed digest, or unexpected argument
  reaches Docker before validation.
- Publication is fixed but release retains the same ordering race.
- The pre-pull is conditional, allowed to fail, or uses a different candidate.
- Runtime testing bypasses validation or tests a different image after pre-pull.
- Runtime or scan failure no longer blocks digest export/promotion.
- Scan thresholds, pinned actions, signatures or existing scanner holds change.
- The repair changes qualified Restore or intervening playback source.

Real hosted ARM publication is the final acceptance mechanism. Local checks
must not be described as container/E2E/publication acceptance. A lightweight
isolated shell check is necessary because browser journeys cannot prove that
invalid publication inputs cause zero Docker calls, nor that a failed pull
prevents starting a container test. Static workflow contracts protect ordering
and unchanged promotion prerequisites that a browser journey cannot observe.

Write these regression checks first and retain their baseline failure. Then
introduce the smallest shared validated pull and a foreground immutable-candidate
pull before each existing runtime/scan parallel group. Keep both verifiers and
digest export unchanged. Heavy local builds, cleanup, credentials, manual
deployment and source publication remain outside this preparation.

## Recovery publication disposition

The coordinator subsequently authorized task01a11221-d2d7-7711-a9f1-d380830b0020
to publish a separate recovery draft after fresh remote inventory. The24 remote
branches, eight open PRs and current CI-path history contained no competing
repair on these patch paths. Main remained the exact preparation base above.
Original owner checkout/ref and initial preparation evidence remain untouched.
This recovery changes no scanner policy. Hosted exact-head gates and exact-main
ARM publication remain required before delivery or closing original PR485.
