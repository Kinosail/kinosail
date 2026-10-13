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
The later executed trial was rejected during artifact inspection.
Fresh-profile startup now passes, but actual Activity acceptance still fails.
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

## Boot correction verified; Activity failure retained

Both profiles at `912871d331069ac88f7ecf56392ba8c6021bd418` booted and verified the owned emulator, physical dimensions, and setup state.
Both also completed process and transport cleanup, with four final inventory observations.
Their three selected tests ran without skips, but JUnit reported failure.

| Profile | Job | Artifact | ZIP SHA-256 |
| --- | --- | --- | --- |
| Small round | `115583868730` | `11857289139` | `f7a98f28a5001e2a13b4624745a98a43f09270b561724736806f4c12a8a1d910` |
| Large round | `115583868742` | `11858435101` | `24252b30a17de8d60982f30dbcf8d0d9a7c6a7880772cd2c1b532e0908a9de2e` |

Each artifact independently verifies sixteen source pins and seven retained child hashes.
Both launch compositor images show the system watch face, with centered round boundaries.
Neither launch semantics file contains a Compose tree; recreation screenshots are absent.
The old collector retained the fixed-path missing-file response in place of semantics and correctly rejected it.
The raw instrumentation logs were not uploaded, and job output does not contain their exception details.
The exact failing assertion remains unknown; no production Activity defect is claimed from these artifacts.

The next diagnostic revision retains closed-enum failure signals, status counts, and bounded locations in the two fixed test files.
It never copies exception messages, arbitrary class names, stack lines, device identifiers, or raw logs into the receipt.
Capture records its fixed phase and foreground Boolean before reading the Compose tree.
A failed tree read can no longer erase those observations, and screenshot capture remains independent.
The existing hosted journey owns this failure; its assertions and deadlines remain unchanged.

## Diagnostic trial: different failing phases

At `6b520b27c34c93b5974b223524145e8fcde7c202`, both profiles record two successful tests and one failure, with no skips.
Both retain successful boot, physical-display checks, setup flags, and cleanup.
All sixteen source pins and seven child hashes per profile verify independently.

| Profile | Artifact | ZIP SHA-256 | Observation |
| --- | --- | --- | --- |
| Small round | `11858856792` | `2a3b8dcadf701aab62a474ff1b66da6ad3a0aebbaa3a2868585fcf7989765259` | The disconnected component test records `IllegalStateException` at line 47. Actual launch and recreation captures show the app and retain true foreground ownership. |
| Large round | `11858911477` | `9ce0fc8c38e0ea4b4b2e36e239197357d9f9a25e21fc397a3212e83cce52543d` | The launch test records a Compose timeout at the foreground wait. Its capture records false ownership and `phase=foreground`. |

The small-watch actual captures show centered round geometry and the expected disconnected content.
Its separate component failure still prevents accepting the complete profile.
The large-watch failure proves missing app foreground. The small-watch exception does not establish the same cause.
No production Activity cause is proven.

The next single-variable fixture experiment launches the system Home Activity after fresh-device setup and APK installation.
Android's [ADB reference](https://developer.android.com/tools/adb#am) defines `am start -W` as waiting for launch completion.
The owned emulator must return exit zero, exact successful status, and completion within a bounded command.
Only success Booleans and elapsed time enter the receipt.
This establishes Home launch completion; it cannot guarantee a later system transition will not replace the app.
Instrumentation assertions, deadlines, and UiAutomation flags remain unchanged.
If failures persist, the next observation should capture bounded Activity lifecycle and window-focus state at the original failure.

## Home launch completed; foreground failure persists

Both profiles at `056ffc0fc419ea9adecf4eca7ba99dc96fe55be8` completed Home launch, in 0.046 and 0.053 seconds respectively.
Both still fail the original foreground wait, record two successful tests and one failure, and clean up completely.
Both capture headers record false foreground ownership and `phase=foreground`; both compositor images show the watch face.
This rejects Home launch completion as a sufficient repair.

| Profile | Artifact | ZIP SHA-256 |
| --- | --- | --- |
| Small round | `11858539003` | `655084e01dff23cddec66223b2ac1f373851df722fb743a12aa54cfadc142e33` |
| Large round | `11858279156` | `21ff988acc572c48bb093bc3ef6d4de654b76d8b0e432339fcdeae422286c9c0` |

