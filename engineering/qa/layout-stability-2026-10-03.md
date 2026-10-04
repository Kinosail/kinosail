# Responsive layout stability

## Failure analysis before implementation

The public interface is the rendered Player and Subtitles web UI. The reported failure is content changing position after an initial paint, without a user requesting a layout change.

Failure paths to measure:

- A late Manrope font changes text width, line wrapping, header height, and card placement.
- Deferred scripts reveal, move, or remove controls after HTML has painted.
- Settings initialization hides categories and inserts description text after first paint.
- Artwork, video metadata, and missing artwork change intrinsic geometry.
- HTMX pending state changes target minimum height, text width, grid sizing, or button width.
- Delayed, failed, aborted, and retried requests leave busy state or skeletons behind.
- Empty or error feedback appears above content without reserved geometry.
- Narrow, landscape, enlarged text, reduced motion, and keyboard flows expose wrapping and focus failures.

Measure unexpected LayoutShift entries separately from entries with recent input. Record element rectangles over time because recent-input filtering can conceal a visibly jerky pending state. Resize and explicit navigation are intentional layout changes; delayed changes after those settle are not.

Use native disposable Go servers on supported loopback HTTP, small generated media, and synthetic accounts. Delayed responses retain real server bodies. Injected error responses and emulated device APIs must be labeled. No production media, credentials, deployment, TLS bypass, or physical-device claims.

## Evidence

Baseline native Player measurements: `.verification/layout/20261003T191945Z/player/measurements.json`.
At 390px, aggregate unexpected shifts were 0.70048 for bookmarked Connections, 0.05269 for Movies, and 0.03206 for default Settings. At 1440px, bookmarked Settings measured 0.54017. These initial values are aggregate shifts, not session-window CLS.

The first implementation's CLS score concealed a wrong category at first paint. The final runner records initial/final visible sections, document-coordinate element boxes, supported timing APIs, and CLS session windows. Playwright screenshot font-readiness can itself force an optional-font substitution. Chromium pixels therefore use CDP capture, and measurements precede non-Chromium screenshot capture.

Independent review found and corrected template-definition matching, a library ID selector applied to Settings, a Subtitles control-mode regression, and native panel sizing. A real Subtitles run found the in-flow toolbar removed after the 2.2-second idle timer, shrinking the player 69.6px and moving primary actions. Normal inline toolbars now stay visible; theater/fullscreen overlay controls retain idle behavior.

Fonts use `font-display: optional`: slow cold loads retain the system fallback for that navigation. Warm loads still use Manrope. This changes typography availability, without delaying content rendering.

Repeatable frozen-source command:

```sh
KINOSAIL_LAYOUT_ENFORCE=1 KINOSAIL_LAYOUT_FLOWS=1 KINOSAIL_LAYOUT_VARIANTS=1 KINOSAIL_LAYOUT_APPLE_SHIM=1 python3 scripts/testing/test-layout-stability-local.py
```

Chrome is the default. Set `KINOSAIL_LAYOUT_BROWSER=webkit` or `firefox` for those engines. `KINOSAIL_LAYOUT_QUICK=1` selects 390/1440; omit it for 320/390/844/768/1024/1440/1920. Receipts record exact switches and source drift. The desktop-UA iPad capability shim is synthetic and does not establish physical-device or Safari proof.

Final matrix and artifact references will be appended after the frozen-source runs. No production origin, Nox, live playback, manual deployment, TLS bypass, or container/image operation is part of this verification.

## Transition failure analysis

An encoded fragment can select Connections during initial projection, then switch to Playback when deferred code reads the raw fragment. Malformed escapes must retain a safe default. A theater/fullscreen idle timer can leave the normal in-flow toolbar hidden after exit. A later pointer event then restores its height. Measure both transitions with real server pages before changing their behavior.

## Inspector and initial fragment failure analysis

The native inspector already computes a subtitle Review before rendering HTML. Empty quality and cue containers grow after a delayed duplicate API request. Initial known review content must render with the HTML, using escaped values and the same first 40 cues as the interactive view. Preserve no-subtitle content, warnings, pagination, track choice, and later edits. Test request failure and recovery without removing the existing review. Native Subtitles bookmarks also wait for deferred scripts before scrolling; align the first paint to the requested fragment, including closed disclosures and malformed-escape safety.

A source review also found that editing during an inspector refresh invalidates its response and can leave Preview unavailable. Lock only form inputs while that real refresh is pending, using the existing busy-state restoration. Preserve outer operation locks, recover on errors, and re-enable edits after the response. The native browser flow checks this actual pending state before choosing a synthetic local file.

## Hosted enlarged-text failure analysis

At d42ef0e77, native Chromium measured a 10.8px Player toolbar growth when PiP became available at 200% root text size. The 44px empty slot does not account for the scaled icon and button padding. Reserve those intrinsic dimensions with relative units. A personalized mobile tab replacement also grew the dock by 24px; project the saved/default destinations while parsing HTML, retaining the full More menu until the interactive editor initializes.

