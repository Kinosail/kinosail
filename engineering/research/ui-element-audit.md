# UI element audit — 2026-09-05

Status: component-family audit and nine fixes complete within the tested scope below. A passing screenshot or source search is not proof of every possible state.

## Design intent

Review the three monorepo web apps and Player native component families. Help household members find media or services and let Owners configure them confidently. Priority: primary task, current state, then secondary configuration. Preserve Player's cinematic media hierarchy, Dashboard's network board, and Subtitles' coverage ledger. Use existing tokens, icons, native semantics, and compact task groups. Keep essential actions visible at 320px and allow vertical scrolling; show pending work and actionable recovery without discarding entered values. Avoid decorative glass panels, anonymous icon-only actions, and replacing a usable native control solely for novelty.

## Coverage ledger

Distinct families, not repeated list rows, are the audit unit. Source review, rendered geometry/accessibility, interaction tests, and hardware evidence are separate.

| Family | Source / rendered evidence | Status |
| --- | --- | --- |
| Dashboard setup, login, passkey offer, password and token controls | `apps/dashboard/e2e/dashboard.spec.ts` | Full browser run: 60 passed, 15 skipped |
| Dashboard navigation, search/command dialog, filters, view modes, app cards, drag/reorder, edit/remove/restore, settings and import | Same populated workflow at desktop, tablet and phone sizes | Save recovery passed across three browsers; inventory rerun: 25 passed |
| Player entry pages, shell, browse/filter/sort/search, media cards, playback and recovery, lists, settings families | `apps/player/e2e/layout-audit.spec.ts` | Broad run: 97 passed; all 8 failures passed on a 9-test rerun. Final playback: 8 passed; startup connection reset passed on isolated rerun |
| Player API keys, Media Shares, MFA/passkeys, imports, MCP approval, supporter and transient states | `apps/player/e2e/conditional-states.spec.ts`; exact production templates at five widths | 15 passed across three browsers; fixtures do not prove external authentication |
| Subtitles coverage/wanted ledger, item actions, language ordering, providers and onboarding | `apps/subtitles/e2e/subtitle-dashboard.spec.ts` | 13 browser tests passed |
| Subtitles inherited account/share/import/approval states and populated supporter badges | `apps/subtitles/e2e/conditional-states.spec.ts` | 14 passed; Firefox 320px timeout passed on isolated rerun |
| Native setup/approval, home shelves/cards, detail, playback, action buttons and screen recovery | `apps/player/apps/native/src/components` | 61 unit tests passed; five-width browser run passed; recovery passed in three browsers |

## Findings

1. Dashboard Save board had no pending indication or repeat-submission guard. App saves disabled their button but not implicit form submissions. Preserve action-specific progress text, expose busy state, ignore repeats, and restore controls on failure.
2. Subtitles' populated supporter hero overflows at 720px. Fix the layout constraint, not page-level overflow hiding.
3. Playback choice descriptions inherited a two-column field layout. The settings panel could extend behind the title or collapse to a thin strip on phones. Give descriptions their full width, bound the scroll region, keep Close visible, and reserve room below the video. Test a usable panel height, real selection changes, keyboard recovery, and clearance from the next action row.
4. Theater mode inherited the embedded video's aspect ratio. Recovery also retained a normal-page margin. Keep the theater stage at full viewport height.
5. Subtitles' Media Share player exceeded 320px. Reuse Player's explicit shrink constraints for the shared-media container and video.
6. Native Connect could submit twice through the keyboard while its button was disabled. Guard the operation itself, retain the URL, show Connecting with the spinner, and expose busy state. Decorative spinners are hidden from the accessibility tree; standalone loading states have names.
7. Native approval failures still said Waiting. Show that polling stopped and provide Retry approval without generating another device code.
8. Native empty libraries asked people to refresh without providing a control. Add Refresh library through the existing loading operation. Align the viewer name with the account buttons.
9. Screenshot review found white header text on white in desktop light-theme playback settings. The offscreen automated scan missed this. Use semantic surface/text tokens throughout the panel. Scroll the header into view before checking contrast and save its screenshot.

## Element decisions