Each artifact verifies sixteen source pins and seven retained child hashes.
The next diagnostic records lifecycle stages and focus, finishing, destroyed, and configuration-change Booleans for `WearActivity`.
It observes once after launch, before accessibility setup, and again at capture before the Compose tree read.
Queries run on the main thread, as required by Android's [ActivityLifecycleMonitor contract](https://developer.android.com/reference/androidx/test/runner/lifecycle/ActivityLifecycleMonitor).
Each observation waits at most 500 milliseconds and reports a fixed timeout or failure marker if unavailable.
It retains at most two matching Activities per fixed lifecycle stage, without object identities or arbitrary class names.
The monitor may release old Activities; an empty observation does not prove that an Activity never existed.
No lifecycle listener, Activity reference, production seam, assertion relaxation, or added retry is introduced.

## Lifecycle evidence narrows the displaced Activity

At `fec8cfa7a298cd095072e985d546187b750f6ba7`, both complete profiles still fail, with two successful tests and one failure each.
The small-watch Activity is resumed without focus before the foreground wait, then stopped without finishing or destruction at capture.
Its screenshot shows the system watch face. This identifies Activity displacement, rather than destruction, in that run.
The large-watch launch and recreation pass with resumed, focused Activities and the expected disconnected app screenshots.
Its disconnected component test fails at the Compose query instead.

| Profile | Artifact | ZIP SHA-256 |
| --- | --- | --- |
| Small round | `11859052868` | `51bc068597e97b0203b8ac77f41d72e342953879a42f82142afb3ee068191fe0` |
| Large round | `11859980959` | `e195f8a05ba7ef9d2a246fc38c7f30c08781f5d825e788e506e364b49e275f1b` |

Each artifact verifies sixteen source pins, seven child hashes, and complete cleanup.
The successful actual journey does not establish a repair: the change only added observations, and the complete profile fails.

The next diagnostic shares the bounded observer across the fixture and actual Activities.
It adds only interactive, keyguard-locked, and device-locked Booleans, without changing power or lock state.
Component failures retain this observation and a compositor image before the test rule closes the Activity.
The original exception remains the test failure; evidence errors become suppressed exceptions.
A fixed `missing-compose-root` signal identifies the known Compose exception without exposing its raw message.
The runner, inspector, and assertions continue to reject failed tests and incomplete evidence.

[Wear power-state documentation](https://developer.android.com/training/wearables/always-on) describes inactivity transitions that can return to the watch face.
This is a hypothesis for the observed displacement, not a confirmed cause.
The [interactive-state API](https://developer.android.com/reference/android/os/PowerManager#isInteractive()) and
[lock-state API](https://developer.android.com/reference/android/app/KeyguardManager#isDeviceLocked()) distinguish device state without taking a wake lock or dismissing the keyguard.

## Displacement occurs while interactive and unlocked

At `470234c6e40c000ef80c2bb6d610dda0f0ce79ef`, the large profile passes all three tests.
Its seventeen source pins, eight child hashes, four decoded PNGs, foreground checks, and cleanup all verify.
Visual inspection confirms the actual app after launch and recreation; component images remain isolated render evidence.
The small profile passes both component tests but fails the actual foreground wait.
Its seventeen source pins, seven available child hashes, and cleanup verify.
Before the wait, its Activity is resumed without focus. At capture, it is stopped without finishing or destruction.
Both snapshots show an interactive device, an unlocked keyguard, and an unlocked user.
The fixed `missing-compose-root` signal also occurs. The launch screenshot shows the watch face.
These observations do not support a power or lock-state change at either sampled instant.
They do not exclude an unobserved intermediate transition.

| Profile | Artifact | ZIP SHA-256 |
| --- | --- | --- |
| Small round | `11860158000` | `c4864362b01e71bfad8b880fabfa32b4d28aa755f740d33ec3306a7b58ad3962` |
| Large round | `11859154369` | `5a703ddc6d58c542c48651e3561713cb93e29e8ba75acfdaa14fad297f0cc38a` |

The next experiment runs the actual Activity journey before the two component tests.
It changes only class order, retaining all three tests, assertions, timeouts, and automation flags.
A bounded sequence of known test names from instrumentation status lines confirms the observed order and reports truncation.
Truncation or an observed component-first order makes this experiment inconclusive.
Each test normally appears at start and finish; the sequence is not an execution count.
The existing exact selection and three-pass checks remain authoritative.

The pinned [AndroidX test-core 1.7.0 source](https://dl.google.com/dl/android/maven2/androidx/test/core/1.7.0/core-1.7.0-sources.jar)
starts a temporary Activity when finishing a scenario, then broadcasts its cleanup.
Preceding fixture teardown is therefore a testable hypothesis, not a diagnosed library defect.
An actual-first failure would reject preceding component teardown as a necessary cause.
An actual-first pass alone would not prove the hypothesis or establish a repair.
Reordering also changes time since wakeup and instrumentation startup.
