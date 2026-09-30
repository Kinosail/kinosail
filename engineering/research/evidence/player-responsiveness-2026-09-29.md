# Player responsiveness measurement record

This record preserves the commands, fixture, environment, revision, and measured results.
Browser runs used an isolated local production binary. Simulator results establish correctness only.

```json
{
  "date": "2026-09-29",
  "auditedBase": "517fdf6b25047845adefd7bd9489214b82f0fc24",
  "implementationRevision": "0a141f9ce66803d9293952d45a8133caac98e293",
  "environment": {
    "platform": "macOS-27.0-arm64-arm-64bit-Mach-O",
    "architecture": "arm64",
    "cpu": "Apple M1 Pro",
    "go": "go1.27.1",
    "concurrentHostBuilds": true
  },
  "browse": {
    "command": "go test ./internal/server -run '^$' -bench '^BenchmarkLargeLibraryBrowse$' -benchtime=2s -count=5",
    "module": "apps/player",
    "fixture": "10,000 synthetic video items, Movie 0 through Movie 9999, title sort, default 100-item page",
    "before": {
      "nsPerOp": [
        17616642,
        33091550,
        31727464,
        24721001,
        27426629
      ],
      "bytesPerOp": [
        7428783,
        7428865,
        7428816,
        7428698,
        7428857
      ],
      "allocationsPerOp": [
        41591,
        41590,
        41590,
        41590,
        41590
      ]
    },
    "after": {
      "nsPerOp": [
        10623983,
        9444286,
        9446794,
        13076944,
        8622863
      ],
      "bytesPerOp": [
        4708542,
        4708450,
        4708447,
        4708536,
        4708445
      ],
      "allocationsPerOp": [
        11591,
        11590,
        11590,
        11590,
        11590
      ]
    },
    "responseBytesBeforeAndAfter": 25319,
    "result": "passed",
    "caveat": "Wall-clock timings are noisy because unrelated builds ran concurrently. Allocations and response size are more stable."
  },
  "compression": {
    "command": "go test ./internal/server -run 'TestStatic(Bundle|Compression)' -count=1 -v",
    "assets": [
      {
        "path": "/static/app.css",
        "identityBytes": 212487,
        "gzipBytes": 42371
      },
      {
        "path": "/static/player.js",
        "identityBytes": 107554,
        "gzipBytes": 27040
      },
      {
        "path": "/static/hls.min.js",
        "identityBytes": 618156,
        "gzipBytes": 195347
      }
    ],
    "decodedBytesEqual": true,
    "redRegressionObserved": true,
    "result": "passed"
  },
  "browser": {
    "run": "Populated local production Go binary over TLS; separate from blocked container gate",
    "fixture": "36 Movie 00 through Movie 35 copies plus SpeedFixture, original locally generated 8-second H.264/AAC testsrc2 video; no user media",
    "results": [
      {
        "revision": "517fdf6b25047845adefd7bd9489214b82f0fc24",
        "command": "pnpm --dir e2e exec playwright test static-compression.spec.ts --project=chromium --workers=1",
        "fixture": {
          "populatedMovies": 37,
          "serviceWorkers": "blocked"
        },
        "environment": {
          "viewport": {
            "width": 1440,
            "height": 900
          },
          "browser": "chromium",
          "version": "154.0.8037.92"
        },
        "result": "passed",
        "plainBytes": 212487,
        "compressedBytes": 42371,
        "resources": [
          {
            "path": "/static/theme.js",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 3817,
            "decodedBodySize": 12746
          },
          {
            "path": "/static/app.css",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 42371,
            "decodedBodySize": 212487
          },
          {
            "path": "/static/htmx.min.js",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 13478,
            "decodedBodySize": 36716
          },
          {
            "path": "/static/main.kinosail.bundle.js",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 13310,
            "decodedBodySize": 50047
          },
          {
            "path": "/static/icon.svg",
            "duration": 5.0999999940395355,
            "transferSize": 525,
            "encodedBodySize": 225,
            "decodedBodySize": 225
          },
          {
            "path": "/static/supporter.js",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 3069,
            "decodedBodySize": 8768
          },
          {
            "path": "/api/v1/supporter/collection",
            "duration": 5.4000000059604645,
            "transferSize": 336,
            "encodedBodySize": 36,
            "decodedBodySize": 36
          },
          {
            "path": "/static/manrope.woff2",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 53776,
            "decodedBodySize": 53776
          },
          {
            "path": "/static/cinema-backdrop.jpg",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 271293,
            "decodedBodySize": 271293
          },
          {
            "path": "/static/icon.svg",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 225,
            "decodedBodySize": 225
          },
          {
            "path": "/static/icon.svg",
            "duration": 3.7999999970197678,
            "transferSize": 525,
            "encodedBodySize": 225,
            "decodedBodySize": 225
          }
        ]
      },
      {
        "revision": "517fdf6b25047845adefd7bd9489214b82f0fc24",
        "command": "pnpm --dir e2e exec playwright test static-compression.spec.ts --project=chromium --workers=1",
        "fixture": {
          "populatedMovies": 37,
          "serviceWorkers": "blocked"
        },
        "environment": {
          "viewport": {
            "width": 390,
            "height": 650
          },
          "browser": "chromium",
          "version": "154.0.8037.92"
        },
        "result": "passed",
        "plainBytes": 212487,
        "compressedBytes": 42371,
        "resources": [
          {
            "path": "/static/theme.js",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 3817,
            "decodedBodySize": 12746
          },
          {
            "path": "/static/app.css",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 42371,
            "decodedBodySize": 212487
          },
          {
            "path": "/static/htmx.min.js",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 13478,
            "decodedBodySize": 36716
          },
          {
            "path": "/static/main.kinosail.bundle.js",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 13310,
            "decodedBodySize": 50047
          },
          {
            "path": "/static/icon.svg",
            "duration": 1.8999999910593033,
            "transferSize": 525,
            "encodedBodySize": 225,
            "decodedBodySize": 225
          },
          {
            "path": "/static/supporter.js",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 3069,
            "decodedBodySize": 8768
          },
          {
            "path": "/api/v1/supporter/collection",
            "duration": 3.4000000059604645,
            "transferSize": 336,
            "encodedBodySize": 36,
            "decodedBodySize": 36
          },
          {
            "path": "/static/manrope.woff2",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 53776,
            "decodedBodySize": 53776
          },
          {
            "path": "/static/cinema-backdrop.jpg",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 271293,
            "decodedBodySize": 271293
          },
          {
            "path": "/static/icon.svg",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 225,
            "decodedBodySize": 225
          },
          {
            "path": "image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='2' stroke-linecap='round'%3E%3Ccircle cx='10.8' cy='10.8' r='6.8'/%3E%3Cpath d='m16 16 5 5'/%3E%3C/svg%3E",
            "duration": 0,
            "transferSize": 0,
            "encodedBodySize": 0,
            "decodedBodySize": 0
          },
          {
            "path": "/static/icon.svg",
            "duration": 4.4000000059604645,
            "transferSize": 525,
            "encodedBodySize": 225,
            "decodedBodySize": 225
          }
        ]
      }
    ],
    "sourceWorkingTreeAtRun": true,
    "binaryBuiltBeforeParserRefactor": "Behavior preserved except further rejection of malformed encoding entries"
  },
  "native": {
    "command": "xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-tvOS -destination 'platform=tvOS Simulator,id=64468BFE-9114-4FBF-860B-79DF45F70011' -derivedDataPath .build/speed-tvos -jobs 4 -parallel-testing-enabled NO -only-testing:Kinosail-tvOSTests/ArtworkLoaderTests -only-testing:Kinosail-tvOSTests/ArtworkCancellationTests -resultBundlePath .verification/speed-artwork-final.xcresult CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- test",
    "environment": "tvOS 27 Simulator, Apple TV 1080p",
    "tests": 16,
    "result": "passed",
    "scope": "Correctness and cancellation, not physical-device smoothness"
  },
  "limits": [
    "Container-backed populated browser gate could not build: Podman storage full. No storage was pruned.",
    "Full shared-package check stops at 111 pre-existing lint findings. Changed-code package lint passes.",
    "No physical iPhone or Apple TV frame/hitch trace.",
    "No end-to-end playback first-frame, cross-browser matrix, production network, or deployment performance proof."
  ]
}
```
