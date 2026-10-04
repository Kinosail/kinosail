# R08 Now Playing after audio queue advance

The approved audit starts from the frozen R03 delivery `6ec0b5c`. Its source
finding is not runtime proof. The existing authorized queue API already projects
title, artist, album, track, artwork, and stream. No ClientItem keys are added.

## Failure analysis before implementation

- A successful source change can leave heading, byline, accessible label, document
  title, or system media metadata describing the previous track.
- Cached artwork can remain visible while the next artwork is loading or failed.
  Missing artwork must clear the previous cover rather than invent a replacement.
- Queue position and previous/next handlers can retain the old cursor or skip twice
  during overlapping ended, user, or system-media actions.
- Source, progress, cast, and trace endpoints can refer to different items. Buffered
  old trace events must flush to the old endpoint before identity changes.
- The next media URL can lose playback-session correlation. Keep the existing
  page session and attach it to each authorized source.
- A failed final progress save must retain R03 Retry and explicit Continue without
  saving. A manual queue action must not bypass current progress ownership.
- A queued source can fail before metadata loads and leave media time at zero.
  Pause and Previous recovery must not replace its saved position with that reset
  value. Keep the current details link and recovery controls available.
- Offline and receiver playback have separate ownership. A local queue action must
  not replace a receiver-owned source or write its progress.
- Watch Together holds the original media identity in its WebSocket closure.
  Keep in-page queue changes disabled in a room. Existing room selection loads
  the full current-item page and stays authoritative for shared playback.
- A delayed or malformed queue response can arrive after a profile/item change,
  contain an unauthorized empty stream, or contain unsafe URLs or unbounded tags.
  Validate before source, metadata, or queue side effects; use safe text APIs.
- Missing or unsupported Media Session APIs must not prevent local audio playback.
- Original item-management forms contain Server-rendered toggle values. Retargeting
  only their URLs could mutate the wrong item or apply stale state. After queue
  advance, hide those original item controls and offer a validated link to the
  current track's full Server page. Initial page functionality stays available.
  Loading that page ends the in-page queue history and restores canonical actions.

## Test-first public reproduction

The planned regression runs a real native Go Kinosail Server with two generated
fictional album tracks and local metadata/artwork. It uses actual playback ended
events and the canonical queue, progress, trace, and watch routes. Only native OS
action-handler registration is observed, preserving the actual Media Session API;
headless Chromium cannot drive a physical operating system's media-control panel.

Before code, repeat the regression twice. Record source switches to track two
while the rendered/system metadata still describes track one. Preserve receipts,
safe snapshots, screenshots, and private traces. No real user data or deployment
is touched. Root serializes encoder, Go build, and browser execution.

The runner hashes relevant source and test inputs before execution. Before building,
it inventories actual Go dependency sources and embedded assets, records local
module/workspace inputs and dependency module sums, and binds the native binary
checksum to that inventory. It rejects source changes during build or execution.
Red proof requires two named failures with attached post-advance snapshots that
show the defect. Setup, build, and harness failures remain separate failed receipts.
Raw failure details stay private; normal output reports only result and receipt.

```sh
GOMAXPROCS=2 python3 scripts/testing/test-player-audio-queue-local.py --red
```

CI can reuse fixture generation without another credential-preparation path:

```sh
python3 scripts/testing/test-player-audio-queue-local.py --fixture-only <new-disposable-media-directory>
```

The fixture directory must not exist. This mode creates no Owner, Server, or
browser session. Root retains the existing CI Owner/MFA preparation helper.

Further regressions must protect manual previous/next, missing/failed artwork,
authorization or malformed queue rejection without source/progress side effects,
offline/cast ownership, unsupported system controls, and progress-save recovery.
Use isolated cases only where real Server fixtures cannot cause the boundary fault.

## Confirmed public reproduction

At `ee2c009cd5a3e49cb920491f674a0b954498c5f7`, the native Go Server reproduced
the defect twice. Both cases switched audio source and progress to the second
track while heading, accessible label, document title, artist, and native Media
Session metadata retained the first track. The trace endpoint also retained the
first item, and the second source lost playback-session correlation.

