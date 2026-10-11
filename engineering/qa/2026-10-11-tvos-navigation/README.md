# tvOS navigation audit

Status: simulator audit and fixes complete.
Eleven confirmed navigation or presentation findings are fixed.

## Scope and method

Use a disposable Apple TV simulator, the loopback navigation fixture, generated
media, and synthetic titles. Predict each arrow's destination from the rendered
screen before pressing it. Check the observed focus, activation, and Back return.
Count inputs for common tasks and preserve native remote conventions.

The fixture exercises the real app and tvOS focus engine with isolated HTTP
responses. It is not populated production-server E2E proof. Existing browser
journeys cannot detect tvOS focus traps, wrong spatial movement, or focus loss.

## Journey checklist

- Home: Search/Settings, every Continue watching card and Browse category,
  recent/unwatched/genre shelves, horizontal edges, vertical columns, deep Back.
- Library: grid edges and rows, sorting, A–Z index, paging, detail return.
- Search: keyboard, scope, sorting, results, empty results, Back.
- Movie and show: primary/secondary actions, season selection, episode grid,
  bookmarks, direct playback, nested Back and originating-card restoration.
- Music and audiobooks: albums/tracks, transport, options, queue, mini player.
- Photos: selection, zoom/pan, fit and Back.
- Settings: all rows, playback preferences, progress sync, Supporter collection,
  About, sign-out cancellation, Change Server and pairing cancellation.
- State/accessibility: pending, loaded, empty, failed/retry, focus visibility,
  accessible control names, 1080p/4K; increased contrast and Reduce Motion where
  the simulator exposes them. Keep physical remote/VoiceOver proof separate.

## Test authoring gate

New regressions must assert user-visible focus or activation from directional
remote input. First reproduce the mismatch twice and record its screenshot.
Extend the owning remote journey where possible. Document the distinct coverage
gap before adding a new case. No production-only test hook is authorized.

## Environment and evidence

- Worktree: `codex/tvos-navigation-audit-20261011`.
- Xcode 27.0 (27A266a), tvOS 27.0 simulator.
- Disposable 1080p simulator: `5ECCEA70-5605-4192-80B9-36F91EDCC156`.
- Private evidence archive: `/Users/mikeo/.codex/artifacts/tvos-navigation-20261011`.
- Local main had unrelated changes to the Xcode project and string catalog.
  They are preserved. The worktree audit also reported eight unrelated inactive
  checkouts, including one dirty checkout; their files and leases are unchanged.

## Findings and validation

### NAV-01: Up jumps across Home (P2, confirmed twice)

On base `655b7d6d`, start at Search, press Down, then Right three times.
Moonrise is beneath Settings. Up instead focuses Search on the opposite side.
`topbar-red.xcresult` repeats the revised spatial assertion twice; both fail.
Remove the blanket Up-to-Search override and retain the explicit Back shortcut.
The expected Settings path saves one extra Right press.

### NAV-02: Media focus is hard to see (P1, confirmed twice)

The first and second focused Continue watching cards retain the same silhouette.
Only the clipped artwork zooms. The native accessibility query reports focus,
but the screenshot does not clearly identify the selected card. The committed
TV design requires a visible outline. Extend the existing card-spacing journey
with a rendered high-contrast outline check, including the missing-image card.
Both pre-fix trials fail in `detail-focus-red.xcresult`.

### NAV-03: Detail actions do not follow visible geometry (P2, confirmed)

Open the first movie from Movies. Resume appears directly above My List.
Down focuses Bookmarks instead; both automated pre-fix trials fail. A manual
walkthrough confirms Bookmarks, Right to Remove, and Up remaining on Remove.
Measured control frames show Bookmarks starts two pixels above My List. Align
the secondary row's focus regions so Down reaches My List. Give the primary row a
full-width focus section so secondary actions can return upward without a Left
detour. The primary control keeps its native size.

Home also needs a full-viewport focus section: after Up reaches Settings,
Down must return to the card underneath instead of remaining on Settings.
`row-regions.xcresult` confirms that two-way path and Down to My List. The
extended detail journey also checks Bookmarks cancellation and Up from Remove.

