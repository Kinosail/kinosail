# Application-wide quality-of-life audit

Research snapshot: 2026-08-24.

This report reviews Kinosail's current web surface and identifies evidence-backed quality-of-life improvements across discovery, playback, administration, and recovery. It combines a route/code inventory with the populated test-instance browser journey; it is not a moderated usability study or an implementation specification. Real household and device testing should still validate and reprioritize the candidates before implementation.

## Executive recommendation

Kinosail does not need a wholesale redesign. Its strongest foundation is already present: server-rendered pages, plain media terminology, visible search, useful Home shelves, progressive infinite loading with cancellation, native media controls, passkeys and Quick Connect, explicit owner/viewer boundaries, safe diagnostics, and one-container operation.

The largest remaining quality-of-life gains come from seven cross-cutting changes:

1. **Make location and return predictable.** Reduce the global destination overload, preserve browse/search/sort/scroll/focus state, and give every detail page a reliable route back to the exact catalog position.
2. **Treat known-title lookup as the primary fast path.** Rank exact and title-prefix matches first, support tolerant normalization, and add locale-aware A–Z jumping only to title-sorted long lists.
3. **Make every background operation legible.** Scans, metadata refreshes, downloads, transcodes, backups, imports, DVR work, and connection tests need consistent queued/running/succeeded/failed states with progress where knowable and a next action on failure.
4. **Create a real ten-foot interaction mode.** The current responsive pages are not yet a directional-focus system. TV and keyboard use need explicit focus grouping, arrow-key movement, focus restoration, large targets, predictable Back, and remote-native playback behavior.
5. **Separate everyday viewing from owner work.** Keep Home and details media-first; turn the very long Settings surface into an overview plus task-oriented sections, with high-risk actions and diagnostics one level deeper.
6. **Design recovery as a user journey.** Preserve entered form data on errors, make failures actionable, offer undo where safe, verify offline/download/backup outcomes, and connect diagnostics to concrete recovery steps.
7. **Add a Linear-inspired control layer for frequent users.** Keep visible controls authoritative, then add a context-aware command menu, discoverable shortcuts, consistent selection/focus, and per-view display preferences. Project the same actions differently for touch and D-pad rather than forcing desktop controls everywhere.

The preferred product pattern is **progressive enhancement around the existing API-driven monolith**: the same application operations remain authoritative for API and web adapters; HTML links, forms, and pagination remain the fallback; HTMX or small browser scripts add live results, status, focus restoration, and remote navigation.

## Evidence model and limits

This report deliberately separates evidence from guidance:

- **Measured evidence** means a controlled experiment, transaction-log study, large observational/quasi-experimental study, or measured first-party case study. It can justify priorities or hypotheses, but its population and task may not be identical to a private media server.
- **Normative guidance** means a web standard or accessibility requirement.
- **Platform guidance** means first-party Apple, Android, Unicode, Chrome, or other official product guidance. It establishes expected interaction conventions, not measured superiority.
- **Kinosail inference** means a design conclusion drawn from current code plus the evidence/guidance. These require validation with Kinosail users and representative devices.

No source found directly compares every proposed pattern inside a self-hosted movie/TV/music/book server. In particular, the evidence for alphabetic jumping comes from mobile contact lists, and known-item search evidence comes from library catalogs. Those are useful analogues, not proof of the exact Kinosail design.

## Linear benchmark: adopt the control model, not the density

**Decision: yes, selectively.** Linear is a useful benchmark for making a large capability set feel fast, but Kinosail serves phones, televisions, remotes, and casual household viewers as well as keyboard-heavy Owners. The transferable pattern is one action system exposed through several inputs, not a desktop shortcut clone.

Linear's official documentation distinguishes global search (`/`) from the context-aware command menu (`Cmd/Ctrl` + `K`), supports current-view search (`Cmd/Ctrl` + `F`), preserves per-view display choices, mirrors many keyboard actions in contextual menus, and lets focused items be navigated and previewed without losing list context. [Linear Search](https://linear.app/docs/search), [Display options](https://linear.app/docs/display-options), [Contextual menus](https://linear.app/now/invisible-details), [Peek](https://linear.app/docs/peek)

Kinosail should adapt that model as follows:

- `/` focuses global media search. `Cmd/Ctrl` + `K` opens a separate command menu for navigation and available actions such as **Go to Movies**, **Open Downloads**, **Scan libraries**, **Show playback settings**, or **Add focused item to My List**. Search and commands must not become one ambiguous result list.
- Commands are derived from the same authorized application operations as visible buttons and API routes. Viewer/Owner permissions, validation, confirmations, step-up authentication, and audit behavior do not change because an action came from the palette.
- Arrow keys move a clearly focused card or row; Enter opens it; Escape closes transient UI and restores focus. A shortcut-help surface (`?`) lists only actions available in the current context. Single-letter shortcuts must never fire while typing or operating a form/control.
- Each poster/detail row keeps a visible **More actions** control. Pointer context menus and shortcuts are accelerators that teach one another, never the sole path. Touch long-press may enhance the menu but cannot be required.
- Per-Profile display options may remember grid/list density, sort, grouping, and hidden empty groups, with an explicit reset. Shared Owner defaults are a later choice; a viewer changing density must not silently alter the household experience.
- A lightweight preview that preserves browse position may be valuable on desktop, but Linear's Space-to-Peek shortcut should not be copied directly: Space already has playback and web-control meanings. Test a labeled preview action and D-pad detail pane instead.
- On TV, expose the same command inventory through Search, Jump, and focused **More** menus with large targets and predictable Back. Do not require command chords, hover, right-click, or memorized letters.

