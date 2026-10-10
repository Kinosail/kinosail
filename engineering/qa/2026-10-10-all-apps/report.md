# All-app QA — October 10, 2026

## Run record

This audit exercised Player and Subtitles web, iPhone, iPad, Apple TV, Apple Watch, Android phone, Android tablet, Android TV, and Wear OS. All writes used disposable data and generated media. Supporter and Home Assistant repositories were outside scope; their Player integration surfaces used local fixtures.

- Baseline: `db9a35d53d7aaadab669fb3dcde2b4ac79837235`.
- Branch: `codex/all-apps-qa-20261010`; pull request: #542.
- Reconciled main: `406e9549`, including native caption validation and independent Android parity fixes.
- Apple evidence uses the baseline native build. Final Android runs use `1562a0d74cd4024479426b6f27709f96fbc44b51`.
- Web fixes were tested from unique image tags containing the production changes. Later edits changed CSS formatting and test contracts. Receipts record each revision; `source-sha256.tsv` records the final changed source contents. GitHub Actions checks the exact submitted commit.
- Private evidence: `/Users/mikeo/Documents/Codex/all-apps-qa-20261010/`. Evidence paths below are relative to this directory.
- `evidence-sha256.tsv` identifies selected receipts, results, logs, and screenshots. A full manifest, source patch, and replay helpers remain private. Raw logs, credentials, traces, media, and simulator results are not committed.

Environment: macOS 27.0.1 arm64, Xcode 27.0 (27A266a), Apple runtimes 27.0, Android API 36 arm64 emulators, JDK 17, Podman Linux arm64, Node 26.11, pnpm 11.22, and Playwright 1.63. Local Chromium build: 1243. Android used SwiftShader with Vulkan disabled.

Disposable web servers used ports 38127 and 38128. Chromium trusted each generated certificate's public-key pin; Node used the disposable CA. Playwright `ignoreHTTPSErrors` remained false. This does not establish ordinary OS certificate trust.

## Journey coverage

Counts describe selected audit journeys, not every repository test or supported device.

| App | Success and persistence | Failure and recovery | Interaction and accessibility | Result and evidence |
| --- | --- | --- | --- | --- |
| Player web | Owner/MFA, six media views, search, tracks/progress, playlists/Collections, API keys, Viewer Profiles, sharing, Quick Connect, settings, offline playback/seeking | Blocked autoplay, direct retry, failed progress save, storage restrictions, session timeout, missing artwork, recovery over settings | Phone through desktop, keyboard, themes, accessibility, layout/focus | 87 selected cases passed across final batches; `player/browser-*-final`, `player/repairs-final`, `player/startup-contracts-final` |
| Subtitles web | Owner/MFA, dashboard, wanted/search/save, inspection, pairing/merge, history/restore, settings, Quick Connect | Pending/empty/failed dashboard and inspection, missing artwork, long comparisons, input rejection | 320–1920px, light/dark, keyboard and accessibility | 66 selected cases passed in one final run; `subtitles/browser-complete-final` |
| iPhone | Loaded playback, pause/play, close, navigation return | Opening, first-frame wait, failed video/retry, dismissal while opening | Touch, portrait/landscape, swipe/edge-back | Six unique journeys passed after targeted retry; PiP unsupported skip; `apple/iphone*.xcresult` |
| iPad | PiP/restore; manual loaded pause/play/close | Opening, first-frame wait, failed video/retry, dismissal while opening | Touch and landscape edge-back | Six automated journeys passed; one automated loaded-playback check fails. Manual equivalent passed; `apple/ipad*.xcresult`, `apple/ipad-manual-receipt.json` |
| Apple TV | Home, catalogs/details, seasons, playback options, repeated detail returns | Pending/empty/failed Home and retry | Directional focus, Back, Select, Play/Pause; 100 detail returns | Eleven remote journeys plus four seed tests passed; `apple/tv-loaded-2`, `apple/tv-{pending,empty,failed}` |
| Apple Watch | Build/install/launch, attribution and return | Disconnected paired-phone state | Manual layout and accessible controls | Manual checks passed; `apple/watch-*.png` |
| Android phone | Home/catalog/details, My List, seasons, playback controls, receiver chooser | Pending/failed/empty/loaded catalog and show; reopen recovery | Touch, semantics, contrast, 130% font size | Ten final tests plus two font-size journeys passed; `android/phone`, `android/phone-font-130.log` |
| Android tablet | Selected library and playback journeys | Same state transitions and reopen recovery | Tablet geometry, contrast, accessibility | Ten final tests passed; `android/tablet` |
| Android TV | Home/catalog/details, My List, seasons, playback controls | Same catalog/show transitions and reopen recovery | D-pad focus and Back | Seven final tests passed; `android/tv` |
| Wear OS | Remote screen and attribution | Disconnected and heart-rate permission messaging | Accessible controls and watch layout | Two final tests passed; `android/wear` |

