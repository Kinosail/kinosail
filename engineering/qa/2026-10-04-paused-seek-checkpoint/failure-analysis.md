# Paused seek and page-exit checkpoints

Failure analysis recorded before changing production code. Start revision:
`ee567e9b9d6b4dc4211055e1f2ef968c5ce63209`.

The actual authenticated Simulator Safari sample on owner-reported Nox
`5f91114d16e7c049e0da1d9c119653a6e8161b90` exposed a paused seek followed by
Library navigation and reentry near the prior pause checkpoint. This campaign
keeps that live observation separate from local public-Server reproduction.

Affected interface: the shared browser asset POSTs validated session/revision
checkpoints to the existing `/progress/{item}` endpoint. No API contract changes.

Concrete failure paths:

1. A completed seek while already paused emits no new pause event. The periodic
   timer skips paused media. Deployed source also has no page-exit checkpoint.
2. Current main registers presentation teardown before progress pagehide. Source
   removal and `load()` clear metadata before the checkpoint's metadata guard.
3. A queued teardown pause can subsequently read reset zero or an old native-HLS
   timeline origin and overwrite a newer checkpoint.
4. An unload checkpoint must dispatch immediately when an older save is in flight.
   Late responses must not clear a replacement sender or its failure notice.
5. Completed watched state must retain precedence over seek/pause/unload events.
   Existing Retry, explicit Continue, and awaited audio-queue drain must survive.
6. Preparation, Viewer Profile, item, cast and offline ownership must remain scoped.
   A seek without valid metadata must not invent successful saved progress.
7. Initial metadata resume and decoder/source-switch seeks must not overwrite a
   completed watched state or report an unloaded decoder position.

The new populated journey uses the real Go Server, generated MP4, actual decoder,
existing progress API and normal Library navigation. It does not stub currentTime,
readyState, pause, load, or progress responses. Baseline replay replaces only the
progress asset with pinned deployed or current-main bytes on that same Server;
this proves browser behavior, not a historical deployed binary or Safari runtime.
Existing isolated suites retain their documented HTTP/ownership race coverage.

Acceptance: seek while staying paused, verify the real stored checkpoint before
leaving, reopen the title and verify resumed position and advancing frames. A
second journey leaves during playback between periodic checkpoints, verifies the
pre-teardown position persisted, and rejects reset-position clobbering. Preserve
safe receipts, exact revisions, commands, generated data and artifact checksums.

Post-merge Simulator Safari retest requires separately authorized deployment.
No manual Nox deployment is part of this repair.