`red-results.json` contains the safe runtime snapshots. The two red screenshots
show the actual populated page. `red-receipt.json` binds source, Go dependency and
embed inventory, native product, fixture, and raw private artifact hashes. Its
strict named result check distinguishes two intended assertion failures from a
setup or navigation failure. No production changes preceded this reproduction.

The generated WAV fixture uses Python's standard library, with no encoder process.
Raw browser traces, Server logs, account state, and failed-run details remain in
ignored `.verification/r08-now-playing/20261004T091152Z`. All state is preserved.

## Candidate behavior and limits

The shared Go template projects stable metadata hooks, optional cover, queue
controls, and a current-track details/actions link. The existing ClientItem API
stays unchanged. Each move reads that target item and checks its Viewer profile,
identity, audio kind, canonical same-origin stream/artwork, and metadata bounds
before any current progress write or source change. The response limit is 4 MiB,
the queue limit is 10,000 items, and each displayed tag is limited to 65,536
characters. Oversized inputs fail explicitly; no queue or tag is silently cut.

After acknowledgment, one synchronous operation moves source, progress/cast/trace
identity, page metadata, and Media Session metadata. Old trace events flush first.
Each source retains the page playback session. Native source-load pause events
cannot write progress until the new metadata loads. Previous/next use the current
cursor and a fresh Server progress projection rather than stale queue positions.

The cover reserves a 320-pixel image frame while actual image work is pending.
Missing/failed covers clear the old image. Unsupported system metadata clears a
previous title while local controls remain available. In-page queue history and
the latest pending progress remain bounded page memory; no persistent URL or
credential queue is introduced. Closing a page keeps the existing R03 limitations.

Original item forms and disclosures are hidden after a move. The current-track
link loads the existing full Server page, restoring its canonical toggle values
and permissions. Returning to the first track also requires that page before
using item actions, because the originally rendered toggle values may be stale.

## Frozen R03 ordering regression

The isolated sender test replayed the unchanged frozen R03 source separately from
R08 loading guards. After source moved to `/media/next`, a pause at three seconds
queued a new revision while the old watched save awaited playback continuation.
Releasing that continuation left the new position unsent. The intended HTTP
assertion failed: two requests expected, one received. No tests skipped or global
errors occurred. `drain-frozen-red.json` records source/test hashes, safe debug
context, and raw artifact hashes. Its debug fields are evidence context; the
correctness contract is the actual new-item HTTP dispatch and position.

The first ten-second attempt timed out during teardown and is excluded. It also
recorded video unintentionally; that artifact is preserved. The accepted replay
used video off and finally released the held playback promise. The one-line
sender drain correction and standalone regression are isolated in `41fa08d7`
for the integration owner's R03 assessment. GREEN verification is pending.

Independent review identified a second ordering failure in that candidate: a
pagehide sender can supersede the original sender during suspended continuation.
The retired sender must recheck flight ownership before draining new progress.
The standalone browser regression holds the closing HTTP response, releases the
retired continuation, and requires each latest payload to dispatch exactly once.
It also requires the closing policy failure and safe request ID to remain visible.
The first root run hung because the synchronization assignment returned the held
Promise. That setup timeout is excluded. `1944cc3c` returns `undefined` from the
assignment while preserving the observable HTTP and policy assertions. Root's
corrected replay at `df47fe8d` completed one intended failure, with no skipped,
flaky, or global errors. The HTTP assertion received the duplicate next-item
revision 3. This branch mirrors the root guard from `d72451da`: the original
sender drains only while it still owns the current flight. The standalone test
lineage is retained. Root owns that RED receipt and its GREEN verification;
at that mirror checkpoint, this R08 branch had not run GREEN.

## Failed source position reproduction

At `8cffedd6`, the isolated Chromium case reached the newly assigned audio source
with saved position 42 seconds, reset media time zero, and a media failure before
metadata. Pause and Previous sent two incorrect zero-position writes, revisions
2 and 3. The intended HTTP no-write assertion failed. The named case took 515 ms;
the complete run took 11.57 seconds, with no skips, flaky cases, or global errors.
`pre-metadata-red.json` binds the safe snapshot, dispatched payloads, source hashes,
committed bundle, and raw artifacts. Production was byte-identical to `f6cf967e`.

