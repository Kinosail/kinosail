# Wear launch and recreation acceptance

## Coverage gap

Existing Wear tests render `WatchApp` inside `ComponentActivity`.
They check disconnected announcements and heart-permission copy, but do not launch `WearActivity`.
This follow-up adds an actual launch and Activity recreation journey on two round emulator sizes.
It follows the Android caption repair in PR #593.

The journey requires a disconnected watch to show `Connect phone` and `Pair Android phone.`.
The status must use a polite accessibility announcement.
Playback controls must remain absent without a selected player.
Recreation must restore the same usable state.

## Failure contract

The runner must reject crashes, missing announcements, incorrect platforms, skipped tests, timeouts, and incomplete artifacts.
It requires all three selected tests on each size, including the two existing component tests.
An ADB success exit alone is insufficient.

The runner refuses existing devices and creates a watch with a unique name.
It confirms the owned process and watch identity before device actions or console shutdown.
Cleanup targets only the owned emulator.
Cleanup errors must retain a failed receipt.

The launch test captures available semantics and compositor images before closing a failed Activity.
Collection attempts every fixed artifact even when an earlier capture is missing.
Raw emulator and instrumentation logs remain private.

The receipt records the revision, source hashes, APK hashes, image revision, runtime properties, test inventory, artifact hashes, and cleanup result.
`inspect-wear-evidence.py` independently verifies the source and artifact inventories and fully decodes each PNG.
APK bytes are not uploaded, so the inspector cannot independently rehash their recorded digests.
Visual inspection remains required after automated artifact checks.

## Hosted trial

The [Wear OS 6 setup guide](https://developer.android.com/training/wearables/versions/6/setup) documents the signed API 36 image.
The candidate uses `system-images;android-36;android-wear-signed;x86_64` and records the installed image revision.
The [emulator guide](https://developer.android.com/training/wearables/get-started/emulator) distinguishes simulated inputs from physical watch behavior.

1. Build `:wear:assembleDebug` and `:wear:assembleDebugAndroidTest` on a disposable hosted runner.
2. Run `python3 apps/player/scripts/test-wear-journeys.py --output <new-output-directory>`.
3. Retain only the output directory's `safe` child as the artifact.
4. With Pillow available, run `python3 apps/player/scripts/inspect-wear-evidence.py <artifact.zip> <full-tested-commit>`.
5. Inspect both sizes' launch and recreation images and accessibility semantics.

The runner checks PNG headers and dimensions. Full image decoding belongs to the independent inspector.
Neither layer establishes physical watch acceptance.

## Current status

Source preparation and independent source review are complete.
Compilation, emulator execution, artifact decoding, and visual acceptance remain pending.
A separate hosted Wear job now runs when deep verification selects Android.
The app's required check rejects missing, failed, cancelled, or incorrectly skipped Wear execution.
Before integration, the regression check proved missing Wear execution was accepted.
All 24 affected CI contract tests pass after integration.
Local application execution remains held.

Pairing, remote playback, active-session permission denial, heart sensors, battery use, and physical input remain separate coverage gaps.
