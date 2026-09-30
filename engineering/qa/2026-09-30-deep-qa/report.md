# Kinosail deep QA — September 30, 2026

This audit identified nine confirmed defects and implemented corrections. The evidence below separates completed local checks from hosted image verification. It does not establish that every product path or device is free of bugs.

## Run record

- Scope: Player, Subtitles, shared Go packages, Apple clients, and Android clients. Dashboard is absent from current main.
- Starting revision: `55c0b765c16eb68a403b1a8f4c76f8a71340bdad`.
- Fix commit: `b6288a48`; reconciled source and final host binaries: `d95bf65a4d4b9f3cd9462ddf489bcbce8ee1bc1f`.
- Reconciled upstream: `77494fdff25d5e97a6d1f8e87a0659eb31dc990a`, `2a62a537a` (UI polish and HLS speed fixes), `96818c5e5` (native artwork validation and catalog ordering), `05da53145` (responsive configuration contrast), `60d11e205` (Player layout audits and narrow WebKit settings), `199bbcfae` (catalog projection and maintenance-test fixture timing), `f28d32c36` (missing seek completion events), `3c19d732e` (subtitle review-state evidence), `8a488dda4` (player script content hashes), and `74e18c46a` (tvOS playback options and progress storage).
- Second reconciled app binaries: `4b92154499531410a596ad2b2db705c9abeace7d`; final landscape header follow-up is recorded separately below.
- Environment: macOS ARM64, Xcode 27, iOS 27, tvOS 27, installed Android API 36 images, repository-pinned Playwright 1.63.
- Local evidence: `.verification/deep-qa-20260930/` in the task checkout. This directory is ignored and contains private disposable session state.
- Test data: generated fictional movies, episodes, music, audiobooks, books, photos, and a loopback metadata provider. Native captures use the existing fictional preferences fixture.
- Web instances: task-owned Go binaries on loopback HTTPS ports 38147 and 38148. These are host builds, not container or deployment evidence.
- The original final host binaries were verified with `go version -m`. Later header and Supporter follow-ups have separate source hashes and run records; the original identity does not certify those binaries.
- Primary checkout and unrelated worktrees were preserved.

## Confirmed findings

### QA-001 — Partial metadata results trigger a refresh loop (high; fixed)

1. Configure a local provider with valid movie details and artwork.
2. Return HTTP 503 for optional credits.
3. Start either app with a movie requiring metadata.
4. Observe repeated credits, detail, artwork, persistence, and library-refresh work without another user scan.

The automatic refresh stored unchanged partial records and rescanned the library. That scan queued another metadata refresh. The disposable host fixture received hundreds of thousands of requests and eventually exhausted local connection capacity.

The shared operation now skips unchanged downloads and publication when the record is already reflected in the library. It still publishes repaired artwork and records awaiting index publication. Failed optional work remains eligible for a later real scan.

- Red: `metadata-feedback-red.log` recorded 13 self-triggered credits requests in both repetitions on the original Player build.
- Focused operation red: `metadata-unchanged-red.log`, `metadata-download-red.log`.
- A first implementation exposed a concurrent publication regression in the existing TVmaze web/API test. A baseline overlay passed five repetitions; the regression failed repeatedly in both apps.
- Additional red: `metadata-concurrent-red.log`, `metadata-stale-index-red.log`.
- Final green: `player-metadata-publication-green.log`, `subtitles-metadata-publication-green.log`, 20 repetitions per app.
- Final operation/race proof: `metadata-publication-green.log`, `metadata-final-race.log`.
- Tests cover no repeated publication, no redundant downloads, successful artwork repair, failed artwork, pending index publication, later retry, and provider recovery.

### QA-002 — Failed Supporter artwork overflows badge cells (medium; fixed)

1. Render the production Subtitles Supporter template with twenty badges.
2. Abort the badge artwork requests.
3. Check phone and desktop layouts in light and dark themes.

The implicit grid column allowed image alternative text and descriptions to exceed each badge cell. The fix explicitly constrains the column with `minmax(0,1fr)` and advances the application CSS cache version.

