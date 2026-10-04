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

## Ownership

R08 owns `player-progress.js`, `player-presentation.js`, shared Go metadata template
hooks, its E2E/local runner, and this evidence. R10 owns caption status markup in
`apps/player/internal/server/player.go`; this task leaves that file untouched.
Root owns CI invocation, reconciliation, required checks, PRs, and merge.
