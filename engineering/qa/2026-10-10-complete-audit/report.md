# Complete QA audit — October 10, 2026

## Scope and run record

This full audit extends [the same-day all-app audit](../2026-10-10-all-apps/report.md), merged in #542. It covers Player, Subtitles, shared packages, tooling, documentation, containers, and the Apple and Android clients. Supporter and Home Assistant implementations remain outside this repository. Their integration boundaries are in scope.

Baseline: `2f62453888c3707587de0875709772290a728106`. The audit crosses midnight UTC but remains October 10 in Denver. All consequential testing uses disposable data. The primary checkout's unrelated Xcode project and localization edits are preserved.

The branch reconciled main at `d25a605134fa11ec35954a1b73851386bb9b58a6`, which adds Android progress-session ownership checks. The reconciliation commit is `6dfdf8dafff0f7f1d6c449efd375802ef03104a7`. Android unit checks were rerun after reconciliation. Web, Go Server, and Apple production source did not change during that reconciliation.

Final reconciliation incorporates dependency update #543 from main `e9f7ae98` in commit `17192f58a0ac004da27d161e7722d6ae6f601587`. The earlier matrix retains its original dependency versions. Focused compilation, Quick Connect replay, and required PR gates validate the reconciled delivery separately. Container rebuild receipts identify the exact source used.

Private evidence is preserved at `/Users/mikeo/Documents/Codex/complete-qa-20261010/`. Raw logs, traces, native results, session data, and device identifiers are not committed. [The evidence index](evidence.md) records commands and checksums. This audit establishes bounded evidence and an explicit backlog; it does not establish that every possible device or journey works.

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

## Results

- All 143 selected artifacts from #542 match their committed SHA-256 index. Receipt: `prior-evidence-validation.json`. This verifies artifact integrity, not a new execution.
- `make tooling-check` passed. Its 169 CI contract tests include five environment-dependent skips. Other tooling suites and workflow validation passed.
- Complete local iPhone contracts passed: 321 tests in 67 suites, with the unsupported simulator PiP callback case skipped. Xcode 27.0, iOS 27.0, a disposable iPhone 18 Pro simulator. Result: `ios-tests.xcresult`.
- All seven iPad touch journeys passed after the coordinate repair. The iPhone touch suite passed six journeys and skipped unsupported simulator PiP. These tests decode synthetic AVC/AAC media and use actual XCTest taps and gestures. Pending first-frame, loaded, failed/retry, close, swipe, edge-back, landscape, and iPad PiP outcomes are asserted.
- The broader iPad contract run passed 319 tests and failed three phone-only orientation tests. A focused repeat reproduced all three failures. These are recorded below, not counted as passes.
- The production Kotlin client passed eight real-Server replay cases across two repetitions: direct playback/media retrieval, 36-item category pages, 200-item library pages, and 200-item persisted history. Existing decoder unit checks also passed. Receipt: `android-server-contract/receipt.json`. This uses a real Go Server and JVM client, not Android UI or decoder proof.
- After reconciliation, all 174 Android app, watchcore, and Wear unit tests passed, including the six progress ownership tests. These Robolectric/JVM checks are isolated checks, not emulator interaction.
- `make -C apps/player performance-test` passed all three existing benchmark repetitions. Scan took about 11.3–11.7 ms; 10,000-item search took 3.4–6.7 ms; progress operations took 0.26–0.59 ms. These measurements are not a playback or UI performance SLA.
- Route inventory and script duplication checks passed. `make max-loc`, `git diff --check`, and the committed native change's `make -C apps/player verify-changed` passed. The latter selected source cap, diff checks, and native compilation. After the CSS commit, `make -C apps/player verify-changed` also passed, including affected Go compilation and native checks.
- The repaired isolated offline hash-recovery owner and its mismatched-manifest neighbor pass, two of two. They verify three-chunk corruption rejection, the visible Resume action, successful retry, and rejection before file requests or storage changes. They use a populated Server for setup and preparation, then synthetic transfer responses.

### Hosted deep run

