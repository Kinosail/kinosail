# Native artwork size and cache reuse

Date: September 30, 2026. Production source revision: `05917f8f91c43ca30fba465cea10732a953aafe4`.

This experiment runs the real native `ArtworkLoader`, `ServerClient`, bounded HTTP reader, and protected `LocalMediaCache`. Only the transport uses the existing `HTTPFixture` and `FixtureURLProtocol`. No production source or artwork dimension changes.

## Result

Exact decoded-size reuse dominates the small cold-decode difference. A warm 800px request returns the same image in 0.025 ms. Requesting 400px from that loader instead takes 4.985 ms and retains another 640,000 decoded bytes. A warm 400px request also reuses its exact-size image, in 0.028 ms.

Cold 400px loads take about 10% less time through the controlled transport and 11% less time from saved bytes. Each 400px image occupies 75% fewer decoded bytes than its 800px counterpart. These size comparisons measure this fixture; they are not a delivered application speedup.

| Loader state and request | Median of six batch medians, ms | Returned decoded bytes | Bytes in unique held images | Shared seed image | Fixture HTTP requests, all 150 calls |
| --- | ---: | ---: | ---: | --- | ---: |
| `network-800` | 5.699 | 2,560,000 | 2,560,000 | No | 150 |
| `network-400` | 5.129 | 640,000 | 640,000 | No | 150 |
| `saved-800` | 5.574 | 2,560,000 | 2,560,000 | No | 0 |
| `saved-400` | 4.960 | 640,000 | 640,000 | No | 0 |
| `warm-400-to-400` | 0.028 | 640,000 | 640,000 | Yes | 0 |
| `warm-400-to-800` | 5.523 | 2,560,000 | 3,200,000 | No | 0 |
| `warm-800-to-800` | 0.025 | 2,560,000 | 2,560,000 | Yes | 0 |
| `warm-800-to-400` | 4.985 | 640,000 | 3,200,000 | No | 0 |
| `warm-1600-to-800` | 5.598 | 2,560,000 | 12,800,000 | No | 0 |
| `warm-1600-to-400` | 5.154 | 640,000 | 10,880,000 | No | 0 |

`network` means the controlled URLProtocol response path. It includes the production HTTP reader and decoding, but no socket, TLS, server work, or network delay. Every encoded response remains 297,827 bytes. This experiment establishes no transfer savings.

Unique held bytes sum the returned image and the optional seed, counting identical objects once. They are pixel-storage accounting, not measured process memory or GPU residency. Seeds are prepared before timing. Each iteration uses a new loader. Saved cases reuse fresh encoded bytes in one protected temporary cache. Every loader is cleared after its measured call.

## Decision and remaining work

The 48-point MiniPlayer requests 800px. Ordinary media cards and prefetch request 800px at standard text sizes. The album grid, album detail, and full audio player use the 1600px default. Requests only share decoded pixels when client identity, URL, and dimension match.

A fixed MiniPlayer reduction would save pixels after a 1600px-only path. It would lose the 800px cache hit after a matching card path. The latter adds a decode and increases combined retained pixels from 2.56 MB to 3.20 MB. The experiment measures conditional paths, not their frequency in real navigation. Keep the current setting until that distribution and rendered quality are measured.

A useful next experiment would compare a small-image policy that can reuse an already decoded suitable variant. It must preserve profile isolation, freshness, cancellation, foreground admission, and accurate byte accounting. The current API promises an exact maximum dimension. Returning larger pixels through that API would change its contract.

Album cover identity can also differ from the playing track. `OrganizeMusic` selects an artwork-bearing track before sorting album tracks. The album detail displays the first sorted track. The loader keys URLs rather than artwork file contents. Do not assume identical album art implies an exact cache hit.

## Method and limits

- Xcode 27.0, build 27A266a; macOS 27.0, build 26A428; Apple M1 Pro host.
- tvOS 27.0 simulator, OS build 24J360, arm64, dedicated `Kinosail Speed TV` simulator.
- Release optimization with `ENABLE_TESTABILITY=YES` and `ONLY_ACTIVE_ARCH=YES`. This is an optimized test host, not an unmodified store-distribution binary.
- Ten workloads, six alternating forward/reverse batches, 25 calls per batch: 1,500 timed calls.
- One Swift Testing case passed in 13.944 seconds. Assertions verify image dimensions, exact seed identity, and request counts. Timing has no pass/fail threshold.
- All builds finish before `test-without-building`. No task-owned builds, lint, or other suites overlap measured calls. Other host and simulator activity remains uncontrolled.
- The generated 3200 × 3200 JPEG matches the earlier standalone fixture byte-for-byte. Its SHA-256 is `29b3975cac9ae3e1e30a11643909c79a16859362927a303a4e9645ce5d15e22b`.
- Encoded file and decoder caches can be warm. These are application-cache scenarios, not a cold operating-system/storage benchmark.
- No SwiftUI view, texture upload, focus transition, physical device, image-quality judgment, production network, or deployed workload was measured. No iOS run was added.
- The first eight-case run restored an old task-owned loopback session and issued background requests. Its samples remain diagnostic. A guarded test removed only the matching synthetic session. An isolated eight-case control and the final ten-case run passed afterward.
- The final log has zero HTTP transport/load failure entries. That does not establish the absence of all background framework work.
- An initial build command ran outside the native project, then a reused result path blocked the correction. A Release build also failed because testability was disabled. The final builds used explicit testability and succeeded. No failed build is counted as passing.
- ImageIO logged a simulator RawCamera bundle warning. The JPEG fixture decoded correctly and all assertions passed.

Replay extraction matches the compiled harness byte-for-byte. All raw samples, source/result hashes, and local links validate. `make max-loc`, `make tooling-check`, and `git diff --check` pass. The CI selector reports no affected application surface for these three research Markdown files.