This is a P1 cross-app accelerator. Search ranking, browse-state restoration, A-Z jumping, form recovery, playback failure handling, and the TV focus foundation remain P0 because they help people who will never learn a shortcut.

## Current application snapshot

The review covered the route inventory and public adapters in `internal/server`, the embedded HTML in the home, player, detail, Live TV, downloads, authentication, settings, backup, and system views, the shared CSS and browser scripts, and the Playwright/accessibility journey coverage.

### Existing strengths to retain

- Home already puts **Continue watching**, **My List**, **Recently added**, and **Recently played** ahead of the full catalog, with explicit destinations for media types.
- The library query is URL-addressable and supports search, view, sort, limit, and offset; infinite loading retains a **Load more** fallback and now cancels stale requests.
- Search already covers title, show, year, plot, genres, director, studio, artist, album, and cast.
- Show, album, and book pages use strong primary actions; the player retains native audio/video controls and exposes quality, trickplay, chapters, skip markers, audio, subtitles, casting, Watch Together, downloads, playlists, and collections.
- Setup is short, sign-in supports password managers, passkeys, TOTP, OIDC, and Quick Connect, and viewing-history migration is explicitly optional and preview-first.
- Settings distinguish externally managed values from GUI values, include hardware/transcoder tests, and protect sensitive operations with step-up authentication.
- Downloads, backups, viewing imports, scans, maintenance, and diagnostics already have application operations and API seams that a better UI can project rather than reinvent.
- Existing tests cover API/web parity, authorization, representative phone/laptop/TV geometry, axe-core checks, the main owner journey, populated routes, and the infinite-scroll stale-response regression.
- For this audit, a freshly regenerated populated instance completed the five targeted Chromium journeys covering direct and compatible playback, every media section, local metadata, phone-width beta surfaces, and adaptive-quality controls. Its separate verification script also passed Movies, Shows, Music, Audiobooks, Books, Photos, Live TV, playback, API inventory, and local metadata enrichment. This proves reachability on the fixture, not subjective ease, real remote behavior, or production-library performance.

### Current friction signals from the code

These are code-review observations, not live-user findings:

- The top navigation exposes Home, My List, nine media/curation destinations, Live TV, Unwatched, and History in one horizontally scrolling row, followed by install/connect/sync/settings/profile/sign-out actions. The destination hierarchy is hard to scan on compact and remote-driven layouts.
- Every poster in Home/library results is marked `loading="lazy"`, including likely first-viewport/LCP artwork, and most do not declare intrinsic dimensions. This can delay the first meaningful artwork and increases layout-shift risk.
- Live search replaces `#main`, but the current templates do not expose a visible result count/status adjacent to search, result ranking rationale, suggestions, or a clear button. Results are grouped by type, while the underlying match is a simple case-insensitive substring over a combined string.
- Infinite loading improves continuity, but there is no title index/jump API, explicit position indicator, or documented restoration of the exact loaded extent/focused card after returning from a detail page.
- `/watch/{id}` combines detail, player, curation, social playback, cast, owner metadata/marker forms, and technical information in one long page. Powerful controls are present, but priority changes sharply between couch playback and owner maintenance.
- The player uses browser-native controls plus a separate trickplay range and additional actions below the media. Native controls are robust, but captions/audio/quality/recovery are distributed across several regions and remote focus behavior is not specified.
- `/settings` is one large page containing security, playback, providers, transcoder, remote access, integrations, appearance, profiles, migration, libraries, scanning, analysis, cache, sessions/devices/API keys, backup, and maintenance controls. Anchor links help, but task scope and save feedback vary.
- Many web form failures become a generic localized error response. That loses the original form context and can force re-entry even though server-side validation is strong.
- The offline-download page polls and reports state/error text, but does not show bytes/percentage, storage impact, queue order, pause/retry, expected tracks, or a verification distinction between “rendition prepared,” “file saved,” and “proven playable offline.”
- Live TV currently lists channel cards and a linear next-24-hours program list. It does not yet project an electronic program guide, current-time axis, favorites, program details, recording actions/conflicts, or return-to-current-channel behavior.
- Playlist/Collection curation is one item at a time. Playlist order exists in the API, but the web page offers neither reorder controls nor multi-select/batch addition; deletion is permanent after confirmation rather than undoable.
- System and backup pages expose valuable data, but the presentation is largely flat text. “Needs attention” does not yet decompose into severity, affected capability, likely cause, next action, or a safe support bundle/reference ID.

## What the research actually supports

### Measured evidence