[Run 38099145590](https://github.com/Kinosail/kinosail/actions/runs/38099145590) executes the baseline with `coverage_diagnostic=true` and `scan_diagnostic=true`.

| Surface | First-attempt result |
| --- | --- |
| Player Go race and coverage | Passed, 89.3% statement coverage, minimum 89% |
| Subtitles Go race and coverage | Passed, 91.5% statement coverage, minimum 89% |
| Shared packages, web tooling, repository tooling and docs | Passed their selected hosted gates |
| Go vulnerabilities, supply chain and CodeQL | Passed; five CodeQL language jobs and findings policy |
| Production images | Player and Subtitles passed on Linux AMD64 and ARM64 |
| Installers and local pipeline contracts | Passed for both apps |
| Android | Compilation, unit tests and lint passed |
| Apple | iOS failed one manifest-delivery assertion; tvOS passed its complete contract suite |
| Browser matrix | All six jobs passed; execution counts below |

The unchanged complete Swift retry passed: 321 iOS tests and 292 tvOS tests, with one skipped case per platform. The deep run's final conclusion is success. The first-attempt failure remains evidence. The hosted runner used Xcode 27.1 beta; local Apple proof used Xcode 27.0.

| Browser | Player passing executions | Player skipped executions | Subtitles passing executions | Subtitles skipped |
| --- | ---: | ---: | ---: | ---: |
| Chromium | 861 | 350 | 99 | 0 |
| Firefox | 857 | 354 | 99 | 0 |
| WebKit | 855 | 356 | 99 | 0 |

These are executions across seven Player batches and one Subtitles batch per engine, not unique journeys. Every batch reports zero unexpected failures and zero flaky results. Player combines populated-container journeys, additional real-Server fixtures, response interception, and rendered fixtures. A passing rendered fixture is not populated-server proof. Skips are not passes; some tests skipped in the main batch pass in a dedicated batch. The private `browser-combined-scope.json` records their combined scope.

Chromium also passed the deep media checks: first-fragment regeneration, cold and damaged HLS cache reopen, copied-video timelines and audio, complete HEVC preparation, source freshness, bounded startup, and phase evidence. Scheduled-only mutation and deep quality metrics were not selected by this manual run.

## Findings and investigations

### QA-001 [P2, open] — Full shared-package gate stops at existing lint findings

`make packages-check` fails with 95 existing lint findings. The categories include context propagation, complexity, exhaustive switches, formatting, security heuristics, and style. This agrees with the earlier audit. No lint rule or threshold was changed.

The reviewed security heuristics include intentional public gateway binding, private-key persistence in the private Owner state, restrictive Unix socket directory modes, and synthetic test cases. These findings alone do not establish an exploitable defect. They remain full-tree gate failures.

### QA-002 [P2, open] — Full static quality stops at existing complexity limits

`make quality-static` passes reviewed dependency hashes, changed-revision Go lint, source caps, TypeScript checks, and browser-script lint. It then fails at Go complexity limits. Later commands in that aggregate target did not run. No threshold was changed.

### QA-003 [P2, repaired] — iPad reveal gesture targets a zero-size XCTest window

The unchanged loaded playback journey failed twice at the Pause hittability assertion. A diagnostic run measured the application frame as `1032×1376` while `app.windows.firstMatch` was `0×0`. The reveal control covered `1032×1356`. The helper calculated its tap from the zero-size window, so it tapped the screen origin instead of revealing the controls.

Before changing the helper, the test authoring gate established:

- Observable contract: a real background tap restores playback controls, then Pause, Play, and Close work through touch.
- Credible regression: hidden controls fail to return, or a control stops accepting input. All existing functional assertions remain.
- Coverage gap: the current UI journey is the primary owner. No duplicate test is needed. Its coordinate helper, also used by swipe and rotation journeys, must use the application frame.
- Production seam: none. Only XCTest coordinates and frame observations change.

Commit `5e2443f8f1f6928306bc56fdc35f4a8b97bf44a9` changes the helper and orientation measurements to use the application frame. The focused loaded journey then passed, followed by all seven iPad journeys and six supported iPhone journeys. No assertion, deadline, or production code changed.

An initial focused command used a nonexistent target selector. Xcode rejected it before testing. That command is excluded from passing evidence; the corrected command selects `Kinosail-iOSTouchUITests`.

### QA-004 [P2, open] — Halstead analysis cannot load three source configurations

The full Halstead command and three focused repetitions fail while loading packages. These are tool failures, not measured complexity scores:

- `hls_copied_process_other.go` excludes Linux, Darwin, and Windows. The quality script selects Windows for `_other.go`, so the file is excluded.
- `apps/player/e2e/compose-template-fixture.go` requires the `q47proof` build tag; the untagged loader finds no selected Go files.
- `scripts/testing/hls-readiness/main.go` relies on the workspace. The typed metric loader disables the workspace and cannot resolve its imports.

Correct platform, tag, and module selection before treating this metric as evidence. Do not skip the files or relax thresholds to obtain a pass.

### QA-005 [P3, open] — Dead-code gate reports existing unused declarations

The separate dead-code command exits one and reports existing declarations across both apps and shared packages. Its private log retains the exact list. No unrelated declarations were removed. Script duplication and route inventory checks, run separately after the static aggregate stopped, pass.

### QA-006 [P2, unverified product impact] — Hosted iOS manifest request never reaches the fixture

`SessionRefreshTests.coldBackgroundAuthorizationUsesMatchingSavedCredentials` fails at `BackgroundAuthorizationTests.swift:273`: the observed request list is empty. Numeric diagnostics show enqueue completing after 17.9 seconds, followed by two ten-second observation windows without URLProtocol admission or request arrival. Polling continues normally. This does not identify the root cause or establish a production authorization failure.

The same contract passes locally on iPhone and iPad. Other hosted background authorization cases pass. The unchanged complete hosted Swift job passed on its second attempt. No timeout or assertion changed. The failure is intermittent and its cause remains unverified; keep its first result when assessing reliability.

### QA-007 [P2, open test scope] — Phone rotation fixtures run on iPad

The complete iPad run and focused repeat fail the same three `PlaybackOrientationJourneys` tests. Two wait for the phone-only landscape button; the denial/restoration case assumes a phone orientation request established prior state. Production `TouchPlaybackView` omits that button on iPad, and `PlaybackOrientation.toggle` requires the phone idiom.

The full suite passes on iPhone. Actual iPad touch rotation, close, and PiP journeys pass. Define the contract suite's phone prerequisites and add an explicit tablet applicability check before changing test selection. No test was disabled to make this audit green.

### QA-008 [P2, environment boundary] — Native BFCache admission is not established

The extra real-Server Chromium browse-return run passes eight cases, then fails the explicit prerequisite that native `pageshow.persisted` is true. A focused repeat fails the same prerequisite. Safe browser diagnostics include `response-cache-control-no-store`, `masked`, and `other` non-restoration reasons. This prerequisite states that its failure is not product RED. The browser cache restoration path remains unproved. Chromium passes 20 cases across normal return, cold return, safety, Home restoration, and watch navigation groups. WebKit passes 18 cases across normal return, safety, Home restoration, and watch navigation. No synthetic lifecycle event substitutes for native BFCache admission.

The first local browse command also lacked this worktree's Playwright installation. It failed before browser execution. Installing the frozen dependencies resolved that setup failure; it is excluded from product failure counts.

Extra local Firefox journeys stop before reaching the app: Firefox reports that it cannot find its profile folder. Reinstall verification and a repeat using a task-owned temporary directory do not resolve startup. Hosted Firefox proof remains valid; the extra local Firefox groups are unproved.

A disposable Linux ARM64 browser container avoids the macOS profile failure. WebKit passes the exact compact-layout and both-theme Quick Connect owners against the final production Server source. Firefox stops before app navigation with `SEC_ERROR_UNKNOWN_ISSUER`, including after a private CA installation policy. Certificate validation remains enabled. Trust changes and the synthetic Server stay inside the disposable container, which was removed after evidence capture.

### QA-009 [P3, design metadata] — Design sidecar is stale

The impeccable context check reports that `.impeccable/design.json` is stale relative to `DESIGN.md`. This affects detector calibration. Refresh it through `impeccable document` in a separate design maintenance pass.

### QA-010 [P2, open; product impact unverified] — Isolated cross-tab download verification misses its deadline

The additional native-browser download run passes 47 cases and fails two of 49. In `download-pause-ownership.spec.ts`, the paused peer's Resume path still shows pending or 49% progress instead of verified completion within ten seconds. A focused repeat fails the paused-peer case again; the new-owner case passes on that repeat. The latter observation is intermittent.

These checks use a real browser, service worker, native locks and storage, with an isolated Node HTTP peer and rendered Downloads fixture. They are not populated-server E2E. The hosted real-Server download batches pass. The focused trace records resumed range responses, but does not establish why verification or its UI acknowledgement misses the deadline. The cause and production impact remain unverified. No assertion, fixture response, or timeout was changed to obtain a pass.

Replay: set `KINOSAIL_DOWNLOAD_PAUSE_ISOLATED=1` and run the existing ownership spec with `--grep 'a paused peer offers Resume|a pending Resume wait preserves the new-owner'`, one worker and zero retries. Exact command, source revision, trace, and range-only diagnostics remain private.

### QA-011 [P2, repaired] — Quick Connect digits overlap compact navigation

At 720×450, the populated Server's manual code fields end at 398.45 pixels. The bottom navigation starts at 385.02 pixels. The existing compact-landscape journey fails twice for this overlap.

The primary regression owner is `layout-audit-library.spec.ts`, “compact landscape shell keeps search and primary actions reachable.” Its unchanged assertions already detect the failure, so no duplicate test or production seam was added. Commit `740fbe99f39a4542f6e11775988d7175d2290741` reduces the form's vertical padding and gaps within the existing short-screen breakpoint. It moves Quick Connect's rules into a focused stylesheet to preserve the 300-line source cap. Touch target sizes, text, and asynchronous behavior remain unchanged.

The unchanged compact-landscape owner passes after the repair. The final focused run passes all 26 layout, mobile Quick Connect, QR rejection, pending-permission and recovery cases. Mocked camera proof remains separate from real camera permission behavior. A new public-interface theme check uses the actual Settings radio control, verifies contrast with axe, and checks actionability at 390×844, 720×450, 1440×900, and 1920×1080 in both themes. It passes on the spacing repair without any foreground change.

Twenty-two additional captures compare empty, entered, pending permission, denied, and recovered states. All measured input heights are at least 47.75 pixels, and trial clicks reach manual entry and Authorize after scrolling. Tall camera states scroll; they are not claimed to fit entirely in a short viewport. Compact empty-state digits end at 366.88 pixels, above navigation at 385.02 pixels. No skeleton or asynchronous production behavior changed. The pre-fix trace and screenshot remain private.

Initial theme-test selectors targeted an obsolete select and an obsolete Dark label. Those setup failures are excluded from product defects. A manual theme override also produced a misleading contrast sample. The actual Theme control passes contrast in both themes; no foreground repair was made.

### QA-012 [environment, resolved] — Host-mounted media stalls the expanded browser fixture

The first expansion completed 75 passing cases before interruption. Two settings cases could not edit the environment-authoritative Server name. Removing that override made all six settings widths pass.

The next expansion completed 87 passes, five failures, and 68 unrun cases. One failure is QA-011. Four page-load failures followed stalled artwork reads. An independent request to the local fixture's movie metadata timed out. Copying the same media files into a task-owned Podman volume restored that request. All five tests covering the affected shortcuts, episode ledger, and static-compression files then passed unchanged against the same pre-fix image.

These failures do not establish keyboard or static-compression product defects. The original traces, failure results, fixture health probe, copy receipt, and successful repeat are preserved. The expansion receipts record the checkout revision; the running pre-fix image is explicitly tied to `6dfdf8dafff0f7f1d6c449efd375802ef03104a7`.

### QA-013 [P2, repaired fixture] — Hash-recovery test follows an obsolete button name

Both preserved runs of “offline download stores and verifies every transfer chunk” pass the first three range assertions and the verification-error assertion. They fail at line 105 while checking that the original “Download to this device” locator is enabled. The screenshot shows the enabled action has changed to “Resume on this device.” The focused trace contains all three expected HTTP 206 transfers before the final assertion. The earlier range-counter diagnosis was incorrect.

Before changing the test, the authoring gate recorded its observable contract and isolation gap. The existing owner must reject a known whole-file hash mismatch, expose Resume, retry every chunk, and produce a verified offline source. Real-Server batches do not inject that precise corruption. Commit `e15d4e47` follows the public Resume action. All range, hash, persistence, and deadline assertions remain. No production transfer logic changes.

The full repaired case and its mismatched-manifest neighbor pass on Linux ARM64 Chromium, using the final production Server source and a private-CA SPKI pin. The latter verifies rejection causes no file request or storage change. Positive traces, results, source hashes, and cleanup receipts remain private. This uses a populated Server for sign-in and preparation, then replaces the transfer script's chunk size and intercepts the manifest and file responses. It is isolated transfer proof, not a real large-file transfer.

### QA-014 [P2, open; cause unverified] — Hosted Firefox Home movement with delayed assets

On commit `740fbe99`, the native-layout Firefox job fails its enforced movement check for the Player Home page at 1440×900. Its 35 rendered-fixture cases pass. The real-Server measurement records multiple moved Home nodes after the initial baseline. Other page invariants and all measured asynchronous flows pass. Subtitles measurements pass. The matching WebKit job passes its 33 fixtures, with two skips, plus both real-Server measurements.

The changed CSS only affects Quick Connect below 900×600. Home rules are unchanged. This does not establish a task-caused regression. The delayed asset measurement requires a focused baseline comparison before changing Home. Preserve the hosted failure and subsequent PR checks separately.

The complete layout workflow on `6905726e` passes all three engines. Its Firefox and WebKit measurements each cover 40 Player pages, 46 Subtitles pages, and 37 asynchronous flows. They record zero moved nodes, unstable pending layouts, or lost focus. This passing repeat does not establish the earlier failure's cause.

## Visual and accessibility review

Implementation integrity passes for the reviewed sample: the interfaces retain their media and subtitle tasks, consistent navigation, and honest pending/empty/failed states. Sixteen baseline captured renders were inspected at phone and desktop widths, including both themes, forced colors, and the twenty-language preference limit. The Player sample covers pending, loaded, empty, and failed pagination. The Subtitles sample covers those inspection states plus history and narrow setup.

The detector emitted 142 advisories across three stylesheets: 115 font-size, 25 radius, and two color observations. These are not 142 confirmed defects. The two colors are a dialog backdrop and a beta-badge border, with an explicit light-theme override. Inspection did not establish a contrast or usability defect. Font and radius findings need the current design metadata before promotion to defects.

| Dimension | Sample score / 4 | Evidence and limit |
| --- | ---: | --- |
| Accessibility | 3 | Existing axe, keyboard and accessible-control assertions pass; no complete assistive-technology audit |
| Performance | 2 | Benchmarks and startup checks pass; no comprehensive rendering or energy profile |
| Responsive design | 3 | Reviewed 320/390/1440-width renders and native gestures; physical input remains untested |
| Theming | 3 | Dark, light and forced-color evidence; representative pages only |
| Implementation integrity | 3 | Coherent reviewed states; stale design metadata remains |
| Total | 14/20 | Good within this sample; not a product-wide certification |

Quick Connect adds 22 inspected state captures and eight actual-theme viewport captures. The final 26-case focused run passes, including axe contrast and scroll-aware actionability. Some browser state tests use interception or rendered templates. Their layout and accessibility proof does not establish real remote failure behavior. The Quick Connect spacing repair is the only frontend production change in this follow-up.

## Verification boundaries

- The public-instance expansion and repaired hash-recovery owner add 234 formerly unproved collected cases with at least one passing local execution. This does not close each engine's gap. The final index contains 24 collected cases without a passing execution in these artifacts. The repaired Quick Connect live-Server replay passes in Chromium and Linux WebKit. The WebKit replay uses revision `fa6a30b4`, with unchanged production source from the final dependency reconciliation. It passes both focused owners, including eight actual-theme viewport combinations, axe contrast, and scroll-aware actionability. Firefox's exact repaired journey remains unproved because of the certificate boundary above.
- Public expansion stages completed 75 passes before interruption, then 87 passes, five failures and 68 unrun cases. After the fixture corrections and spacing repair, the remaining-file replay passed 71, failed three, and skipped three. The two Supporter cases pass after supplying their explicit context base URL. QA-013 then passes after its locator repair. Stage counts overlap and must not be summed as unique journeys.
- A separate worktree audit finds unrelated expired `nox-local-live` and `kinosail-concurrency` leases with clean, unique commits. Their checkouts, commits, and leases were preserved. The primary checkout retains unrelated Xcode project and localization edits; these prevent automatic local-main cleanup.
- Full local package lint, complexity, Halstead loading, and dead-code gates fail as recorded above. No required gate, lint rule, source cap, or security threshold was bypassed.
- CRAP diagnostics were attempted with hosted Linux coverage against local Darwin source. Those platform-mismatched results are excluded from canonical gate evidence.
- The earlier audit's Android phone/tablet/TV/Wear emulator journeys and enlarged-font cases were integrity-checked, not rerun here. JVM replay and unit proof do not replace them.
- Physical inventory reports paired Apple TV and iPhone devices, but neither has an active device tunnel. No physical interaction or installation is claimed.
- Paired watch commands, real receivers, network handoff, codec/HDR diversity, long playback, accessibility services, and store distribution require separate environments.
- Manual and scheduled-only mutation testing, exhaustive rendering performance, every opted-in browser fixture, and every external-service failure were not executed. [The remaining browser cases](remaining-browser-cases.md) are indexed separately.
- Swift code scanning and the scheduled deep quality job were skipped by the workflow. The five other CodeQL language jobs ran.
- Image validation proves the tested Linux images, not publication or deployment. The current PR additionally runs affected Player security, compilation, container, and populated-browser gates; the protected merge requires those checks to pass. The touch helper does not change containers. The Quick Connect CSS change affects the Player production image. No Nox deployment, deployed revision, remote health, or production TLS claim is made.
- Source tests, rendered-fixture browser tests, populated-server E2E, simulator interaction, physical interaction, publication, deployed revision, container health, and TLS trust remain separate facts.