Firefox screenshots show Subtitles' large watch title exceeding its container and mobile bottom-navigation labels overlapping at 200%. Allow words and labels to wrap inside their real grid cells, without clipping content. Add overflow offender rectangles to the real-server reports before selecting any further fixes. Chromium's same-size Subtitles cases passed, so retain cross-engine proof rather than inferring equivalence.

The first hosted WebKit run exited before measurements. Add a bounded stage/error-class artifact, excluding raw error messages, URLs, cookies, tokens, and process logs. Preserve partial flow results on failure. Diagnose authentication/transport before choosing an HTTP or trusted-TLS harness; verification must remain enabled.

Independent source review identified focus loss when the currently focused language selector is disabled during its own refresh. Record the focused control and restore it after success/failure only if the user has not moved focus or clicked elsewhere. Real browser flows cover retained focus and an explicit Tab away. Verify both early More customization and the later parsed Settings customization button, including Escape returning focus.

The safe 914660d88 WebKit projection records login POST 303, zero retained cookies, and a login-redirect timeout for both Servers. Chromium and Firefox authenticate on supported loopback HTTP; this WebKit run does not retain the secure authentication cookie there. A proposed native HTTPS harness is held pending parent confirmation because installing a generated CA changes runner trust. Its scope is one disposable GitHub Linux runner, one public CA certificate per app, verified hostname/chain, and removal after each Server. It is not activated or executed. Never export authority/private-key files or disable verification.

## Enlarged dock and early interaction failure analysis

The 914 Firefox enlarged-text screenshot shows bottom-navigation labels wrapping into one or two letters per line. Intrinsic columns can become two rows, exceeding the existing 7rem content reservation. Measure the actual dock border box before paint and on resize; reserve that height plus breathing room, including keyboard scroll padding. Check the last control above the dock at 320/390 with 200% text and short landscape. Keep content and document scrolling available. These real Server flow assertions are added before the new dock initializer.

An early Customize proof must hold the main bundle response until the dialog and Escape/focus checks finish. A pending unrelated supporter script is insufficient evidence. Exclude native option/optgroup rectangles from the bounded offender list while preserving the document overflow gate; use the next hosted Firefox render to identify the remaining 56px Settings overflow. The global language picker minimum of 13rem is a source candidate, not a verified diagnosis.

The transport-controlled early interaction flow blocks service workers explicitly so the held main-bundle response cannot come from its worker cache. Initial-page cases retain normal worker behavior. The barrier releases in a finally block. Dock proof checks both the final focusable control and its content-block tail, since trailing text can hide an insufficient reservation. Actual Settings uses .settings-shell; reservation applies to every dashboard main, not only .subtitle-main. New timeout bookmarks cover Player #session-timeouts and Subtitles #security after their integration.

The 35f hosted run found a retained navigation assertion that identifies the main nav by its opening attribute order. Keep the semantic aria-label first while retaining the dock marker. The Subtitles flow incorrectly opened the legacy Movies adapter; use the real subtitle Library route and record when that surface has no HTMX search. Firefox still reports 56px of document overflow at 200% Settings, while every reported non-option viewport rectangle remains within the screen. Before a CSS repair, record horizontal scroll position, document-coordinate edges, and bounded scroll-container geometry; a scrolled viewport or native control's internal overflow can conceal the responsible element.

The 35f Chromium enlarged-dock flow reaches the last link, but native focus scrolling places its bottom at 322px under a dock starting at 248px in 844×390 landscape. Root scroll padding alone does not prevent this. Correct focus visibility after native focus scrolling, scoped to mobile dashboard controls, without trapping focus or changing content geometry. The last link precedes trailing text, so focusing it can intentionally scroll away from the page end. Measure focused-control visibility first, then explicitly scroll to the end and measure the content tail; record both states and the actual reserved padding. A single rectangle after those two different actions cannot establish both requirements.

Independent review identifies an asynchronous focus-restore boundary: inspector refresh deliberately restores focus with preventScroll after a response. A global focusin adjustment would override that contract. Run visibility correction only after an unprevented Tab action actually changes focus, leaving programmatic restoration and mouse actions untouched. Inspect focus on the next animation frame, after native Tab navigation has completed.

The 5e Firefox probe rules out horizontal scrolling: scrollX stays zero. Trusted HTTPS's heading has a 272px border box but 387px scroll width; its text extends to document x=446, matching the 56px root overflow. Wrap long Settings heading words rather than clipping them. Native multi-select internals have overflow hidden and do not match the document edge. The preserve-scroll fixture must retain equal content geometry: switching to a language with no installed sidecar removes review rows and clamps the document's old maximum scroll. Install identical synthetic English/French sidecars, select that second installed language, and require a real offscreen scroll plus unchanged workspace height.

