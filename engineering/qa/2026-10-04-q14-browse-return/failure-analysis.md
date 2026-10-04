# Q14 test-first checkpoint

This is preparation, not runtime bug proof. Production remains the frozen Q09
handoff `1b68c55b8fa176154f2a17052606f71867208308`. Q14 is an approved feature
opportunity. No Q14 Go build, browser run, production edit, or CI edit has run.
The parent owns the serial execution schedule and subsequent implementation grant.

## Public behavior and source boundary

The canonical Q14 record requests a contextual return after playback, including
the current browse URL, sort/query, loaded extent, selected title, focus, and
scroll. The Go catalog parser accepts only `q/view/sort/offset/limit/lang/letter`.
It bounds offset at 1,000,000, limit at 200, and query at 512 bytes. The current
Viewer Profile policy filters both `/api/v1/library` and rendered browse results.
Replay must request current visible data through these routes. Cached card HTML
must never substitute for current authorization.

The current Player Library link points to `/`. Show detail's Shows link points to
`/?view=shows`. Neither carries the originating query, sort, offset, or extent.
`pwa-navigation.js` stores only clicked href and scroll under the document's
initial URL. It restores on native `pageshow` for Back/Forward. It misses Show
wrapper links and ordinary flat Home shelf cards. Live HTMX URL changes do not
change the key. These are source hypotheses until an actual journey fails.

`pwa-library.js` validates canonical identities, deduplicates grouped cards,
aborts stale requests, and retains an accessible pagination fallback. Automatic
appends do not change the URL. Q14 must coordinate with this owner rather than
start a competing loader or weaken R04's contracts.

The pinned HTMX 4.0.0 history path emits `htmx:before:history:restore` with
`cacheMiss:true` and fetches the current path into `hx-history-elt`. It has no DOM
history cache or after-history-restore event. This in-document replacement is
distinct from a native browser document restored from the Back/Forward cache.

Pinned Playwright 1.63.0 explicitly passes `--disable-back-forward-cache` to
Chromium. The cold-native suite retains that flag. The separate cache control
removes that exact default argument and requires observed native admission:
`pageshow.persisted`, retained document identity, and no new root document GET.
Removing the argument alone does not prove admission. If admission fails, that
case is a cache prerequisite failure, not a product RED or a passing control.

## Test authoring gate

1. The primary owner is a real Go Server plus browser journey. It protects the
   approved public return URL, current Viewer Profile, card identity/order and
   extent, focus, and settled scroll after the visible Player Back action.
2. A credible regression sends the user Home, drops query/sort/offset, restores
   scroll before the later card exists, focuses a different Show action, or
   appends duplicate/stale cards while restoring.
3. Existing populated pagination tests cover completeness and stale search
   replacement, but do not leave for playback and return. Navigation-menu tests
   cover menus, not browse history restoration. Native cache and HTMX history
   require distinct cases because their lifecycle boundaries differ.
4. No production export, flag, or injection seam is needed. An opt-in Go test
   creates a temporary catalog and serves the normal application. Its outer
   peer records bounded fixture-only root requests without modifying responses.

## Fixture and independent expectations

The fictional catalog has 32 `Return Movie NN` titles, 12 `Return Show NN`
one-episode folders, and `Anchor Movie`/`Zeta Movie` for real title-letter history.
Expected titles and order come from these literal fixture names. Public API
IDs are validated and used only for canonical identity, never to manufacture
expected sort order. The initial Movies journey uses query `Return Movie`,
sort `title`, offset 4, and limit 4. It must load 28 distinct Movie cards and
select `Return Movie 25` beyond the first page.

The authorized fictional R03 MP4 is copied read-only into selected fixture
titles. Original SHA-256:
`9dbd85e7863d921e209978c8349992e5f8505b0d7ffa468c37b8caf97af3f0a4`.
FFmpeg and FFprobe point to unavailable fixture paths. No encoder or device runs.
The return contract does not claim playable-media decode unless a separate
assertion observes advancing playback. Other catalog entries are empty files.

## Execution slices (all pending)

- `primary`: visible Player Back at 390 and 1440px; run twice in separately
  preserved processes before any production change. Baseline absence is an
  expected feature opportunity, not an incident.
- `cold`: native Back at both widths with BFCache disabled. Require a new
  document identity, `back_forward` navigation, non-persisted pageshow and a new
  root GET before claiming a cold native boundary.
- `bfcache`: an actual native cache admission control, as described above.
- `htmx`: title-letter push and in-document Back. Require the real history
  request header, unchanged document identity, and main replacement; this is
  browse history evidence, not playback evidence.
- `shows`: direct Show Play and Show-details-to-episode-to-Player, preserving
  each original Show action's focus. No flat-card assumption.
- `search`: live HTMX search then playback and cold Back. Record original and
  updated URLs independently before checking restored selection.

Each case attaches fixture-only before/Player/after URL, profile, card order,
selected href, focus, scroll, lifecycle events, native request receipts, and
served asset byte counts/digests. Capture the pre-action browse viewport and
actual returned viewport. Harness, discovery, cache prerequisite, and timeout
failures remain distinct from intended acceptance failures. Completed raw
counts must have zero skips, flaky cases, or global errors before a GREEN claim.

## Failure modes and required follow-ups before implementation completion

- Restore cards before focus and scroll; bound extent replay and prevent loops.
- Preserve native cached DOM without duplicate continuation requests.
- Abort replay when the main generation, URL, profile, or destination changes.
- Preserve R04 canonical identity validation, grouped Show cards, deduplication,
  cancellation, request correlation, and ordinary pagination fallback.
- Revalidate current library visibility; handle removed/renamed titles and
  changed ordering with a safe, accessible fallback rather than stale content.
- Bound and validate session state. Reject malformed, oversized, external,
  non-browse, and duplicate/unknown-query contexts before navigation or loading.
- Do not reopen playback, add history entries during replay, or clobber native
  Back/Forward behavior. Modified/new-tab clicks must not replace the old tab's
  return context.
- Keep state tab-local and session-only. Query strings must not enter persistent
  caches or diagnostic logs. Logout clears only Q14-owned state.
- Different Viewer Profiles must not reuse another viewer's private query,
  title, or scroll. Initial fixture cases assert `local-owner` consistently;
  this is not cross-profile runtime proof. A disposable public-profile control
  is required before final implementation acceptance.
- Storage denial, offline continuation errors, navigation during pending replay,
  empty results, and failed/recovered loading need dedicated test-first cases
  after the initial public RED establishes the feature boundary.
- Assert exact focus role and actual pointer/keyboard reachability. Observe
  native smooth scrolling settle rather than measure a mid-animation rectangle.

## Ownership and limits

Only this analysis, new browser test/support files, and a new opt-in Go fixture
are authorized now. Q09's frozen branch and all 250 artifacts stay intact.
`pwa-browse-return.js` and coordinated production/template seams remain gated
on runtime RED and the parent's explicit go-ahead. Production, CI, CSS, Player
progress/audio/subtitle, identity/auth, Android, and Apple sources are untouched.
Native BFCache admission, media decode, different-profile enforcement, other
browsers/devices, required integration gates, merge, and ancestry are pending.

MAIN: NO — test preparation only; runtime grant and implementation are pending.
