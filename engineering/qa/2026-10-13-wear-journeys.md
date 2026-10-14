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
Home readiness permits both actual Activity journeys to pass, with complete artifact decoding.
The corrected app name fits both round displays at launch and recreation on the final Wear source.
Required repository gates remain pending, including a newly discovered production-image vulnerability repair.
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

## Actual-first execution still fails

Both profiles at `af7c723fd31dddfba0878cdf9017a4c2f83c8c9f` execute the actual Activity journey first and still fail it.
Both later component tests pass. The six status entries confirm start/finish order without truncation.
Each artifact verifies seventeen source pins, seven child hashes, and complete cleanup.
Both actual Activities change from resumed without focus to stopped without finishing or destruction.
Both snapshots remain interactive and unlocked; the launch screenshots show the watch face.
The later component snapshots show resumed, focused Activities.
This rejects preceding component teardown as a necessary cause of the foreground failure.

| Profile | Artifact | ZIP SHA-256 |
| --- | --- | --- |
| Small round | `11860108712` | `cbace5c673736e6764dff012484c18efdbfd90af3dc94594716cad7fd192e7bd` |
| Large round | `11860545506` | `e205e373024d8bc3c87e903ec5e19a51c0c06fcf8427615720189cc3c847499e` |

The next experiment connects UI automation before launching the Activity and reuses that observer.
The [Android 16 implementation](https://android.googlesource.com/platform/frameworks/base/+/refs/tags/android-16.0.0_r1/core/java/android/app/Instrumentation.java)
creates and connects this observer on first access; the no-argument accessor uses default flags zero.
Previously, the journey first accessed it inside the foreground wait, after Activity launch.
Moving this setup earlier tests whether observer initialization contributes to Activity displacement.
Flags, lifecycle operations, assertions, deadlines, class order, and screenshots remain unchanged.
No retry, wake action, forced focus, or production behavior change is added.
A successful trial would still need final-source verification and repeat acceptance before claiming the intermittent failure is resolved.

## Early observer connection is insufficient

Both profiles at `6fdba7b955ad584914a0120b902b2c6f97380391` still fail the actual foreground wait.
Both later component tests pass. Actual-first execution is confirmed without truncation.
Seventeen source pins, seven child hashes per profile, and complete cleanup verify.
Both actual Activities change from resumed without focus to stopped, without finishing or destruction.
Both sampled instants remain interactive and unlocked. Both launch screenshots show the watch face.
Connecting UI automation before launch is therefore insufficient to prevent this failure.

| Profile | Artifact | ZIP SHA-256 |
| --- | --- | --- |
| Small round | `11861255926` | `0a9ade04d20085cfeb62b868bcc030ccbaf8802f2370ea3e501f5c1f43fc4a09` |
| Large round | `11861250798` | `1311951d7d1ea1b83c4953477e81cee7b7bc829b5c2908f920355c13dc607949` |

The next diagnostic retains a bounded system lifecycle timeline from the owned emulator.
It clears only that fresh emulator's event log immediately before instrumentation.
After instrumentation, it reads the unfiltered event-log tail with a ten-second command limit.
The private reader requests at most 1,000 records, then projects seven fixed lifecycle tags.
Raw bytes stay in a private file outside the uploaded directory, capped at one MiB.
The receipt contains at most 64 events, fixed Activity roles, known reason labels, and user-leaving Booleans.
The resolved Home component identifies the `home` role; unknown components and reasons become `other`.
No component names, process or task IDs, action strings, URIs, or unknown reason text are uploaded.
Input, tail, and event limits are explicit; malformed records and missing diagnostics are reported.
A reset failure or truncated timeline cannot establish a complete instrumentation window.
The system ring buffer can overwrite older records; absence of an event does not exclude a transition.
Events include teardown after the test, so a final Home transition alone is not a launch-failure cause.
The original assertions, deadlines, class order, and observer setup remain unchanged.

The projection follows the pinned [Android 16 event schema](https://android.googlesource.com/platform/frameworks/base/+/refs/tags/android-16.0.0_r1/services/core/java/com/android/server/wm/EventLogTags.logtags).
Its source SHA-256 is `bfbb60473bb0c9c1a2244d66f7b6fa0e7754d2c0f7db0e6d8032a40c6dba93f7`.
Five isolated source checks protect diagnostic privacy, malformed-input rejection, and explicit truncation.
The first three were written before the projector; the initial run failed because the module did not yet exist.
Independent review caught a filtered-tail limit that could hide earlier event loss.
A fourth check reproduces that mistake and passes after removing reader-side tag filtering.
A fifth rejects invalid signed 32-bit event fields, including intent flags, before projecting transitions.
This is not a reproduced Android failure or native acceptance.
The hosted journey remains the owner for lifecycle behavior and visual acceptance.

All 31 focused source checks pass after these repairs, including the existing inventory and workflow contracts.
Source caps, whitespace checks, and the final independent source review pass.
Hosted execution is pending; these local checks execute no Android runtime.

## Another Activity resumes before teardown

Both profiles at `9b0dcda95857407db408fb5112c4bc24203b46da` fail the same actual foreground wait.
Both later component tests pass. Eighteen source pins, seven child hashes per profile, and complete cleanup verify.
Both launch screenshots show the watch face. Power and lifecycle snapshots match the preceding failed trial.
The event buffer resets successfully; the small profile retains 37 events and the large profile retains 41.
Neither reports malformed events, byte truncation, event truncation, or reaching the tail limit.

Both timelines start with this sequence:

1. Create Wear and pause the resolved Home Activity.
2. Resume Wear and stop Home.
3. Create another Activity, represented by `other`.
4. Pause Wear with `pauseBackTasks` and `userLeaving=true`.
5. Resume `other` and stop Wear.
6. Create the test-driver Activity for later teardown.

This locates the displacement before scenario teardown. It does not identify the initiating caller or the other Activity's package.
The `userLeaving` field is a platform observation, not proof of a physical user action.
Ring-buffer limitations still apply despite the absence of truncation flags.

| Profile | Artifact | ZIP SHA-256 |
| --- | --- | --- |
| Small round | `11861542395` | `6c33d0c62b6190999260888c38327536061e47803d2405f19522eafa60ca163b` |
| Large round | `11861672556` | `ea874c88c9b6348f11ef72d4018c32109ef119c172074165cd64a172698fb566` |

The next diagnostic refines only the uploaded role classification.
It distinguishes another Activity in the resolved Home package, the target package, or the instrumentation package.
A fixed map recognizes known Android and Wear system packages without uploading their names.
An exact `android.service.dreams.DreamActivity` class match has the `system-dream` role.
That label describes a class-name match; it does not establish an ambient state, system ownership, the initiating caller, or a complete cause.
All unmatched packages and classes remain `other`.
Raw component names, IDs, URIs, and unknown reasons remain private.
The new isolated privacy control fails on the previous classifier and passes after refinement.
All 32 focused source checks, source caps, and whitespace checks pass. Runtime acceptance remains open.

## The replacement belongs to the Home package

Both profiles at `a0fc41055bb46186cd6a0027d7048eafc0ba6e11` fail the actual foreground wait.
Both later component tests pass. Eighteen source pins, seven child hashes per profile, and complete cleanup verify.
Both launch screenshots show the watch face. Lifecycle and power observations match the preceding failure.
The small profile retains 41 events and the large profile retains 40.
Both reset the event buffer and report no malformed events or truncation flags.

The initial sequence now identifies the replacement as `home-package`: another Activity in the resolved Home package.
It is created and resumed while Wear pauses and stops, before the test-driver Activity is created for teardown.
This identifies package membership, not the initiating caller, exact component, or complete cause.
Neither replacement has the `system-dream` role.

| Profile | Artifact | ZIP SHA-256 |
| --- | --- | --- |
| Small round | `11861593241` | `4f9663f3d8f7d2a12bef282e4d7b9d5915b3b9cf6e43798a77e852516c011ab8` |
| Large round | `11861498956` | `cf5c700ae6da5745034f71ab7734518cf1361e79613a5e5e23d8468b48dbb0fc` |

The next controlled experiment observes current Home state after the existing wake action, immediately before instrumentation.
It waits at most 30 seconds for one global top-resumed Activity in the Home package, different from its routing entrypoint.
Each read has a two-second limit. Missing, ambiguous, oversized, or per-display-only records cannot satisfy readiness.
Raw snapshots remain private and capped at one MiB; the receipt contains only fixed Booleans, attempt count, and elapsed seconds.
Timeout retains a failed receipt and owned-emulator cleanup. It does not fall back to accepting the routing Activity.
A timeout is an experimental prerequisite failure, not evidence of an app defect.
A successful observation proves current resumption only, not visible readiness or absence of a later takeover.
No additional wake action, forced focus, retry of instrumentation, or assertion change is introduced.

The pinned [Android 16 Activity manager source](https://android.googlesource.com/platform/frameworks/base/+/refs/tags/android-16.0.0_r1/services/core/java/com/android/server/wm/ActivityTaskManagerService.java)
prints `ResumedActivity:` from the global `getTopResumedActivity()` result.
Its SHA-256 is `e6a7aba9a938a77b7aae5ff0e289b1972975c2cb32e1972626bc8c69de3512d0`.
The readiness privacy control was written before the parser and initially failed because the function did not exist.
All 33 focused source checks, source caps, whitespace checks, and independent source review pass.
These checks execute no Android runtime. Hosted acceptance of this readiness experiment remains pending.

## Home readiness passes; visual review finds brand clipping

Both profiles at `966408cea25c88b6252bf2c9877c59c95800ddea` pass all three selected tests without skips.
The small profile observes the follow-on Home Activity after 45 reads and 11.034 seconds.
The large profile observes it after 41 reads and 10.130 seconds.
Both observations are available, unambiguous, untruncated, and in the expected Home package.
Eighteen source pins and all eight child hashes verify per profile; all four PNGs per profile decode.
Both independent inspectors accept the receipts. All eight images were inspected, and both owned emulators clean up fully.
The app owns the active window at launch and after recreation on both sizes.
This supports a Home startup-order interaction, not identification of the initiating caller or proof against every future takeover.

| Profile | Artifact | ZIP SHA-256 |
| --- | --- | --- |
| Small round | `11862950809` | `9e127b23639e8583111bfa060faf744a2a4451ad009b5d7e69c0079dd473f9b9` |
| Large round | `11863060754` | `f7264a62ea3d9e59e50929af4e967bb35a5a99c91f11511c5b063003ad827ecd` |

The large compositor image visibly clips the first letter of `KINOSAIL` against the round edge.
Both retained launch semantics place its bounds at `[44, 72, 158, 108]` pixels.
The farthest corner is 239.821 pixels from the large display's center, outside its 227-pixel radius.
On the small display, that distance is 190.536 pixels, within its 192-pixel radius.
Rectangular Compose visibility therefore missed a real circular-display defect.

A new assertion checks the displayed app name's nonempty bounds against the round viewport at launch and recreation.
It was written before the production edit; the retained geometry establishes the failing large-display baseline.
The new Kotlin assertion itself has not yet executed on the hosted watches.
The correction centers only the app name, preserving vertical spacing, copy, controls, and state handling.
Disconnected and permission-message states have retained images; paired loaded states, pending transitions, enlarged type, and physical watches remain separate boundaries.

The final candidate removes temporary Kotlin lifecycle/power observations and the system event timeline.
It retains bounded Home readiness, private failure artifacts, foreground ownership, all original tests, and strict cleanup.
The readiness helper and source-pin inventory are renamed together; the existing privacy regression remains.
All 27 remaining focused source checks pass. Final-source hosted execution and visual review are pending.

The broader `966408ce` run also fails the Swift build before native tests execute.
Its private log identifies a tvOS asset-catalog runtime launch/handshake failure, without a Swift source error location.
Log SHA-256: `0c47778b1bcc90500ae127ca1761cdf341da566916172a93b8cf7b3527e8afd7`.
This is a separate hosted build failure, not a passing native test run. The next candidate must pass that gate too.

## Final Wear source passes both profiles

At `660bc08747da33aa640ef169a4c50fbfc02a74d1`, both profiles pass all three tests without skips.
The temporary lifecycle and event instrumentation is absent.
Seventeen source pins and eight child hashes verify per profile; all eight PNGs decode and were visually inspected.
Both inspectors accept the artifacts. Foreground ownership, source stability, process cleanup, and transport removal all verify.

| Profile | Artifact | ZIP SHA-256 | Home readiness |
| --- | --- | --- | --- |
| Small round | `11862492979` | `987d2c94b05fa3017429a0c3fbf40867b5ec17ae5a8ea96d034042406db08cbd` | 43 reads, 10.759 seconds |
| Large round | `11863311946` | `a8e48c8a442142018108a2eed180815600df78841acc554cdae5bb18cc3d54eb` | 53 reads, 12.804 seconds |

The app name now has bounds `[135, 72, 249, 108]` on the small profile and `[170, 72, 284, 108]` on the large profile.
These remain identical after recreation. Farthest-corner distances are 132.850 and 165.148 pixels, within radii of 192 and 227 pixels.
The new Kotlin geometry assertion passes at both stages on both profiles, and the images show the complete label.
Disconnected and permission-message images preserve the existing controls and content flow.
Paired loaded states, pending transitions, enlarged type, physical input, and physical watches remain unverified by these journeys.

Swift compilation succeeds in this run. Separate production-image scans reject a cached vulnerable `librsvg2-2` package.
The [security repair note](2026-10-13-librsvg-security.md) records that new blocker and its correction.
This Wear result does not establish that all required gates, image publication, or deployment have passed.
