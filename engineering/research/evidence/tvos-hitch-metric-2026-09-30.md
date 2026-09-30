# tvOS XCTest hitch-metric probe

Date: September 30, 2026. Native source revision: `b2e72bd8b78fbe2f0720bc18c24e9368439cb9c1`.

## Result

The Release simulator seed and real remote-navigation test passed. The navigation test executed once, with zero failures, in 47.982 seconds. Its result bundle contains three CPU metrics and three samples for each. **It contains no hitch metric.** The successful test does not establish zero hitches, presentation latency, or physical-device smoothness.

[Apple documents XCTHitchMetric](https://developer.apple.com/documentation/xctest/xcthitchmetric) as a UI hitch metric. The installed Xcode 27 headers expose its application initializer on tvOS 26 and later. This verifies the compile-time API. The result bundle's missing metric shows why API availability and test success are insufficient evidence of usable presentation timing here. The omission's underlying runtime cause is not established.

The earlier Instruments Hitches route also lacked usable simulator presentation data. This independent XCTest attempt narrows the remaining measurement boundary. Physical-device presentation timing and input-to-visible-feedback remain unmeasured. CPU time below is aggregate app CPU during automated traversal windows, not input latency or per-frame CPU.

## Workload and environment

Xcode 27.0, Release optimization, ad-hoc signing, `ENABLE_TESTABILITY=YES`, tvOS 27 simulator, task-owned Apple TV 4K third-generation 1080p configuration. The synthetic loopback fixture has 160 movies, populated continuation, additional media kinds, generated artwork, and a missing-artwork case. It uses no production account or Server data.

The test launches the app, opens Movies, and traverses the grid in both directions before measurement. Each measurement window repeats three one-second right/left remote holds. It requests `XCTHitchMetric` and `XCTCPUMetric`, uses manual stop, and sets iteration count to three. Setup and screenshots are outside measurement windows. Remote automation and idle waits remain within those windows. Cache-hit status and physical display intervals were not independently measured.

The first fixture attempt stopped before simulator admission because the default Python lacked Pillow. The retry uses the bundled Python runtime. Temporary test sources were removed in `finally`; every tracked native source hash matched its original value. The owned fixture stopped and the task simulator shut down. Unrelated simulators and work remain untouched.

## Exported metrics

| Metric | Unit | Recorded samples |
| --- | --- | --- |
| CPU Cycles (player) | kC | 4667567.456, 4613168.237, 4515229.063 |
| CPU Time (player) | s | 1.5994314840000001, 1.5702622800000001, 1.5617907370000002 |
| CPU Instructions Retired (player) | kI | 5545034.033, 5359268.952, 5314113.3 |

No baseline/candidate UI change was compared. These samples establish a CPU measurement route, not a performance improvement. The metric export was read with:

```sh
xcrun xcresulttool get test-results metrics --path .verification/tvos-hitch-metric/metric.xcresult --compact
```

## Source bindings

Fixture SHA-256: `9dcfb092f29e9ddd4ea29c1527a465027efcd650101f76864c3314a13fb8b338`. Probe SHA-256: `5dd10c56e638bfcacef7d2fa5047eb249e863a444cafb7b4cf488a0547951981`. Seed SHA-256: `16c136e8ae4fbc76b952932e3e4b3a2a9a7c8e8d42362456b81ec46518f7fb20`. Built app executable SHA-256: `5c6fb7c392ddbbf1eec4350e33bc6099dd55962633de1abd9669edef55574765`. The private manifest retains all tracked native hashes and the exact commands. The following files own the measured grid, artwork, and home behavior.

| Path under native client | SHA-256 |
| --- | --- |
| `Sources/Design/MediaViews.swift` | `4cfe896857cb59524802bbf8e87440e7e72f5e8e32e34244cfe18152bd9dad44` |
| `Sources/Design/Artwork.swift` | `5c520575a5f052a98c3f66c047439951cb48c1a68d176c0a76423b2da062e01b` |
| `Sources/Design/ArtworkPrefetch.swift` | `0eefb3abfbfa22c2a78806e30e7361aac332868fb49df5cb4b5457424e938492` |
| `Sources/Features/Home/HomeScreen.swift` | `888b39cbc16cd180e33c74d949efede266b4511715a0eca0b62778d0fd0c4c4d` |
| `Sources/Platform/ArtworkLoader.swift` | `2c71b58d54284ae8c68ad8d7265739cf0f334fde66b96f28c38090ebbb8608aa` |

Build/test commands use `xcodebuild`, `Kinosail.xcodeproj`, `Release`, destination `id=64468BFE-9114-4FBF-860B-79DF45F70011`, `-jobs 4`, `-parallel-testing-enabled NO`, the signing/testability flags above, and derived data `apps/player/apps/native/.build/speed-tvos-hitch-metric`. The seed selects `Kinosail-tvOSTests/RenderProfileSessionSeedTests` under scheme `Kinosail-tvOS`. The probe selects `Kinosail-tvOSRemoteUITests/RemoteHitchMetricProbeTests` under scheme `Kinosail-tvOS-Remote`. Both use `test` and separate result bundles. Logs, result bundles, command JSON, fixture, seed source, and complete bindings remain in private verification storage.

## Probe source

This temporary test does not enter a production app target. It reuses the repository's `RemoteTestCase` screenshot helper.

```swift
import XCTest

final class RemoteHitchMetricProbeTests: RemoteTestCase {
    @MainActor
    func testRepeatedMovieGridTraversal() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15))
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 15))
        remote.press(.down)
        remote.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 15))
        remote.press(.down)
        for _ in 0..<3 {
            remote.press(.right, forDuration: 1)
            remote.press(.left, forDuration: 1)
        }
        record("hitch-grid-precondition", app)
        let options = XCTMeasureOptions()
        options.iterationCount = 3
        options.invocationOptions = [.manuallyStop]
        measure(metrics: [XCTHitchMetric(application: app), XCTCPUMetric(application: app)], options: options) {
            for _ in 0..<3 {
                remote.press(.right, forDuration: 1)
                remote.press(.left, forDuration: 1)
            }
            stopMeasuring()
            XCTAssertTrue(app.staticTexts["Sort: Title"].exists)
        }
        record("hitch-grid-after-measurement", app)
    }
}
```