- Red: `badge-failure-red-confirmed.log`; three assertions failed for overflow at 390px and 1024px. A fourth failure was an instance startup race and is excluded from defect proof.
- Green: `badge-green.log`, 15 Chromium checks; `badge-cross-browser.log`, 30 Firefox/WebKit checks.
- Sizes: 1440, 1024, 720, 390, and 320px. Themes: light and dark.
- Each failed-artwork check asserts twenty failed images, child bounds inside every cell, and no horizontal page overflow, then saves a screenshot.
- Existing loaded/empty/conditional fixtures also passed. Their renderer now uses the real origin so relative artwork URLs resolve.
- The new failed-artwork regression is selected by both Subtitles browser launch scripts. The dark phone case is included in hosted smoke checks.

### QA-003 — TMDB instructions run off narrow viewports (medium; fixed)

The TMDB setup list reused a class styled as a horizontal wizard rail. At 390px, steps two and three fell outside the visible page in configuration and onboarding. The list now uses its own class, so the three instructions wrap vertically.

- Reproduced by `beta-stylesheet-green.log` and a second signed-in route observation in `beta-second.log`.
- Focused red: `setup-skip-red.log`.
- Green: eighteen instruction checks across Chromium, Firefox, WebKit, both routes, and six widths from 320px to 1440px. Each saves the rendered section.

### QA-004 — Show skip navigation lacks a focus target (medium; fixed)

The application shell mistook `data-palette-id` for the main element's `id`. The show page therefore linked to an absent `#main`. Adding the target alone still failed keyboard focus in all three browsers.

The shell now distinguishes the normal attribute and makes main content programmatically focusable. Existing IDs and tabindex values are preserved.

- Reproduced by the populated beta audit and `beta-second.log`.
- Red target proof: `setup-skip-red.log`; red focus proof: `setup-skip-green.log`.
- Final green: `setup-skip-final.log`, 21 total instruction and keyboard checks across three browsers.
- The keyboard check activates the visible skip link with Enter and asserts the main content receives focus. Phone instructions and skip navigation are selected by hosted smoke checks.

### QA-005 — Long Server names hide the landscape Support link (medium; fixed)

The reconciled landscape header let the brand column consume the space needed by Support. A normal eighteen-character Server name reproduced the overlap at 390px in Chromium, Firefox and WebKit. A sixty-four-character name also overflowed at 720px.

The header now reserves the Support column and truncates the brand within the remaining space. The Player stylesheet version initially advanced to `electric-39`, including the album adapter and existing cache-version contracts. After reconciliation, main already used that version, so the combined stylesheet advances to `electric-40`. The cache regression also proves version 39 is refreshed. A later merge from `60d11e205` introduced different styles at version 40, so the final combined styles advance to `electric-41`; the regression now also proves version 40 is refreshed.

- Red: `player-reconciled-ui.log`, three browsers; `header-names-red.log`, focused long-name bounds.
- Green: `header-names-green.log`, nine checks across three browsers. Existing landscape keyboard behavior remains covered.
- The added smoke check exercises eighteen- and sixty-four-character names at 390px and 720px and saves rendered screenshots.
- Source/cache contracts: `player-header-cache-tests.log` passed. Reconciliation cache proof: `player-reconciliation-cache-red.log` failed with stale version 39; all ten relevant HTTP/cache tests passed in `player-reconciliation-cache-green.log`. The exact selection is saved in `player-reconciliation-cache-command.json`.

### QA-006 — Stale Supporter reads change newer navigation state (medium; fixed)

The old lifecycle tests still expected separate status and preference requests. Updating them to the current collection API exposed three real failures: responses applied after leaving during headers, after leaving during the JSON body, and after a newer navigation response had already completed.

Recognition now aborts its previous read on navigation swaps and cancels the current read on page exit. It checks cancellation before applying the collection and removes its exit listener when the read finishes. Back restoration requests a fresh collection. The script cache version advances to 18.

