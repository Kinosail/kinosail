# R05 — remove a displayed photo after access is revoked

Status: tested, independently reviewed; PR checks and merge pending.
Implementation: `bc7892d2709356ae06af538dbb77be3e8ff0c190`.

A photo already displayed from the protected cache remained visible after the
Server returned 401, 403 or 404. The completed baseline rendered the fictional
photo after each denial in both repetitions. The 503 control passed, preserving
the photo during a transient Server failure.

`PhotoScreen` now propagates the existing cache-discard error class even when
saved content is visible. Its terminal error handler clears both the image and
private title after denial. It checks cancellation before writing failure state.
The existing origin, Server and Viewer Profile cache boundaries are unchanged.
The non-discard failure path retains saved content.

The fresh signed simulator build at the implementation commit completed from
`2026-10-04T10:07:36Z` to `10:07:57Z`, within its 120-second bound. Build and test
exit codes were `[0, 0]`. Both completed repetitions passed for 401, 403, 404
and the 503 preservation control, with zero skipped cases. The retained manifests
record 461 stable native input files and 93 signed product files. All eight fresh
screenshots match their recorded hashes. Independent review checked the source,
exact artifact hashes, selected repetitions and saved/settled screenshots.

The safe, hash-pinned records are [RED](evidence/r05-red.json),
[GREEN](evidence/r05-green.json) and
[independent review](evidence/r05-independent-review.json). The RED record
explicitly preserves the initially rejected receipt and separately identifies
the reviewed offline correction for Xcode's actual repetition schema. It is not
a new runtime run. Original results, raw private logs, screenshots and both
byte-exact signed product archives remain in the ignored local evidence folder.

Repeat the focused test on a task-owned available simulator:

```text
python3 engineering/qa/2026-10-04-apple-library-recovery/run-native-test.py \
  PhotoAuthorizationJourneys <new-phase> <simulator-uuid> \
  --iterations 2 --timeout-seconds 120
```

This is production SwiftUI rendering against fictional loopback HTTP on an
iOS simulator. It does not certify a physical device, tvOS, production Server,
release or deployment. The build also included two pending, uncommitted
R11/R17 regression files; their exact hashes are disclosed in the evidence.
They are excluded from this R05 batch. Their rejected readiness results do not
change the completed R05 behavior proof.

Local `make max-loc` passed. The integration owner retains required affected-app
verification, protected PR checks, merge and fetched ancestry confirmation.
