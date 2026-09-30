# Android reader feedback verification

The Android PDF and comic reader now shows progress whenever a page image is pending, including after the page count is known. Loaded and failed states stop the indicator. Saving errors retain the loaded page and use a full-width opaque background with the existing control insets.

The change uses the existing `ReaderPageView`. No API, input validation, persistence, format support, or playback behavior changed.

## Evidence

- Loading baseline: 6 Compose tests ran before the production change. The known-count pending and pending-to-loaded transition tests failed because no progress node existed. The other 4 passed.
- Readability baseline: the native rendered white-page test failed at 1.707:1. An intermediate inset background passed under the text but failed at the viewport edge. The final background covers the full width.
- Final focused run: all 7 permanent reader tests and 3 temporary gallery tests passed. The gallery records 18 populated captures across 390×844dp phone, 834×1210dp tablet, and 844×390dp landscape configurations.
- Final full suite: 112 app, 6 watchcore, and 5 Wear unit tests passed, with zero skipped tests. Mobile/TV and Wear debug APKs and the instrumentation APK built successfully.
- Render audit: every captured semantics rectangle stays within its viewport. The final phone saving warning measures 11.42:1 against its rendered background. Both edge pixels use that background. Loaded-page pixels remain identical at all 3 sizes.
- `make max-loc`, `git diff --check`, and the committed Player `verify-changed` gate passed. The changed-app gate selected file-cap and diff checks only.

The loading tests cover initial unknown counts, known-count pending work, success, subsequent pending work, terminal failure, unsupported format, and save failure. Existing polite error announcements remain covered. The gallery compares initial pending, known-count pending, loaded, failed, unsupported, and loaded-with-save-failure states. A settled empty document is rejected by the existing reader contract; the unavailable-format state has no placeholder or retry action.

## Repeat

Use the repository JDK 17 and Android SDK. Run from `apps/player/apps/android`:

```sh
./gradlew :app:testDebugUnitTest --tests com.kinosail.player.core.ReaderAccessibilityTest --max-workers=2
./gradlew :app:testDebugUnitTest :watchcore:testDebugUnitTest :wear:testDebugUnitTest :app:assembleDebug :wear:assembleDebug :app:assembleDebugAndroidTest --max-workers=2
```

To repeat the gallery, copy `ReaderPolishGalleryTest.kt.txt` into the existing core test package as `ReaderPolishGalleryTest.kt`. Set `KINOSAIL_READER_CAPTURE_DIR` to a task-owned output directory. Select that class with `--tests`, then remove the temporary source. The archived test draws fictional page content and exercises the real themed reader component. It adds no production seam or dependency.

The contexts record revision, source hashes, command, fixture, environment, and result. The source was tested before committing; `run-context.json` binds its unchanged hashes to the final source commit. Baseline production and regression source snapshots are included. Artifact hashes are verified before delivery.

## Boundaries

These are Compose component tests and Robolectric native graphics captures on macOS, not emulator, physical-device, live API, or full-reader E2E proof. This batch did not rerun Android instrumentation on a device. It compiled that APK. Android TV and Wear have unit/build evidence; the book reader is reached through the mobile library route.

Apple clients and web applications did not change in this batch. Their local suites were not repeated. GitHub Actions remains the authority for the selected hosted suites, security checks, and publication. No Nox deployment, physical iPhone/Apple TV/Android device check, signed release, store distribution, Cast, paid activation, or long playback was verified here.

The primary checkout remains intentionally dirty. The independent worktree audit reported 16 expired checkouts with unique commits. They were preserved and do not block this task's lease or commit.

[View the responsive evidence gallery](gallery.html).