| Element family | Decision and evidence |
| --- | --- |
| Rails, bottom navigation, section links, account and overflow menus | Retain product-specific navigation; exercise compact Owner/Viewer routes, keyboard access and reflow. |
| Primary, secondary, confirmation and icon buttons | Keep clear action labels and native semantics; repair pending/recovery behavior. Check accessible names and target geometry in the populated workflows. |
| Text, password, numeric, URL and multiline fields | Retain native input types, labels and existing validation. Preserve entered values on failures. No server validation rules were weakened. |
| Radios, checkboxes, segmented choices and cards | Keep the earlier selection-control modernization; distinguish selection from focus. Fix inherited playback-card layout. |
| Long or dynamic selects | Retain where appropriate; a custom popup is not automatically more usable. See `modern-selection-controls.md` for the primary-source rationale. |
| Search, filter chips, command search and result lists | Retain task-specific search/filter patterns; test keyboard selection, no-results recovery and compact layouts. |
| Dialogs, disclosures and scrolling panels | Keep native dialog/details behavior. Repair playback panel bounds and persistent dismissal; verify open and closed states. |
| Media cards, shelves, artwork and detail metadata | Preserve media-first composition and useful density. Review populated desktop/phone layouts, long labels and missing-artwork fallbacks. |
| Tables, ledgers, service tiles and activity history | Keep compact data views instead of turning every row into a decorative card; test responsive navigation and row actions. |
| Status, progress, loading, success and error messages | Keep text alongside status color; correct stale waiting text, unnamed progress, repeated submissions and missing recovery actions. |
| Video/audio controls, playback settings and recovery | Retain platform controls where appropriate. Verify policy switching, bounded panels, alternate display modes and simulated decode-failure recovery. Hardware decoding is a separate boundary. |
| Account, sharing, permission and integration states | Review actual templates and local fixtures, including one-time output and denied/recovery states. External approvals are not certified by those fixtures. |
| Theme, focus, reduced motion and forced colors | Reuse existing semantic tokens; keep the established dark media identity and each app's information hierarchy. Do not add decorative effects or new UI dependencies. |

## Reproducible per-element evidence

`scripts/testing/ui-element-inventory.ts` records the DOM elements in each tested state: tag, identifier, label evidence, visibility, disabled/checked/expanded/busy/focus state and dimensions. It deliberately excludes field values, URLs and arbitrary status text. It is DOM evidence, not a computed accessibility-tree replacement.

The Dashboard workflow, Player route/conditional matrix, Subtitles conditional matrix and native-web audit save `*-elements.json` beside screenshots. Repeated rows and viewport variants are not counted as unique components. Each screen also retains its existing accessibility and interaction assertions. Native web fixtures cover setup, connecting, connection error, approval error/retry, populated home, details, playback error/return, library error/retry, empty-library refresh and sign-out.

Rendered review found more than three material weaknesses (findings 2–8). The review kept the cinematic media stage, household board and operational ledgers, and removed no useful information merely to make the screenshots look simpler.

## Visual critique

The anti-ai-slop-ui review required populated screenshots, not only successful tests. Screenshot inspection caught the light-theme header defect after the automated offscreen scan passed.

Reviewed corrections include Dashboard's pending save, native setup and recovery, Subtitles' 720px supporter and 320px share, and Player's light desktop and dark phone panels. The final light panel has readable text, a visible Close action and consistent surface colors. The phone panel scrolls without covering the following actions.

Review judgment for these representative compositions: AI Slop 3/10 and Distinctiveness 8/10. Slop categories are palette 0, layout 0, components 1, typography 1 and decoration 1. Distinctiveness categories are product fit 2, system clarity 2, layout 2, typography 1 and signature interaction 1. These are qualitative judgments, not accessibility metrics.

No new design library, decorative animation or generic dashboard shell was introduced. Remaining texture and typography choices belong to the existing product system.

## Verification boundaries

Server tests passed for Player, Subtitles, and Dashboard. Player and Subtitles changed-path checks passed. Native lint, formatting, types, coverage tests and web export passed.

Dashboard's fast gate stopped at two historical secret-scan findings. This audit does not clear that gate or modify repository history. Browser skips remain skips.

The broad Player run had five stale stylesheet-version expectations, two connection resets during local fixture work, and one timeout. All passed on isolated rerun. Subtitles had one Firefox timeout that also passed on isolated rerun. These are separate runs, not a claimed uninterrupted green suite.

Native playback failures are simulated; hardware decoding remains unverified.

The final panel-color change received focused Player asset/player tests and Subtitles changed-path checks. The earlier full server tests preceded that final CSS-only change.

Final playback tests covered policy changes, dismissal, dark/reduced-motion, light contrast, forced colors and theater recovery across Chromium, Firefox and WebKit. Subtitles' final 320px and 720px fixture rerun passed both tests. Native's five-width browser run passed five tests; its final simulated playback/recovery run passed three browser projects.

Local run logs use `/tmp/kinosail-element-*.log`. Ignored screenshot and JSON evidence remains in each app's test output directories. Player final playback evidence is in `apps/player/.kinosail-test/final/theme-verified` and `theme-chromium`; conditional evidence is in `conditional-verified`. These artifacts are not committed.

Do not infer real passkey/MFA/provider approval, screen-reader behavior, native hardware playback, deployment, or production health from template fixtures or browser automation. Physical iPhone/iPad/TV, VoiceOver/TalkBack, real external provider approval and production revision/health were not verified in this task. Supporter and Home Assistant standalone repositories were outside this monorepo audit.

The ledger covers listed component families and tested states. It is not a claim that every possible data combination, browser setting or device was exercised.