The first command selected no tests because an anchored selector matched against
Playwright's full test name. Its discovery failure is preserved and excluded.
The retry used one worker, video off, and a 20-second global limit. This controlled
source failure is isolated proof; it is not a populated Server journey.

The candidate now separates source loading from position readiness. A media error
can restore Previous/details recovery while the source remains ineligible for
progress writes. Metadata acknowledgment restores normal saves. A recovery move
skips saving a source that never acquired a usable position; it makes no saved
claim. Full isolated and actual Server GREEN verification remains pending.

## Initial automated GREEN and mobile visual blocker

At `38b43c3a`, 50 isolated Chromium cases passed in 23.56 seconds. No case skipped,
failed, or became flaky; no global errors occurred. Every prehashed bundle/test
input remained unchanged. `isolated-green-receipt.json` records each named result.

The native Go Server also passed both canonical album journeys twice, in 29.74
browser seconds. The four cases verified current metadata, source/session/progress/
cast/trace identity, previous/next, cache delivery, and correct item mutation. Its
Go/embed inventory and source remained stable. `initial-native-receipt.json` is a
byte-identical receipt; `initial-native-results.json` preserves the safe projections.

Visual inspection then found a blocker at 390 pixels: responsive CSS made hidden
original item actions and the hidden Retry notice visible again. The pre-viewport
hidden assertions passed at desktop width. The 1440/1920 renders stayed correct.
`initial-phone-visible-actions.png` preserves the finding. This initial automated
pass is not final UI GREEN. Raw receipts, screenshots, and all state stay preserved.

The strengthened actual Server journey checks hidden actions and notice at every
viewport. A named mobile R03 control uses actual Go HTML/assets and successful
204 acknowledgments; its later routed 503 is explicitly isolated failure injection.
Canonical album journeys remain unrouted. `--visibility-red` requires both mobile
failures twice, after their real source/acknowledgment prerequisites. CSS remains
unchanged until that valid RED. Its proposed repair scopes only the existing
mobile primary-action display rule to elements without `hidden`, including the
exact Subtitles style derivation. Root allocated only matching CSS cache tokens
and existing version expectations; other composition and CI stay untouched.

## Repeated mobile visibility RED

At `ab489c14`, the strengthened native Go Server regressions reproduced both
mobile failures twice in 55.84 browser seconds. All four named cases reached
the required source or real 204 acknowledgment before the visibility assertion.
There were no skipped, flaky, or global-error outcomes. Production CSS and cache
versions remained unchanged. `visibility-red-results.json` contains the safe
snapshots, and `visibility-red-receipt.json` is the byte-identical native receipt.

The actual queue journey advanced every current-track identity correctly, then
exposed the original watched/list forms and hidden notice at 390 pixels. The R03
control received a real 204 acknowledgment while its hidden notice still rendered.
That control is classified as isolated 503 injection in Go-backed UI; this RED
stopped before its routed fault. Canonical queue journeys remain unrouted.
`visibility-red-queue-mobile.png` and `visibility-red-progress-mobile.png` show
both failures. Raw private traces and disposable state remain preserved.

The mobile `.primary-player-actions{display:grid!important}` rule appears after
the global `[hidden]{display:none!important}` rule with equal specificity. The
repair scopes only that existing display rule to `.primary-player-actions:not([hidden])`.
It preserves the global hidden contract and all existing layout geometry. The
matching Subtitles patch derivation receives the same selector change. Freshly
fetched `origin/main` at `0e99a52f` still uses Player `electric-47` and Subtitles
`cinema-15`; the narrow cache increments are `electric-48` and `cinema-16`.
Existing tests retain all assertions and only update those expected version values.
Final native GREEN and visual verification remain pending.

## Ownership

R08 owns `player-progress.js`, `player-presentation.js`, shared Go metadata template
hooks, its E2E/local runner, and this evidence. R10 owns caption status markup in
`apps/player/internal/server/player.go`; this task leaves that file untouched.
Root owns CI invocation, reconciliation, required checks, PRs, and merge.
