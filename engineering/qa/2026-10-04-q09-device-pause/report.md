# Q09: Pause a device download without removing it

Status: actual Go-rendered/browser GREEN, including supplemental phone control;
protected delivery gates pending.
Those RED commands used production unchanged from frozen R10 handoff
`9b59524ddaadc6ab34c8354d9b139760673d7e40`.

The prepared acceptance test starts a real native-size HTTP range transfer,
observes a verified first block, then requires keyboard Pause, canceled second
response, retained bytes/hash, released native locks, an offline Resume failure
with retained progress, and online Resume fetching only missing ranges. It
requires final digest validation, the same Viewer Profile/item, a new transfer
owner, unchanged ready Server preparation, and zero Remove/DELETE requests.

The first bounded native HTTP run at `6d45ea01` reached two intended missing-Pause
assertions, one each for OPFS at 390 pixels and IndexedDB at 1440 pixels. Both
independently verified an 8 MiB first block before the assertion and preserved
traces, error contexts and pending screenshots. The baseline button remained
disabled, and no Pause existed. This confirms expected baseline feature absence.

The external 28-second process limit interrupted aggregate completion. Trace
worker-cleanup hooks record about 10.4 seconds in Chrome cleanup for each failed
worker; the second worker completes at about 28.25 seconds. This run has case-level
evidence but no completed aggregate report. It is not claimed as a complete RED
repeat. No second baseline or profile control was launched in that slot.

Subsequent fixture-only changes close the browser context while the HTTP peer is
still available, then stop the peer listener before closing connections. The
isolated shell now explicitly declares UTF-8. All assertions and production
sources remain unchanged.

Clean repeats at `c8749dc978902447c3df84bb66f3111bd7d08c2b` selected cached
Chromium 153.0.8010.12 with one worker. `isolated-red-2` and `isolated-red-3`
each completed two cases with exit 1, two intended missing-Pause failures,
zero skips, zero flaky cases and no runner errors. Both storage paths reached
the same verified 8 MiB first-block boundary before requiring Pause. The runs
completed in 9.285 and 8.774 seconds. This proves expected baseline feature
absence; it is not an incident classification.

The separately selected profile controls completed in 7.315 seconds. Opening
another tab with the same Viewer Profile interrupted the first owner before any
Pause assertion: the peer recorded one closed response and the original status
changed to the existing interrupted message. The changed-profile control passed:
the old owner canceled while verified chunks and their original profile remained
intact. There were two executed controls, no skips, no flaky cases and no runner
errors. The same-profile interruption is a separately confirmed existing failure.

Read `clean-baseline-summary.json` for decoded independent byte/lock/peer receipts,
`clean-baseline-context.json` for revision/source/cache executable hashes, and
each `*-receipt.json` for exact command, environment, time bound and complete
aggregate result. Raw JSON reports, traces and screenshots are retained. Text
logs and error contexts are preserved as verified lossless gzip files. The
initial interrupted run remains separately labeled; it is not a completed repeat.

Read `failure-analysis.md` for the authoring gate and isolation limits. Read
`reproduction-commands.md` for the baseline and eight-case real Go/browser
commands. At this baseline checkpoint, no Q09 Go build had run; the later section
records the completed eight-case Go/browser proof. No device, encoder or
deployment work has run. Root owns integration gates. No implementation was
applied before these complete repeats and the dedicated controls ran.

Product snapshot `bf10089e` adds owner-only Pause/Pausing/Resume through a
focused DownloadsUI module. Explicit Pause cancels the existing owner signal,
retains verified blocks, waits for native transfer work to settle and restores
keyboard focus. Waiting admission cannot broadcast another owner's cancellation.
The existing manifest checks, range digests, verified-block reads, full-file
digest and native locks remain in place. The same-profile repair compares the
captured transfer profile at the existing event; actual profile changes still
cancel. Offline identity revision and playback behavior are unchanged.

Player/Subtitles effective immutable download tokens are 30/14, with matching
offline templates and consumer assertions. The frozen R10 Player template seam
is byte-identical after normalizing its download token. Both apps' Downloads
templates expose the new labels through existing translation hooks, and the
original localized device-action label is preserved after completion.
`make max-loc`, composed browser lint and four bundle composition checks passed;
read `prepared-product-checks.json` for exact source hashes.
The five-case isolated native-browser GREEN completed at QA revision
`ed7b926fb05314837b1385825217f1d7db96131f`, with the same product snapshot, in
10.102 seconds. All five cases passed; skips, unexpected results, flaky cases and
runner errors were zero. Native OPFS and IndexedDB retained the verified first
block, canceled the partial second response, released native locks, kept data
through an offline Resume failure and resumed only offsets 8 MiB and 16 MiB.
The final independently read 16,777,247-byte digest matched the public manifest.
Same-profile tab handoff and actual navigation/reload both preserved verified
data and resumed explicitly; the isolated different-profile control still canceled
the old owner. All peer receipts recorded zero removal requests.