## Required startup fixture boundary

CI 37167849218 failed its existing Adopt initialization poll after successful cold, warm and direct playback. Its safe receipt has no terminal-event acknowledgement or scheduler admission state, so the cause is not confirmed. Source coordination with the startup owner found the fixture pauses/navigates without awaiting its buffered trace response. An active trace lasts 35 seconds, exceeding the unchanged 20-second init gate and 30-second preparation queue lifetime. Observe the actual current session's accepted playing/first-moving-frame event, arm its pause response before pausing, and await status 204 before navigation. Also await its pagehide session-end acknowledgement and verify the next page has no old current media. Do not inject trace bodies, reset admission, extend the init gate, or alter the later active-playback priority rejection. Record only enum/count/timing acknowledgement evidence.

A prior pause's concurrent response can arrive later than its subsequent playing trace. Match terminal sequences newer than all observed current-session requests, as well as item/session/event/paused/status, to exclude that stale response. These receipts establish HTTP delivery, not scheduler admission; the unchanged init and active-priority gates independently test scheduler behavior.

The unchanged startup fixture passed on ed44 in required CI 37168585657. The earlier failure is intermittent; the observed request sequencing gap remains a fixture weakness, not a confirmed production scheduler fault. The explicit delivery boundary repair requires fresh hosted proof.

The ed44 Firefox 320px enlarged dock flow exposes another boundary: Settings retains a 352px content column and scrollX becomes 19.2px when focus scrolls. Its single 1fr grid track retains the content's automatic minimum. Allow that responsive track and its sections to shrink, with readable word wrapping; add the flow's document-overflow gate and 320/844 enlarged initial Settings cases. The inspector preserve-scroll check reaches document maximum and changes by 21px although its workspace height stays equal. Measure an interior reading position 160px before the end, keeping the focused selector offscreen; end-of-page availability has its separate dock-tail check.

The c762 Firefox initial cases show the outer Subtitles Settings shell, not only its inner track, retains twelve columns. Eleven 1rem gaps alone need 352px at 200% text, wider than the measured 281.6px main. Make that mobile shell one column. Its header actions also retain nowrap and extend beyond 320px; allow those actions to wrap within the real header. Player's DLNA status badge extends to x=354.45 in the same viewport. Bound and wrap that scoped badge. Keep all text and controls available.

The c762 startup acknowledgement fixture fails before its first navigation. A direct media.pause() bypasses the public keyboard playback-intent path; use the actual K shortcut after establishing current playback. Preserve matching pause/session-end responses and record bounded current-session request/response event enums if an acknowledgement fails. This is a fixture boundary repair; the failure receipt does not establish a production cause.

Subtitles' native Library does not always have an active HTMX header search. Counting a hidden or inactive legacy header input is insufficient. Record actual visibility and HTMX availability before selecting that flow, and label absent or inactive surfaces explicitly. Other native inspector, keyboard and dock flows must still run.

The c762 Chromium preserve-scroll flow still moves by 21px at an interior position, with equal final workspace height. The temporary one-line loading status precedes that workspace, while the known loaded status uses two lines. Preserve known status text during refresh and expose its pending state through aria-busy and a loading label. Measure the status border box through the request, rather than masking the movement with a scroll reset.

Chromium also records a 0.04088 DOM-attributed shift on one motion-enabled Provider bookmark, 30.8ms after first paint and before the chosen initial sample. Scheduling fragment projection on the next animation frame can miss the paint in which the target is parsed. Project synchronously when that target is present in a DOM mutation callback; do not hide content or wait for unrelated scripts.

Independent review requires stylesheet/dock readiness and enough parsed document height for the desired alignment; otherwise an early scroll clamps and cannot retry. Use parser completion at readyState interactive as the fallback, before deferred bundles finish. The narrow enlarged Provider screenshot also places its heading near the dock: root 9rem scroll padding combines with the section's 72px-plus-1rem margin. Use measured mobile Subtitles header height plus one section margin, and record the actual bookmark heading/header/dock geometry. Clear the transient loading label when a draft action invalidates a refresh or an error is published.

Review also traces a slow-stylesheet boundary: a nonempty reservation can still describe unstyled navigation. Require both header and dock reservations to match current rendered rectangles after stylesheet readiness, and retry when those reservations update. Add delayed real CSS responses to bookmark journeys. A missing known bookmark must fail its presence gate; only explicit malformed or unknown negative cases may remain unresolved.

The native Subtitles Library uses its own fetch navigation through #subtitle-search. Its feedback region precedes the existing content and starts hidden. Add real delayed success, injected transport failure, and empty retry journeys before deciding whether that region moves known content. Measure header, heading, filters, and existing content through pending work; new search results may legitimately change after the user requests them. Do not describe an inactive HTMX header input as coverage of this native path.