No application behavior changed. Fresh full Go, native feature, populated UI, and physical-device suites were not rerun for this documentation-only result. Prior delivery evidence remains separate.

## Primary-source follow-up

[Apple's WWDC26 SwiftUI session](https://developer.apple.com/videos/play/wwdc2026/269/) describes default HTTP caching for `AsyncImage`, plus configurable `URLRequest` and `URLSession` support. The transcript's data-flow section and code example were read. Those APIs do not establish Kinosail's decoded-size reuse, profile partitioning, or protected-cache behavior. A replacement needs equivalent evidence before adoption.

The same session describes lazy initialization of observable objects stored in `@State`, with back deployment. Kinosail already builds with Xcode 27. This framework change is a research input; this experiment measures no view-allocation gain.

[Apple's WWDC26 performance lab](https://developer.apple.com/videos/play/wwdc2026/8003/) recommends appropriate asset sizes and narrower view observation. Its thread-hopping discussion also recommends checking critical-path work and contention in Instruments. Our size mismatch result supports evaluating reuse together with downsampling. It does not transfer Apple's general guidance into a physical-frame improvement claim.

[ImageIO's immediate-cache option](https://developer.apple.com/documentation/imageio/kcgimagesourceshouldcacheimmediately) controls decoding at image creation. The production thumbnail path already uses it inside its detached decode task. This experiment exercises that path.

## Reproduction

Use a disposable simulator and an otherwise clean task worktree. Restored sessions can add unrelated background work. Record that state before timing; do not reset another task’s simulator or clear a real session.

The Swift block below is the complete measured harness. It depends on the existing native test fixtures. Extract it to `apps/player/apps/native/Tests/ArtworkSizeProfileTests.swift` only when that file does not already exist. The fixture creates temporary cache directories and removes them afterward.

```sh
cd apps/player/apps/native
xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-tvOS -configuration Release \
  -destination 'platform=tvOS Simulator,id=64468BFE-9114-4FBF-860B-79DF45F70011' \
  -derivedDataPath .build/speed-tvos -jobs 4 -parallel-testing-enabled NO \
  -only-testing:Kinosail-tvOSTests/ArtworkSizeProfileTests \
  ENABLE_TESTABILITY=YES ONLY_ACTIVE_ARCH=YES CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- build-for-testing
xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-tvOS -configuration Release \
  -destination 'platform=tvOS Simulator,id=64468BFE-9114-4FBF-860B-79DF45F70011' \
  -derivedDataPath .build/speed-tvos -parallel-testing-enabled NO \
  '-only-testing:Kinosail-tvOSTests/ArtworkSizeProfileTests/profileColdSavedAndWarmArtwork()' \
  ENABLE_TESTABILITY=YES ONLY_ACTIVE_ARCH=YES CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- test-without-building
```

The original commands also used unique result-bundle paths and redirected logs under `.verification/artwork-loader-profile/`. Results print their temporary JSON path. Copy that file before a later test launch removes its container. Afterward, remove only the temporary harness whose bytes still match this record.

```swift
import Foundation
import CryptoKit
import ImageIO
import Testing
import UniformTypeIdentifiers
@testable import KinosailPlayer

/// Copy into native Tests for this opt-in experiment; never asserts timing thresholds.
struct ArtworkSizeProfileTests {
    private struct Workload {
        let name: String
        let dimension: Int
        let seed: Int?
        let saved: Bool
    }
    private struct Row: Codable {
        let batch: Int
        let workload: String
        let dimension: Int
        let seed: Int?
        let elapsedMilliseconds: [Double]
        let decodedBytes: Int
        let heldUniqueBytes: Int
        let sharedSeed: Bool
        let networkRequests: Int
    }
    private struct Record: Codable {
        let fixture: String
        let encodedBytes: Int
        let encodedSHA256: String
        let samplesPerBatch: Int
        let rows: [Row]
    }

    @Test func profileColdSavedAndWarmArtwork() async throws {
        let data = try patternJPEG()
        let network = try HTTPFixture(data: data, headers: ["Content-Type": "image/jpeg"])
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        let viewer = try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true),
                "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
        let saved = try HTTPFixture(data: data, headers: ["Content-Type": "image/jpeg"], viewer: viewer, cacheDirectory: directory)
        defer { network.remove(); saved.remove(); try? FileManager.default.removeItem(at: directory) }
        let store = try #require(await saved.client.cacheStore())
        try await store.write(data, key: saved.client.server.mediaURL("/art/track").absoluteString, kind: .artwork)
        let workloads = [
            Workload(name: "network-800", dimension: 800, seed: nil, saved: false),
            Workload(name: "network-400", dimension: 400, seed: nil, saved: false),
            Workload(name: "saved-800", dimension: 800, seed: nil, saved: true),
            Workload(name: "saved-400", dimension: 400, seed: nil, saved: true),
            Workload(name: "warm-400-to-400", dimension: 400, seed: 400, saved: true),
            Workload(name: "warm-400-to-800", dimension: 800, seed: 400, saved: true),
            Workload(name: "warm-800-to-800", dimension: 800, seed: 800, saved: true),
            Workload(name: "warm-800-to-400", dimension: 400, seed: 800, saved: true),
            Workload(name: "warm-1600-to-800", dimension: 800, seed: 1600, saved: true),
            Workload(name: "warm-1600-to-400", dimension: 400, seed: 1600, saved: true)
        ]
        var rows: [Row] = []
        for batch in 0..<6 {
            for workload in (batch.isMultiple(of: 2) ? workloads : Array(workloads.reversed())) {
                rows.append(try await measure(workload, batch: batch, fixture: workload.saved ? saved : network))
            }
        }
        let record = Record(fixture: "Generated 3200x3200 RGB block-pattern JPEG, quality 0.88; real native loader, URLProtocol transport",
            encodedBytes: data.count, encodedSHA256: SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined(),
            samplesPerBatch: 25, rows: rows)
        let encoder = JSONEncoder(); encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        let output = FileManager.default.temporaryDirectory.appendingPathComponent("artwork-loader-profile.json")
        try encoder.encode(record).write(to: output, options: .atomic)
        print("ARTWORK_PROFILE_FILE=\(output.path)")
        await network.client.close(); await saved.client.close()
    }

    private func measure(_ workload: Workload, batch: Int, fixture: HTTPFixture) async throws -> Row {
        var durations: [Double] = []
        var decodedBytes = 0, heldBytes = 0, sharedSeed = false
        let initialRequests = fixture.requests.count
        for _ in 0..<25 {
            let loader = ArtworkLoader()
            var seed: CGImage?
            if let dimension = workload.seed {
                seed = try await loader.image(path: "/art/track", client: fixture.client, dimension: dimension)
            }
            let start = ContinuousClock.now
            let image = try await loader.image(path: "/art/track", client: fixture.client, dimension: workload.dimension)
            let elapsed = start.duration(to: .now).components
            durations.append(Double(elapsed.seconds) * 1000 + Double(elapsed.attoseconds) / 1e15)
            decodedBytes = image.bytesPerRow * image.height
            sharedSeed = seed.map { $0 === image } ?? false
            heldBytes = decodedBytes + (sharedSeed ? 0 : seed.map { $0.bytesPerRow * $0.height } ?? 0)
            #expect(image.width == workload.dimension && image.height == workload.dimension)
            #expect(sharedSeed == (workload.seed == workload.dimension))
            await loader.clear()
        }
        let requests = fixture.requests.count - initialRequests
        #expect(requests == (workload.saved ? 0 : 25))
        return Row(batch: batch, workload: workload.name, dimension: workload.dimension, seed: workload.seed,
            elapsedMilliseconds: durations, decodedBytes: decodedBytes, heldUniqueBytes: heldBytes,
            sharedSeed: sharedSeed, networkRequests: requests)
    }

    private func patternJPEG() throws -> Data {
        let context = try #require(CGContext(data: nil, width: 3200, height: 3200, bitsPerComponent: 8, bytesPerRow: 0,
            space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.noneSkipLast.rawValue))
        for y in stride(from: 0, to: 3200, by: 16) {
            for x in stride(from: 0, to: 3200, by: 16) {
                let n = (x * 31 + y * 17) % 255
                context.setFillColor(CGColor(red: Double(n) / 255, green: Double((n + 79) % 255) / 255,
                    blue: Double((n + 137) % 255) / 255, alpha: 1))
                context.fill(CGRect(x: x, y: y, width: 16, height: 16))
            }
        }
        let output = NSMutableData()
        let destination = try #require(CGImageDestinationCreateWithData(output, UTType.jpeg.identifier as CFString, 1, nil))
        CGImageDestinationAddImage(destination, try #require(context.makeImage()), [kCGImageDestinationLossyCompressionQuality: 0.88] as CFDictionary)
        #expect(CGImageDestinationFinalize(destination))
        return output as Data
    }
}
```

## Source and result binding

Local result bundles, failed attempts, logs, diagnostic samples, and the replay source remain under `.verification/artwork-loader-profile/`. Hash records use arrays of `{file, sha256}` entries.

```json
{
  "createdUTC": "2026-09-30T14:58:34.864300+00:00",
  "revision": "05917f8f91c43ca30fba465cea10732a953aafe4",
  "sourceHashes": [
    {
      "file": "apps/player/apps/native/Sources/Platform/ArtworkLoader.swift",
      "sha256": "2c71b58d54284ae8c68ad8d7265739cf0f334fde66b96f28c38090ebbb8608aa"
    },
    {
      "file": "apps/player/apps/native/Sources/Platform/ArtworkDownloads.swift",
      "sha256": "13c242b7cb7db44d757687478b62e7d0f38c44b5edb0154cc8580dc1aa3d40d5"
    },
    {
      "file": "apps/player/apps/native/Sources/Platform/ArtworkWarmup.swift",
      "sha256": "fcea04d151fd3835a683d31a32186ba17212cb0ddffae071a58e5a57ef272f19"
    },
    {
      "file": "apps/player/apps/native/Sources/Platform/LocalMediaCache.swift",
      "sha256": "fd426225be6b5a0c8b4a8bef7b2283e371b7309948938939aafdb68fb216cbba"
    },
    {
      "file": "apps/player/apps/native/Sources/Services/ServerClient.swift",
      "sha256": "9038e0fef5f627ee04247e3028dc21114d91297368cac788ba307599b8d5e2b2"
    },
    {
      "file": ".verification/artwork-loader-profile/ArtworkSizeProfileTests.swift",
      "sha256": "9039a7d9b20f271664ff7bc0c7484f50dda17d7fc8decbaa8c328e85c66806f9"
    }
  ],
  "platform": "ProductName:\t\tmacOS\nProductVersion:\t\t27.0\nBuildVersion:\t\t26A428",
  "toolchain": "Xcode 27.0\nBuild version 27A266a",
  "destination": "platform=tvOS Simulator,id=64468BFE-9114-4FBF-860B-79DF45F70011",
  "testability": "Release -O, ENABLE_TESTABILITY=YES, ONLY_ACTIVE_ARCH=YES",
  "timingBoundary": "No task-owned build/lint/tests overlap measured run; other host processes uncontrolled",
  "diagnosticStartup": "First eight-case run restored a prior task-owned loopback session and issued background requests. Its samples are diagnostic. A guarded test discarded only the matching synthetic session. Eight-case isolated controls passed before the final ten-case run.",
  "resultHashes": [
    {
      "file": ".verification/artwork-loader-profile/results.json",
      "sha256": "3163488a94b43866b0392d714bb0b8e1fdb959af4a8e9d00cc83e6f77551e4fb"
    },
    {
      "file": ".verification/artwork-loader-profile/summary.json",
      "sha256": "a13bd05296870fa1da086611f919c4955f01e1369540c477db3fbf87be0549e5"
    },
    {
      "file": ".verification/artwork-loader-profile/diagnostic-startup-results.json",
      "sha256": "8584a97c51a45ee51277138bf6c54790aa4d586b72a6bae35965f9294894c29c"
    },
    {
      "file": ".verification/artwork-loader-profile/final-run.log",
      "sha256": "6efaf94569271cfc28495c3e26866dc81aa09866424834a25fa98ab1513d3570"
    },
    {
      "file": ".verification/artwork-loader-profile/cleanup.log",
      "sha256": "033007d5ea681604ba690169a255970d8f6d441b34a59cda1382a3d4a048996d"
    }
  ],
  "finalRunBackgroundTransportFailureLogEntries": 0,
  "finalRun": "One selected Swift Testing case passed; 60 batches, 25 calls each, 1500 timed calls."
}
```

## All measured samples

Rows preserve execution order. Durations are milliseconds. The raw workload labels retain `network` for the URLProtocol case described above. Each line is one batch with all 25 calls.

```json
{"encodedBytes":297827,"encodedSHA256":"29b3975cac9ae3e1e30a11643909c79a16859362927a303a4e9645ce5d15e22b","fixture":"Generated 3200x3200 RGB block-pattern JPEG, quality 0.88; real native loader, URLProtocol transport","samplesPerBatch":25,"rows":[
{"batch":0,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[23.090666,6.763459,6.397583,6.138834,5.792667,5.850333,5.942792,5.897333,5.612583,5.608042,6.078167,5.890791,6.085,6.092167,5.985417,6.048334,6.003833,6.005666,6.071375,6.024584,5.525708,5.492916,5.850666,5.9305,6.216],"heldUniqueBytes":2560000,"networkRequests":25,"sharedSeed":false,"workload":"network-800"},
{"batch":0,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.493541,5.488583,5.219666,5.526833,5.376625,5.312542,5.376791,5.548291,5.497833,6.117208,5.1285,5.388917,5.347209,5.441333,5.373291,5.455041,5.450542,5.071792,5.267417,5.141375,5.122167,5.369416,5.353041,5.214625,5.324208],"heldUniqueBytes":640000,"networkRequests":25,"sharedSeed":false,"workload":"network-400"},
{"batch":0,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.832375,5.696125,5.805625,5.648959,5.776583,5.794583,5.563042,5.477125,5.567125,5.892,6.006,6.196833,5.871167,5.763291,5.41375,5.4595,5.424458,5.455917,5.589417,5.670709,5.532417,5.739042,5.644875,5.513833,5.679041],"heldUniqueBytes":2560000,"networkRequests":0,"sharedSeed":false,"workload":"saved-800"},
{"batch":0,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.079458,5.151667,4.924125,5.166,5.070666,4.920333,4.959042,4.643625,4.827667,4.711125,4.816958,5.182958,4.906458,5.065208,4.746208,5.228875,4.936792,5.353917,5.311,5.15075,5.239792,5.228208,4.967542,4.948833,4.94675],"heldUniqueBytes":640000,"networkRequests":0,"sharedSeed":false,"workload":"saved-400"},
{"batch":0,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[0.04,0.01675,0.03175,0.025875,0.01825,0.020042,0.037041,0.038167,0.050292,0.048167,0.015458,0.012,0.016167,0.014542,0.04225,0.044375,0.036583,0.047584,0.035333,0.034291,0.018,0.041125,0.016416,0.040416,0.016],"heldUniqueBytes":640000,"networkRequests":0,"seed":400,"sharedSeed":true,"workload":"warm-400-to-400"},
{"batch":0,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.195417,5.523292,5.745458,5.088208,5.549375,5.323834,5.654042,5.234916,5.696042,5.628417,5.522333,5.511625,5.61675,5.902083,6.136125,5.902125,6.09925,5.947291,5.801042,6.091959,6.129959,5.8615,5.894542,5.9525,5.796291],"heldUniqueBytes":3200000,"networkRequests":0,"seed":400,"sharedSeed":false,"workload":"warm-400-to-800"},
{"batch":0,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[0.043792,0.036958,0.065,0.047959,0.104584,0.058834,0.048584,0.055,0.023875,0.044959,0.026292,0.022125,0.024667,0.037625,0.018375,0.0195,0.014125,0.017833,0.025,0.016584,0.038667,0.051625,0.042583,0.03525,0.019083],"heldUniqueBytes":2560000,"networkRequests":0,"seed":800,"sharedSeed":true,"workload":"warm-800-to-800"},
{"batch":0,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[4.606834,4.671083,4.90775,5.152834,7.209375,5.497375,4.892208,5.469875,5.092917,4.866792,4.913875,4.727208,5.137625,5.107875,4.9265,4.725416,4.606709,4.81425,4.827292,4.695209,4.599084,4.92925,4.939333,5.053333,4.968125],"heldUniqueBytes":3200000,"networkRequests":0,"seed":800,"sharedSeed":false,"workload":"warm-800-to-400"},
{"batch":0,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.455916,5.422084,5.475583,5.393875,5.420083,5.274167,5.246958,5.52075,5.702083,5.434708,5.410875,5.263583,5.37025,5.443792,5.305958,5.387583,5.294916,5.384584,5.36125,5.305917,5.382792,5.277333,5.323875,5.119666,5.505167],"heldUniqueBytes":12800000,"networkRequests":0,"seed":1600,"sharedSeed":false,"workload":"warm-1600-to-800"},
{"batch":0,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.199416,5.312542,5.300958,5.016375,5.149334,5.074583,5.009792,4.987208,4.956583,4.894333,4.856167,5.255458,5.022916,5.03675,5.127375,5.339833,5.113834,5.06075,4.9635,4.916541,5.156834,4.967,4.94525,5.282,4.911666],"heldUniqueBytes":10880000,"networkRequests":0,"seed":1600,"sharedSeed":false,"workload":"warm-1600-to-400"},
{"batch":1,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[4.996333,4.950708,5.016042,4.9165,4.935708,4.930833,4.894958,4.947875,5.097375,5.090917,5.046542,4.839208,5.015417,4.893084,4.902291,5.1255,4.923958,5.304291,4.993042,5.13125,5.151291,5.066125,5.123042,5.459125,5.144416],"heldUniqueBytes":10880000,"networkRequests":0,"seed":1600,"sharedSeed":false,"workload":"warm-1600-to-400"},
{"batch":1,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.57725,5.614833,5.464791,5.301458,5.343875,5.321917,5.33525,5.341709,5.325792,5.324833,5.243916,5.280084,5.5,5.411625,5.421792,5.451542,5.643542,5.600791,5.496667,5.449625,5.526291,5.358375,5.3335,5.292291,5.679458],"heldUniqueBytes":12800000,"networkRequests":0,"seed":1600,"sharedSeed":false,"workload":"warm-1600-to-800"},
{"batch":1,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[4.840584,4.668375,4.663333,4.756458,4.583167,4.672791,4.640875,4.746375,4.704,5.119,4.910667,4.952584,4.852917,5.4485,5.031167,5.219792,5.287375,4.97375,4.700625,4.579875,4.838042,5.18025,4.55125,4.562875,4.524083],"heldUniqueBytes":3200000,"networkRequests":0,"seed":800,"sharedSeed":false,"workload":"warm-800-to-400"},
{"batch":1,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[0.034375,0.0235,0.012709,0.020958,0.019666,0.023583,0.020959,0.04825,0.042209,0.041458,0.02375,0.020125,0.035292,0.021792,0.0435,0.0395,0.02325,0.038709,0.014042,0.023666,0.0185,0.014417,0.030625,0.016292,0.048333],"heldUniqueBytes":2560000,"networkRequests":0,"seed":800,"sharedSeed":true,"workload":"warm-800-to-800"},
{"batch":1,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.216041,5.617208,5.714375,5.681667,5.065875,5.523208,5.931291,5.70025,5.534666,5.324166,5.233042,5.817666,5.649375,5.582167,5.096167,5.101458,5.436375,5.559666,5.413334,5.327,5.337917,5.40875,5.864208,5.538583,5.69075],"heldUniqueBytes":3200000,"networkRequests":0,"seed":400,"sharedSeed":false,"workload":"warm-400-to-800"},
{"batch":1,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[0.028458,0.019291,0.045791,0.023375,0.0235,0.037875,0.02175,0.012166,0.011834,0.033417,0.038583,0.0335,0.020125,0.014584,0.042042,0.049542,0.038875,0.031834,0.016458,0.017709,0.013042,0.011875,0.011167,0.017209,0.01325],"heldUniqueBytes":640000,"networkRequests":0,"seed":400,"sharedSeed":true,"workload":"warm-400-to-400"},
{"batch":1,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[4.674,4.7225,5.063042,4.668417,4.870875,5.021959,4.957333,5.037333,5.01925,4.712583,4.855916,4.953042,4.78375,4.967,4.884416,5.171667,5.079125,5.154958,4.733583,4.805375,4.581917,5.001083,5.275,5.277042,4.871625],"heldUniqueBytes":640000,"networkRequests":0,"sharedSeed":false,"workload":"saved-400"},
{"batch":1,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.8655,5.722375,5.759125,5.436833,5.640334,5.751167,5.638,5.501542,5.617208,5.629333,5.929208,5.943083,5.791875,5.507875,5.502458,5.259709,5.395459,5.623833,5.742542,5.591583,5.571541,5.366458,5.074917,5.380375,5.7645],"heldUniqueBytes":2560000,"networkRequests":0,"sharedSeed":false,"workload":"saved-800"},
{"batch":1,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.289,5.043708,5.019666,4.991083,4.94725,5.198958,5.26875,5.216875,5.124208,4.830917,4.898291,5.065958,4.974667,5.358167,5.266625,5.467625,5.080084,4.945125,5.113542,5.347208,5.336708,5.09325,5.17,5.42375,5.167834],"heldUniqueBytes":640000,"networkRequests":25,"sharedSeed":false,"workload":"network-400"},
{"batch":1,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.756125,5.957542,5.940625,6.007833,5.919667,5.597208,5.524666,5.630417,5.706208,5.512666,5.762833,5.518834,5.606666,5.625958,5.602708,5.452917,5.5855,6.02475,5.869,5.68675,5.735791,5.827375,5.509917,5.700875,5.72825],"heldUniqueBytes":2560000,"networkRequests":25,"sharedSeed":false,"workload":"network-800"},
{"batch":2,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.763959,5.333458,6.067208,6.06425,6.093292,5.848417,5.722667,5.781208,5.524625,5.6355,5.662,5.539666,5.610625,5.614791,5.961417,5.5375,5.552292,5.916667,5.836875,5.939041,5.674833,5.777958,5.855375,5.737041,5.844875],"heldUniqueBytes":2560000,"networkRequests":25,"sharedSeed":false,"workload":"network-800"},
{"batch":2,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.526,5.18675,5.093458,5.116709,4.888959,5.350791,5.073542,5.374833,5.269375,5.38425,5.17025,5.380791,5.478209,5.258792,5.540375,5.410708,5.134791,4.969625,5.110584,5.176291,5.385542,5.063666,5.176625,5.239709,5.066208],"heldUniqueBytes":640000,"networkRequests":25,"sharedSeed":false,"workload":"network-400"},
{"batch":2,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.546917,5.570125,5.810792,5.767209,5.334167,5.149125,5.378125,5.636791,5.688542,5.481,5.862166,5.651292,5.597875,5.414458,5.682041,5.789792,5.54225,5.867042,5.570042,5.311792,5.015833,5.487709,5.305417,5.0535,5.456333],"heldUniqueBytes":2560000,"networkRequests":0,"sharedSeed":false,"workload":"saved-800"},
{"batch":2,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.203333,5.012583,4.74625,4.642791,4.571625,5.358125,4.971584,5.049167,4.7555,5.271167,4.791333,5.007291,4.950291,4.976541,4.799084,5.330958,4.9645,5.521208,5.303917,5.032583,4.95075,5.092292,5.407541,4.807041,4.966042],"heldUniqueBytes":640000,"networkRequests":0,"sharedSeed":false,"workload":"saved-400"},
{"batch":2,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[0.026791,0.037542,0.019875,0.013875,0.010833,0.014,0.010833,0.01175,0.010542,0.010167,0.012917,0.01275,0.053834,0.015417,0.013875,0.015666,0.018959,0.015334,0.041459,0.028042,0.016333,0.042625,0.029209,0.02,0.012709],"heldUniqueBytes":640000,"networkRequests":0,"seed":400,"sharedSeed":true,"workload":"warm-400-to-400"},
{"batch":2,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.523709,5.531459,5.284083,5.206167,5.779625,5.74675,5.273,5.294625,5.455583,5.982125,5.616542,5.707083,5.572833,5.683791,5.851834,5.5655,5.622584,5.91175,5.713125,5.538792,5.3285,5.575125,5.2155,5.317291,5.539792],"heldUniqueBytes":3200000,"networkRequests":0,"seed":400,"sharedSeed":false,"workload":"warm-400-to-800"},
{"batch":2,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[0.026916,0.02175,0.036959,0.012916,0.044208,0.030292,0.04225,0.04425,0.015375,0.015625,0.012792,0.043459,0.018959,0.013083,0.054167,0.014417,0.051667,0.015958,0.012041,0.012,0.012541,0.049417,0.016959,0.051041,0.017167],"heldUniqueBytes":2560000,"networkRequests":0,"seed":800,"sharedSeed":true,"workload":"warm-800-to-800"},
{"batch":2,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.293958,4.953375,4.960666,4.846084,5.415083,5.19575,5.264458,4.961792,5.131,5.004834,5.214083,4.92725,5.072958,5.25675,4.908125,4.995083,4.984709,4.6485,4.855667,4.610166,5.124084,5.005541,4.777792,4.645833,5.041],"heldUniqueBytes":3200000,"networkRequests":0,"seed":800,"sharedSeed":false,"workload":"warm-800-to-400"},
{"batch":2,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.751584,5.630792,5.570042,5.499625,5.606625,5.370458,5.746917,5.403833,5.815209,5.490041,5.572209,5.777542,5.753292,5.727667,5.815792,5.4225,5.397084,5.367667,5.463292,5.385916,5.754291,5.570334,5.78325,5.387791,5.766375],"heldUniqueBytes":12800000,"networkRequests":0,"seed":1600,"sharedSeed":false,"workload":"warm-1600-to-800"},
{"batch":2,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.088291,5.1045,5.351542,5.144083,5.223916,5.041583,5.150542,5.081292,5.15425,5.172042,5.167084,4.924541,5.115042,5.132584,5.185209,4.982083,5.238666,5.041958,5.20425,5.130666,5.1185,5.308917,5.053791,5.238709,5.252625],"heldUniqueBytes":10880000,"networkRequests":0,"seed":1600,"sharedSeed":false,"workload":"warm-1600-to-400"},
{"batch":3,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.26475,5.211167,5.124375,5.312416,5.142958,5.240291,5.339708,5.514875,5.177708,5.312916,5.369167,5.233833,5.159166,5.185417,5.155083,5.253167,5.13175,5.259916,5.276541,5.371375,5.574,5.27275,5.434959,5.348625,5.2045],"heldUniqueBytes":10880000,"networkRequests":0,"seed":1600,"sharedSeed":false,"workload":"warm-1600-to-400"},
{"batch":3,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.730375,5.832792,7.33675,5.687667,5.680166,5.788959,5.927625,5.822458,5.67225,5.825,5.716666,5.787417,5.754709,5.852125,5.717042,5.727458,5.630875,5.628792,5.870208,5.555375,5.732958,5.957292,6.000917,5.648084,5.71275],"heldUniqueBytes":12800000,"networkRequests":0,"seed":1600,"sharedSeed":false,"workload":"warm-1600-to-800"},
{"batch":3,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.250042,5.317125,5.376708,4.895417,4.965542,5.215208,4.934583,5.198083,5.005041,4.833333,4.786084,5.1035,5.149959,5.028917,5.170208,5.013917,5.149625,5.359,5.073959,5.103042,5.045667,5.016292,4.702083,5.246209,4.611541],"heldUniqueBytes":3200000,"networkRequests":0,"seed":800,"sharedSeed":false,"workload":"warm-800-to-400"},
{"batch":3,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[0.013166,0.043875,0.034125,0.048375,0.05125,0.044209,0.043,0.05175,0.026833,0.038959,0.023166,0.031833,0.038625,0.027416,0.015958,0.014125,0.011666,0.014041,0.013333,0.013708,0.012208,0.052417,0.026209,0.020625,0.015667],"heldUniqueBytes":2560000,"networkRequests":0,"seed":800,"sharedSeed":true,"workload":"warm-800-to-800"},
{"batch":3,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.294625,5.291792,5.534375,5.355833,5.2945,5.395917,5.289125,5.729625,5.199917,5.065125,5.575709,5.407917,5.3055,5.208792,5.383042,5.48275,4.917417,5.317584,5.049875,5.381833,5.849958,5.185833,5.019041,5.203417,5.486834],"heldUniqueBytes":3200000,"networkRequests":0,"seed":400,"sharedSeed":false,"workload":"warm-400-to-800"},
{"batch":3,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[0.023625,0.026292,0.036833,0.039791,0.029417,0.031667,0.088333,0.050708,0.021792,0.014708,0.028292,0.012625,0.017375,0.011959,0.04175,0.012666,0.035,0.011416,0.05575,0.010708,0.045792,0.01925,0.029417,0.031584,0.052167],"heldUniqueBytes":640000,"networkRequests":0,"seed":400,"sharedSeed":true,"workload":"warm-400-to-400"},
{"batch":3,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.075916,4.718208,4.920291,4.755583,4.548917,4.82375,5.068458,4.565708,4.49225,4.682666,4.5155,4.620209,4.504542,4.752917,4.794375,4.627417,4.837416,4.587583,5.004875,5.055875,4.847458,4.881833,4.766416,4.732375,4.887875],"heldUniqueBytes":640000,"networkRequests":0,"sharedSeed":false,"workload":"saved-400"},
{"batch":3,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.142583,5.240167,5.715292,5.655708,5.567083,5.770584,5.645291,5.301291,5.226916,5.260917,5.162166,5.289541,5.087,5.479375,5.680459,5.1365,5.38025,5.629,5.392875,5.350958,5.518291,5.615959,5.657958,5.526792,5.349625],"heldUniqueBytes":2560000,"networkRequests":0,"sharedSeed":false,"workload":"saved-800"},
{"batch":3,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.215708,5.002084,5.146333,4.945958,5.013375,4.800333,4.801792,5.139375,4.790875,5.07775,5.234541,5.187209,4.992708,4.957666,4.898,5.08025,4.779208,5.100125,5.275292,5.052875,5.072041,4.999084,5.122708,5.041208,5.04125],"heldUniqueBytes":640000,"networkRequests":25,"sharedSeed":false,"workload":"network-400"},
{"batch":3,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.393584,5.780459,5.235375,5.516166,5.762458,5.817791,5.610833,5.568375,5.692625,5.618667,5.686208,5.733667,5.66025,5.376334,5.88125,5.641833,5.734958,5.790334,5.902833,5.598125,5.6615,5.558584,5.584875,5.638458,5.792625],"heldUniqueBytes":2560000,"networkRequests":25,"sharedSeed":false,"workload":"network-800"},
{"batch":4,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.712833,6.040167,5.777209,5.696791,5.863416,5.989,5.713708,5.549833,5.325541,5.76375,5.559584,5.636625,5.593333,5.93775,5.683375,5.592791,5.419208,5.830375,5.95425,5.853583,5.679667,5.749542,5.581458,5.695833,5.591125],"heldUniqueBytes":2560000,"networkRequests":25,"sharedSeed":false,"workload":"network-800"},
{"batch":4,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.308958,5.357834,5.191583,4.978291,4.991834,5.10075,4.97925,4.846042,4.931041,4.711,4.952667,5.08575,4.90325,5.019458,5.412792,5.28725,4.982166,6.301334,6.5535,5.882958,5.360042,5.34575,5.437417,5.157208,4.993125],"heldUniqueBytes":640000,"networkRequests":25,"sharedSeed":false,"workload":"network-400"},
{"batch":4,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.254625,5.380041,5.526,5.777583,5.723167,5.712875,5.495,5.551667,5.623208,5.752166,5.791417,5.582,5.473291,5.830375,5.585042,5.467,5.578125,5.669084,5.583833,5.640666,5.398291,5.618042,5.78825,6.24725,5.601584],"heldUniqueBytes":2560000,"networkRequests":0,"sharedSeed":false,"workload":"saved-800"},
{"batch":4,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.1575,5.269833,5.14575,4.987334,5.394792,5.050084,5.250042,4.959541,4.83925,4.916875,4.730917,4.746375,4.862083,4.984833,4.839875,4.903167,5.084917,5.005917,5.008166,4.820167,4.97325,4.999583,4.620125,4.894625,5.2915],"heldUniqueBytes":640000,"networkRequests":0,"sharedSeed":false,"workload":"saved-400"},
{"batch":4,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[0.014,0.037875,0.027375,0.036459,0.038083,0.019375,0.042584,0.026666,0.01475,0.015458,0.043583,0.021375,0.021125,0.014834,0.011709,0.059709,0.024,0.038125,0.033125,0.018292,0.038958,0.027375,0.03125,0.021459,0.012458],"heldUniqueBytes":640000,"networkRequests":0,"seed":400,"sharedSeed":true,"workload":"warm-400-to-400"},
{"batch":4,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.468792,5.284084,5.806208,5.332333,5.215875,5.65825,5.032167,5.284875,5.510625,5.694458,5.333834,5.433917,5.571416,5.698167,5.118375,5.288458,5.557083,5.546375,5.792375,5.853708,5.512334,5.723,5.54575,5.06325,5.325584],"heldUniqueBytes":3200000,"networkRequests":0,"seed":400,"sharedSeed":false,"workload":"warm-400-to-800"},
{"batch":4,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[0.040458,0.044375,0.040458,0.045875,0.045417,0.046959,0.037125,0.047083,0.019,0.047625,0.04075,0.041667,0.029958,0.056084,0.017375,0.061,0.045708,0.018875,0.016,0.047541,0.039084,0.034959,0.018083,0.038375,0.046208],"heldUniqueBytes":2560000,"networkRequests":0,"seed":800,"sharedSeed":true,"workload":"warm-800-to-800"},
{"batch":4,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.251417,5.220542,5.349417,4.981917,5.024125,5.124958,5.232916,5.091417,4.908042,5.257916,4.74,4.90075,5.001834,5.222917,5.123834,4.814625,4.779375,4.863833,4.686167,4.914042,5.112667,5.259167,5.36525,5.004,5.039041],"heldUniqueBytes":3200000,"networkRequests":0,"seed":800,"sharedSeed":false,"workload":"warm-800-to-400"},
{"batch":4,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.59525,5.710334,5.875459,6.732459,5.673958,5.915292,5.4915,5.659583,5.675417,5.566333,5.744208,5.822584,5.6115,5.611125,5.542375,5.585292,5.623708,5.26825,6.222625,5.272958,5.576958,5.480459,5.653875,5.6615,5.466125],"heldUniqueBytes":12800000,"networkRequests":0,"seed":1600,"sharedSeed":false,"workload":"warm-1600-to-800"},
{"batch":4,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.132667,5.284708,5.139,5.289291,5.176333,4.730208,5.284708,5.297041,5.045,5.256,5.21325,5.173708,5.31675,5.098083,5.019042,5.023791,5.178958,5.3015,5.302375,5.2955,5.200917,4.982792,5.415583,5.264833,5.080583],"heldUniqueBytes":10880000,"networkRequests":0,"seed":1600,"sharedSeed":false,"workload":"warm-1600-to-400"},
{"batch":5,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.314625,5.58475,5.047208,5.131875,5.21975,5.240542,5.11675,7.198042,5.370667,5.298125,5.230208,5.146917,5.266,5.202,5.133125,5.1035,5.164375,5.057791,5.029208,5.175375,5.112333,5.336,4.941291,5.065417,4.953917],"heldUniqueBytes":10880000,"networkRequests":0,"seed":1600,"sharedSeed":false,"workload":"warm-1600-to-400"},
{"batch":5,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.645625,5.601833,5.622208,5.578084,5.798417,5.657709,5.497459,5.402916,5.545291,5.631208,5.944541,5.833542,5.61425,5.792916,5.67075,5.711583,5.916417,5.523584,6.093667,5.918375,5.682625,5.846083,5.875083,5.802459,5.774875],"heldUniqueBytes":12800000,"networkRequests":0,"seed":1600,"sharedSeed":false,"workload":"warm-1600-to-800"},
{"batch":5,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.236333,4.677667,5.175417,4.967041,4.8325,4.60125,4.621958,5.060417,4.97325,5.208375,4.576208,4.993084,5.372125,4.97875,5.147166,4.730084,4.862708,4.812208,4.867958,4.950292,4.974917,5.110791,5.244625,5.23075,5.217291],"heldUniqueBytes":3200000,"networkRequests":0,"seed":800,"sharedSeed":false,"workload":"warm-800-to-400"},
{"batch":5,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[0.048042,0.018209,0.021583,0.052292,0.039,0.04575,0.045375,0.051458,0.048833,0.029667,0.021709,0.021583,0.049708,0.035416,0.019125,0.015,0.048208,0.020167,0.014458,0.012417,0.01225,0.011625,0.055625,0.016541,0.01975],"heldUniqueBytes":2560000,"networkRequests":0,"seed":800,"sharedSeed":true,"workload":"warm-800-to-800"},
{"batch":5,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.852375,5.126,5.48325,5.61075,5.19725,4.928792,4.935959,4.912458,5.585625,5.013583,4.897667,5.577,5.485708,5.1585,5.028,5.085042,5.780875,5.436875,5.575875,5.075708,5.207334,4.878375,5.441458,5.156708,5.673167],"heldUniqueBytes":3200000,"networkRequests":0,"seed":400,"sharedSeed":false,"workload":"warm-400-to-800"},
{"batch":5,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[0.03625,0.059834,0.031459,0.043958,0.017208,0.049084,0.02925,0.039125,0.021916,0.04625,0.016875,0.044167,0.046792,0.014125,0.036334,0.039042,0.040208,0.039375,0.01475,0.038667,0.012167,0.047375,0.046167,0.015875,0.011458],"heldUniqueBytes":640000,"networkRequests":0,"seed":400,"sharedSeed":true,"workload":"warm-400-to-400"},
{"batch":5,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[4.681458,4.880708,4.711959,4.598625,4.538208,4.595375,4.601166,4.748125,4.546292,4.838541,5.178292,4.727875,4.52075,4.805375,5.170542,4.726458,4.446458,4.525875,4.796334,5.075459,4.470542,4.65125,4.622542,4.836291,4.771875],"heldUniqueBytes":640000,"networkRequests":0,"sharedSeed":false,"workload":"saved-400"},
{"batch":5,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.36525,5.104458,5.396792,5.69,5.434375,5.233916,5.521459,5.51575,5.377958,4.935291,5.743959,6.060166,5.609708,5.504667,5.405167,5.545,5.293917,5.574,5.519292,5.751,5.638875,5.191792,5.290291,5.194958,5.581375],"heldUniqueBytes":2560000,"networkRequests":0,"sharedSeed":false,"workload":"saved-800"},
{"batch":5,"decodedBytes":640000,"dimension":400,"elapsedMilliseconds":[5.129125,5.017333,5.126042,5.060541,5.479709,5.066542,5.170375,5.073,5.163708,5.27775,5.307459,5.064833,5.154834,5.148583,5.054042,5.29625,5.01475,5.041583,5.003958,4.988458,5.177417,5.133417,5.1915,5.143208,5.175],"heldUniqueBytes":640000,"networkRequests":25,"sharedSeed":false,"workload":"network-400"},
{"batch":5,"decodedBytes":2560000,"dimension":800,"elapsedMilliseconds":[5.857083,5.377792,5.174542,5.399667,5.593708,5.361917,5.725375,5.596958,5.498458,5.793292,5.858917,7.087333,6.298875,5.956042,5.411625,5.360625,5.815583,5.3535,5.658541,5.836875,6.158459,5.776584,5.403166,5.947417,5.459542],"heldUniqueBytes":2560000,"networkRequests":25,"sharedSeed":false,"workload":"network-800"}
]}
```