Read `isolated-green-summary.json`, `isolated-green-context.json` and
`isolated-green-receipt.json` for decoded storage/byte/peer observations, source
and environment hashes, exact command and completed result. Eight pending,
paused, offline-failed and ready captures were inspected at 390 and 1440 pixels:
labels remain readable, Resume retains visible keyboard focus, page-open guidance
appears while transferring, Remove stays in its separate disclosure, and Play
appears only after completed verification. These are declared fixture-shell
captures; they establish the separately declared isolated boundary.

The actual Go proof completed at exact revision
`20611fe1a5b79b2857c9e67c13f8b2662d024a9b`, unchanged product `bf10089e`, in
29.463 seconds including the bounded Go build/test. `TestDownloadPauseBrowserJourney`
passed, and all eight browser cases passed with zero skips, unexpected results,
flaky cases and runner errors. The public API prepared the fictional original
file in temporary Server directories; no encoder ran. Six primary cases exercised
Go-rendered controls and native OPFS/IndexedDB at 390, 1440 and 1920 pixels.
Two additional cases covered same-profile tab handoff and actual navigation/reload.
The changed-profile supplied-fixture control remains the separate isolated proof.

Each primary Go case canceled one held partial range, retained the independent
verified 8 MiB first block, released native locks, survived offline failure,
fetched only missing ranges and verified the 16,777,247-byte final digest
`28c957c2b18f6998dc4e067e9b9d316497c5c14df1a94af55bddaaa6d3344647`.
The actual Go-served asset `/static/downloads.js?v=30-htmx4` matched the independently
reconstructed 69,737-byte composition in all six cases: SHA-256
`2fb188e2744fe553afb7c4730d58bb0a3df38c2191c6f20732898a07503a2fc5`.
Ready Server preparation and profile/item ownership stayed intact; every peer
recorded zero Remove requests. Read `server-green-context.json`,
`server-green-receipt.json`, `server-green-summary.json` and the raw aggregate for
repeatable commands, exact source/environment hashes and independent receipts.

All 24 Go-rendered pending, paused, offline-failed and ready captures were inspected.
The new Pause/Resume controls and keyboard focus are readable at each width.
The 390-pixel full-page captures include the existing fixed bottom-navigation band
crossing later flow content at the recorded scroll position; the existing Play
link can intersect that band. These captures do not establish its hit target at
every scroll position or a Q09 regression. CSS and navigation were not changed.
Read `server-visual-inspection.json` for exact capture hashes and this boundary.

Independent review cleared the exact eight-case Go evidence `25339d12`, with
the historical sentence correction `7acb6d9a`. The reviewer matched all 205
artifact hashes and decoded raw receipts, reviewed the exact product bytes,
and inspected the 12 unique captures covering 24 paired storage renders.

The separate phone hit-target control at `7acb6d9a` completed in 15.123 seconds:
one failure, zero skips, flaky cases or runner errors. Pause/Resume pointer hit
testing, the resumed full-file hash and first Play pointer navigation passed.
After a fresh Downloads navigation, the helper sampled the Play link before
the shipped smooth scrolling settled; it hit the navigation at that instant.
The later failure capture has focused Play clear of the nav. Keyboard activation
and final peer assertions were not reached. This establishes an unsettled-scroll
test boundary, not a persistent product layout defect. Trace inspection found
zero Remove/DELETE requests. Read `server-hit-targets-summary.json`, its exact
context/receipt and retained raw report/trace. The subsequent test-only wait
retains every original assertion and observes settled native scroll geometry.

The repeat at independently source-reviewed `0606ac62` completed in 15.276 seconds:
Go PASS and one browser case PASS, with zero skips, unexpected or flaky results
and runner errors. Pause, Resume and ready Play all received center pointer input
with focus after explicit native scrolling settled. Both pointer Play and keyboard
Enter after a fresh Downloads navigation opened the same fictional stored job.
The final hash remained exact, the peer recorded only ranges
`[0, 8388608, 8388608, 16777216]`, one canceled response and zero Remove requests.
The real Go-served bundle retained the reviewed checksum. The 390-pixel viewport
capture shows focused Play clear of the nav at this settled position. Read
`server-hit-targets-stable-context.json`, its receipt, decoded summary, raw report
and visual inspection. No CSS or production change was needed. This closes the
explicit-scroll hit-testing/navigation gap; it makes no every-position or media
decoding claim. Independent review cleared the supplemental evidence snapshot
`2aeb0e64520f05e5f2e77f62305f83235860495c` with no findings. The reviewer matched
all 249 artifact hashes, raw single-case results and attachments, exact source
aggregates, pointer/keyboard navigation receipts, gzip preservation hashes and
the 390-pixel capture. Read `handoff.json` for the frozen execution/review
boundaries, source serialization recipe and remaining integration gates.

The fixtures contain deterministic non-playable bytes: this proves transfer and
integrity, not media decoding. Other browsers, physical/native app devices,
standalone Subtitles runtime, actual BFCache admission, profile/account API changes,
actual user data and production are outside this execution. Required hosted gate
wiring, affected consumer checks and current-main integration remain root-owned.

MAIN: NO — required integration gates, merge and ancestry proof pending.
