# Progress acknowledgement, reentry and source ownership

Before production edits, investigate the existing restart receipts through supported public interfaces. The historical HTTP 204 responses do not establish the accepted durable Viewer Profile/item/session/revision record. Use disposable data; do not access the database directly or change authentication persistence.

The web form endpoint intentionally returns204 when an ordered same-session update is ignored. The canonical PUT progress API returns409 for the same condition. A discarded update is distinct from a persistence failure. Accepted changes persist before the memory projection is published.

Failure modes before implementation:

- An old or duplicate revision is ignored while the web client clears its pending notice on 204.
- The checkpoint and reentry read different item/profile records.
- A late closing write or new-page zero write overwrites the accepted checkpoint.
- Saved source position is projected incorrectly through a marker timeline.
- Correct projected start produces a zero stream origin because source duration or selection is unavailable.
- An old decoder time/duration is combined with a newly selected stream offset.
- A stale generation callback changes the current selection or progress.
- Metadata arrives before resume selection; position is restored twice or rejected by a short advertised duration.
- A native seek clock mismatch conceals successful source seeking.
- Recovery jumps to a later buffer start, overrides Pause or retries cancelled media.
- Static cards or black transitions are classified as stalled decoded playback.

Start by extending the existing populated checkpoint journey to distinguish ignored updates from accepted/readable progress and to follow accepted seek 22 through Browser Back and Library reentry. Keep response status, session/profile/item matches, revision and decoded position in the receipt; exclude credentials, raw URLs and request bodies. Use a 70-second synthetic H264 clip locally so seek 22 is outside the final 10-second restart region. The hosted 30-second fixture uses a target within half its duration.

The existing native-HLS seeking fixture may cover the source-offset transition gap. Any retained isolated case must name the precise old-decoder/new-offset failure which a Chromium populated journey cannot force. Do not claim native decoder or physical-device proof from this fixture. Add the failing regression before changing behavior.

The populated public chain passed six executions. An ignored web update returned 204 while the accepted session/revision/position stayed unchanged. The canonical API rejected it with 409. Accepted seek 22 survived both Browser Back and Library navigation through decoded reentry. This does not establish the cause of the historical live zero starts.

Before production edits, two same-origin native-HLS fixture cases failed on unchanged source e85544ebc. Unknown full duration dropped saved 22 and selected an unshifted stream. Replacing a source at 22 posted 44 while the old decoder duration 70 became 92. The fixture retains old metadata and queues Pause and seek completion around replacement. New metadata must release the requested position, so later decoder 1.25 projects 23.25. Invalid unknown-duration inputs must not create offset recipes. These isolated cases protect native event ordering that a populated Chromium decoder cannot force; they do not prove decoded HLS or physical-device behavior.

The existing native-seek fixture exposed two implementation regressions before delivery. Holding the source-seek guard on an initial load blocked a later seek. Restricting that hold to resumed replacements preserves the existing cold-load flow. A superseded initial metadata listener could clear the new requested position before the active resume listener, causing an already-buffered frame to rewind. The native cleanup now checks its existing adaptive generation. Offsets use the same inclusive 604800-second limit as HLSRecipe; exact-boundary and just-over inputs share the existing bounds case. No new unit tests or backend changes are needed.

Task7 repairs only player.js and player-streaming-adaptive.js. Open PR 490 changes backend fixture/workflow files. PR 479's player.js change concerns direct source inference and does not overlap these getter/seek/source-offset lines. A relay request remains in the parent checkpoint because this task has no outbound thread tool. Backend HLS/startup Go paths, encoder/remux/cache/manifests remain with owner01a102e0. Server and shared catalog storage are read-only diagnosis targets. Preserve the signed-in simulator and frozen 50 artifacts. No Nox or Library writes.
