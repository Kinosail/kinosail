# Apple TV startup lifecycle repair — 2026-10-03

## Failure analysis before implementation

The initial analysis and loopback journeys were written before the production repair. The tests were committed as `65c42ed35`, corrected to the authoritative preferences schema in `6cd8c4978`, and extended before implementation in `aacfa302d`.

1. A playback task already cancelled, or cancelled while awaiting the ServerClient profile actor, calls `stop()` before checking cancellation. It can clear another active player and start unwanted negotiation.
2. Preparation A1 → B → A2 has cleanup keyed only by item/client. Late A1 completion matches A2 and clears its producer or replaces its cache.
3. A cancelled preparation waiter awaits an unstructured producer's result. It does not return promptly when the screen leaves.
4. Any preparation failure causes a second foreground pair of endpoint reads. This includes terminal authorization, schema and input failures; HTTP GET retry policy already belongs to ServerClient.
5. Speculative title preparation can cancel the producer used by an active foreground startup. That foreground caller receives cancellation and can leave an opening surface without a failure message.

## Verification design and baseline

Use an isolated loopback HTTP listener, fictional Viewer Profile/token, the production ServerClient and PlaybackEngine, and a dedicated tvOS 27 simulator. The precise lifecycle ordering is an isolation exception: UI taps cannot deterministically park a caller at an actor hop or producer completion. No fixture is in a production app target.

Exact pre-repair revision: `aacfa302d`. Xcode 27.0 (`27A266a`), tvOS simulator 27.0 (`24J360`), ARM64, Apple TV 4K third-generation profile at 1080p. Five journeys fail without retries:

| Journey | Before repair |
| --- | --- |
| Already-cancelled startup | Active AVPlayer removed, generation/title changed, one unwanted source request |
| Terminal HTTP 403 | Two source requests |
| Cancelled preparation | Caller completion 504 ms after cancellation with delayed fixture replies |
| Background preparation during foreground startup | One request for the competing title and foreground cancellation |
| A1 → B → A2 with late A1 completion | Three source requests for A, where two are required |

The first run's A-B-A fixture had invalid preferences. That run establishes only the other three observed defects. The corrected second run establishes all five.

## Repair

Check cancellation before and after the initial profile actor hop. A preparation object owns its producer, terminal result and cancellable waiters. Cancelling a screen removes only that waiter so foreground playback can adopt the producer. Replacing a preparation cancels its producer and wakes its callers; late results can publish cache state only if the same object still owns the engine slot. Speculative preparation yields while foreground startup is loading. Terminal preparation failures return through the existing error path instead of starting a second negotiation.

Direct First, Apple TV container selection, AVPlayer buffering, media transport, codecs, caption/audio selection, resume/seeking, server APIs and authentication policy are preserved.

## Repeat

From `apps/player/apps/native`, use an available task-owned tvOS simulator:

```sh
xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-tvOS \
  -configuration Debug -destination 'platform=tvOS Simulator,id=<task-simulator-id>' \
  -derivedDataPath <task-output>/derived -resultBundlePath <task-output>/run.xcresult \
  -jobs 2 -parallel-testing-enabled NO -collect-test-diagnostics never \
  '-only-testing:Kinosail-tvOSTests/PlaybackStartupJourneys' \
  CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- ONLY_ACTIVE_ARCH=YES test
```

Do not reset an existing simulator to repeat this check. Exact file hashes, revisions, command and result summaries belong in the companion evidence JSON.

## Boundaries

These are native HTTP lifecycle journeys, not a moving-frame or physical Apple TV certificate. General tap-to-first-frame speed, direct/HLS/HDR decoding, audio/subtitle changes, seek/resume, interruptions and network recovery need representative playback acceptance. A read-only inventory found a paired physical Apple TV; it was not connected anew, installed, launched or exercised. Existing simulators and the dirty user repository were preserved. No Nox request, deployment, image operation or release was performed.

Source review also identified that optional external subtitle reads precede playback, and readiness failures can have a long retry budget. Neither path was changed without measured media evidence. Reusing a compatible URL during recovery was not established as an expiry defect.

## Decoded-media acceptance, before a resume repair

At `b3bcfdda8`, all five lifecycle journeys pass. A separate opt-in native media journey sends a bounded fictional H.264/AAC MP4 through the production authenticated transport and AVPlayer. Both cold and prepared playback decode moving frames and pass seek, pause and resume checks. Both fail initial resume: the plan requests three seconds, but AVPlayer starts at zero. This is an unresolved acceptance failure; these timings do not establish correct startup performance.

The additional failure modes are: initial AVPlayer time jumps replacing saved resume intent; an unsuccessful restoring seek being mistaken for a native scrub; stale pending progress between repeated starts; or incomplete HTTP request bodies in the fixture. The next diagnostic run separates them before any resume behavior changes.

Generate the fictional sample, then include `-only-testing:Kinosail-tvOSTests/PlaybackMediaJourneys` in the command above:

```sh
ffmpeg -nostdin -hide_banner -loglevel error \
  -f lavfi -i 'testsrc2=size=128x72:rate=30' \
  -f lavfi -i 'sine=frequency=440:sample_rate=48000' \
  -t 12 -threads 1 -c:v libx264 -preset ultrafast -pix_fmt yuv420p \
  -c:a aac -ac 2 -movflags +faststart -y /tmp/kinosail-appletv-task7-media.mp4
```

The decoded-media journey is opt-in because the generated media file is external to the repository. Its artifact records the sample hash. It does not verify HEVC Main10, HDR, EAC3, HLS or a physical Apple TV.

## Resume diagnosis and repair

The diagnostic run at `45e9e19a0` with recorded temporary numeric instrumentation confirms the same sequence twice: source and retained position are three seconds; AVPlayer emits a zero-second jump during loading; the engine replaces the target with zero; seeking to zero succeeds. Fresh per-case progress stores and complete fixture request bodies exclude those alternate explanations.

The resume repair uses the existing tvOS AVPlayerViewController delegate to retain user-selected targets for the current player. Its `timeToSeekAfterUserNavigatedFrom:to:` callback represents user navigation and returns AVKit's target unchanged. Automatic player time jumps no longer replace saved positions. iOS touch controls retain their existing explicit coordinator seek path. An interrupted restoring seek uses a changed recorded user target, or reports failure; it no longer assumes the current position came from a scrub.

Media acceptance and required iOS/tvOS compilation for this repair are pending the next coordinated build slot.