- Red: `supporter-lifecycle-red.log`, three Chromium failures with the current API and header controls.
- Green: `supporter-lifecycle-green.log`, fifteen checks across Chromium, Firefox and WebKit. `deep-followup-go-green.log` passed five repetitions of the maintenance, startup, Supporter HTTP and application-shell race checks. Source cap, shellcheck and tooling checks also passed. `deep-followup-run-record.json` records commands, source hashes, environment and results.
- Checks preserve the single collection request, apply hidden preferences, prevent stale updates, restore on back navigation, and avoid authentication-page reads. Rendered state screenshots accompany the lifecycle checks.

### QA-007 — WebKit encoding controls overflow the inspector at 320px (medium; fixed)

The full hosted WebKit suite found four pixels of page overflow in both themes. The Text encoding select exceeded its label's internal width even though normal element rectangles stayed inside the viewport. A local baseline matrix passed forty checks and failed these same two cases.

Select labels now use block layout with the existing half-rem spacing. Checkbox and numeric-input layouts retain their existing behavior. The inspector stylesheet cache version advances to 4.

- Red: `inspector-webkit-320-red.log`, hosted run `36757578425`, and `inspector-layout-grid-green.log`. The last filename is misleading: its proposed grid change was rejected by a failing diagnostic precondition, so it records unchanged production CSS and forty passes plus two failures.
- Diagnosis: `inspector-overflow-block-probe.log` and `inspector-overflow-select-gap-probe.log` isolate the encoding label; block layout removes the overflow, while forcing the select itself to block layout brings it back.
- Green: `inspector-layout-final-green.log` (fourteen Chromium checks), `inspector-layout-final-cross-browser.log` (twenty-eight Firefox/WebKit checks), and `inspector-final-go.log` (focused inspector HTTP and rejection tests).
- The matrix checks both themes at 320, 390, 568, 720, 1024, 1440 and 1920px. It saves pending, loaded, empty and failed renders, checks video geometry, keyboard/navigation landmarks, accessibility at desktop/phone sizes, and no writes during review. Empty and failed states also assert no horizontal overflow. Browser artifacts now record their actual project name.

### QA-008 — Production images retain vulnerable OpenSSL packages (high)

Current-head container scans failed for both apps on amd64 and arm64. They reported six high-severity findings: CVE-2026-75804 and CVE-2026-84782 in `libssl3t64`, `openssl` and `openssl-provider-legacy`, installed at `3.5.7-1~deb13u2`.

