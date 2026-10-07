# Native progress intent test adoption

This test-only successor preserves the native intent regression spec from PR479
head `9fe7f86ad11a62dd8f759edb40810d82b69e92df`. Its original test blob is
`e7416924608149e842f083181782a7be383e1f16`. The source is copied unchanged onto
qualified main `589264a08c296ed5947b9cf189cd4e1f474b75c8`.

The original failure inventory preceded the regression code. This adoption adds
no production repair. The following failures justify retaining the controlled
test alongside the populated browser journeys:

1. Metadata and Library departure can write decoder zero before a viewer plays
   or seeks, replacing positive saved progress.
2. A queued automatic preparation event can grant played intent after the
   preparation owner has paused and restored the source.
3. An unfinished managed restore can suppress the first explicit paused native
   seek, including zero.
4. A fractional or rounded automatic restore can be mistaken for user intent.
5. A replacement that cancels the old pending seek can retain stale intent or
   discard the replacement's first explicit seek.

Real-media E2E cannot deterministically schedule those decoder event orderings.
The fixture keeps shipping listeners and public progress request shapes while
controlling synthetic clocks, media events and HTTP responses. Script errors
reject the result. It is isolated browser coverage; it does not prove decoded
media, audible audio, fullscreen, physical devices or deployed acceptance.

Quick Player browser checks run the unchanged spec separately with one worker
and zero retries after the existing smoke selection. Its artifacts use separate
output directories. Complete browser selection already includes the spec and
does not repeat it through the quick-only command. Existing journeys, timeouts,
scanner policy, workflow permissions and production behavior remain unchanged.

Current-source GREEN requires the hosted result for the exact successor head;
the old historical results do not establish that result. PR503 retains the
remaining SDK/progress/download/cache changes and its security hold. Original
PR479 and its three historical QA documents, plus the separate intent proof
branch, remain preserved. This document publishes no private capture diagnostics
or historical raw receipts.
