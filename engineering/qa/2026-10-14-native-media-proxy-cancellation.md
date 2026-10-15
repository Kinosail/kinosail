# Native media proxy cancellation and tvOS playback QA

## Confirmed failure

Native Apple playback sends every stream through a loopback proxy on `127.0.0.1`.
AVPlayer connects to this proxy, and the proxy forwards each request to the Server.

The proxy had two defects:

1. It accepted only 4 player connections and 4 upstream requests. HLS playback
   with adaptive variants, audio, subtitles, and segment prefetch needs more.
   The proxy reset extra connections. AVPlayer reported
   `NSURLErrorNetworkConnectionLost` (`-1005`).
2. It did not read from a player connection after the request header. When
   AVPlayer abandoned a request, for example after a seek, the proxy did not
   notice. The upstream request stayed open and kept its slot.

The Server waits up to 30 seconds for each transcoded HLS segment. It keeps the
transcode alive while the request is open. Abandoned requests therefore held
proxy slots and Server encoder work. The next segment requests were reset.
The engine classified `-1005` as a network failure and showed
`Connection interrupted. Reconnecting…`, then reinstalled the player.

tvOS prefers the compatible HLS stream for every source that is not MP4 or
QuickTime, so tvOS reached this failure most often. iOS shares the proxy.

## Repair

- The proxy keeps one receive open after each request header. A closed or failed
  player connection closes the proxy connection and cancels the upstream task.
- Player connections, upstream tasks, and upstream connections per host share one
  fixed bound of 16.
- A rejected connection at the bound emits a `media-proxy` warning with
  `operation`, `outcome`, `failure_class`, and the active count. It logs no URL,
  path, token, or header.

## Failure modes covered

- More than 4 parallel segment requests must all reach the Server.
- Abandoned player requests must cancel their upstream Server requests.
- A new segment request after abandonment must succeed immediately.
- The proxy must still bound upstream requests when a client opens 40.

`MediaProxyCancellationJourneys` covers these with a loopback fixture. The fixture
holds segment responses open, like a Server that is still transcoding. It records
when each upstream connection opens and closes. This is isolated native and
loopback proof. It is not populated-server E2E or physical-device evidence.
Existing E2E journeys cannot hold Server segment responses or observe upstream
cancellation, so this isolated check is retained.

## Evidence

Base revision: `3f292de8c5d13b14c67ce8d7cdd5c1a12db2c022`.
Environment: Xcode 27.0 (27A266a), tvOS 27.0 simulator runtime (24J360),
Apple TV 4K (3rd generation) simulator.

Command, run from `apps/player/apps/native`:

```sh
xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-tvOS \
  -configuration Debug -destination "platform=tvOS Simulator,id=${QA_SIMULATOR_ID}" \
  -derivedDataPath .build/tvos-simulator -jobs 4 -parallel-testing-enabled NO \
  -collect-test-diagnostics never CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- \
  -only-testing:Kinosail-tvOSTests/MediaProxyCancellationJourneys test
```

| Run | Parallel upstream | Abandoned closed | Request after abandon | Bounded (of 40) |
| --- | ---: | ---: | --- | ---: |
| Before repair | 4 of 8 | 0 of 4 | `-1005` connection lost | 4 |
| After repair | 8 of 8 | 8 of 8 | HTTP 200 in 12 ms | 16 |

Log SHA-256 values:

- Before repair: `ee339c96b52f1673e64656a9dfadca5bc3baf720fd83afa56e4ca688d47fd308`
- After repair, with `MediaDepartureJourneys` and `PlaybackEndpointJourneys`
  (11 tests, all passed): `875344d9c6efcb20aa960ecc811339cfe3d123d9244d93ccd1176f488060a039`
- Full tvOS test target after repair (320 tests in 71 suites, all passed):
  `de833d5c9ba158f0ba1c0f7b7dc9a06bdbda8edb345eec29f6a9766e69ef0a91`
- iOS 27 iPhone simulator, `MediaProxyCancellationJourneys` and
  `MediaDepartureJourneys` (4 tests, all passed):
  `83eeaeff7afe3998a3973f921f220c0c0ef1bc65b95fd271231c95d452c41599`

