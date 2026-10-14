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
