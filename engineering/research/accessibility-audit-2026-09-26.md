# Accessibility audit — 2026-09-26

## Scope and standard

This audit covers the current Player and Subtitles web apps, plus the Player clients for Apple and Android devices. The standalone Dashboard has been retired. Web review uses [WCAG 2.2 Level AA](https://www.w3.org/TR/WCAG22/) as the target. Native review uses the [Apple accessibility guidance](https://developer.apple.com/design/human-interface-guidelines/accessibility) and [Android accessibility guidance](https://developer.android.com/guide/topics/ui/accessibility/testing). The [WCAG evaluation method](https://www.w3.org/TR/WCAG-EM/) informs the evidence and limitations below.

## Evidence

| Surface | Review and verification | Result |
| --- | --- | --- |
| Player web | Populated Chromium instance at phone and desktop widths, including axe checks, keyboard journeys, responsive layout, forced colors, and recovery paths. `make -C apps/player test-instance-check` | 53 tests passed. No violation in the states checked by axe. |
| Subtitles web | Populated Chromium instance with keyboard, high contrast, 200% reflow, responsive layout, and recovery journeys. `make -C apps/subtitles test-instance-check` | 19 tests passed. No failure in the checked states. |
| Player on iOS, iPadOS, tvOS, and watchOS | Source review of labels, VoiceOver announcements, progress values, Dynamic Type layouts, loading semantics, motion settings, and playback controls. | No source-level defect confirmed in this review. No assistive-technology session was run. |
| Player on Android phone, TV, and Wear OS | Source review of Compose semantics, selection labels, focus, motion, and asynchronous status; Android unit suites. | Missing status announcements found and fixed on phone and TV. Focused semantics tests and app, watchcore, and wear unit suites passed. Wear code was reviewed and its unit suite passed. |

## Finding and change

Asynchronous Android playback, connection, library, photo, and reader errors appeared as text without live-region semantics. A TalkBack user could miss those statuses while focus stayed elsewhere. These notices now have polite live regions, consistent with the existing playback notices. Compose semantics tests check the rendered playback and reader errors. This does not replace listening to the notices on a device with TalkBack enabled.

## Conformance boundary

These results do **not** establish full WCAG 2.2 AA conformance or certify every native screen. Axe and source inspection cover only part of the criteria and states. This run did not include Firefox or WebKit, a complete manual WCAG criterion review, VoiceOver, TalkBack, Switch Control, keyboard-only TV remote traversal on hardware, large-text device inspection, or deployed production content. Run those checks before making a product-wide conformance claim.

Android Wear lint passed. App lint produced no accessibility findings, but the task failed on six existing API-level errors in `CastApi.kt` (`Instant` calls require API 26 while the app minimum is 23). Those calls exist in `origin/main`; this accessibility change does not address that separate compatibility issue.