### Baseline evidence

Eleven existing remote journeys passed against the base revision before changes.
They cover Home rows and deep scrolling, all five Browse categories, movie,
show/season, photo, album/audio, Settings/Playback, video/options, and Back
restoration. Their screenshots are in `baseline-attachments` and their full
results in `baseline.xcresult`. The new assertions above expose gaps in those
previous tests; they do not change API behavior.

The synthetic catalog previously returned an empty letter index even with 160
movies. Its new index reports each title group's actual offset and count so the
A–Z navigation can be exercised. Four existing fixture safety checks passed.

### NAV-04: Library top-row Up is trapped (P2, confirmed twice)

From Sort, press Down and Right five times to Journey 008. Up leaves focus on
Journey 008. The test records this mismatch in `focus-library.log` before the
test runner stalls while reporting failure. A separate manual walkthrough
reproduces it. Five Left presses and one Up are required to reach Sort.
The filter row now has a full-width focus section. Its regression requires
one Up from the sixth column and also traverses both grid rows.

### NAV-05: The selected A–Z letter is unreadable (P2, confirmed)

Right from Sort reaches A. Down twice reaches J and loads Journey 006 onward.
Both selected letters render pale text on a pale native focus fill. The J
highlight also overlaps its neighboring letters. See `library-index-j.png`.
The letter-index journey now checks the rendered glyph as well as its focus,
letter jump, adjacent-card return, and originating-card restoration.

### Host and reconciliation notes

The `detail-green` run stalled before test launch and was stopped. The
`focus-library` run reached the grid mismatch but timed out while reporting it;
it is observation evidence, not a completed test suite. Only this task's test
processes and simulator were restarted. Later runs disable optional Xcode
diagnostic collection and retain their explicit test attachments.

Checkpoint `3d1eef0b` preserves the initial fixes and audit. Merge `3cece51e`
reconciles current main, including the movie hero artwork/layout changes from
PR #548. Final acceptance must use that integrated layout.

