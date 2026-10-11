# tvOS navigation audit

Status: in progress. Results below are added only after observation.

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
- Local evidence: `/tmp/kinosail-tvos-navigation-20261011`.
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
the secondary row's top edges so Down reaches My List. Give the primary row a
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
Its controls retain their native centered layout and horizontal spacing.

### 4K observations

The disposable 4K simulator is `EEDDB21C-EA99-4EC1-B773-424FB129CCEC`.
The nested Settings journey passed all six destinations and restored each
originating row. Search passed keyboard entry, result activation, result return,
empty results, and Back to Search. The A–Z journey passed its readable-glyph
check and restored Journey 011 after detail Back. More spacing keeps the native
letter highlight clear of its neighbors; final width verification remains open.

Native menus and the system keyboard do not consistently expose their selected
child through `hasFocus`. Their expanded-menu assertions therefore use visible
screenshots and the selected value after a directional input. This distinguishes
an accessibility-query limitation from an actual focus defect. Collapsed menus
expose their visible title through static-text descendants of the focused view.

`make -C apps/player verify-changed` passed max-loc, diff-check, and both native
client builds at `e82c49e7` plus the then-pending test edits. Subsequent production
changes require the final committed verification below.
