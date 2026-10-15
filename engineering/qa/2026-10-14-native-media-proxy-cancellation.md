# Native media proxy cancellation QA

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

## Not verified

- Playback on the physical Apple TV against the deployed Server.
- Seek recovery with a real Server transcode. The Server log on Nox had no
  playback traffic after its latest restart, so the original failure was not
  read from Server logs.
