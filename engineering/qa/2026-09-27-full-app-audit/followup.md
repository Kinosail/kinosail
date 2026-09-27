# Full app QA follow-up — 2026-09-27

## Run record

- Mode: full audit follow-up to [the initial report](report.md), starting with Player web. Source baseline: `7f69539c20307f7b92f4541a6f7b651188d47908`. The rebased implementation commit is `bb256fc8`; the final main revision is in Git history.
- Web environment: macOS, local Podman, repository-pinned Playwright and axe, and isolated synthetic Player and Subtitles instances. The Player instance used port 38127; Subtitles used port 38128. Both had fresh Owner accounts and fixture media. No production records were used.
- Android environment: fresh API 23 ARM emulator, debug APK, isolated Player account and synthetic Example Movie. A loopback-only proxy exposed the local HTTPS test instance to the emulator. No Cast receiver was available.
- Reset: each test instance used a unique `KINOSAIL_TEST_PROJECT` and `KINOSAIL_TEST_ROOT`. Run `./scripts/test-instance.sh down --volumes` in each app with those same variables. The Android test clears app data before setup-sensitive tests.
- The new Playwright tests include controlled negative runs. A missing badge SVG failed the image-load assertion; an invalid Quick Connect secret failed the expected 201 assertion. The temporary changes were restored before the green runs.
- [The regression control record](evidence/followup-regression-controls.txt) has the exact red and green outcomes. The Android XML files preserve the full failure stack and final connected-test counts.
- `origin/main` advanced to `7a2aaa4c12bc71384306ace866e74b81e8afafcb` during this run. The task was rebased cleanly. Both changed-path gates and the hosted Subtitles browser gate passed again after reconciliation.
- The first hosted tooling check found stale generated architecture snapshots after that main update. The repository generator refreshed Player and Subtitles snapshots; `make tooling-check` then passed locally. The refreshed hosted result is tracked by the PR.

## Findings and fixes

### QA-004 — Expanded Subtitles tests used Player journeys (P2, resolved)

The original nondefault suite imported Player-only shell, media, and playback tests. Its failures expected controls that Subtitles does not provide. Those copied tests were removed from Subtitles; their Player-owned counterparts remain under `apps/player/e2e`. The valid Subtitles Viewer MFA and Supporter tests remain. The suite now adds a real Quick Connect authorization and token-exchange test, and verifies all twenty badge images, unique artwork, accessible names, phone layouts, and axe findings. The local test-instance and hosted container browser commands now run this suite. The hosted harness generates the fixture pages, so its two Supporter fixture tests run instead of skipping.

The earlier failed run was a test ownership problem, not evidence of a Subtitles product bug. The repaired local Chromium suite passed 28/28, and the full local Firefox/WebKit suite passed 56/56 before the final hosted fixture update. The hosted container passed 30/30 with the fixtures enabled.

The new badge check captured [desktop](evidence/subtitles-badges-desktop.png) and [320 px phone](evidence/subtitles-badges-phone.png) views. It also checks 390 px and forced-colors rendering.

### QA-006 — Tapping Cast crashes Android 6 phone (P1, confirmed, fixed)

- **Expected:** Tapping Cast while playing a video opens the route chooser, or shows that no receiver is available, without ending playback.
- **Actual:** On API 23, the app crashed with `IllegalStateException: You need to use a Theme.AppCompat theme (or descendant) with this activity.` The Cast chooser comes from AppCompat, while the activity used the platform Material theme.
- **Reproduce:** (1) Install the debug phone app on a fresh API 23 emulator. (2) Pair with the isolated populated Player instance. (3) Open Example Movie and start playback. (4) Tap Cast. The original build exits to the launcher. This happened in two manual attempts and in the new instrumented regression test.
- **Fix:** Set the app theme parent to `Theme.AppCompat.NoActionBar`. This preserves the dark player and supports `MediaRouteChooserDialog`.
- **Evidence:** [red instrumented result](evidence/android-api23-cast-red.xml) records the exact exception. [Green result](evidence/android-api23-cast-green.xml) records five passing phone/core/TV accessibility and Cast tests on API 23. [The manual after screenshot](evidence/android-api23-cast-after.png) shows the chooser over Example Movie with “Looking for devices…” and the app still running.
- **Regression proof:** `CastChooserTest` was written before the theme fix and failed 1/1 with the exact AppCompat exception. It passed after the one-line theme change, then failed again in a controlled run using the original theme. After restoring the fix, the full API 23 connected suite passed 5/5. Receiver discovery and an actual Cast session remain unverified.