Web counts mix actual-server journeys and isolated presentation checks. Real media, offline storage, CRUD, authentication, and persisted settings used the running Server. Mocked failures, synthetic media events, and rendered fixtures establish narrower UI behavior. Native loopback fixtures do not establish compatibility with the production Server.

Android screenshots compare pending card geometry with loaded cards and labels. Empty and failed states remove skeletons. Phone, tablet, and TV retain readable headings. Web recovery covers phone/desktop, light theme, and accessibility. Apple TV screenshots compare pending, loaded, empty, and failed Home.

## Confirmed findings and fixes

### QA-001 — Android empty catalog stays stale after reopening (P2; fixed)

Expected: reopening Movies loads newly available content. Actual: an empty active catalog stayed cached after navigating through Home.

1. Open Movies against the pending fixture.
2. Change the fixture to failed, retry, and return an empty catalog.
3. Make the fixture return content, open Home, and reopen Movies.
4. The baseline does not show the new film.

The changed journey failed on phone and TV before the fix. Explicit navigation now refreshes an empty active view. Automatic synchronization keeps its existing behavior, avoiding a request loop. Existing request handling and diagnostics are reused.

Regression: `NativeParityJourneyTest.catalogGridKeepsPendingFailedEmptyAndLoadedStatesDistinct`. Final phone, tablet, and TV suites passed. Evidence: `android/{phone,tv}-red`, final receipts, and `*-catalog-{pending,failed,empty,loaded}.png`.

### QA-002 — Playback recovery remains behind open settings (P2; fixed)

Expected: stalled playback with settings open offers a readable recovery action. Actual: native settings moved outside the media stage, so opening them marked only the options container. The recovery overlay stayed behind it.

Open a populated title, open playback settings, and trigger the existing stalled-media presentation case. The layout journey reproduced this twice. Shared presentation now synchronizes the media stage's settings state. No network failure handling or logging path changed.

The rebuilt-server recovery journey and six final layout/integration repairs passed. Evidence: `player/playback-layout-red2`, `player/recovery-final-2`, `player/repairs-final`.

### QA-003 — Recovery action has insufficient text contrast (P2; fixed)

Inherited black text on the dark button produced approximately 1.1:1 contrast after QA-002 exposed the action. The accessibility check reproduced the failure.

Player's stylesheet now uses the signal background for this action and `ButtonFace` in forced-colors mode. Shared stylesheet text remains stable because Subtitles patches it during startup. Final recovery accessibility passed. Evidence: `player/recovery-contrast-red2`, `player/recovery-final-2`.

### QA-004 — Android library headings have insufficient contrast (P2; integrated fix)

Black headings on the dark library surface produced approximately 1.16:1 contrast. Compose semantics checks missed the rendered failure.

The theme correction independently landed in main through #541 during this audit. This branch uses that implementation. A retained pixel check protects the concrete accessibility gap at the catalog journey boundary. Final phone/tablet suites passed. Evidence: `android/tablet-contrast-red-2` and final screenshots.

## Test and environment repairs

These changes preserve the product behavior being asserted:

- Measure visible actions, excluding the hidden progress notice.
- Find continued titles in the current Featured title region; check native fullscreen controls through their current contract.
- Start native video explicitly when layout checks need moving media. Reset progress before testing the short clip's uninterrupted playback.
- Use the executing browser's capabilities for blocked autoplay. Separate Apple launcher suites cover native presentation.
- Assert the failed-save warning and choose “Continue without saving” before expecting navigation.
- Wait for a decoder `playing` event before measuring compatibility startup. Positive restored time alone does not prove playback.
- Wait until Subtitles can read the host-written subtitle fixture before opening review. Both themes and the complete 66-case suite passed.

