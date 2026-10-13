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
2. Run `python3 apps/player/scripts/test-wear-journeys.py --profile <small-round-or-large-round> --output <new-output-directory>` on a fresh runner for each profile.
3. Retain only the output directory's `safe` child as the artifact.
4. With Pillow available, run `python3 apps/player/scripts/inspect-wear-evidence.py <artifact.zip> <full-tested-commit>`.
5. Inspect both profiles' separate artifacts, launch and recreation images, and accessibility semantics.

The runner checks PNG headers and dimensions. Full image decoding belongs to the independent inspector.
Neither layer establishes physical watch acceptance.

## Current status

Source preparation and independent source review are complete.
The first hosted trial compiled both APKs but failed before emulator creation.
The later executed trial was rejected during artifact inspection, and fresh-profile startup remains under repair.
Complete artifact decoding and visual acceptance remain pending.
A separate hosted Wear job now runs when deep verification selects Android.
The app's required check rejects missing, failed, cancelled, or incorrectly skipped Wear execution.
Before integration, the regression check proved missing Wear execution was accepted.
All 24 affected CI contract tests pass after integration.
Local application execution remains held.

Pairing, remote playback, active-session permission denial, heart sensors, battery use, and physical input remain separate coverage gaps.

## Initial ADB inventory repair

The first trial tested `0f5b2ca3dc2db46fe5f193e49bc0ea07a04fd497` in [job 115565070651](https://github.com/Kinosail/kinosail/actions/runs/38475473706/job/115565070651).
It retained `runner_device_inventory_invalid`, zero cases, and no created emulator.
Artifact `11854276433` has SHA-256 `857536458e708571aa166b87038eedb95b808c0c11c9ab3cf12df3b3cd3b4459`.
The original receipt did not retain the inventory text, so its exact contents remain unknown.

The runner merged ADB stderr into stdout before checking the inventory header.
[ADB source](https://android.googlesource.com/platform/packages/modules/adb/+/refs/heads/android15-qpr2-s8-release/client/adb_client.cpp) writes daemon startup diagnostics to stderr.
The repair keeps those diagnostics private and validates stdout separately.
Both initial and final inventories require the exact header.
Receipts expose only header validity and entry counts, without device identifiers.

The isolated CLI regression uses inert SDK executables and synthetic APK bytes.
It covers cold and warm empty inventories, connected, offline, and unauthorized devices, malformed headers, and command failure.
These controls protect emulator ownership; the hosted journey cannot safely introduce foreign devices or corrupt ADB output.
Before the repair, cold startup was rejected and a malformed header was accepted.
The control creates no real device and does not establish native acceptance.
Run it with `python3 -B -m unittest scripts.ci.test_wear_preflight`.

## First executed journeys: acceptance rejected

[Job 115570796858](https://github.com/Kinosail/kinosail/actions/runs/38477204665/job/115570796858) tested `730220fa76d3f1da50ff298ffa3a5efd757f4eae`.
Both sizes reported all three tests passing, but the complete run failed cleanup.
The owned process exited while its ADB transport remained in the immediate final inventory.
No later inventory was recorded, so delayed detachment is a hypothesis, not an observed outcome.

Artifact `11856950445` has SHA-256 `8c51adb66f76102ba0f5d226ec69ec3d59b035cf43c6b805039adc136c6b7998`.
All sixteen source pins and sixteen artifact hashes verify independently.
Both small-watch compositor images show the system's `Hello` screen despite passing Compose visibility assertions.
Those images do not establish app visibility. The large-watch images show the app but an off-center circular boundary.
Changing logical display size has not yet established faithful physical round-screen geometry.

The journey now requires the app to own the active accessibility window before capture.
Its semantics artifact records only the ownership Boolean, and the inspector requires a true value.
Android's [UiAutomation reference](https://developer.android.com/reference/android/app/UiAutomation#getRootInActiveWindow()) defines this active-window observation.
Visual inspection remains mandatory; foreground ownership alone cannot prove layout quality.

The existing shutdown assertion remains the regression owner for transport detachment.
Cleanup will observe the owned serial for up to five seconds after process shutdown.
It must still fail if the serial remains, inventory fails, or process cleanup fails.
It must never restart ADB or stop another process to manufacture disappearance.
Hosted verification of these acceptance changes remains pending.

## Fresh round hardware profiles

Each profile now boots its own fresh AVD with the physical size configured before startup.
The runner rejects logical size or density overrides and records physical-display verification.
Both matrix jobs must succeed, and each retains a separate artifact with three test executions.
The workflow contract rejects a missing profile or a shared artifact name.
Its failing baseline lacked the independent-profile matrix.

The fixture waits for system boot completion before device setup.
It sets `device_provisioned` and `user_setup_complete` only on its newly created, ownership-verified emulator.
The receipt retains their bounded before and after values.
This models a usable watch with no connected phone and avoids first-time setup obscuring the app.
No account is added, package is disabled, or phone is paired.
Android's [test harness guidance](https://source.android.com/docs/core/tests/debug/harness) recommends minimal setup-screen customization for automated UI tests.
The runner does not enable full Test Harness Mode or reset any existing device.
Foreground ownership and visual acceptance remain required after this fixture setup.

## Fresh-profile startup failure

Both profiles at `5e7db444fdbf34832e2370f2abd13b15a954d6a1` failed with `wear_boot_failed` before setup, installation, or test execution.
Each receipt records zero cases, successful owned-process shutdown, and transport removal after four inventory observations.
All sixteen source pins in each artifact verify independently.

| Profile | Job | Artifact | ZIP SHA-256 |
| --- | --- | --- | --- |
| Small round | `115578191236` | `11855804085` | `b44fae8a5b13aa8444cd919edb44d300da120b9ae6cbd67b8f133d513731dc3a` |
| Large round | `115578191370` | `11856173743` | `3b7208b3e966cd5a50bec88cb98c8186580b066d9c03e63010b07771f2bf3d62` |

The runner required `init.svc.bootanim=stopped` while launching with `-no-boot-anim`.
Android's [emulator command reference](https://developer.android.com/studio/run/emulator-commandline) says that option disables startup animation.
A disabled service need not establish the required stopped state.
The old receipts did not retain boot-property observations, so the exact unmet prerequisite remains unknown.

The correction requires the owned process and `sys.boot_completed=1`, with the same 180-second boot limit.
It retains bounded last-observation Booleans, command-failure status, and attempt count without raw ADB output.
Physical display, setup, foreground ownership, test selection, visual artifacts, and cleanup assertions remain required.
The hosted journey remains the regression owner; source checks alone cannot establish successful watch startup.
