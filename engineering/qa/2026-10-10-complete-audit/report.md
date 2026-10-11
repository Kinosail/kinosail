# Complete QA follow-up — October 10, 2026

## Scope and run record

This full audit extends [the same-day all-app audit](../2026-10-10-all-apps/report.md), merged in #542. It covers Player, Subtitles, shared packages, tooling, documentation, containers, and the Apple and Android clients. Supporter and Home Assistant implementations remain outside this repository. Their integration boundaries are in scope.

Baseline: `2f62453888c3707587de0875709772290a728106`. All consequential testing uses disposable data. The primary checkout's unrelated Xcode project and localization edits are preserved.

Private working evidence is under `.verification/complete-qa/` in this task's checkout. Raw logs, traces, native results, session data, and device identifiers are not committed. The final evidence index records replay commands and checksums.

## Audit plan

The first pass checks observable outcomes and persistence. The second checks rejected input, roles, boundaries, empty states, terminal failures, and recovery. The third checks keyboard and platform interaction, accessibility, responsive layout, console/network evidence, and resource behavior.

| Surface | Actions and expected outcomes | Adversarial and recovery checks | Interaction and environment checks |
| --- | --- | --- | --- |
| Identity | Owner setup, MFA, Viewer sign-in, Quick Connect, sign-out | Role isolation, malformed requests, expiry, cross-origin requests, no unauthorized effects | Accessible forms, focus, phone and desktop |
| Player library | Browse all media types, search, My List, playlists, collections, persisted progress | Empty, pending, failed, paging and stale data | Keyboard, responsive renders, accessible names and contrast |
| Player playback | Direct First, tracks, captions, seeking, preparation, downloads and offline playback | Decoder failure, retry, damaged cache, cancellation, stale sessions, progress conflicts | Three browser engines, decoded media, phone/desktop, native controls |
| Subtitles | Wanted search, inspection, pairing, save, cleanup, history and restore | Untrusted providers and files, write rejection, missing content, pending/failed save and recovery | Three browser engines, responsive renders and accessibility |
| Settings and integrations | Library, identity, API, MCP, Home Assistant, receiver and update settings | Owner permissions, rejected inputs, outbound restrictions, secret redaction | API and presentation outcomes; simulated external peers identified separately |
| Apple clients | Contract suites, playback, library navigation and watch remote | Validation, transport, authorization, cancellation, failed media and resource state | iPhone/iPad/tvOS simulator; physical-device proof separate |
| Android clients | Production client contracts, phone/tablet/TV journeys and Wear remote | Validation, empty/stale data, playback failure/recovery, saved state | JVM and emulator proof separate; physical-device proof separate |
| Installation and delivery | Installer, independent production images, health and published artifacts | Architecture checks, image scans, provenance, rejected release inputs | Linux AMD64/ARM64, CI, publication and deployment separate |
| Source and tooling | Tests, race/coverage, modules, source cap, workflows and lint | Guard contracts, unsafe input, failure behavior | Full-tree findings distinguished from changed-revision gates |

## Verified evidence so far

- All 143 selected artifacts from #542 match their committed SHA-256 index. Receipt: `prior-evidence-validation.json`. This verifies artifact integrity, not a new execution.
- `make tooling-check` passed. Its 169 CI contract tests include five environment-dependent skips. Other tooling suites and workflow validation passed.
- Complete local iOS contracts passed: 321 tests in 67 suites. Xcode 27.0, iOS 27.0, a disposable iPhone 18 Pro simulator. Result: `ios-tests.xcresult`.
- The production Kotlin client passed eight real-Server replay cases across two repetitions: direct playback/media retrieval, 36-item category pages, 200-item library pages, and 200-item persisted history. Existing decoder unit checks also passed. Receipt: `android-server-contract/receipt.json`. This uses a real Go Server and JVM client, not Android UI or decoder proof.
- Manual deep CI was dispatched for the baseline: [run 38099145590](https://github.com/Kinosail/kinosail/actions/runs/38099145590). Its final outcome is pending.

## Findings and investigations

### QA-001 — Full shared-package gate stops at existing lint findings

`make packages-check` fails with 95 existing lint findings. The categories include context propagation, complexity, exhaustive switches, formatting, security heuristics, and style. This agrees with the earlier audit. No lint rule or threshold was changed.

The reviewed security heuristics include intentional public gateway binding, private-key persistence in the private Owner state, restrictive Unix socket directory modes, and synthetic test cases. These findings alone do not establish an exploitable defect. They remain full-tree gate failures.

### QA-002 — Full static quality stops at existing complexity limits

`make quality-static` passes reviewed dependency hashes, changed-revision Go lint, source caps, TypeScript checks, and browser-script lint. It then fails at Go complexity limits. Later commands in that aggregate target did not run. No threshold was changed.

### QA-003 — iPad reveal gesture targets a zero-size XCTest window

The unchanged loaded playback journey failed twice at the Pause hittability assertion. A diagnostic run measured the application frame as `1032×1376` while `app.windows.firstMatch` was `0×0`. The reveal control covered `1032×1356`. The helper calculated its tap from the zero-size window, so it tapped the screen origin instead of revealing the controls.

Before changing the helper, the test authoring gate established:

- Observable contract: a real background tap restores playback controls, then Pause, Play, and Close work through touch.
- Credible regression: hidden controls fail to return, or a control stops accepting input. All existing functional assertions remain.
- Coverage gap: the current UI journey is the primary owner. No duplicate test is needed. Its coordinate helper, also used by swipe and rotation journeys, must use the application frame.
- Production seam: none. Only XCTest coordinates and frame observations change.

The correction and affected phone/tablet journeys are pending. No production defect or repair is claimed.

An initial focused command used a nonexistent target selector. Xcode rejected it before testing. That command is excluded from passing evidence; the corrected command selects `Kinosail-iOSTouchUITests`.

## Verification boundaries

The audit is in progress. Pending checks are not passes.

- Full browser, race/coverage, security, Android build, and multi-architecture container results are pending in the deep CI run.
- iPad interaction investigation and local performance measurements are pending.
- Physical inventory reports paired Apple TV and iPhone devices, but neither has an active device tunnel. No physical interaction or installation is claimed.
- Paired watch commands, real receivers, network handoff, codec/HDR diversity, long playback, accessibility services, and store distribution require separate environments.
- Source tests, rendered-fixture browser tests, populated-server E2E, simulator interaction, physical interaction, publication, deployed revision, container health, and TLS trust remain separate facts.

