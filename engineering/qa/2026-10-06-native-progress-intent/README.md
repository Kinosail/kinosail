# Native progress intent regression proof

Test-first baseline: PR479 head6cd20174578d4a4bef3592875d01922430f61281. This test-only task does not repair production files owned by PR479.

`apps/player/e2e/player-native-intent-regression.spec.ts` verifies the public checkpoint contract across three concrete failures: unplayed metadata/Library exit, automatic Apple inline preparation completion, and first native seek during unfinished automatic source restoration. The failure inventory was written before this spec or any production change.

The existing deployed e30 signed-in Safari journey observed saved79.270→opening restart→stored16.640, then a never-played catalog-card Watch/Library visit changed16.640→0.000001. These controlled checks supplement that live proof. They use actual production listeners and HTTP checkpoint requests, replacing decoder clocks/events and HTTP responses for reproducible ordering. They do not establish real decoded media, Apple native UI or physical-device behavior.

Isolation is required because a real populated E2E cannot deterministically deliver the pending preparation's queued `playing` or interleave a native seek before a managed restore completes. The controlled unplayed case intentionally supplies a settled zero decoder clock while synthetic saved progress is20; it proves that an unplayed page cannot overwrite a record, rather than attributing the live rendered `data-start` to one cause. Reads use supported GET `/api/v1/items/movie` with the canonical item/progress envelope; writes use the real browser POST `/progress/movie` shape.

From `apps/player/e2e`, with installed browsers/dependencies:

```sh
KINOSAIL_BROWSER_PROJECT=webkit KINOSAIL_BROWSER_WORKERS=1 KINOSAIL_E2E_VIDEO=off pnpm test player-native-intent-regression.spec.ts --project=webkit --workers=1 --retries=0
```

For exact old-source replay, set `KINOSAIL_REGRESSION_PLAYER_SOURCE` to a private immutable previously verified assembled e30 script, select Chromium, and use `--grep='unplayed Watch metadata'`. An unrecognized source or script error is not a valid RED. The baseline receipt is recorded in `baseline-results.json`; full private JSON/HTML/trace/image artifacts are retained in task7/pr479-intent-regression-evidence.

Independent review required fixture corrections before accepting the final RED: queued preparation `playing` now precedes managed restore `seeked`, metadata-ready native source and seekable range are coherent, and readback uses the supported API route/envelope. The original exploratory runs remain separately retained. No production repair was written here, and no live record was changed by this regression runner.

The owner must run this spec GREEN on its exact repair candidate, obtain independent PR492 offset-safeguard review, and satisfy required protected checks before merge. This deliberately failing baseline is a review handoff, not a GREEN delivery. Broad49-title live acceptance waits for repair; no Nox rollout is implied.
