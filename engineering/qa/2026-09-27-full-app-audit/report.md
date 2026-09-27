# Full app QA audit — 2026-09-27

## Run record

- Mode: full audit. Player web was first, followed by Subtitles web, Android phone/TV/Wear, and Apple iOS/tvOS/watchOS.
- Source baseline: `2efa5638fdd521e7e9c44c35f584e19114b5f893`. The two web test images were built from this source and rebuilt after their applicable fix. Native apps were built from this worktree. The final PR and main revisions are recorded in Git history.
- Host: macOS, local Podman, pinned repository Playwright and axe packages, Chromium/Firefox/WebKit, Android API 36 emulators, and Apple simulators. All browser and Android account/media changes used isolated synthetic test instances. No production records were changed.
- Isolated Player: `KINOSAIL_TEST_PROJECT=kinosail-fullqa-player-20260927`, `KINOSAIL_TEST_ROOT=/Users/mikeo/.cache/kinosail-fullqa-player-20260927`; run `cd apps/player && ./scripts/test-instance.sh up`, `verify`, and `browser` with those variables set. Reset with `down --volumes` and the same variables.
- Isolated Subtitles: use the equivalent `kinosail-fullqa-subtitles-20260927` project and root under `apps/subtitles`. Reset with its own `down --volumes`. The test harness creates temporary Owner/Viewer accounts and synthetic media. Credentials are not in this report.
- Android phone and TV connected only to that local Player instance. A loopback-only HTTP proxy bridged the emulator to its self-signed local HTTPS certificate. The Wear emulator was fresh and unpaired. Apple simulator login was not completed.
- Evidence is in [evidence](evidence). The JSON observations record 12 desktop/phone routes per web app, with page/console errors, failed requests, HTTP failures, axe findings, and horizontal overflow. Captures include synthetic data only.

## Journey coverage

| App and journey | Verified outcome | Error, empty, role, input, or navigation checks | Boundary |
| --- | --- | --- | --- |
| Player web: Owner, Home, browse/search, detail, books/music/photos | Populated Chromium suite 59/59; selected browser matrix 6/6 | Viewer/MFA, library and empty search, responsive/axe/keyboard checks; 12 observed routes had no console/page error, failed request, HTTP failure, axe `main` violation, or horizontal overflow | Full Firefox/WebKit suite and physical remote unrun |
| Player web: video, subtitles, offline, settings | Synthetic playback, compatibility, download/offline, settings journeys in the 59-test suite | Existing negative and recovery cases plus phone layout | Long sessions and real network interruption unrun |
| Subtitles web: Owner overview, wanted, library, languages, provider, cleanup | Isolated verify and Chromium suite 22/22; selected browser matrix 6/6 | Viewer MFA regression; responsive/axe/keyboard checks; 12 observed routes had no recorded errors, failed request, HTTP failure, axe `main` violation, or overflow | External subtitle provider credentials and actual subtitle fetching unrun |
| Android phone: setup, pairing, Home, Library, search | Fresh API 36 emulator paired to isolated synthetic Player; populated Home/Library rendered | Empty Server URL rejected; absent-title searches in All media and Movies reproduced QA-001, then showed corrected result text; 4 instrumented tests passed | Downloads, actual playback, background recovery, API 23 runtime, physical phone unrun |
| Android TV: setup, pairing, browse, detail, playback | Fresh API 36 TV emulator paired; D-pad reached Library and Arrival detail; Play reached end of the 12-second synthetic video ([detail](evidence/android-tv-detail.png), [playback end](evidence/android-tv-playback-end.png)) | Remote directional navigation and player controls exercised | Broader search/roles and physical remote unrun |
| Android Wear: launch and unpaired state | Fresh API 36 Wear emulator launched the APK and showed [Connect phone](evidence/android-wear-connect.png) | Unpaired empty state checked | Phone pairing, remote control, and heart data unrun |
| iOS: build, source tests, launch | Simulator build passed; 241 tests in 47 suites passed; app installed and displayed Connect screen | Source tests cover state/validation; setup screen rendered | Authenticated simulator journeys and physical device unrun; launch screenshot withheld because nearby discovery showed a private address |
| tvOS: build and remote focus | Simulator build passed; 11/11 remote UI tests passed | Directional focus and navigation tested in simulator | Live authenticated playback and physical Siri Remote unrun |
| watchOS: build and launch | Simulator build passed; installed and launched on Apple Watch Series 12 simulator; [unpaired screen](evidence/apple-watch-unpaired.png) showed “Nothing playing” | Empty/unpaired state rendered | Paired iPhone control and physical watch unrun |

## Prioritized findings

### QA-001 — Android search reports the library is empty (P2, confirmed, fixed)

- **Expected:** When a populated library has no matches for a search, the result should say that the search has no results. A genuinely empty library and an empty My List need their own guidance.
- **Actual:** Both All media and Movies displayed “Nothing in your library yet.” for absent titles while Home and Library held synthetic media.
- **Reproduce:** (1) Boot a fresh Android phone API 36 emulator. (2) Pair the debug app with the isolated populated Player. (3) Open Library and search for a title absent from the fixture. (4) Repeat with another absent title in Movies. Both attempts showed the library-empty sentence.
- **Evidence:** [before screenshot](evidence/android-phone-empty-search-before.png), [populated Home](evidence/android-phone-home.png), and [after screenshot](evidence/android-phone-empty-search-after.png).
- **Regression proof:** `CatalogEmptyMessageTest` was written before the production fix. With the old branching logic extracted into the shared helper, the no-results assertion failed while the true-empty assertion passed. After the fix, both passed. The fresh phone was then reinstalled and showed “No results. Try another search.” The phone and TV now use the same empty-state choice. The TV search screen was not manually retested.