## Verification

| Surface | Result |
| --- | --- |
| Subtitles local Chromium expanded suite | 28/28 passed before the final hosted fixture update |
| Subtitles local Firefox/WebKit expanded suite | 56/56 passed before the final hosted fixture update |
| Subtitles hosted container browser gate | 30/30 passed with generated fixture pages, before and after reconciliation |
| Player web reconciled isolated instance | `verify` passed for synthetic Movies, Shows, music, books, photos, playback, and API inventory; Chromium browser suite passed 60/60 |
| Player web Firefox/WebKit expanded suite | [107 passed, 10 skipped, 1 Firefox login-load timeout](evidence/player-cross-browser-followup.txt) in 118 attempts; the timed-out test passed 1/1 when rerun alone |
| Player web reconciled Firefox/WebKit selection | 6/6 passed: populated library accessibility, Compatibility behavior, and preferred-language subtitle choices |
| Android API 23 | Cast red 1/1 on original theme; green 5/5 on fixed theme; lint, unit tests, and APK build passed |
| Repository changed-path gates | Player passed (2 paths); Subtitles passed (10 paths, including container-native), before and after reconciliation; `make max-loc`, shell syntax, and `git diff --check` passed |
| Repository tooling | `make tooling-check` passed after regenerating both architecture snapshots |

Replay the local browser checks with each isolated test instance running and `KINOSAIL_TEST_TOTP_SECRET` loaded from its private test root. From `apps/subtitles`, run `./scripts/test-instance.sh browser` with its `KINOSAIL_TEST_PROJECT` and `KINOSAIL_TEST_ROOT`; run `make browser-test` for the hosted container gate. From `apps/player`, run `./scripts/test-instance.sh browser` for Chromium. The Firefox/WebKit expanded run used the same sixteen specs listed by that script with `KINOSAIL_BROWSER_MATRIX=full`, `--project=firefox --project=webkit --workers=1`. Android used `:app:lintDebug :app:testDebugUnitTest :app:assembleDebug :app:connectedDebugAndroidTest` with JDK 17, the Android SDK, and `ANDROID_SERIAL=emulator-5570`.

## Remaining boundaries

This follow-up does not establish complete product coverage. It does not verify a real Cast receiver, physical Android or Apple devices, an authenticated Apple simulator journey, external subtitle providers, paid supporter checkout, production deployment, or long-duration playback recovery. The initial report records the sampled full audit of web, Android phone/TV/Wear, and Apple iOS/tvOS/watchOS. Those app journeys remain separate from the source and local test evidence here.

## Further iteration — passive passkey error

On a fresh Player sign-in page, the automatic passkey offer received HTTP 503 from the isolated local server and showed “Could not use your passkey” before any sign-in attempt. This reproduced in Chrome and the in-app browser. The automatic offer now stays quiet on failure; an explicit click still shows the error. The passkey script URL moved from `v=14` to `v=15` because the old URL is cacheable for 24 hours.

The test was added first: `KINOSAIL_E2E_URL=http://127.0.0.1:38129 pnpm --dir apps/player/e2e exec playwright test passkeys.spec.ts --grep 'unavailable automatic' --project=chromium` failed on the original script. After the fix, the full passkey spec passed 24/24 across Chromium, Firefox, and WebKit. The updated sign-in script URL failed its Go server test before the version bump and passed afterward; the related `TestPasskey|TestPasswordLoginOffersPasskey` selection passed. A rebuilt isolated Player instance showed an empty status on first load at the previously cached `127.0.0.1` origin, then showed the error after the passkey button was clicked. The fixture used generated media and a fresh Owner; its container and volumes were removed after verification.

The same iteration rendered a populated Chrome desktop library, a 390 px phone library and movie detail, and playing Arrival on the phone through its 12-second fixture. A current iOS simulator build succeeded and showed the Connect screen; simulator input control was unavailable, so this does not verify authenticated iOS playback. The task-created simulator was removed after capturing the setup state. No claim of 20/20 or complete device coverage follows from these checks.
