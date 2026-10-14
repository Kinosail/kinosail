# Indexed HLS completion evidence

PR #595 run `38520041411`, job `115708078217`, fails before the damaged-cache recovery operation.
`damagedCompletedIndexedCache` reports `indexed_not_complete` after the unchanged 15-second completion deadline.
The test has not damaged the map or reopened playback at that point.
The two cold-cache cases and the unknown-field and duplicate-field recovery cases pass.
All five cases preserve their synthetic source; recorded encoder lifecycles finish with no active workers.

Artifact `11874295755` has SHA-256 `804d34ba227410840af2ab6c557e8136a8dbc483e977d6a2ee828975c417e20d`.
The receipt SHA-256 is `c20010f08c355b760b3e5fa02134ab851a5b33e47546d15daf19ff7041dc4693`.
Its revision is `2d0bba2abf7077ba57deb28096b8556696bb764a`, the verified PR checkout for branch head `11a9a696`.
All five pinned harness sources match the earlier passing deep revision `b57e3313`.
Raw process logs and the final private playlist were not uploaded, so the cause remains unresolved.

## Evidence repair plan

Before editing the harness, identify the missing observations:

- Record the current stage before preparation and completion checks.
- Record worker count, bounded playlist facts, and map/startup marker presence at the completion deadline.
- Distinguish completed, paused, and failed encoders using fixed counters from private structured logs.
- Preserve the original failure if diagnostic collection also fails; expose only its exception class.
- Keep the deadline, damaged-map operation, decoded-frame assertions, and result gate unchanged.

The workflow also skips source-freshness and startup producers after an earlier failure.
Their always-run consumers then fail because the evidence is missing.
Continue those independent producers unless the run is cancelled, while preserving the earlier failed job result.
Retain a source contract for this GitHub condition; a native or browser journey cannot test its own skipped CI step.

One unchanged retry is additional evidence. It does not erase the failed run or establish a root cause.
Local Go, browser, and media execution remain held.
