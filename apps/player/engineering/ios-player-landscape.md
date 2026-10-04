# iOS video landscape and dismissal

The iPhone player has a 44pt circular landscape action with native opposing diagonal arrows. It operates on the retained video surface's own window scene. The return action and playback departure request the interface orientation captured before entry. Playback retains the same AVPlayer, item, aspect-fit rendering and caption surface.

The action uses `UIWindowScene.requestGeometryUpdate` and `setNeedsUpdateOfSupportedInterfaceOrientations`. It does not change the system orientation lock. Requests are serialized, confirmed against actual scene geometry and bounded to three seconds. Denial, inactive windows and stale callbacks cannot stop playback or mark a failed request successful. Recoverable warnings include the fixed operation, request UUID and numeric error code; raw errors and media URLs are excluded.

The video background also accepts a downward close gesture. At least 100pt of movement and a vertical-to-horizontal ratio above 1.5 are required. Starts in the top 60pt or timeline region are excluded. Interactive controls are above the background hit surface. VoiceOver, options, volume and system Picture in Picture exclude the gesture. The visible Close player button remains available. Both routes use one idempotent close gate and the existing navigation callback. `PlaybackScreen` retains its existing pause and progress-saving departure behavior.

## Failure analysis and acceptance

Before production edits, the task recorded failure cases for wrong-window rotation, denial, inactive windows, stale callbacks, repeated taps, timeout, dismissal during rotation, prior-orientation restoration and player continuity. The swipe addition separately recorded accidental close, short/cancelled/horizontal drags, scrubbing/control conflicts, repeated close and lost progress.

The retained isolated native journeys protect the gaps not covered by the populated browser suite:

- An absent window and a portrait-only controller produce recoverable failures.
- Repeated requests and a superseding return cannot leave stale state.
- Actual portrait/landscape scene geometry retains player/item identity, paused position, speed, current track IDs, caption state and aspect fit.
- Reentry, immediate departure during rotation and rotation during active decoded playback preserve continuity.
- An accepted drag decision routes through a real `NavigationStack` and `PlaybackScreen`, pauses the player, saves the five-second position through the production writer and restores portrait.
- Short, sideways, upward and excluded-region drag decisions do not close; repeated gesture/button activation closes only once.

These are UIKit-hosted integration tests with a fictional loopback Server and synthetic decoded media. They are not populated Server E2E or physical-device proof. The gesture decision test does not inject finger touches. Audio/subtitle IDs are unchanged but nil in this fixture; caption continuity uses synthetic caption text. Nondefault track selection, physical audible audio, VoiceOver interaction, scrubbing hit testing, iPad cover dismissal and Portrait Orientation Lock enabled behavior require device acceptance. Pending network/buffering transitions are outside this fixture.

## Repeatable verification

Use an owned disposable iPhone simulator and serialize builds with other native work. Do not operate a physical device or another task's simulator. Generate the opt-in fixture on the Mac running the tests:

```sh
ffmpeg -f lavfi -i 'testsrc2=size=640x360:rate=24:duration=12' \
  -f lavfi -i 'sine=frequency=440:duration=12' \
  -f lavfi -i 'sine=frequency=660:duration=12' \
  -map 0:v -map 1:a -map 2:a -c:v libx264 -threads 1 \
  -preset veryfast -crf 32 -pix_fmt yuv420p -c:a aac \
  -metadata:s:a:0 language=eng -metadata:s:a:1 language=fra \
  -movflags +faststart /tmp/kinosail-landscape-task10-media.mp4
```

From `apps/player/apps/native`, run the focused tests with an actual owned simulator ID:

```sh
xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-iOS \
  -configuration Debug -destination "platform=iOS Simulator,id=$KINOSAIL_TEST_SIM" \
  -derivedDataPath .build/ios-simulator -jobs 2 -parallel-testing-enabled NO \
  -maximum-concurrent-test-simulator-destinations 1 \
  -collect-test-diagnostics never -test-timeouts-enabled YES \
  -maximum-test-execution-time-allowance 30 \
  -only-testing:Kinosail-iOSTests/PlaybackOrientationJourneys \
  -only-testing:Kinosail-iOSTests/PlaybackDismissalTests \
  -resultBundlePath "$KINOSAIL_TEST_RESULTS" \
  CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- test
```

The decoded-media journeys are skipped if the fixture file is absent. Successful runs write PNGs under the test app's `Documents/orientation-artifacts`, print actual window dimensions and explicitly mark `physicalLockProof=false`. Retain the result bundle, exact commit, SDK/device identifiers, command log, fixture checksum and screenshot checksums with task evidence. Inspect loaded portrait/landscape, return, large text, failure, opening and dismissed-library captures.

Run `make max-loc`, `git diff --check` and the affected Player `verify-changed` after committing. Required hosted checks and an independent review remain merge requirements. Device rollout belongs to the existing native owner after merge.