Debian records `3.5.7-1~deb13u3` as fixed for both advisories: [QUIC flow control](https://security-tracker.debian.org/tracker/CVE-2026-75804) and [DTLS retransmission](https://security-tracker.debian.org/tracker/CVE-2026-84782). Direct checks of the official trixie-security package indexes confirmed that version for all three packages on both architectures (`debian-openssl-security-candidates.json`). This is package vulnerability evidence; no Kinosail exploit was demonstrated.

Both runtime recipes now check all three minimum versions beside their existing libaom version check. Changing the installation layer also invalidates the older cached layer. Future builds fail if an insufficient package remains. The existing image scan and public production-path tests provide the behavioral verification; no new test mirrors the recipe text, and no advisory is ignored.

- Red: required run `36765200869`, deep run `36765430447`, `final-ci-player-arm-container-failure.log` and `final-ci-subtitles-container-failure.log`.
- Green: [required run 36767118287](https://github.com/Kinosail/kinosail/actions/runs/36767118287) passed all four app/architecture image scans and public production-path checks at source `19dc200d6`. The manual deep run passed those same image jobs. A separate Chromium image build refused a Debian mirror size/hash mismatch; its log is retained. The final head still requires its own hosted checks before merge.

### QA-009 — Changed playback code retains immutable script URLs (medium; fixed)

The final seek-status merge changed the shared player bundle while watch pages retained Player version 95 and Subtitles version 57. Versioned scripts are cached as immutable for one year. Existing clients could retain the earlier code and miss the seek fix.

The first correction requested Player version 96 and Subtitles version 58, after both public watch-page regressions failed with the old URLs. Main then introduced content hashes for both player-script URLs in PR #391. The final reconciliation retains that solution and its public HTTP contract, which proves each URL matches the served script bytes and retains immutable caching. The focused playback fixtures accept these versioned URLs; playback policy remains unchanged.

- Red: `player-seek-cache-red.log`, `subtitles-seek-cache-red.log`.
- Failure modes and source identity: `seek-cache-failure-modes.json`.
- Green: both app HTTP/cache checks passed; the eighteen seek-status browser checks passed across Chromium, Firefox and WebKit. Commands, source hashes and environment are recorded in `seek-cache-run-record.json`.
- The content-hash reconciliation passed both apps' URL/cache, chapter and direct/compatible playback checks. Its source identity and commands are recorded in `reconciliation-8a-run-record.json`.
- The reconciled browser regressions exercise missing and late seek events, pending and resumed overlays, paused playback, and phone/desktop geometry.

## Verification

The complete [manual deep run 36788186257](https://github.com/Kinosail/kinosail/actions/runs/36788186257) passed on `1be4230f0`. It includes all six browser suites, source/race/coverage and security checks, native compilation, and all four image scans and production-path jobs.

The subsequent tvOS-only reconciliation passed 258 tests in 53 suites on a fresh task-owned tvOS 27.0 simulator. This includes the new default-journal regression. Its record is `reconciliation-74e-native-run-record.json`, with `tvos-74e-suite.log` and `tvos-74e.xcresult`. Browser, service, shared-package and workflow inputs remain byte-identical to the passing deep revision; `reconciliation-74e-unaffected-web-go.json` records that comparison. The new remote playback-options journey was not rerun in this audit. PR #392 reports passing populated 1080p and 4K options, preferences, dismissal and playback journeys; physical TV interaction and separate preference pending/failed renders remain unverified. Final-head required CI remains a separate delivery gate.

| Surface | Result | Evidence and limits |
| --- | --- | --- |
| Shared Go packages | Passed full source tests, race suite, tidy, consumer compilation, and 97.7% coverage | `packages-baseline.log`, `packages-remainder.log`, `packages-final-coverage.log` |
| Changed Go code | Zero new lint findings in packages and both apps | `packages-lint-reconciled.log`, `player-lint-changed-final.log`, `subtitles-lint-changed-final.log` |
| Vulnerabilities | No reachable vulnerabilities | Scanner also reported one unused required-module advisory; see `packages-remainder.log` |
| Player Go suite | All packages passed again after the setup and skip-navigation fixes | `player-source-setup-skip-final.log`; server suite 387 seconds. Earlier run: `player-source-final.log` |
| Subtitles Go suite | All packages passed | `subtitles-source-final.log`; server suite 187 seconds |
| Reconciliation | Catalog/metadata/operations checks and both app HTTP race regressions passed; final Player metadata/TMDB/shell/HLS race checks passed | `reconciled-packages.log`, `reconciled-player.log`, `reconciled-subtitles.log`, `player-final-reconciliation.log` |
| Source cap and shell scripts | Passed | `max-loc.log`, `subtitles-shellcheck.log` |
| Reconciled Subtitles badge, conditional and shell matrix | 48 passed across Chromium, Firefox, WebKit | `subtitles-reconciled-ui.log`; loaded and failed artwork, pending/empty/error states and shell polish; 320–1440px |
| Reconciled Player setup, skip and masthead matrix | 33 passed; three new landscape failures were reproduced and fixed | `player-reconciled-ui.log`, followed by all nine `header-names-green.log` checks passing |
| Player complete beta route audit at 390px and 1440px | 18 passed | `beta-fixed.log`; includes library, utilities, configuration, onboarding, media detail/reader, playlists and collections |
| Player populated Chromium journey set | 61 passed, one failed | Playback, seeking/startup/bandwidth, navigation, keyboard, themes, MFA, Quick Connect, checkout and responsive views; `player-populated-final.log` |
| Player masthead fixture | Six checks passed after isolating continued playback | `masthead-fixture-green.log`; prior failures came from earlier media progress. Exact feature and geometry assertions remain |
| Subtitles populated host set | Initial run: 24 passed, two failed, 24 skipped | Missing root and hard-coded canonical port were corrected in test setup; see follow-up below |
| Subtitles dashboard and instance fixture follow-up | 30 passed, plus one separate defaults pass | `subtitles-other-fixtures-green.log`, `subtitles-defaults-final.log`; cleanup accounts for the existing Spanish VTT and verifies its bytes remain recoverable |
| Exploratory web routes | 34 valid route/viewport observations without overflow, axe violations, broken images, or page exceptions | `browser-exploration.json`; two invented `/settings/subtitles` routes returned 404 and are excluded |
| iPhone simulator | 270 tests in 54 suites passed; 47 captures | `ios-baseline.log`, `ios-baseline.xcresult`, `iphone-gallery/` |
| iPad simulator | Five focused lifecycle/preference tests passed; 47 captures | `ipad-baseline.log`, result bundle and `ipad-gallery/`; not a separate full-suite pass |
| tvOS simulator | 257 tests in 52 suites passed | `tvos-baseline.log`, `tvos-baseline.xcresult`; includes rendered focus regressions, not a full remote playback journey |
| Android source | Phone, watchcore and Wear unit tests passed; app and Wear debug/instrumentation APK builds passed | `android-baseline.log`, `android-emulator-build.log`, `android-wear-build.log` |
| Android phone emulator | Four instrumentation checks passed; populated connection, home, search, detail and short movie playback observed | `android-phone-instrumentation.log`, `android-phone-extra-instrumentation-second.log`, saved XML and screenshots; API 36 read-only task overlay, debug app only |
| Android TV emulator | One accessibility instrumentation check passed; pairing, populated Home, D-pad scrolling, short decoded video playback and Back navigation observed | Earlier captures plus `android-tv-followup-loaded.png`, two changing playback frames and `android-tv-followup-return.xml`; current clean Player source `dc8744db5`, current Android APK, read-only overlay. Full playback-control and long-session behavior remain unverified |
| Wear emulator | Two accessibility instrumentation checks passed; disconnected and scrolled empty views inspected | `android-wear-instrumentation.log`, saved API 36 renders; debug data cleared only inside the read-only task overlay |
| Android reader and tablet rendering | Fresh retry passed EPUB unsupported-state transitions twice; PDF rendered at tablet portrait and landscape sizes | `android-reader-retry-open.png`, repeated XML, `android-tablet-pdf-loaded.png`, `android-tablet-pdf-landscape.png`; phone AVD geometry override, not separate tablet hardware |
| Containers | Blocked before build | Podman VM full; test-instance build failed creating a temporary builder directory |
| Local changed-app gates | Both stopped at the full shared lint gate | `player-verify-changed.log`, `subtitles-verify-changed.log`; 113 repository-wide findings. No gate override was used |
| Design detector | Two unchanged border-style findings retained | `badge-ui-lint.log`; owned recognition and error status use these accents |

CodeQL identified a check/use race in the new optional VTT fixture helper. It now reads directly and restores with exclusive creation. An existing destination must still match the captured bytes. Both present-VTT and missing-VTT cleanup variants passed: `subtitles-cleanup-exclusive-green.log`, `subtitles-cleanup-missing-vtt-green.log`. The subsequent current-head CodeQL findings policy passed on run `36752460006`; final delivery still requires scans of the later Supporter change.

The larger Subtitles follow-up (`subtitles-fixtures-final.log`) passed nine checks, timed out in the multi-width preferred-language check, skipped nineteen fixture-dependent cases, and did not run nineteen later cases. The earlier focused thirty-plus-one passing checks remain valid. This run is incomplete and its timeout is not confirmed as a product defect.

The initial broad browser runs are diagnostic evidence, not passing suites. They included copied Player routes in Subtitles, stale numeric CSS expectations, already-initialized setup state, and connection failures during the metadata storm. Their raw logs are retained separately.

The final follow-up run record is `followup-run-record.json`. Phone, TV and Wear AVDs used read-only overlays and were stopped after evidence capture. The existing release app and unrelated simulators were preserved. Android used a task-only HTTP loopback proxy to the host HTTPS server; this does not prove native certificate trust.

The first Android reader attempt captured an app-unresponsive dialog during severe host memory pressure. A fresh emulator retry completed the transition twice; its captured ActivityManager/AndroidRuntime error log contains no app ANR or fatal markers. This observation is not treated as a confirmed product defect. Android EPUB reading remains unsupported by the current implementation; PDF rendering was verified.

Native loaded, pending, empty, error, large-text, reader, photo, preference, approval, and casting views were inspected from populated captures. Some snapshots labelled `supporter-failed` were taken during request retries and still show pending geometry; they do not prove the terminal failure render. Physical interaction and media decoder behavior remain separate boundaries.

The manual deep run `36750352124` caught three additional verification problems. The Supporter lifecycle mismatch exposed QA-006 above. Subtitles created its private HTML fixtures inside the media bind mount, making scans fail with `open /media/ui-fixtures: permission denied`. The container script now keeps those fixtures in its separate temporary artifact root; it retains private permissions and the exact cleanup assertions.

The Player maintenance regression failed because startup maintenance pruned its deliberately over-limit cache before the test started a stream. `maintenance-race-reproduction.log` reproduced this repeatedly, and `maintenance-before-stream-diagnostic.log` confirms pruning before the media request. The test now invokes the same automatic operation explicitly with its background scheduler disabled, and always releases its blocked response writer. Separate tests still verify scheduled eviction and startup lifecycle behavior. This is a test setup correction, not a confirmed production playback failure.

The next deep run passed the scan but reached a later cleanup conflict. Browser-created sidecars were owned by the runner and lacked write access for container UID 10001. Linux protected hardlinks reject the cleanup operation under those permissions. The two temporary cleanup files now receive writable test permissions before confirmation; an attached artifact records their owner and before/after modes. Production no-overwrite safeguards stay intact. The next deep run passed cleanup in Chromium and Firefox; WebKit reached later, unrelated failures recorded below.

The required PR run `36757473280` passed on its second attempt. Its initial failures were temporary FFmpeg execution (`text file busy`) and Chromium startup, before the relevant assertions. The ordinary failed-job rerun passed without a source change. The subsequent deep run `36757578425` passed Player's complete three-browser matrix, shared/app Go race and coverage checks, native compilation, security policy and both app container builds. Its only failing browser job was Subtitles WebKit: QA-007 above and an inspector pending-state response gate bypassed by the service worker.

The dashboard browser tests now block service workers so their explicit API response gates control pending states. A local WebKit reproduction failed before that test-context correction and passed afterward (`defaults-webkit-red.log`, `defaults-webkit-service-worker.log`). Product service-worker behavior is unchanged; offline/service-worker checks remain separate. The complete local dashboard matrix then passed all 75 checks across Chromium, Firefox and WebKit (`dashboard-final-cross-browser.log`, `webkit-followup-run-record.json`). This host instance uses the earlier recorded Subtitles binary; current-head image checks remain separate. The combined Player UI run passed eleven checks and timed out once in Firefox before the sign-in page load event. Its nine resource requests all returned HTTP 200; the unchanged focused retry passed in 16.3 seconds (`player-reconciliation-ui-final.log`, `player-reconciliation-ui-firefox-retry.log`, `player-reconciliation-ui-run-record.json`).

The next complete Linux WebKit run passed the fixed inspector cases, but the cleanup journey timed out waiting for a generic Settings button. Both saved failure renders show the MKV fixture's explicit unsupported-decoder prompt and its available “Open playback settings” status control. The test now uses that public control and retains the exact two-option subtitle assertion. The local focused cleanup matrix passed in all three browsers (`cleanup-playback-settings-chip.log`). This corrects the test's decoder assumption; no transcoding is started or playback policy changed. Hosted Linux verification remains required.

Browser jobs now allow thirty minutes for dependency installation, image startup and the unchanged full suite. An earlier twenty-minute Player WebKit job spent seventeen minutes installing dependencies and was cancelled only fifty-three seconds into the test step (`36754933827`, job `110022753859`). A later unchanged Subtitles rerun again spent over fourteen minutes installing dependencies. The test selection, assertions and `failOnFlakyTests` policy remain intact. CI contract tests passed 48 cases; actionlint and source caps passed (`browser-timeout-ci-contracts.log`, `browser-timeout-actionlint.log`, `browser-timeout-max-loc.log`).

The next Linux WebKit run caught a cleanup checkbox interaction during smooth anchor scrolling. Its trace records the document moving from 1095px to 1117px between the click and the state check; the retry passed, and the flaky-test policy rejected the job. This file-operation journey now selects the existing reduced-motion preference before navigation. It retains native clicks, exact preview and subtitle counts, recovery assertions and the flaky-test policy. Normal-motion UI checks remain separate (`cleanup-motion-red-analysis.json`).

## Repeatable commands

Run these from the repository root or the indicated app directory. Evidence artifacts record the isolated environment and result; synthetic credentials stay local.

```sh
go -C packages test ./... -count=1
go -C apps/player test ./... -count=1
go -C apps/subtitles test ./... -count=1
go -C packages test -race ./operations ./metadata -count=1
make -C packages coverage test-race vuln consumer-test
make max-loc
make -C apps/player verify-changed
make -C apps/subtitles verify-changed
```

From `apps/subtitles`, generate fixtures with `KINOSAIL_UI_FIXTURE_DIR=<directory> go test ./internal/server -run TestWriteUIStateFixtures -count=1`. Set the local HTTPS URL and fixture directory, then run:

```sh
KINOSAIL_BROWSER_MATRIX=full pnpm --dir e2e exec playwright test \
  supporter-badge-layout.spec.ts conditional-states.spec.ts --workers=1
```

Apple runs used task-owned simulators, `xcodebuild test`, isolated DerivedData, saved result bundles, `-parallel-testing-enabled NO`, and `-jobs 2`. The iPhone gallery used `TEST_RUNNER_KINOSAIL_POLISH_GALLERY=1`; iPad used the same build with `test-without-building -only-testing:Kinosail-iOSTests/NativePreferencesUXTests`. Task simulators and rebuildable DerivedData are removed after captures are preserved.

Upstream native QA evidence from PR #383 is retained in `engineering/qa/2026-09-30-native-polish/report.md`. It adds a confirmed Android canonical episode-artwork fix and terminal Apple Supporter error captures. Those results belong to that recorded native revision and are separate from this audit's earlier simulator totals. Catalog ordering from PR #384 was reconciled with generated snapshots; focused shared and app publication checks are rerun after that merge.

## Verification boundaries

- Reclaim storage with authorization, then run container startup/media/API checks and the supported populated browser matrix against exact images.
- Complete longer Android reading, comic, full TV playback-control and paired Wear remote/heart-rate journeys, Apple TV remote and watchOS interaction journeys.
- The completed local dashboard matrix passed 75 cases; the inspector matrix passed 42. The hosted complete matrices remain separate and must pass on the final source/image revision.
- Exercise physical codecs, HDR, hardware acceleration, AirPlay, casting, PiP/background playback, long playback, and paid activation on suitable devices.
- Finish required and manual deep GitHub checks, then record merged source ancestry, image publication, and any later deployment as separate evidence.
- The first manual deep run caught stale generated architecture snapshots. Both were regenerated, and full local `make tooling-check` passed (`tooling-final.log`). The refreshed deep run exposed the lifecycle and fixture failures recorded above. A final deep run is required after their corrections.

The complete hosted matrix at `12015d17c` passed all six browser jobs, including 61 Subtitles WebKit checks without a flaky retry. Required run `36773560477` passed. Main then advanced with the Player layout changes above; the combined revision receives fresh required and deep checks in the PR. A subsequent catalog merge retains the same stylesheet and both maintenance-test protections: background scheduling is excluded from the explicit streaming-guard test, and its over-budget cache is created only after the stream starts.

This report captures pre-merge evidence. [PR #381](https://github.com/Kinosail/kinosail/pull/381) records the final hosted checks and delivery. Local Podman storage and the device boundaries above remain explicit limits.