Repeated sign-ins exhausted the disposable Player rate limit. The remaining group passed after restarting that instance. Authentication limits were not weakened. Runs using random mapped ports or explicit authentication URLs were excluded from canonical-origin proof; the attempted mapped-port workaround was reverted.

Interrupted Android installations, simulator IPC failures, zero-test selection, and a shared image-tag collision were invalid environment runs. Final images use unique tags. Failed evidence remains private and is not classified as product regression proof.

## Verification boundaries

- **iPad:** `testLoadedVideoHasWorkingPauseAndPlay` repeatedly reports Pause as not hittable. Manual Device Hub interaction paused at 0:36, resumed to 0:37, and returned Home. A product defect is unconfirmed; the automated gap remains. No assertion was weakened.
- **Apple TV logs:** Xcode reports synchronous AVAudioSession activation warnings. Navigation passed, but physical-device responsiveness under load remains unverified.
- **Watches:** positive paired-phone commands and live heart-rate data were not verified on Apple Watch or Wear OS.
- **Native integration:** real-Server authentication, downloads, receiver interoperability, network handoff, and codec/HDR diversity remain outside fixture proof.
- **Physical devices and deployment:** no physical devices, store distribution, release signing, production deployment, or real casting targets were tested.
- **Browsers:** local audit used Chromium. GitHub's layout workflow provides separate Chromium, Firefox, and WebKit evidence. Complete local cross-browser suites were not run.
- **Source checks:** both app Go suites, Android units/builds, shared webassets tests, source cap, and script lint passed. Both local `verify-changed` commands stop at 95 existing shared-package lint findings. Those full-tree findings predate this task. Required CI uses its configured changed-revision contract. No gate was bypassed.
- **Deeper gates:** race/coverage, load/performance, complete native unit suites, all container architectures locally, and production scans were not duplicated locally. GitHub Actions owns required selected suites and container checks.
- **Logs:** server, browser, Android instrumentation, and Apple runtime logs are separate private artifacts. Browser traces use generated data. No new diagnostic failure path was added. Physical-device and deployment logs are absent.
- **Cleanup:** unrelated Xcode project and localization edits remain in the primary checkout. They prevent the clean-main prerequisite for `make agent-finish` and were preserved.

## Repeatable verification

Private receipts contain exact command arrays, revisions, results, elapsed times, and APK/media hashes. Helpers read disposable credentials from private files without printing them. The browser labels below refer to those receipts and reports.

```sh
python3 .verification/all-apps-qa/run-browser.py player startup-contracts-final playback-startup.spec.ts
python3 .verification/all-apps-qa/run-browser.py player browser-tail-final test-instance.spec.ts title-jump-first-paint.spec.ts ui-happy-paths.spec.ts web-qa-fixes.spec.ts test-instance-direct-retry.spec.ts
python3 .verification/all-apps-qa/run-browser.py subtitles browser-complete-final polish-shell.spec.ts subtitle-dashboard.spec.ts subtitle-history.spec.ts test-instance.spec.ts subtitle-inspector-layout.spec.ts subtitle-pairing.spec.ts subtitle-merged-group.spec.ts supporter-badge-layout.spec.ts
make max-loc
./scripts/quality/check-script-lint.sh
make -C apps/player verify-changed
make -C apps/subtitles verify-changed
```

Apple playback uses `xcodebuild test`, project `apps/player/apps/native/Kinosail.xcodeproj`, scheme `Kinosail-iOS-Touch`, the recorded device ID, and the generated fixture. Original commands appear at the start of each Xcode log. Apple TV arrays and source hashes are in each `apple/tv-*/run.json` and `source.json`.

Android arrays are in `android/{phone,tablet,tv,wear}/receipt.json`. They install APKs with recorded hashes, run selected instrumentation classes, and export screenshots/semantics. `android/run-device.py` and build logs preserve setup details.

Verify an artifact with `shasum -a 256 <private-evidence-path>` and compare its digest with `evidence-sha256.tsv`. The committed indexes contain no raw logs or credentials.
