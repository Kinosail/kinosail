# Native polish checkpoint — September 30, 2026

This continues the [web polish checkpoint](../2026-09-30-all-app-polish/report.md). The native audit confirmed and fixed an Android library defect. It does not certify every app or physical device.

## Confirmed defect and change

A populated library containing an episode with canonical `/episode-art/{id}` artwork failed Android's shared response validator. One valid episode caused rejection of the whole library page. The isolated production Go API reproduced the failure twice through the Android client. The retained [synthetic response](library-response.json) contains the failing paths.

The shared artwork allowlist now accepts that canonical route. Phone and TV use the same validator and artwork client. Existing `/art/{id}` paths remain supported. Missing and oversized IDs, traversal, encoded IDs, extra path segments, queries, fragments, and unknown routes are rejected before an artwork connection opens.

The two new regression tests failed on the original validator. All 117 Android unit tests passed after the fix: app 106, watchcore 6, Wear 5. The authenticated phone journey then loaded a populated library and displayed the correct absent-search message. Its [library](evidence/android-phone-library.png) and [empty search](evidence/android-phone-search-empty.png) captures show the resulting UI.

## Apple rendering

The existing production SwiftUI render fixture produced 37 screens on iPhone and 37 on iPad. Seven captures per device use accessibility-size text. Separate lifecycle passes captured pending, loaded, empty, and failed Home and Library states on iPhone, iPad, and Apple TV. These fixtures use fictional library data over loopback. They are separate from the authenticated Go API launches.

The native UI suite passed with five test functions on both devices. It checks download preference persistence, scene transitions, cached progress, and artwork placement. The first artwork kept exactly the same vertical position when loading finished. See the [iPhone states](evidence/native-lifecycle-contact.png), [iPad states](evidence/native-ipad-lifecycle-contact.png), [TV states](evidence/native-tvos-lifecycle-contact.png), and [gallery](gallery.html).

The original Supporter failure capture ran before the 503 retry delay ended. Its test-only capture now waits through the retry window. The corrected [terminal state](evidence/supporter-failed.png) shows the error and Try again, with no loading placeholder. No Apple production behavior changed.

A real session was validated against the isolated Go API and saved through production Keychain code on task-owned iPhone and Apple TV simulators. Apple TV rendered [populated Home](evidence/native-tvos-home.png). Its focused control and Quick Play checks passed except for a focus test in the combined UI run. That test passed alone; concurrent UI suites can contend for the key window. The combined run remains recorded as failed.

## Verification and boundaries

| Check | Result |
| --- | --- |
| iOS and tvOS simulator builds | Passed; repeated by `make -C apps/player verify-changed`. |
| Android app, instrumented test APK, and Wear builds | Passed. |
| Android setup, playback error, reader error accessibility | 3/3 passed on API 36 phone emulator. |
| Android Cast chooser | 1/1 passed; proves chooser opens, not receiver playback. |
| Authenticated Android library and absent search | Failed twice before the fix; 1/1 passed after it. |
| iPhone gallery and lifecycle suite | Final runs passed. An initial cached-progress timeout did not recur; it remains unverified. |
| iPad gallery and lifecycle suite | Both passed. |
| Apple TV lifecycle gallery | 8 states captured; disposable fixture test passed. |
| Apple TV focus | Combined UI run failed; isolated focus run passed. |
| Apple Watch | Unpaired and paired simulators rendered waiting/fallback states. Nine protocol/progress tests passed. The paired session did not exchange usable app state; playback control remains unproved. |
| Source-file limit, diff check, changed-app gate | Passed. |

The iPhone URL search journey remains blocked at the system Open confirmation because Device Hub control timed out. Gallery Search captures are still pending and do not prove loaded search. Android Home's initial capture was pending; the journey verifies loaded Library. No physical phone, tablet, watch, Wear pairing, heart data, receiver casting, remote hardware, long playback, or native store release was verified. Android tablet and broader TV journeys remain. Android TV startup failed three times because its base userdata image requires about 7.4GB of free space; the host had about 3GB. Unrelated images and simulators were preserved. No Nox deployment was performed.

The web PR's exact merge revision `2a62a537af0b817f77010360da319168c5f8baed` passed main CI, both populated browser gates, both architectures' production-container gates, and Player/Subtitles container publication. That publication is separate from native device and Nox health evidence.

## Reproduction

Run Android unit regressions from `apps/player/apps/android` with JDK 17 and the repository Android SDK: `./gradlew :app:testDebugUnitTest :watchcore:testDebugUnitTest :wear:testDebugUnitTest`. The canonical-route regressions are in `CatalogApiTest` and `ArtworkClientTest`.

For Apple rendering, use a task-owned simulator and the existing `NativePreferencesUXTests` suite. Pass `TEST_RUNNER_KINOSAIL_POLISH_GALLERY=1` or `lifecycle` to `xcodebuild test`. Select the `Kinosail-iOS` scheme, that simulator's ID, and `-only-testing:Kinosail-iOSTests/NativePreferencesUXTests`. Preserve its `.xcresult` and the printed `NATIVE_POLISH_RENDER` images before deleting the simulator. Run TV UI suites separately when checking focus.

The disposable Android journey validated a real fixture session, saved it through `SessionStore`, launched `MobileActivity`, clicked Library, asserted a loaded title, submitted an absent query, and asserted the resulting message. Its [source](PolishLiveJourneyTest.kt.txt) and [read-only loopback bridge](android-loopback-bridge.py.txt) are retained as evidence outside production targets. Credentials and raw device results remain outside Git. The structured run context records revision, commands, synthetic data, environment, and artifact hashes.
