# Android and Apple parity

Android phone, tablet, TV, and Wear now share more of the Apple clients' navigation, content hierarchy, and playback behavior.

| Surface | Result |
| --- | --- |
| Phone and tablet | Home, TV Shows, Movies, and Search tabs; persistent More; Viewer-scoped tab customization; navigation rail on larger windows. |
| Home and Listen | Separate watching and listening; resumed title; recent category shelves; unwatched shelves; movie genres; My List. |
| Artwork and details | Contained posters, backdrops, square audio artwork, episode images, metadata, and saved-position labels. Unknown durations do not create completion percentages. |
| Shows | Shared hero and resume action; season selection; episode rows on phone and season rail with episode cards on TV. |
| TV | Watch/Listen entry points, direct continuation playback, focused controls, browsing, details, and My List. |
| Playback | Library navigation disappears during playback. The title and actions hide with the video controller and return on input. Accessibility keeps controls available. |
| Wear | Vertical Remote/Heart pages, play/pause icons, and bounded Go to time requests scoped to the selected title. Heart tracking remains local and opt-in. |
| Loading and recovery | Home, catalog, and show placeholders use the loaded artwork shapes and widths. Errors and empty results remove pending placeholders. |

## Evidence

Open [the screenshot gallery](gallery.html). Each image links to its accessibility semantics. [Run context](run-context.json) records source revisions, APK and bundle hashes, screen dimensions, environment, and unit results.

The final production revision is `3755d45be`. The final TV-capable test harness revision is `39ef73fba`. Later commits only package this evidence and reconcile main.

| Check | Result |
| --- | --- |
| Android unit suites | 144 passed: app 131, watchcore 8, Wear 5; no skipped unit tests. |
| Phone instrumentation suite | 10 passed, 2 opt-in live-Server journeys skipped; runner reports 12 tests. Includes existing setup, playback error, reader error, and Cast chooser checks. |
| Final phone parity journeys | 5 passed with the final harness: catalog states, navigation/My List/tab customization, Home states, playback, and seasons. |
| Tablet and landscape | Four full journeys passed in each layout; three asynchronous-state journeys passed after the final placeholder changes. |
| Android TV | 5 passed using focus requests and D-pad activation. Screens include moved focus, My List, seasons, and hidden/restored video controls. |
| Wear instrumentation | 2 passed: disconnected remote and heart-permission status, including an accessibility check. |
| Build and lint | App and Wear debug builds, test APKs, both release bundles, and both debug lint checks passed. Release bundles are unsigned. |
| Repository checks | `make max-loc`, `git diff --check`, and Player `verify-changed` passed. This local changed-path gate runs max-loc and diff-check for these Android paths. |
| Design detector | No findings on the selected Kotlin UI files. Runtime screenshots and accessibility checks provide separate evidence. |

Unit XML and command output are in [logs](logs). Accessibility checks cover captured phone/tablet/landscape library states. TV checks verify semantics and focus. Playback screenshots use `FLAG_DONT_USE_ACCESSIBILITY`, so screenshot capture does not falsely activate the product's accessibility behavior.

## Repeat the checks

Use Java 17, compile SDK 37, and isolated API 36 ARM64 emulators. Runtime configurations are recorded in `run-context.json`. The tablet layout uses a resized phone emulator; it does not prove physical tablet behavior.

From the repository root:

```sh
apps/player/apps/android/gradlew -p apps/player/apps/android \
  :app:assembleDebug :app:assembleDebugAndroidTest \
  :app:testDebugUnitTest :watchcore:testDebugUnitTest :wear:testDebugUnitTest \
  :app:lintDebug :wear:lintDebug :app:bundleRelease :wear:bundleRelease
make max-loc
BASE=origin/main make -C apps/player verify-changed
```

Install the debug app and test APK on an isolated device, then run:

```sh
adb -s DEVICE shell am instrument -w -r \
  -e class com.kinosail.player.core.NativeParityJourneyTest \
  -e capture phone \
  com.kinosail.player.dev.test/androidx.test.runner.AndroidJUnitRunner
```

For TV, add `-e tv true` and use `-e capture tv`. Omit `-e class` to run the full phone instrumentation suite. On Wear, install its app and test APK, then select `com.kinosail.player.wear.WearAccessibilityTest`.

The fixture runs an on-device HTTP server with one synthetic Viewer, generated artwork, a movie, music, and a two-season show. Its video asset contains a generated 20-second H.264/AAC test pattern. No live library or account is required. Export evidence with `adb exec-out run-as com.kinosail.player.dev tar -C files/parity-evidence -cf - .`.

## Fixes established by regression checks

- A missed initial controller-visibility event kept custom playback chrome on screen. The real decoder journey failed before the fix and passes afterward.
- Accessibility checks found partially visible cards without a spoken title. Cards now expose their title explicitly.
- A wrong-type saved tab value crashed loading. The focused test failed before recovery was added. Invalid values now fall back without rewriting preferences.
- Loading cards now match shelf, grid, and episode dimensions instead of dividing every window into the same number of columns.

## Remaining boundaries

This work aligns presentation and common interactions. It does not add Apple's offline downloads, full EPUB reader, collections, or every advanced Settings capability. Android's existing supported playback and document formats remain the boundary.

Physical phones, tablets, TVs, watches, paired Wear remote delivery, real heart sensors, Cast playback, long playback, live-Server playback, store signing, and store distribution were not verified. Opening the Cast chooser is separate from streaming to a receiver. Wear seek bounds passed unit tests; paired command delivery was not exercised.

API 37 builds passed. The initial API 37 instrumentation attempt stopped in AndroidX input injection before product journeys; the recorded successful runtime checks use API 36.

GitHub's required checks remain the delivery authority. This packet records local evidence, not hosted CI, deployed container health, or physical-device proof. Unrelated dirty primary-checkout work and inactive worktrees were preserved.
