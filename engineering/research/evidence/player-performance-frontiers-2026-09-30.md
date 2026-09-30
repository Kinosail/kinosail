# Performance frontiers: measurement record

Synthetic data only. Source hashes bind the native results to the validated working tree. Hosted results are linked from the pull request after publication.

```json
{
  "starting_revision": "9a45e9444613dbde5a277a8c7426919fcab97aa8",
  "measurement_revision": "starting revision plus the compact collation scratch-storage change",
  "environment": {
    "os": "macOS",
    "arch": "arm64",
    "cpu": "Apple M1 Pro",
    "notes": "Other native builds ran concurrently. Timings are host-dependent; no physical-device gain is claimed."
  },
  "fixture": {
    "titles": 10000,
    "names": "Movie 0 through Movie 9999",
    "kind": "video",
    "year": "2026",
    "profile": "synthetic owner",
    "page_size": 100,
    "response_bytes": 25319
  },
  "baseline": {
    "command": "go test ./internal/server -run '^$' -bench '^BenchmarkLargeLibraryBrowse$' -benchtime=2s -count=5 -cpuprofile=/tmp/kinosail-frontiers-before.cpu -memprofile=/tmp/kinosail-frontiers-before.mem",
    "result": "goos: darwin\ngoarch: arm64\npkg: github.com/MikeO7/kinosail-player/internal/server\ncpu: Apple M1 Pro\nBenchmarkLargeLibraryBrowse-10    \t     343\t   6782572 ns/op\t     25319 response-bytes\t 4708751 B/op\t   11590 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     348\t   6782943 ns/op\t     25319 response-bytes\t 4708623 B/op\t   11590 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     334\t   6930226 ns/op\t     25319 response-bytes\t 4708596 B/op\t   11590 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     342\t   7054822 ns/op\t     25319 response-bytes\t 4708527 B/op\t   11590 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     376\t   6445994 ns/op\t     25319 response-bytes\t 4708575 B/op\t   11590 allocs/op\nPASS\nok  \tgithub.com/MikeO7/kinosail-player/internal/server\t17.355s\n"
  },
  "compact_keys": {
    "command": "go test ./internal/server -run '^$' -bench '^BenchmarkLargeLibraryBrowse$' -benchtime=2s -count=5",
    "result": "goos: darwin\ngoarch: arm64\npkg: github.com/MikeO7/kinosail-player/internal/server\ncpu: Apple M1 Pro\nBenchmarkLargeLibraryBrowse-10    \t     345\t   6804355 ns/op\t     25319 response-bytes\t 2829872 B/op\t   11573 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     327\t   7039870 ns/op\t     25319 response-bytes\t 2829917 B/op\t   11573 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     338\t   6787313 ns/op\t     25319 response-bytes\t 2829878 B/op\t   11573 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     370\t   6628020 ns/op\t     25319 response-bytes\t 2829840 B/op\t   11573 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     354\t   6894637 ns/op\t     25319 response-bytes\t 2829813 B/op\t   11573 allocs/op\nPASS\nok  \tgithub.com/MikeO7/kinosail-player/internal/server\t17.165s\n",
    "source_sha256": "0bcb4d6bb1a9617c8922cbe0a3d1d32fef34e32d3fc82bb3dbf7a9c9926dc3bb"
  },
  "native_fixture": {
    "network": "URLProtocol controlled responses and held requests, no user server",
    "images": "800, 1600 and 4096 pixel squares generated in memory",
    "cache": "private temporary disk cache; 32 MiB decoded-image budget",
    "device": "dedicated Apple TV 1080p simulator",
    "os": "tvOS 27.0",
    "configuration": "Debug correctness tests; not release performance traces"
  },
  "native_red_controls": [
    {
      "stage": "original LRU admission",
      "result": "Displayed image was evicted and requested twice; promoted image lost identity."
    },
    {
      "stage": "refresh growth",
      "result": "Three previously displayed images lost identity after a larger background refresh."
    },
    {
      "stage": "URL-only foreground demand and retained old replacement",
      "result": "Held 800px demand promoted speculative 1600px disk pixels; three displayed identities changed. Next foreground refresh remained 800px after fresh 1600px disk bytes."
    },
    {
      "stage": "replacement above retention budget",
      "result": "4096px refresh retained obsolete 800px pixels. The 1600px case passed."
    }
  ],
  "verification": {
    "shared_packages": "go test ./... passed",
    "catalog_race": "go test -race ./catalog -count=1 passed",
    "large_library_regressions": "go test ./internal/server -run 'TestLarge(Library|Letter)' -count=1 passed",
    "max_loc": "make max-loc passed",
    "tooling": "make tooling-check passed",
    "native_final": "26 tests in five suites passed; refresh growth exercised 1600px and 4096px cases",
    "player_full": "go test ./... passed",
    "apple_builds": "make -C apps/player client-check passed (iOS and tvOS simulator builds)",
    "required_hosted_checks": "pending"
  },
  "boundaries": [
    "Physical iPhone and Apple TV frame timing",
    "Full local browser matrix",
    "Deployed first-frame timings",
    "Production network performance"
  ],
  "native_command": "xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-tvOS -destination 'platform=tvOS Simulator,id=64468BFE-9114-4FBF-860B-79DF45F70011' -derivedDataPath .build/speed-tvos -jobs 4 -parallel-testing-enabled NO -only-testing:Kinosail-tvOSTests/ArtworkAdmissionTests -only-testing:Kinosail-tvOSTests/ArtworkLoaderTests -only-testing:Kinosail-tvOSTests/ArtworkCancellationTests -only-testing:Kinosail-tvOSTests/ArtworkSharingTests -only-testing:Kinosail-tvOSTests/ArtworkPrefetchTests -resultBundlePath .verification/frontiers-artwork-delivery.xcresult CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- test",
  "native_source_sha256": {
    "apps/player/apps/native/Sources/Platform/ArtworkLoader.swift": "2c71b58d54284ae8c68ad8d7265739cf0f334fde66b96f28c38090ebbb8608aa",
    "apps/player/apps/native/Tests/ArtworkAdmissionTests.swift": "0a7a459c3c9e8c22c604d72d9c8fefc0fb4d6415357633b465727dc37d0c1d5f",
    "apps/player/apps/native/Tests/ArtworkFixtureImages.swift": "6b1211c0b73d2b3fbb210d9919e9f442ce985f50c952893b662bbacb129c8fad",
    "apps/player/apps/native/Tests/ArtworkLoaderTests.swift": "596de51884ab26b3c2b0cdfa500361ffbced85d9a067b9ef68ecaed67122b79c"
  }
}
```