## Follow-up tvOS repairs

Physical Apple TV testing found four more defects.

1. **Player view removed during replacement.** The tvOS playback screen removed
   the system player view while a new title or a recovery loaded. A remote
   touch in progress then reached a removed view, which can crash UIKit. The
   screen now mounts the player view when the first player arrives. It keeps the
   view mounted through each replacement and covers it with the opening or retry
   overlay.
2. **Blank Now Playing card.** On tvOS, AVKit owns `MPNowPlayingInfoCenter`.
   The app wrote Now Playing info there, but AVKit replaced it, so the iPhone card
   showed only the room name. Each player item now carries `externalMetadata`:
   title, subtitle (show and episode, or artist and year), description, and
   JPEG artwork.
3. **Progress retried forever for a removed title.** A pending position for a
   title that returns HTTP 404 or 410 stayed in the queue. Every sync retried it,
   and nothing could clear it. The store now drops that entry and emits a
   `progress-sync` warning with the status and item ID.
4. **Progress conflicts after playback from a cached title.** Playback used the
   cached title's progress as the expected server state. When another device
   had changed it, every save conflicted. Playback now reads the current title
   first. Playing a title again also replaces its unresolved conflict.

New diagnostics:

- `playback-recovery` warning: `failure_class`, error `code`, `compatible`,
  recovery count, and `session`.
- `playback-install` info: `compatible`, `live`, and `session`.
- `media-proxy-manifest` debug: playlist type, end-list flag, and variant and
  segment counts. It logs no URL.

### Isolated checks

`TVPlayerSurfaceStabilityJourneys`, `NowPlayingMetadataJourneys`, and
`ProgressSyncRecoveryJourneys` use `PlaybackStartupFixture` on loopback. The
fixture applies the Server's progress conflict rules. These are isolated
native checks. Existing E2E journeys cannot hold a replacement title in the
loading state, read `AVPlayerItem.externalMetadata`, or return 404 for one title.

`ResilienceTests.oneMissingTitleDoesNotBlockOtherProgress` now expects the
missing title to be dropped, not kept.

### Physical Apple TV evidence

Device: Apple TV 4K, tvOS 27.0, "Living Room", against the Nox Server.
Debug build from this branch, run through the `Kinosail-tvOS-Remote` scheme.

| Check | Result |
| --- | --- |
| Start 1917 from Continue Watching | Compatible HLS ready, `live=false` |
| Stale 404 progress entry | Dropped on first sync, one warning |
| Transport bar | Title and year shown, VOD timeline 1:59:00 |
| Skip ×3, hold Right 3 s, Select, pause, resume, Menu | App stayed in the foreground |
| `RemotePlaybackCrashTests` (3 rounds of play, options, held seek, back) | Passed |
| New crash logs on the device | None |

With title metadata, AVKit shows an Info tab below the transport bar. The
action buttons are reached with Up, not Down. `RemotePlaybackCrashTests` now
uses that route. On the base commit, without metadata, the old route passed.

Log SHA-256 values:

- Full tvOS simulator target (325 tests in 74 suites, all passed):
  `1e319d57c8d5fdc89bbf1f3d60dda15b614da7decf3420b8d58529c38a536d5f`
- Full iOS 27 simulator target (341 tests in 73 suites, all passed):
  `c89c111e4c3838a80596360e2316b0d91845c787015ab72e24df1b3a51603622`
- Moved-code tvOS checks after the line-cap split (8 tests, all passed):
  `9764b89b0dd01b6dd15790cbcfd2a61ee57c2061d8f531700a8320a5dc8a2fc4`
- Physical Apple TV `RemotePlaybackCrashTests`:
  `3404836ff5f9c6b9da0e966fbc48ecee45738dd350497ed101533e35fad823c4`

## Not verified

- Touchpad swipe scrubbing on the physical Apple TV. XCUIRemote can press
  buttons but cannot swipe the touchpad.
- The iPhone Now Playing card for the Apple TV. The tvOS transport bar showed
  the new metadata, but the iPhone card was not checked.
- Seek recovery with a forced Server transcode failure. The Server log on Nox
  had no playback failures after its latest restart.
