# Accessibility audit — 2026-09-26

## Scope and standard

This audit covers the current Player and Subtitles web apps, plus the Player clients for Apple and Android devices. Web review uses [WCAG 2.2 Level AA](https://www.w3.org/TR/WCAG22/) as the target. Native review uses the [Apple accessibility guidance](https://developer.apple.com/design/human-interface-guidelines/accessibility) and [Android accessibility guidance](https://developer.android.com/guide/topics/ui/accessibility/testing). The [WCAG evaluation method](https://www.w3.org/TR/WCAG-EM/) informs the evidence and limitations below.

## Evidence

| Surface | Review and verification | Result |
| --- | --- | --- |
| Player web | Populated Chromium instance at phone and desktop widths, including axe checks, keyboard journeys, responsive layout, forced colors, and recovery paths. `make -C apps/player test-instance-check` | 53 tests passed. No violation in the states checked by axe. |
| Subtitles web | Populated Chromium instance with keyboard, high contrast, 200% reflow, responsive layout, and recovery journeys. `make -C apps/subtitles test-instance-check` | 19 tests passed. No failure in the checked states. |
| Player on iOS, iPadOS, and tvOS | Source review of labels, VoiceOver announcements, progress values, Dynamic Type layouts, loading semantics, motion settings, and playback controls. | No source-level defect confirmed in this review. No assistive-technology session was run. |
| Player on watchOS | Source review of remote controls, status, and heart graph; `./scripts/build-apple.sh watchos`. | Dynamic messages lacked an announcement; the remote and heart graph now post announcements while their pages are visible. Simulator build passed; no VoiceOver session was run. |
| Player on Android phone, TV, and Wear OS | Source review of Compose semantics, selection labels, focus, motion, and asynchronous status; Android unit suites. | Missing status announcements found and fixed on phone and TV. Focused semantics tests and app, watchcore, and wear unit suites passed. Wear code was reviewed and its unit suite passed. |

## Finding and change

Asynchronous Android playback, connection, library, photo, and reader errors appeared as text without live-region semantics. A TalkBack user could miss those statuses while focus stayed elsewhere. These notices now have polite live regions, consistent with the existing playback notices. Compose semantics tests check the rendered playback and reader errors. This does not replace listening to the notices on a device with TalkBack enabled.

Watch remote and heart-graph messages could also change away from VoiceOver focus. The visible page now posts an accessibility announcement when its message changes. The watch build checks this code, but a watchOS VoiceOver session is still needed to hear the timing and content.

## Conformance boundary

These results do **not** establish full WCAG 2.2 AA conformance or certify every native screen. Axe and source inspection cover only part of the criteria and states. This run did not include Firefox or WebKit, a complete manual WCAG criterion review, VoiceOver, TalkBack, Switch Control, keyboard-only TV remote traversal on hardware, large-text device inspection, or deployed production content. Run those checks before making a product-wide conformance claim.

Android Wear lint passed. App lint produced no accessibility findings, but the task failed on six existing API-level errors in `CastApi.kt` (`Instant` calls require API 26 while the app minimum is 23). Those calls exist in `origin/main`; this accessibility change does not address that separate compatibility issue.

## Follow-up — 2026-09-27

The follow-up targets the same product scope and WCAG 2.2 AA web target. It extends the populated browser review to Firefox and WebKit, adds Android emulator accessibility checks, and inspects available Apple simulators. The earlier Chromium-only boundary above describes the 2026-09-26 run, not this follow-up.

### Confirmed defects and fixes

- Firefox and WebKit exposed insufficient text contrast in Player setup under forced colors. The active setup step, ready label, and selected settings destination now use `CanvasText` in that mode. Focused axe checks passed in Chromium, Firefox, and WebKit.
- WebKit allowed a selected native language option to widen the Subtitles settings page beyond a 320 px viewport. The select is width-constrained, and its label clips the select's internal overflow while leaving space for its focus ring. The populated 320 px journey is covered in the browser matrix.
- Wear OS connection, remote, and heart status changes could be missed while focus stayed elsewhere. Visible status text now uses polite live-region semantics. Emulator accessibility checks cover the connection and remote error states.
- Browser test timing and service-worker interception caused unrelated Firefox and WebKit false failures. The affected tests now wait for the destination content and block service workers during mocked supporter responses. The accessibility assertions remain enabled.

### Verification and limits

The browser runs used app source at `e7ffb6a76e473d989a44a793a663f7f08d206e44` on macOS 27.0 with Podman 6.0.2 test containers and Playwright 1.63.0 browsers. The test-instance fixture populated Movies, Shows, Music, Audiobooks, PDF, EPUB, CBZ, Photos, playback, and API inventory. Each matrix command below starts and removes its disposable instance. The Android checks used API 36 phone, TV, and Wear OS emulators.

| Surface | Verification | Result |
| --- | --- | --- |
| Player web | Populated test instance; `KINOSAIL_BROWSER_MATRIX=full make -C apps/player test-instance-check` in Chromium, Firefox, and WebKit. Focused forced-colors axe checks also ran in all three engines. | 149 passed, 10 skipped (10.4 minutes). The contrast, navigation, and supporter checks passed. |
| Subtitles web | Disposable populated container; `KINOSAIL_BROWSER_MATRIX=full make -C apps/subtitles test-instance-check` in Chromium, Firefox, and WebKit. | All 57 tests passed, including the 320 px language flow. |
| Android phone, TV, and Wear OS | Android emulator instrumentation calls `enableAccessibilityChecks()` and `tryPerformAccessibilityChecks()` on setup, errors, and status; a deliberately unlabeled control confirmed the validator reports a failure. `:app:testDebugUnitTest :wear:testDebugUnitTest :watchcore:test`. | Instrumentation and unit checks passed. Manual TalkBack, Switch Access, and remote traversal were not run. |
| iPhone and Apple TV simulators | Installed Player opened to Connect and populated Home. iPhone Connect was inspected with extra large text and increased contrast, then settings were restored. | No visible horizontal clipping was found on the inspected Connect screen. The installed binary's source revision was not confirmed; these are spot checks, not exact-branch or VoiceOver proof. |

This is an expanded engineering audit, **not a claim of full product conformance**. [WCAG-EM](https://www.w3.org/TR/WCAG-EM/) says a subset of pages and functions cannot support a whole-product conformance claim. A complete criterion-by-criterion review of all in-scope pages, states, and processes is still needed. The current checks also do not establish the availability of captions and audio description for every household media item under [WCAG 2.2's media criteria](https://www.w3.org/TR/WCAG22/). Android's [Compose testing guidance](https://developer.android.com/develop/ui/compose/accessibility/testing) calls for manual assistive-technology testing alongside automated checks. Physical-device VoiceOver, TalkBack, Switch Access, Apple TV remote, and deployed-production checks remain separate evidence.