Focus-section reference: [Apple's SwiftUI focusSection documentation](https://developer.apple.com/documentation/swiftui/view/focussection()).
The app retains native directional navigation and uses focus sections to bridge
visible gaps between control rows.

### NAV-06: Audio Up skips the adjacent control row (P2, confirmed twice)

With audio paused, move to Shuffle in the options row. Previous is the closest
control above it. Up instead jumps past the Previous/Next row to Back 15 seconds.
Both `paths-4k.xcresult` and `audio-red-repeat-4k.xcresult` reproduce the mismatch.
The Previous/Next row now spans its available width as a tvOS focus section.
The control sizes stay native. A later Up-from-Next check also returns to Play
instead of the expected Forward 30 seconds. Manual and automated runs confirm
that ambiguity. Previous and Next now align beneath the corresponding backward
and forward transport buttons; the journey checks both column geometry and arrows.

### 4K observations

The disposable 4K simulator is `EEDDB21C-EA99-4EC1-B773-424FB129CCEC`.
The nested Settings journey passed all six destinations and restored each
originating row. Search passed keyboard entry, result activation, result return,
empty results, and Back to Search. The A–Z journey passed its readable-glyph
check and restored Journey 011 after detail Back. More spacing keeps the native
letter highlight clear of its neighbors; the final 88-point rail also passes.

Native menus and the system keyboard do not consistently expose their selected
child through `hasFocus`. Their expanded-menu assertions therefore use visible
screenshots and the selected value after a directional input. This distinguishes
an accessibility-query limitation from an actual focus defect. Collapsed menus
expose their visible title through static-text descendants of the focused view.

`make -C apps/player verify-changed` passed max-loc, diff-check, and both native
client builds at `e82c49e7` plus the then-pending test edits. Subsequent production
changes require the final committed verification below.

The final 4K index screenshot shows the complete highlight without clipping or
neighbor overlap. All six letters were reached with single arrows. Home's
pending and empty journeys passed with fresh identities. Pending and loaded
screens keep the same card positions and Search frame; placeholders disappear
when loading succeeds.

A reused manual audio session reached `0:30 of 0:30`. It is not a valid starting
state for a long paused-control walkthrough. The retained journey now waits for
the focused Pause control and pauses before recording screenshots. Final suites
use fresh seeded identities. The failure-state runner also exhausted its old
120-second wall allowance during Xcode preparation. The single-state allowance
is now 300 seconds; a timed-out receipt remains rejected even if Xcode exits zero.
The two existing child-process settlement checks still pass.

### NAV-07: Audio Playback options is clipped (P1, confirmed twice)

The 4K manual walk reaches Playback options with Up from the transport row.
Opening it presents the full TV settings design in a narrow sheet. Text and the
selected preferences row extend beyond its right edge. Both manual openings
reproduce the defect; `audio-options-clipped-4k.png` preserves the guest screen.
Use the same full-screen presentation as the video options. The audio journey
checks the preferences bounds, Done position, and Back restoration.

### Cast return and additional music paths

The 4K cast walkthrough finds Up from Actor 01 landing on Bookmarks, although
My List is directly above the card. Two manual traversals reproduce it. The
previous artwork regression encoded this detour as Up, Left, Up. Its expectation
now requires Up to My List and Up to Resume. A shared scaled minimum label height preserves native button shapes and gives
the row consistent focus bounds. An intermediate per-button focus-section
approach trapped Down before Cast in `audio-cast-1080`; it was rejected.

Manual 4K music coverage also reaches All music tracks with one Up from the
first album, then moves between Sort and the Albums toolbar with single arrows.
Back restores All music tracks. The paused mini player exposes its title, Play,
and Stop with successive Right presses. Opening its full player and pressing
Back restores the mini-player title. NAV-06 now returns Shuffle Up to Previous
in the manual walkthrough. Automated final acceptance remains below.

`audio-sort-cover-4k` passes the complete grid/sort journey, including all three
sort choices and cancellation focus restoration (74.663 seconds). The same run
passes Shuffle Up to Previous, then exposes the Next-to-Play mismatch above.
It does not count as a complete audio pass.

The tvOS simulator reports `unsupported` for `simctl ui appearance` and
`increase_contrast`. These calls changed no settings and are not accessibility
variant evidence.

### Accepted final control journeys

`aligned-controls-1080/remote.xcresult` passes three selected journeys with no
failures: full Cast traversal and return (138.788 seconds), album/audio/options/
mini-player return (64.851 seconds), and every Playback preferences row with
return to Settings (42.994 seconds). The extra movie selector in that exploratory
command did not match a test; movie navigation remains required in the full run.

The audio test confirms Previous/Back 15 and Next/Forward 30 share columns.
Options now fits the full TV viewport. Its toolbar uses tvOS 27’s native “More”
accessibility name. Save preferences exposes focus on its containing Form cell,
so the settings check uses the focused element’s visible label.

Merge `631104c7` incorporates main’s verification-cache repair and QA report
updates from PR #553. It changes no native app or UI-test source.

| Path | Before | After |
| --- | --- | --- |
| Moonrise to Settings | Up, Right | Up |
| Sixth library column to Sort | Five Left, Up | Up |
| Resume to My List | Down, Left | Down |
| First cast card to Resume | Up, Left, Up | Up, Up |
| Next to Forward 30 seconds | Up, Right | Up |
| Shuffle to Previous | Up skipped the row | Up reaches Previous |

### NAV-08: Detail loading reserves the wrong content geometry (P2, confirmed)

The first pending screenshot places its title at y382; loaded content starts at
y179. It omits the progress and secondary-action rows. A pixel assertion added
to the existing pending journey reproduces the 203-point jump in
`pending-retry-red-1080-retry`. The TV placeholder now reserves those rows and
uses matching title/body heights and cast insets. Final green evidence follows.

The first state batch passes detail pending removal, failed-detail retry, empty
backdrop, missing artwork, and audio pending removal. Audio retry initially
asserted focus during presentation. Waiting for settled focus confirms Try again
is selected and recovery returns to Play (20.931 seconds in the repeated run).
That early run did not expose the later intermittent focus failure in NAV-11.

`detail-pending-aligned-1080` passes the new pixel regression (33.234 seconds).
The title placeholder now starts at y186, within seven points of the loaded
title at y179. Visual inspection confirms aligned action rows and cast insets.
The pending and loaded screenshots were exported from the accepted result.

### NAV-09: Audio loading controls jump downward (P2, confirmed)

The 4K pending journey measures the transport placeholder at y394. Loaded
controls need y499 after excluding a new 99-point progress-failure notice.
The new pixel assertion fails before the change. The loading view now reserves
the actual text, timeline, track-control, and options geometry.
`offline-audio-aligned-4k` passes the audio case with y506 versus y499
(30.015 seconds). Its other case failed before reaching the intended outage;
the aggregate bundle is not a pass. Both audio screenshots were inspected.

The expanded audio journey passes all Repeat choices, Shuffle on/off, all five
Sleep timer durations, and Turn off in `recovery-geometry-red-4k`
(117.807 seconds). Each duration checks the displayed deadline. Native menu
screenshots show readable selection and no clipped choices.

### NAV-10: Connection banner covers the detail error heading (P2, confirmed)

An early 1080p error screenshot shows the overlap. A partial HTTP failure can
clear its banner after another successful request, so it cannot prove a
persistent outage. The disposable fixture now supports a bounded connection
failure mode that closes authenticated requests without a response. Its QA
health and reset routes stay available. Invalid state combinations are rejected.
`disconnected-detail-red-4k` confirms the heading remains covered after five
seconds with Try again focused. tvOS now reserves layout space for the banner
above the navigation stack. Recovery and loaded-navigation acceptance follows.

### Combined-run timing and focus observations

At `3a22cb068bdd018585dc02c58a267f72cb930c42`, `final-loaded-1080` executes
25 journeys: 23 pass and two fail. Fixture health stays ready and every child
joins. This bundle is not an overall pass. Playback-to-Home focus was queried
during its return transition. The captured video already shows the originating
card's focus outline before ending. A bounded settled-focus check passes both
Select and Play/Pause returns in `recovery-geometry-red-4k` (25.266 seconds),
without corrective input. Observed focus settlement takes 1.237 and 1.175 seconds.

The stress case records all 100 successful detail returns, then reaches its
600-second limit during final held-input checks. Only that case now requests
900 seconds; the runner permits a 930-second per-test maximum. The failed
receipt is retained. The complete 4K stress acceptance is recorded below.

The tvOS app explicitly uses `.preferredColorScheme(.dark)`. Unsupported
simulator appearance commands are not proof of accessibility variants.

`sustained-outage-detail-4k` passes the complete outage journey (22.289
seconds): visible error heading, focused Try again, Up to Reconnect, Down to
Try again, and recovery to Resume. Earlier fixture attempts still answered
background remote-player PUT requests with 404. Native request diagnostics
identified those responses restoring reachability. Connection fault injection
now drops authenticated GET and PUT requests; those intermediate bundles are
not outage acceptance. The banner reserves its intrinsic height in the TV stack.

The loaded Home and movie-detail journeys also pass after the stack change in
`disconnected-detail-green-4k` (14.528 and 30.942 seconds). Its incomplete outage
simulation failed, so only those individual cases count as accepted evidence.


### Final combined 4K run and harness corrections

The `accepted-loaded-4k` directory name does not imply an aggregate pass.
At `ca03f21e`, its result contains 22 passes and two failures across 24 started
journeys. The last nested Settings journey never starts. One failure is the
card-outline pixel probe; the other is cancellation at the runner's 30-minute
wall limit. Fixture health has no failures, and both Xcode and the fixture join.

The complete 100-return case passes in 695.608 seconds, including held-arrow
reversal and the Home shortcut. Search also passes in 34.164 seconds. The stress
screenshots at returns 20, 40, 60, 80, and 100 preserve the originating card and
scroll position. The final return screenshot was inspected.

The outline failure video's final frame clearly shows the continuous light
border on the long-title card. The test now requires the edge across three
separated points on at least two rows within a narrow band. It captures each
focused card before measuring. This avoids relying on one exact raster row.
The first follow-up passes all four cards (33.792 seconds), reporting light
rows at offsets 0 through 3. No production rendering change was needed.

The retained runner now permits 40 minutes for the loaded suite. Its per-test
limits, timeout rejection, health checks, and bounded child cleanup stay intact.
Both process-settlement checks pass after this change. The focused 4K follow-up
passes the outline, all Settings rows, nested Settings returns, and detail pending
geometry. Their durations are 33.792, 33.531, 34.709, and 32.680 seconds.
The outline also passes at 1080p in 29.731 seconds.

### NAV-11: A new detail error can focus the banner (P2, intermittent)

`accepted-states-1080` shows the error heading clearly but focuses Reconnect
instead of the detail's Try again action. That costs an extra Down press.
The screenshot and five-second focus wait confirm the stable wrong destination.
A second fresh run passes without changes, confirming that initial focus depends
on which asynchronous view appears first. The existing outage journey exposes
this case; its expectation stays unchanged.

The audio failure check also captures an error with no focused element in
`accepted-audio-retry-1080`; its screenshot shows an unfocused Try again button.
The detail and audio resources now request Retry focus when their errors appear.
The request runs once on appearance. Subsequent Up and Down movements remain
native. Other resource and retry screens keep their existing default behavior.
The 1080p detail outage recovery passes in 21.912 seconds. After applying the
same opt-in focus behavior to audio errors, audio retry passes in 18.691 seconds.
The final 4K detail outage passes in 24.076 seconds; audio retry passes in
26.984 seconds. Both failed and recovered screens were inspected.
`2707f1f5` commits the retry focus change; `f52619fd` reconciles main PR #549
without changing native production source.

### Manual library state acceptance

The final 1080p walkthrough uses Device Hub remote buttons and the isolated
library fixture. Empty content leaves Sort focused and shows no placeholder.
A delayed Year sort displays six columns of placeholders in the loaded grid's
positions. Success removes them and retains Sort. A failed Recently added sort
shows a clear error with a reachable Try again action. Down reaches Try again;
Up returns to Sort; Down returns to Try again. Each path takes one press.
Restoring the fixture and selecting Retry loads 160 titles and focuses Sort.
Back restores the Home Movies entry. Every resulting screen was inspected.

`manual-library-states/receipt.json` records predictions, observations, source
input hashes, and screenshot checksums. Two earlier captures contain cached
Title results and do not count as empty-state evidence.

### Accepted coverage and limits

The combined 4K run supplies 22 passing loaded journeys. The focused follow-up
adds the corrected outline assertion and both Settings journeys. Together they
cover all 25 distinct loaded journeys. This is not a single aggregate passing
bundle: the original wall-clock timeout and failed pixel assertion remain in
the evidence. The complete 100-return stress case, including held-input reversal
and the Home shortcut, passes in 695.608 seconds.

The final recovery focus change affects only initial detail and audio error
focus. Loaded rendering and navigation are unchanged from those accepted runs.
The focused state journeys cover the changed failure paths separately.

Six fixture safety checks and two process-settlement checks pass. The fixture
accepts only bounded QA modes and rejects invalid combinations. No production
API or failure classification changed. Existing native request diagnostics
identified the incomplete outage simulation; raw diagnostics remain private.
Server, browser, physical-device, and deployment logs were not verified.

See [evidence.md](evidence.md) for exact environment, reproduction commands,
result counts, source revisions, and local artifact checksums. Physical Apple
TV, Siri Remote, VoiceOver, accessibility variants, production-server behavior,
and production deployment remain outside this simulator audit.

### Final local verification and cleanup

At `5f09978e36a8330698be017476e2836182fbd6ed`,
`source ~/.zprofile; make -C apps/player verify-changed` passes max-loc,
diff-check, and both native client builds. The build stage takes 128 seconds.
The preceding attempt rejected two trailing blank lines; that receipt is retained.
The final cleanup changes whitespace only and has no semantic source difference
from the accepted recovery runs. `make max-loc` also passed at the repository root.

The loopback fixture was stopped and joined. Both task-created simulators were
deleted after their evidence was saved. Other simulators and physical devices
were not changed. Local main remains at `dd420717` with unrelated Xcode project
and string-catalog edits. Those edits prevent the normal main/worktree cleanup.
The task checkout will remain clean after publication.