| Finding | Evidence boundary | Kinosail implication |
| --- | --- | --- |
| Across four transaction-log studies totaling more than 4.2 million library-search sessions, users tended to issue short queries, stay on the first result page, and frequently conduct known-item searches; title/title fragments were common query material. [Schultheiß et al., 2020](https://searchstudies.org/wp-content/uploads/2021/07/Transaction_logs_preprint.pdf) | Academic library systems, not movie libraries; log data cannot explain every intent. | Search must return strong likely-title matches immediately. Exact, normalized, prefix, then substring ranking should beat one undifferentiated substring order. |
| A mobile name-locating experiment comparing slide scrolling, a scrollbar, and an alphabetic index found the alphabetic index fastest regardless of target position; subjective ratings improved with use. [Li et al., 2023](https://doi.org/10.1080/10447318.2022.2041883) | Contact-name lookup on phones, not posters or TVs. | For title-sorted, sufficiently large results, A–Z is a useful secondary accelerator. It should not replace search or appear for Newest/Year sorts. |
| In an eight-menu experiment, lower hyperlink information scent sharply reduced success and increased first-click time. [Tselios, Katsanos, and Avouris, 2009](https://dl.ifip.org/db/conf/interact/interact2009-2/TseliosKA09.pdf) | Small experimental website and tasks. | Preserve plain destination and action labels; simplify navigation by grouping, not by inventing branded names. |
| A quasi-experimental analysis of 23 million video views found abandonment began increasing beyond about two seconds of startup delay; each additional second was associated with a 5.8% abandonment increase, and rebuffering reduced play time. [Krishnan and Sitaraman, 2013](https://doi.org/10.1109/TNET.2013.2281542) | Commercial Internet video traces from 2010-era players; exact thresholds may drift and LAN viewing differs. | Measure click-to-first-frame, failures, rebuffer time, and recovery. Player polish cannot compensate for slow startup or stalls. |
| Chrome reports that back/forward navigations are common, and its bfcache guidance explains that restoration can be nearly instant while retaining page state; a measured Yahoo! JAPAN News rollout improved navigation behavior and mobile revenue. [Chrome bfcache guidance](https://web.dev/articles/bfcache) | News pages, not private media; revenue is irrelevant to Kinosail. | Preserving catalog position and making Back instant is a high-frequency QOL target worth explicit tests and field measurement. |
| Core Web Vitals thresholds combine user research and ecosystem achievability: at the 75th percentile, LCP no more than 2.5 s, INP no more than 200 ms, and CLS no more than 0.1 are the “good” targets. [Chrome threshold methodology](https://web.dev/articles/defining-core-web-vitals-thresholds) | General web experience thresholds, not playback QoE. | Add field-capable browser measurements for Home, browse, detail, settings, and player startup separately. |

### Normative and platform guidance

- WCAG 2.2 requires keyboard operability, visible and unobscured focus, consistent navigation/identification, text error identification, accessible authentication, alternatives to dragging, minimum target sizing with exceptions, and programmatically determinable status messages. [WCAG 2.2](https://www.w3.org/TR/WCAG22/)
- WAI guidance says dynamic search counts, successful submissions, busy states, errors, and progress should be programmatically available without stealing focus; `role="status"` is appropriate for polite updates and `role="alert"` for important time-sensitive errors. [Status Messages](https://www.w3.org/WAI/WCAG22/Understanding/status-messages), [ARIA22](https://www.w3.org/WAI/WCAG21/Techniques/aria/ARIA22)
- WAI requires text identification of invalid fields, and its guidance notes that generic native validation can be transient, expose only one error at a time, and lack useful correction advice. [Error Identification](https://www.w3.org/WAI/WCAG22/Understanding/error-identification)
- WAI explicitly recognizes WebAuthn, password-manager autofill, and paste as ways to reduce cognitive authentication burden. [Accessible Authentication](https://www.w3.org/WAI/WCAG22/Understanding/accessible-authentication-minimum.html)
- Apple recommends brief, optional onboarding, contextual instruction, and postponing nonessential setup. [Apple Onboarding](https://developer.apple.com/design/human-interface-guidelines/onboarding)
- Android TV says D-pad navigation must be efficient, predictable, intuitive, and supported by unmistakable focused/pressed states; Apple distinguishes focus from activation and expects all tvOS controls to be reachable. [Android TV navigation](https://developer.android.com/training/tv/get-started/navigation), [Android focus system](https://developer.android.com/design/ui/tv/guides/styles/focus-system), [Apple focus and selection](https://developer.apple.com/design/human-interface-guidelines/focus-and-selection/)
- Unicode CLDR defines locale-specific collation indexes, multi-character index labels, script boundaries, and special buckets; a hard-coded English A–Z is not globally correct. [Unicode LDML collation indexes](https://www.unicode.org/reports/tr35/tr35-collation.html#Collation_Indexes)
- Apple says live content must look live, playback should be one action away, an EPG should make current channel/program/time obvious, and long guides need easy paging/scrolling/jumping plus favorites. [Apple Live-viewing apps](https://developer.apple.com/design/human-interface-guidelines/live-viewing-apps)
- WCAG requires a single-pointer alternative to dragging; its example for sortable lists is adjacent Up/Down controls. [Dragging Movements](https://www.w3.org/WAI/WCAG22/Understanding/dragging-movements)
- Android's offline-first guidance emphasizes immediate local reads and explicit reconciliation after connectivity changes. [Android offline-first architecture](https://developer.android.com/topic/architecture/data-layer/offline-first)
- Apple recommends determinate progress when progress is knowable and an indeterminate indicator otherwise. [Apple Loading](https://developer.apple.com/design/human-interface-guidelines/loading)
- Chrome says not to lazy-load likely in-viewport/LCP images and recommends explicit dimensions; its Core Web Vitals guidance identifies unknown image dimensions as a common CLS cause. [Browser image lazy loading](https://web.dev/articles/browser-level-image-lazy-loading), [CLS](https://web.dev/articles/cls)
- RFC 9457 standardizes machine-readable HTTP problem details and says human detail should help the client correct the problem without leaking implementation internals. [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457.html)
- OWASP recommends application-level operational and security events, correlation identifiers, testing logging failure, and excluding access tokens, passwords, session IDs, keys, and sensitive paths/data. [OWASP Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html)

## Recommended experience by area

### 1. Information architecture and global navigation

**Recommendation:** use one stable content hierarchy and one utility hierarchy.

- Primary content destinations: **Home, Movies, Shows, Music, Live TV**.
- Secondary content under **More**: Audiobooks, Books, Photos, Collections, Playlists, History, Unwatched.
- Utilities remain visually separate: Search, Downloads, Connect, Viewer/Account; Owner-only Status and Settings.
- Do not remove deep links. `/` query URLs and existing detail routes remain canonical; navigation is only a clearer projection.
- In compact touch mode, prefer a small bottom destination bar only if it does not obscure content or focus; otherwise use a conventional labeled menu. On TV, use an edge rail that opens with focus and closes predictably with Back. On wide pointer layouts, a quiet sidebar or two-row header can work.
- Display the active location consistently. Use breadcrumbs only for real hierarchy (for example Settings → Playback), not as decoration.

**Why this fits Kinosail:** the current horizontally scrolling navigation asks one row to serve content type, curation, history, state filters, and utilities. Grouping preserves all capabilities while improving information scent and reducing remote traversal.

**Acceptance evidence:** first-click task tests for Movies, Downloads, Live TV, Viewer Profile, and Transcoder settings on phone, laptop, and D-pad; no route becomes more than one additional activation from its current path; all destinations remain reachable without JavaScript.

### 2. Home, library browse, and search

**Recommendation:** support both known-item retrieval and exploratory browse.

- Keep Search continuously visible on desktop and one activation away on phone/TV. Add a clear control, a visible/announced result count, and explicit “Searching…”/failure states.
- Rank results by: normalized exact title; normalized title prefix; title token prefix; alternate/sort title; title substring; show/artist/album/person; plot/genre/studio. Preserve deterministic tie-breaking. Highlighting match text is optional and must not create inaccessible markup.
- Normalize case, Unicode, punctuation, spacing, and leading articles without making titles with meaningful articles impossible to find. Treat typo tolerance and phonetic matching as later, measured candidates; they can produce surprising false matches.
- Keep result types grouped when multiple kinds match, but put the strongest likely known item at the top. Every result should show title, year, type, and one discriminating cue.
- For title-sorted long lists, add a locale-derived alphabet index. On pointer/touch it can be a sticky rail or an accessible “Jump to” letter grid; on TV use a large modal/grid reachable from a **Jump** action. Disable or omit empty buckets. Never show the index for Newest or Year sorting.
- Back/Forward must restore query, view, sort, loaded pages, scroll position, and the previously focused card. URL state remains shareable; transient focus/scroll can live in `history.state`/session state with bfcache-safe behavior.
- Keep finite pagination as the semantic fallback. Infinite loading should stop at a bounded DOM or use measured rendering containment; it must announce page/result progress and retain manual **Load more** when observation/script is unavailable.
- Add active filter chips only when real filters exist (genre, year range, watched state, rating, library). Chips must have textual remove actions and serialize to the API/query contract.
- On Home, keep Continue watching first when present, then My List/Recently added. Deduplicate the same item across early shelves when this improves variety; do not insert opaque promotional ranking above user-owned state.

**Acceptance evidence:** task completion and time for “find The Grinch,” “find a movie with Jim Carrey,” and “browse G titles” across small/1,000/20,000-item fixtures; locale fixtures including non-Latin scripts and leading articles; keyboard/remote focus restoration; zero stale append after query changes; search first useful result latency and result relevance checks.

### 3. Media detail pages

**Recommendation:** make the first viewport answer “what is this?” and “what can I do?”

- Present one dominant **Play**, **Resume**, **Read**, or **Listen** action. If progress exists, pair **Resume at …** with a quieter **Start over** action.
- Keep year, runtime, rating, genres, synopsis, and the most useful episode/album/book context adjacent to the primary action. Technical codec/container information stays disclosed lower on the page.
- Use consistent secondary actions and order: My List, Download, Watch Together/Cast, More. Reflect state in the label and accessible name.
- For shows, add season navigation, episode number/runtime/progress, Next Up, watched state, and a clear resume point. Collapse seasons only when the control remains searchable and remote-operable.
- Move Owner tools into an explicit **Manage media** route/panel so a couch viewer does not traverse metadata and marker forms after playback controls. Keep the same application operations and authorization.
- Preserve the originating browse context in the Back link and browser history instead of always returning to `/` or a generic media-type root.

**Acceptance evidence:** first-activation rate for Play/Resume, back-position restoration, no nested interactive elements, 200% zoom/reflow, and owner controls absent for viewers.

### 4. Playback controls and recovery

**Recommendation:** retain native playback as the reliable baseline, then add one coherent Kinosail control layer for capabilities native controls do not expose well.

- Make the media stage the visual priority. Avoid loading cast/admin sections before playback-critical resources.
- Consolidate Quality, Audio, Subtitles/Captions, Playback speed, and Chapters into one labeled options surface that works with pointer, keyboard, and D-pad. The native element remains usable if enhancement fails.
- Offer explicit Resume/Start over, 10-second seek, previous/next episode or track, subtitle/audio selection, and a readable current/remaining time on remote/keyboard layouts. Do not bind standard browser/system shortcuts unexpectedly.
- Keep trickplay preview coupled to the active seek control rather than presenting a competing timeline without synchronized semantics. Provide text time updates for assistive technology without announcing on every frame.
- Treat playback as a state machine visible to the user: preparing direct stream, preparing compatibility stream, playing, buffering, reconnecting, failed. Show recovery actions that match the plan: retry, switch to compatibility, use Original, lower quality, or open diagnostics. Do not expose raw FFmpeg/HLS errors.
- When autoplay is enabled, show a cancellable countdown and identify the next item. Respect reduced motion and never trap remote focus.
- Captions, subtitle kind/language, forced/default state, and audio language/descriptive audio must be accurately labeled. Preserve the viewer's preference but allow per-item override.
- Measure click-to-first-frame, seek-to-frame, startup failure, rebuffer ratio, quality switches, fallback success, and progress-save latency locally without logging title/path/token data in general metrics.

**Acceptance evidence:** native-controls-only fallback; keyboard and D-pad matrix; actual media on LAN and constrained network; subtitle/audio/quality switching; resume and stale-progress tests; HLS failure and recovery; captions and reduced-motion accessibility; click-to-first-frame budgets derived from current baseline rather than unverified claims.

### 5. TV and D-pad focus

**Recommendation:** create a first-class ten-foot mode, not only a 1920×1080 responsive screenshot.

- Establish focus groups: global rail, shelf, grid, detail actions, player controls. Arrow keys move spatially inside a group; Tab can move between groups for keyboard users; Enter/Select activates; Escape/Back closes the current layer or returns one level.
- Make focus unmistakable over arbitrary artwork with an outline plus separation shadow/scrim; focused scale is supplementary and must reserve space to avoid clipping/CLS.
- Restore focus to the invoking card/button after closing dialogs, returning from details, loading more results, or completing an action.
- Do not make focus trigger playback, navigation, or destructive changes. Focus and activation remain separate.
- Use at least the platform-appropriate large target/focus envelope for TV; Apple lists 56×56 pt as its minimum tvOS control size. [Apple Accessibility](https://developer.apple.com/design/human-interface-guidelines/accessibility)
- Test odd grids, one-item shelves, row ends, disabled controls, long localized labels, RTL direction, dialogs, dropdowns, and the browser Back key. Every focus move must have a destination and a way out.

**Acceptance evidence:** an automated keyboard traversal can catch missing focus/loops, but completion requires couch-distance tests with real remotes/gamepads and age-diverse participants. Record moves/activations to complete representative tasks and focus-loss incidents.

### 6. Onboarding, authentication, and account recovery

**Recommendation:** keep the required path short and make readiness explicit.

- Required setup should remain Owner name/password plus at least one strong sign-in factor. Defer migration, remote access, metadata providers, tuner sources, and appearance; present them later as an owner checklist.
- After Owner creation, explain why a passkey is recommended, the canonical trusted address it belongs to, and what alternative/recovery route exists. Detect wrong-origin/private-CA failures and link to the exact corrective step.
- Make “Skip for now” semantically accurate. If a security policy makes a factor mandatory, say **Choose another method** rather than implying the user can enter the library unsecured.
- Preserve password-manager/autofill/paste contracts and offer password visibility only as an explicit control. Do not add composition gimmicks beyond the server's actual password policy.
- Group sign-in choices by likely success: passkey when available on the canonical origin, password/TOTP on trusted LAN, OIDC when configured, Quick Connect for another device. Keep errors generic enough to avoid account enumeration but actionable for rate limits, origin mismatch, expired code, or unavailable provider.
- Recovery codes need a clear save/copy/download acknowledgement, count remaining, and regeneration path. Never show them again after the one-time reveal.
- Add a post-setup readiness overview: Library connected, first scan state, playback test, recovery backup, remote access (optional), Viewer Profiles (optional). Every incomplete item links directly to its task.

**Acceptance evidence:** fresh owner flow on phone/laptop/TV, password manager, passkey canonical/alias origins, TOTP and recovery-code paths, provider outage, rate limit, session expiry/step-up return, and screen reader announcements. No required secret is logged or placed in a URL.

### 7. Owner settings and forms/errors

**Recommendation:** turn Settings into an overview plus focused task pages.

- Overview cards: Server, Libraries, Playback & Transcoding, Access & Security, Viewers, Integrations, Backups, System. Each card shows one health summary, source-of-truth badge where relevant, and a clear open action.
- Keep an advanced all-settings/configuration view for sophisticated operators, but do not make it the only path. Environment/YAML-managed values remain visible and locked with a direct “change this in … and restart” explanation.
- Each form gets a persistent heading, concise consequence text, current/effective value, one primary Save action, and success feedback. High-impact actions state scope before confirmation.
- On validation failure, rerender the same form with safe entered values, an error summary linking to invalid fields, inline text suggestions, `aria-invalid`, and focus on the summary/first invalid field. Password/token fields are deliberately blank and explained.
- Prefer undo for low-risk reversible actions (remove from My List, dismiss Continue watching, playlist membership). Use confirmation for rare, irreversible/high-impact actions (delete profile, revoke key, kill remote access) and step-up authentication where already required.
- Do not auto-save settings with operational consequences. Immediate local preferences such as theme can save immediately with polite status feedback.
- Standardize API errors on a bounded problem-details shape or the existing API error envelope with stable machine codes, field pointers where useful, localized human detail, request ID, and no internals/secrets.

**Acceptance evidence:** negative tests for every changed field and no side effects, browser tests that entered safe values survive rejection, API/web parity, keyboard/error focus, externally managed fields, step-up return, and double-submit/idempotency behavior.

### 8. Downloads and offline

**Recommendation:** distinguish preparation, transfer, verification, and actual offline readiness.

- A download action should show expected quality, size estimate, available/required storage when a client can know it, included audio/subtitle tracks, and whether it is Original or Optimized.
- The queue should expose queued/preparing/ready-to-save/transferring/paused/failed/complete/verified states, determinate bytes or percent when known, and an indeterminate state only while total work is unknowable.
- Offer retry and cancel/remove; add pause/resume only when the underlying transfer semantics make it truthful. Preserve completed immutable bytes and exact range behavior already researched for official Jellyfin clients.
- “Ready offline” should mean the consuming client can open the downloaded media with networking disabled, not merely that the server finished an FFmpeg job. In the web/PWA path, label server-prepared jobs as **Ready to save** unless the browser has actually retained and verified the file.
- Add a **Trip check** that reports local file presence, length/digest when the client supports it, expected embedded tracks, last verified time, and a network-off playback test where possible.
- Show Wi-Fi/charging scheduling only in native clients that can enforce it; the web server should not pretend to control device background policies.

**Acceptance evidence:** large-file interruption/resume, source replacement, server restart, insufficient disk, failed transcode, permission revocation, airplane-mode playback, audio/subtitle selection, Android/iOS official clients, and web/PWA behavior. Report device boundaries explicitly.

### 9. Live TV and DVR

**Recommendation:** replace the linear guide with a current-time-oriented EPG while retaining a simple list fallback.

- Start Live TV with **On now**, Favorites/Recent channels, and a one-action Watch path. Mark live content textually and visually.
- Add a two-dimensional EPG: channels vertically, time horizontally, sticky channel/time headers, a “Now” line, jump to Now/date/time, page by time block, and large remote-focusable program cells. The existing linear list remains the narrow-screen/no-script alternative.
- Preserve playback in PiP/background only where the browser and user intent support it; otherwise make return to the current channel one action.
- Program detail exposes Watch, Start over when supported, Record, Record series when supported, and Favorite in a consistent order. Show schedule conflicts and concurrent-stream limits before committing.
- DVR separates Scheduled, Recording, Ready, Failed, and Watched. Include storage usage/retention, cancel/delete consequences, and a direct path from a failure to tuner/storage diagnostics.
- Channel change must provide immediate focused/selected feedback while the stream starts, then transition to playing/buffering/error state.

**Acceptance evidence:** D-pad traversal across time/channel edges, Now jump, DST/time-zone boundary, missing guide data, tuner outage, concurrent-stream conflict, schedule/cancel API parity, recording storage failure, and return-to-live latency.

### 10. Playlists and collections

**Recommendation:** make curation fast without relying on drag.

- Add multi-select/batch Add to Playlist/Collection from browse/search and a compact membership picker on details.
- Manual playlists support reorder via drag as an enhancement plus explicit Move up/down/top/bottom or position controls. Announce the new position and retain keyboard focus on the moved item.
- Support Play all, Shuffle, and Start here; show total duration/remaining items where available.
- Keep smart-playlist rules human-readable, preview the matched count before saving, and explain why an item matches. Validate conflicting/unknown rules before side effects.
- Make low-risk membership removal undoable. Deleting a playlist/collection may remain confirmed, but a recoverable trash/undo window is better if the persistence model can guarantee it.
- Distinguish Collections (editorial grouping) from Playlists (ordered queue) in labels, layouts, and available actions.

**Acceptance evidence:** 1/100/1,000-item lists, duplicate prevention, reorder concurrency, keyboard/single-pointer alternatives, undo expiry, viewer/owner authorization, smart-rule negative tests, and API/web parity.

### 11. Accessibility, responsiveness, and input parity

**Recommendation:** expand the existing axe/layout gate into behavior coverage.

- Keep skip links, headings, landmarks, native controls, accessible names, and page language. Give repeated navigation/forms unique accessible labels.
- Meet WCAG 2.2 AA across focus not obscured, 24×24 CSS px minimum target or sufficient spacing, reflow at 320 CSS px/200% zoom, text/non-text contrast, reduced motion, captions, status messages, accessible authentication, and dragging alternatives.
- Adopt a larger product baseline—about 44 CSS px touch and 56 px TV—without claiming it is the normative WCAG threshold.
- Never rely on hover, color, artwork tint, spatial position, gesture, or drag alone. Keep labels visible for unfamiliar or consequential icons.
- Test keyboard, mouse, touch, D-pad/gamepad, screen reader, zoom, forced colors, reduced motion, RTL, and long localized strings. Automated axe checks are necessary but not sufficient.
- Define component behavior by available space and input mode, not only phone/desktop width. Avoid hiding essential actions just because the viewport is narrow.

**Acceptance evidence:** automated semantic/accessibility checks plus manual VoiceOver/NVDA/TalkBack sampling; full keyboard and D-pad journeys; no traps; stable focus after HTMX swaps; no horizontal page scroll at 320 CSS px; forced-colors focus remains visible.

### 12. Responsiveness and perceived performance

**Recommendation:** establish explicit product budgets and make progress visible without lying.

- Measure Home/browse/detail/settings at p75 separately on representative low-end phone, laptop, and TV browser: LCP ≤2.5 s, INP ≤200 ms, CLS ≤0.1 are starting web targets, not claims.
- Add media-specific measures: search response, back-to-browse restoration, click-to-first-frame, seek-to-frame, buffer duration, download throughput, scan-to-visible-item, and backup completion.
- Eager-load only first-viewport artwork/LCP candidates; lazy-load below-fold art. Emit width/height or aspect-ratio and responsive image sizes so poster grids do not shift or fetch backdrop-scale assets.
- Keep the initial DOM bounded. If `content-visibility:auto` or virtualization is adopted, measure it with accessibility and browser Back/focus restoration; do not sacrifice semantic fallback for theoretical speed.
- Use bfcache-friendly lifecycle events and restore stale-sensitive data on `pageshow` as needed. Never leak a signed-out viewer's private page through restoration.
- For actions longer than a moment, acknowledge immediately. Use determinate progress when total work is known and a truthful named phase otherwise. Disable only the initiating control, not the whole page, and prevent duplicate side effects server-side.
- Skeletons are optional; reserved poster/text geometry and real early content are preferable to decorative shimmer. Honor reduced motion.

**Acceptance evidence:** cold/warm/LAN/constrained runs, populated fixtures, RUM-capable local telemetry without content identifiers, image-byte audit, long-task/INP trace, bfcache eligibility and sign-out privacy, and regression budgets in CI where stable.

### 13. Notifications, status, recovery, and diagnostics

**Recommendation:** add one owner-facing status model, not a collection of unrelated toasts.

- A small global Owner status indicator shows only actionable states: healthy, work running, needs attention. Viewer-facing notices are limited to capabilities that affect the viewer now.
- A Status page groups Library, Playback/Transcoder, Downloads, Live TV/DVR, Remote Access, Integrations, Storage, Backups, and Security. Each issue states: affected capability, severity, since/last attempted, likely cause, safe detail, and next action.
- Use polite inline status for success/progress; use alerts only for urgent time-sensitive failures. Avoid repeated poll announcements and “alarm fog.”
- Background jobs need a common projection and correlation/request ID across API, UI, activity journal, and safe diagnostics. This does not require a new service; it can be a shared application model in the monolith.
- Make the safe diagnostic bundle previewable: list exactly what is included/excluded, expiration if any, version/build, health summaries, redacted recent failures, and correlation IDs. Never include tokens, secrets, bearer URLs, passwords, full media paths, or private viewing history by default.
- Backups should show last success, last verification, age against policy, destination reachability, and a **test restore**/dry-run path. A recovery guide should explain where Library Content lives, what state is restored, expected downtime, and how to verify success.
- Error pages need a plain explanation, stable request ID, Retry/Back/Status action as appropriate, and no raw internal error. Repeated failures should link to the relevant diagnostic slice, not dump logs into the viewer UI.

**Acceptance evidence:** injected failures for database, disk full, permission, FFmpeg, tuner, metadata provider, remote TLS, webhook, backup destination, and audit logging; correct severity/action; screen-reader announcements; redaction assertions; recovery drill from a disposable fixture.

## Prioritized candidate backlog

Priority reflects likely user value and cross-app leverage, not implementation size. Each item still needs issue-level scope and API/web acceptance criteria.

### P0 — highest-value friction and trust

| Candidate | Why now | Smallest useful slice | Proof before calling complete |
| --- | --- | --- | --- |
| Known-title search ranking + clear/count/status | Search is already global; ranking is currently undifferentiated substring order. | Shared search operation returns deterministic relevance groups; web announces count/state and offers clear. | Relevance corpus, API/web tests, stale-response test, phone/TV keyboard tests, measured latency. |
| Browse-state and focus restoration | Returning from details is one of the most repeated library actions. | Preserve URL, loaded extent, scroll, and focused card across Back; keep pagination fallback. | bfcache/non-bfcache, sign-out privacy, deep-link and 20k fixture tests. |
| Locale-aware A–Z title jump | Directly answers the long-list “Grinch → G” need. | Title sort only; API bucket metadata + jump cursor; wide inline and compact/TV letter grid. | CLDR locale fixtures, empty/script buckets, search interaction, D-pad and screen reader. |
| Async operation status contract | Scans/downloads/backups/imports/tests currently report state differently. | Shared queued/running/succeeded/failed projection with time, progress/phase, safe error, retry link. | API/web parity, cancellation/retry/idempotency, live-region behavior, failure injection. |
| Form error recovery | Generic error pages turn validation into re-entry work. | Rerender one high-value settings form with error summary/inline advice and safe value preservation; generalize the proven seam. | Negative/no-side-effect tests, focus/screen reader, secrets blank, API machine error. |
| TV focus foundation | A TV-sized viewport test does not prove remote usability. | Explicit focus styling/grouping/restoration for Home → search/browse → detail → player. | Real D-pad/gamepad plus automated traversal and no dead ends. |
| Playback startup/failure UX and measurement | Measured stream quality strongly affects behavior; Kinosail already has fallback plans. | Visible preparing/buffering/recovery states; click-to-first-frame and fallback outcome metrics; retry/compatible/original actions. | Actual media, constrained network, failure injection, no sensitive telemetry. |

### P1 — complete core journeys

| Candidate | Smallest useful slice |
| --- | --- |
| Navigation hierarchy | Five primary destinations, More, separate utilities; same routes and fallback links. |
| Linear-inspired command/action layer | Separate `/` search from a context-aware `Cmd/Ctrl` + `K` menu; visible More actions, shortcut help, focus restoration, and permission-identical operations. |
| Detail-page primary action and contextual Back | Resume/Start over plus state-preserving return; move Owner tools behind Manage media. |
| Settings overview/task pages | Health summaries and direct links without removing advanced configuration. |
| Download truth/progress | Prepare vs Ready to save vs Verified offline, bytes/percent, retry, tracks/size. |
| Live TV EPG and current-channel return | On Now/Favorites, Now-centered guide, program detail and record action. |
| Playlist batch/reorder/undo | Multi-add, Play all/Shuffle, drag enhancement with explicit move controls. |
| Setup readiness checklist and recovery | Essential setup only, exact passkey-origin help, backup/recovery readiness. |
| Status/diagnostics actionability | Capability health, severity, next action, safe support bundle/request IDs. |
| Image delivery/CLS budget | Eager first viewport, dimensions/aspect ratios, responsive artwork sizes. |

### P2 — validate before committing

- Typo-tolerant/fuzzy/phonetic search; first build a misspelling and false-positive corpus.
- User-configurable Home shelf order/density; first observe whether stable utility-first defaults fail real households.
- Hover/selection preview panels and autoplay previews; they add bandwidth, motion, focus, and surprise-audio risk.
- Full grid virtualization; first measure DOM/render cost after image sizing, bounded paging, and `content-visibility`.
- Native-like background download controls in the PWA; browser capabilities and platform restrictions may make promises inconsistent.
- Personalized recommendations; Kinosail can improve deterministic state-owned shelves before collecting or inferring more viewing signals.
- Global toast/inbox system; first prove that the shared status model cannot provide contextual feedback without notification noise.

## Recommended delivery order

1. Instrument current browse, search, Back, player startup, and long-running operations on a populated disposable instance.
2. Implement the shared status/error vocabulary and browser focus/return conventions; these are prerequisites for many individual screens.
3. Deliver known-item search ranking and locale-aware A–Z through the shared library operation, with API and web coverage.
4. Simplify navigation and split Settings using existing routes/operations rather than rewriting the application shell; then project those same routes and authorized actions through the command menu.
5. Build and test the ten-foot focus model on Home, browse, detail, and player before expanding it to owner forms.
6. Improve downloads, EPG/DVR, playlists, and diagnostics using the common operation/status patterns.
7. Re-run the full populated browser/accessibility/container gate and conduct moderated task tests; reprioritize P2 from evidence.

## Research and validation plan

### Task-based usability study

Recruit at least three groups rather than one undifferentiated sample: frequent Owner/operators, ordinary household Viewers, and remote/TV users including older or low-vision participants. A small formative round can reveal severe failures, but do not turn it into statistical proof.

Representative tasks:

1. Find and play **The Grinch** by the fastest method you choose.
2. Return and play a different movie near the previous position.
3. Find an unfamiliar movie by person/genre, then add it to My List.
4. Resume an episode, change subtitles, seek, recover from a simulated compatibility failure, and stop autoplay.
5. On TV, open Live TV, find what is on now, schedule a future recording, and return to the current channel.
6. Prepare a movie for a trip and decide whether it is genuinely ready offline.
7. Create and reorder a playlist without drag.
8. As Owner, diagnose a failed scan, run a backup, and identify what a recovery would restore.

Capture completion, error/abandonment, time, first wrong action, Back usage, D-pad moves, focus loss, search terms/reformulations, confidence, and qualitative explanation. Do not capture titles from private live libraries; use neutral fixtures.

### Comparative experiments worth running

- Search ranking: current substring order versus exact/prefix-weighted order.
- Long title browse: continuous scroll versus A–Z versus search, split by phone and TV and by near/far target.
- Navigation: current full horizontal row versus primary + More.
- Settings: monolithic anchors versus overview + task pages.
- Player recovery: raw/generic failure versus state-specific recovery choices.

Use within-participant counterbalancing where practical, the same fixture and network conditions, and preregister the primary measure to avoid selecting a favorable metric afterward.

### Engineering verification layers

- Shared application/domain operation tests, including strict input validation and no-side-effect negatives.
- API response/OpenAPI inventory and error-contract tests.
- Server-rendered web-adapter tests, including fallback without JavaScript.
- Browser tests for HTMX swaps, Back/Forward, scroll/focus restoration, D-pad keyboard semantics, cancellation, status announcements, and reduced motion.
- Axe plus manual assistive-technology and zoom/forced-colors checks.
- Populated 1k/20k catalog benchmarks, browser memory/DOM/INP traces, and race tests for changed concurrency.
- One-container test, real official Jellyfin clients where compatibility is claimed, and real phone/TV device QA.

## Decision summary

The highest-confidence direction is a **search-first, state-preserving, status-explicit Kinosail** with locale-aware A–Z as a secondary accelerator and a first-class D-pad focus model. It improves the whole app without importing a large client framework or weakening the API-driven monolith.

The most important design discipline is to avoid promising more than the underlying operation knows. “Ready,” “healthy,” “saved,” “offline,” and “playing” must each have one explicit, testable meaning across API, web, diagnostics, and device clients. That consistency is the quality-of-life improvement on which the more visible polish depends.