### QA-002 — Subtitles Viewer MFA page names the Owner account (P2, confirmed, fixed)

- **Expected:** A Viewer required to add a second sign-in method receives account-neutral instructions.
- **Actual:** The Viewer was redirected to `/account?mfa=required` and told “One is enough for the Owner account.” The shared template used Owner-specific copy.
- **Reproduce:** (1) Start the isolated Subtitles instance with extra sign-in protection required. (2) Create a temporary Viewer. (3) Sign in as that Viewer in a fresh browser context. (4) Read the instruction on the required MFA page. The exact wrong sentence appeared in two red test runs.
- **Evidence:** [before](evidence/subtitles-viewer-mfa-before.png) and [after](evidence/subtitles-viewer-mfa-after.png) screenshots.
- **Regression proof:** The new Playwright test asserts the Viewer redirect, neutral instruction, and absence of “Owner account.” It failed twice against the unfixed image for the intended copy assertion and passed after rebuilding with the one-line template fix. The Viewer is removed in `finally`.

### QA-003 — Android minimum-SDK lint rejects Cast time calls (P2 release gate, confirmed; runtime impact unverified, fixed)

- **Expected:** `:app:lintDebug` passes for the declared minimum SDK 23, and Cast can use its `java.time.Instant` calls on that SDK level.
- **Actual:** Lint reported six `NewApi` errors at the `Instant` uses in `CastApi.kt` because core library desugaring was not enabled. This is a reproducible build-gate failure; an API 23 Cast failure was **not** observed on a device.
- **Reproduce:** Run `JAVA_HOME=<JDK17> ANDROID_HOME=<SDK> ./gradlew :app:lintDebug` from `apps/player/apps/android` before the Gradle change. The six errors recur.
- **Evidence and fix:** The Android app now enables core library desugaring and pins `com.android.tools:desugar_jdk_libs:2.0.3`. The same lint command and the combined Android lint/unit/APK command passed. Runtime Cast on API 23 remains a coverage gap.

### QA-004 — Expanded Subtitles spec contains Player UI assumptions (P2 test-maintenance gap, open)

The nondefault `test-instance.spec.ts`/`ui-happy-paths.spec.ts` run reached 15 of 26 tests before it was stopped after repeated one-minute failures (13 failed, 2 skipped at that point). Examples expect Player Settings sections (“Jellyfin apps,” “Trusted HTTPS”) or “Watch & view” on Subtitles pages. Those failures do not establish a Subtitles product defect. The default 22-test Subtitles browser suite passed. The stale tests need a separate scope decision and replacement with Subtitles-specific assertions; no intended assertion was weakened here.

## Commands and results

| Surface | Command or suite | Result |
| --- | --- | --- |
| Player web | `./scripts/test-instance.sh verify` and `browser` | Passed; Chromium 59/59 |
| Player web | Selected `test-instance-production.spec.ts` library and Compatibility matrix, `KINOSAIL_BROWSER_MATRIX=full` | Chromium/Firefox/WebKit 6/6 |
| Player web | Ad hoc 6-route × 2-width observation with console/network/axe/overflow capture | Passed; 12/12 states clear in the recorded checks |
| Subtitles web | `./scripts/test-instance.sh verify` and `browser` | Passed; Chromium 22/22 |
| Subtitles web | New Viewer MFA test, old then rebuilt image | Red twice on exact copy assertion; green 1/1 |
| Subtitles web | Selected dashboard/layout matrix, `KINOSAIL_BROWSER_MATRIX=full` | Chromium/Firefox/WebKit 6/6 |
| Subtitles web | Ad hoc 6-route × 2-width observation; `go test ./internal/server` | 12/12 observation states clear; Go test passed |
| Android | `:app:lintDebug :app:testDebugUnitTest :app:assembleDebug :wear:lintDebug :watchcore:testDebugUnitTest :wear:testDebugUnitTest :wear:assembleDebug` | Passed after QA-003 fix |
| Android | `:app:connectedDebugAndroidTest` on fresh phone API 36 | Passed; 4 instrumented tests |
| Android | Focused `CatalogEmptyMessageTest` | Red for absent-search case before fix; green 2/2 after fix |
| Apple | `./scripts/build-apple.sh ios`, `tvos`, `watchos` | All builds passed |
| Apple | iOS simulator `xcodebuild test`, tvOS remote `xcodebuild test` | iOS 241/241; tvOS 11/11 |
| Repository | `make max-loc`, `git diff --check` | Passed |
| Repository | `make -C apps/player verify-changed` | Passed; 5 changed Player paths, file cap and diff checks. Android source was separately checked by Gradle above. |
| Repository | `make -C apps/subtitles verify-changed` | Passed; Go compile/focused tests and Playwright spec listing. |

The audit report, screenshots, and observation JSON are the repeatable E2E run artifact: they identify source, commands, test data, environment, outcomes, and evidence. Browser runner logs remain under the isolated local test roots. Temporary observation specs were removed after the JSON was saved.

## Verification boundaries

This is a sampled full-product audit, not a claim that every feature is tested. Automated axe scans cover the observed `main` regions and do not establish complete accessibility. Targeted keyboard and D-pad interaction do not replace a physical accessibility or remote review. The Android TV and Wear checks used emulators. Apple authenticated flows, real providers, Cast on API 23, physical devices, production TLS, publication, deployment health, and long-running resilience were not verified. Test data stayed isolated; no purchase, email, or destructive production workflow was attempted.

Hosted CI and remote-main integration are separate delivery evidence. They are not established by the local checks above.
