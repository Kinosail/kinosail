# Native audio session QA

## Confirmed failure

The native playback engine synchronously configured and activated `AVAudioSession` on `MainActor`.
Xcode reported possible UI unresponsiveness during successful playback tests.
The warning is evidence of a blocking API call, not a measured user-visible hang.

Hosted deep run `38494806212` tested revision `d1f6dcf90f8b2f707a2866cee54c112067a653a4`.
Its native artifact is `11864165444`.
The archive SHA-256 is `b5148df01dfd16d21c3eeef4ae731242a3b41246607b91548c42671592b575d5`.

| Platform | Passed | Failed | Skipped | Audio main-thread warnings | Other warnings |
| --- | ---: | ---: | ---: | ---: | ---: |
| iOS | 336 | 0 | 1 | 11 | 0 |
| tvOS | 320 | 0 | 1 | 77 | 3 |

The three other warnings concern FocusState in rendering fixtures. They remain separate findings.

## Repair and regression contract

One serial queue performs category selection and activation together.
Only immutable mode and session values cross the queue boundary.
The caller waits for the system operation to finish, then checks cancellation and installation ownership.
Player state stays on `MainActor`.

The deployment minimum remains iOS 26 and tvOS 26.
Apple's asynchronous activation API requires iOS 27 or tvOS 27.
The selected queue approach supports both generations.
See [Apple's activation API](https://developer.apple.com/documentation/avfaudio/avaudiosession/activate(options:completionhandler:)).

Each installation owns its asynchronous transport, track selection, seek, monitoring, and cleanup work.
A replacement for the same title retains the playback generation but receives a new installation identity.
Retired work must not restore a player, clear pending state, save a stale position, or resume a paused replacement.
Pending replacement seeks use the existing loading state and retain the latest requested position.

The added loopback journey holds actual media responses after player creation.
It supersedes one same-title installation, seeks the replacement to nine seconds, and pauses it.
It requires cancellation of the retired installation, preserved pending state, and decoded replacement playback.
Every operation is joined, and the fixture closes its listener and connections on success or failure.
This is isolated native and loopback proof. It is not populated-server E2E evidence.
It does not hold the system audio activation call itself.

## Artifact gate

The deep native job exports each platform's `xcresulttool` test summary.
The gate rejects missing or malformed summaries, incorrect destinations, empty execution, failures, and audio main-thread warnings.
It checks both platforms even when the first export or inspection fails.
It retains skipped counts and unrelated warning counts without printing private warning text.
Receipts include the revision and source summary SHA-256.
The revision comes from the hosted job; the summary does not independently attest source immutability.

The CLI controls were written before the gate. Native journeys cannot create malformed result exports safely.
These controls test that separate artifact boundary, including fixed diagnostics for private or malformed input.

Run the source checks with:

```sh
python3 -B -m unittest scripts.ci.test_native_results scripts.ci.test_workflows
make max-loc
```

Use the repository's pinned actionlint fork for workflows with parallel steps.
The installed upstream actionlint 1.7.12 cannot parse the existing parallel-step syntax.

## Evidence and remaining boundaries

The 26 focused source checks passed. The pinned actionlint fork and `make max-loc` passed.
The gate rejected both retained baseline summaries with the exact warning counts above.
This establishes the baseline failure at the artifact boundary.

Local application runtime work remains on hold. No local Swift compilation, simulator, or media run was performed.
Hosted compilation passed for `f91be11017a48357b4feb72aaf2bf34425be3d3b`.
The new same-title journey was written before production changes, but its pre-fix native execution was not run.
Do not claim native red-green proof until that boundary is exercised.

Audio failure logs use a fixed operation, phase, mode, generated session UUID, and numeric error code.
They omit error descriptions, bodies, URLs, credentials, and user data.
Success uses debug level; platform setup failure uses warning level.
These properties have source inspection only; runtime failure-log evidence is pending.

Physical routing, interruption, Picture in Picture, offline playback, and device responsiveness need their own evidence.
The work does not verify Nox deployment, physical devices, or store publication.

## First repaired-source runtime

Deep run `38501376945` produced artifact `11865579540` at `f91be110`.
Its archive SHA-256 is `5bd9dd36f195d8853c1200270cd578e5d342ee2d6c73341ffc78ef85218edafd`.
GitHub's digest, retained summaries, gate receipts, and independently read Xcode test trees agree.

iOS passed 336 tests with one skip. tvOS passed 320, failed one, and skipped one.
Both platforms reported zero audio main-thread warnings. The three tvOS FocusState warnings remain.
The failed test is the new same-title installation journey's decoded-frame prerequisite.
Its earlier ownership, restored seek, and pause assertions reported no failures.
Playback advanced from 9.0 to 11.885 seconds at rate 1 with a ready item and no reported media error.
The late-added video output returned no frame. This does not establish the underlying cause or a visible playback failure.

The next diagnostic attaches the output to the captured replacement item before releasing held media.
It preserves seek, pause/resume, advancement, and the three-second deadline.
An acquired frame must have a display time after the initial restored position.
The output stays attached to that item and is removed on every exit.
Bounded receipts include attachment state, requested and displayed times, and acquisition count.
This is an unverified diagnostic change, not a confirmed fixture repair.
Apple documents frame acquisition through [AVPlayerItemVideoOutput](https://developer.apple.com/documentation/avfoundation/avplayeritemvideooutput).
The earlier failure remains retained; production playback code is unchanged in this trial.

## Early-output trial and matched control

Deep run `38503683206` tested `fab16419ec0acfcdcb7a3f44c63678b6b06f94fd`.
Native artifact `11867382561` has SHA-256 `71eab75f9e09f86c3adb7b0e018144db866c2ed00989118395d94fcf0361c71f`.
The GitHub digest, gate receipts, independent summaries, and test trees agree.
iOS passed 336 tests and skipped one. tvOS passed 321 tests and skipped one.
Both reported zero audio main-thread warnings; the three tvOS FocusState warnings remain.

The replacement journey acquired a frame with display time 9.25 after restoring to 9 seconds.
Its clock advanced to 9.306, within the unchanged three-second bound.
All four sibling controls acquired frames after their restored six-second positions.
The log SHA-256 is `fee4bc3567e9adda62f5c25d6e18962cdf64064750614e4ba665e9dc000c6d42`.

Read-only MP4 sample-table inspection verifies 144 video samples over twelve seconds.
Thirty-five start after nine seconds, with the last starting at 11.917 seconds.
This rejects a declared video track ending before the failed seek; it does not prove decoding.

The next matched control holds initial loading, seeks to nine seconds, pauses, releases media, and resumes.
It attaches the output late, as in the retained failed trial, but has no retired installation.
This control is pending hosted execution. The successful early-output trial alone does not establish the earlier failure's cause.

## FocusState capture trial

The three retained tvOS warnings point to two `MediaCardRatingTests` invocations and one `TVTopBarTests` invocation.
Those invocations use `ImageRenderer` around focus-dependent views. No visible production focus defect is established.
Apple describes [FocusState](https://developer.apple.com/documentation/swiftui/focusstate/) as state updated by focus within a scene.

The tvOS capture trial hosts those fixtures in a non-key window and preserves the previous key window.
Each card keeps its original width and receives an independently fitted height.
The same normalized raster supplies assertions and image attachments.
Invalid dimensions, failed hierarchy drawing, changed key ownership, or blank pixels reject the capture.
All eight strict card comparisons, dynamic-type variants, dark theme, and visible-text sensitivity control remain.
The iOS renderer and live-window TV focus journey are unchanged.

Failure modes include unwanted focus decoration, safe-area cropping, background-only images, and capture before layout settles.
Hosted acceptance must preserve the strict comparisons and text control, remove the warnings, and retain readable card/header images.
No production focus bindings or warning filters changed. The result follows below.

## Matched late-output result and capture acceptance

Deep run `38505809043` tested `535faec8e74e7c2652c2022c2c3192b9513de8e3`.
Native artifact `11868261650` has SHA-256 `0558c57dedbe98c2aef6e94783a1387848088174f8b5dd9a10b8bc7f5ea5dae0`.
GitHub's digest, gate receipts, independent summaries, and test trees agree.
iOS passed 336 tests with one skip. tvOS passed 321, failed one, and skipped one.
Both platforms reported zero audio warnings and zero other runtime warnings.

The only failed case is `healthyPendingSeekToNineSupportsLateFrameObservation`.
It restores nine seconds and remains paused before resume, without a retired installation.
Its clock then advances to 11.888 at rate 1, with a ready item and no reported media error.
The attached late output acquires no frame. A retired installation is therefore not necessary for this failure.
The same-title early-output case again passes, with a frame displayed at 9.25 seconds.
All four six-second controls pass. The log SHA-256 is `78e5acf005e672ac590fc0fad9659c66741d529618fa3ab2e9dab5f32555766c`.

The next matched trial attaches the healthy case's output before seeking and releasing held media.
It retains the same pause, player identity, clock, frame timestamp, and three-second requirements.
The output belongs to the captured item and is removed on every exit.
This trial remains pending. No production decoder behavior changes.
A pass would support an attachment-timing interaction, without establishing why late observation fails or proving ordinary rendered playback.

Both card capture tests, the header background test, and the unchanged live focus journey pass.
All eight strict card pairs and the visible-text sensitivity control remain active.
All eleven exported images were inspected: eight cards, two text controls, and one header.
They retain readable titles, artwork placeholders, progress markers, and uncropped header controls.
The large poster keeps its existing two-line ellipsis; accessibility titles and landscape titles fit.
The visible control adds PG-13 text while the hidden control omits it.
The fixture repair removes all three FocusState warnings. This is fixture acceptance, not physical-TV proof.

## Verified matched early-output trial

Deep run `38508067234` tested `6920080a718143f540c554d6b139ed37c58b19ca`.
Native artifact `11869640810` has SHA-256 `ef907407c2d7a7aed04d5a6e691fe577bfa899175d4902187f9c158a1010ed25`.
The GitHub digest, gate receipts, independent summaries, and test trees agree.
iOS passes 336 tests. tvOS passes 322. Both have one skip, zero failures, and zero runtime warnings.

The healthy and replacement nine-second cases each execute once and pass.
Both acquire a frame displayed at 9.25 seconds. Their clocks advance to 9.307 and 9.310 seconds.
All four six-second controls acquire frames displayed at 6.25 seconds.
The log SHA-256 is `c09dea8104714971a2c30943306233d0a1bba83e01608878ee6eae5b6ca17fdb`.
This supports an observation-attachment interaction. It does not establish the late-output failure mechanism or ordinary rendered playback without that output.

Both card tests, the header test, and the unchanged live focus journey pass.
All eleven exported images exactly match the pixels and color profiles inspected at `535faec8`.
The strict comparisons, visible-text control, independent dimensions, and warning rejection remain intact.

The intermediate `3792686c` run passes 336 iOS tests but cannot launch the tvOS test host.
It executes no selected tvOS case and produces one failed harness record.
Artifact `11868553995` has SHA-256 `22bc91922582788257c7ba8bac95b13dae7c697273a9f62c6d79d3c080a847ef`.
Its rejected result remains retained. The later pass does not diagnose that launch failure.

The installation race and frame controls remain tvOS-only. iOS execution does not establish that specific race on iOS.
Actual system activation cancellation, runtime failure logs, physical devices, deployment, and complete product acceptance remain separate boundaries.
