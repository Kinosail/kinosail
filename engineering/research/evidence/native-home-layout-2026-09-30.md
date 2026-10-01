# Native Home layout: defer artwork below the viewport

Date: September 30, 2026. Base: `2d00b19a6b9502b6fd02149c8c13dd1f80e96410`.

Home's vertical content stack eagerly constructed its shelves. Each horizontal shelf was already lazy, but its artwork task could start before the shelf was visible. The candidate changes the outer `VStack` to `LazyVStack`. Alignment, spacing, cache budgets, lookahead, and network limits stay the same.

## Measured outcome

Cold startup sends fewer artwork responses in a populated synthetic library. Each comparison uses three original runs, three candidate runs, then three original-source reverse controls. Both platforms retain their catalog request counts and visible first-viewport content.

| Platform | Original artwork responses | Candidate | Reverse control | Reduction | Catalog responses per run |
| --- | ---: | ---: | ---: | ---: | ---: |
| tvOS 27, 1920 × 1080 | 47, 47, 47 | 34, 34, 34 | 47, 47, 47 | 27.66% | 10 |
| iOS 27, iPhone 17 Pro portrait | 47, 47, 47 | 9, 9, 9 | 47, 47, 47 | 80.85% | 7 |

Unique artwork paths change from 43 to 30 on TV and from 45 to 8 on iPhone. Response counts include repeated requests and missing artwork. They are not byte, decode, CPU, frame, or energy measurements. The unchanged catalog counts do not prove unchanged catalog CPU.

The iPhone viewport is mostly a hero and continuation shelf. The TV viewport includes the continuation shelf, Browse, and the top of a recent shelf. This explains why their request reductions differ. It is an inference from the inspected layouts, not a general platform speed ranking.

All response events are retained in [the response samples](native-home-layout-samples-2026-09-30.md). The fixed windows range from 12.000472 to 12.016422 seconds. No accessibility query, screenshot, or Instruments attachment occurs during a counted window. Screenshots are taken afterward.

## Research connection

[Apple's WWDC26 lazy-stack session](https://developer.apple.com/videos/play/wwdc2026/321/) explains lazy vertical loading, nested lazy horizontal stacks, partial prefetch, and estimated offscreen geometry. It also describes risks from variable child counts and layout changes after appearance. These mechanisms support testing Home's existing composition before adding a custom scheduler or larger cache.

This change retains the existing shelf and card prefetch policies. It defers admission of some shelves. It does not remove all speculative work or establish an optimal prefetch policy. The genre section still contains an ordinary vertical stack; it remains a separate measured follow-up candidate.

[Apple's WWDC26 responsiveness session](https://developer.apple.com/videos/play/wwdc2026/268/) recommends identifying the cause of expensive work and verifying the result with Instruments. Reduced startup artwork work may leave more resources for visible content. That presentation benefit still needs device measurements here.

## Workload and controls

The private fixture listens on loopback port 38359. It contains 160 synthetic movies, six continuation items, four genres, one show, four episodes, and one item in each other media kind. Movies include whole-second added timestamps and nine-digit fractional progress timestamps. Artwork is generated before startup: 600 × 900 posters and 1600 × 900 backdrops. One movie deliberately returns missing artwork. Artwork responses wait 100 ms in both variants. Catalog responses have no added delay in the counted experiment.

The driver terminates only the owned simulator app, installs a bound app bundle, and removes only that app's `Library/Caches`. It preserves the seeded synthetic session. It starts a monotonic clock immediately before `simctl launch`, counts fixture response events until the fixed window ends, and captures the screen afterward. It repeats the same sequence for both source variants and the reverse control.

The fixture logs its response event inside `send_response`, before headers and body are written. `relative_ms` in the CSV measures that event relative to the launch command. It is not request arrival, actual first byte, first frame, or full image presentation time. Counts include every response with an artwork path, including 404 responses.

The timing experiment runs serially with no task build in progress. The machine is shared, so unrelated host load remains uncontrolled. Stable response counts across both control directions support the narrow artwork-work conclusion.

Both counted variants use Release, arm64, ad hoc signing, testability enabled, and the same SDK and device destination on their platform. Native production source differs only in the Home stack. The source-revision label differs deliberately between builds. Test-only seed/probe code is excluded from production source comparisons.

An earlier iOS matrix used a generic original bundle and a device-specific candidate. It produced the same counts, but differing build settings confounded that comparison. It is retained privately as exploratory evidence. The published iOS result uses a rebuilt original with matching settings.

## Source and binary bindings

SHA-256 bindings for the counted experiment:

| Input | SHA-256 |
| --- | --- |
| Original Home source | `888b39cbc16cd180e33c74d949efede266b4511715a0eca0b62778d0fd0c4c4d` |
| Candidate Home source | `b19a69d0ac18fd0707f4b6f1c3d3c4bf94e5aeb0cd046b49cd32d8f6ff9ac750` |
| Fixture source | `3b2eb339ec07d2663d71b8f45a8c83fb6acbc14f5622c2feb4cf8f418300e5e8` |
| TV original executable | `d63602ebffb30306d1d7e32dc172e3c002e7f2e674a08c855a03a66e93b189e3` |
| TV candidate executable | `03641975bd96d26d86b439a85434288f446115850ddef13f1f68dc84478d656a` |
| iOS matched original executable | `92c6f4bd106a0bbd0fb808fc43038cc45de7a91744497de0a803f0f2ee5485fa` |
| iOS candidate executable | `b8f69e667f9f225e3b54847a344854e138b0e0e40d736ca638be9ae989dd02ce` |
| TV launch driver | `5d4281cf7d94844b8b5e6aa422d5b2a32cf04b1601815c1c76d009952adfea6f` |
| Matched iOS launch driver | `351966e7f33e5b386f54d7980ecc757faa7ca5af34535a5340adcf0b2727da41` |

The initial exploratory TV app has a different executable hash from the bound original bundle above. The counted matrix uses separately archived, explicitly bound bundles. No byte-identity claim is made for that earlier discovery app.

## Regression evidence

The new public remote journey passed on the original source before the production edit. It reaches the last genre shelf, reverses horizontal movement, opens details, returns to the selected card, and returns to Search. Its screenshots and focus assertions make loss of below-viewport content observable.

The candidate passed that journey plus existing grid-detail Back, far-edge top-rail return, and continuation-card clearance checks. Four tests passed with no failures or skips. Their durations include automation and accessibility queries. They are not input-latency comparisons.

The private iPhone journey passed populated deep scrolling, fast reversal, detail-and-back navigation, landscape rotation, and return to portrait. Screenshots and accessibility trees are retained in its result bundle. Wider iPad layouts and physical touch timing remain separate boundaries.

Private cold-cache state checks passed on both platforms. A 12-second catalog delay displays a pending skeleton, then loaded content removes it. Empty and failed responses remove loading status and expose their existing empty/retry controls. TV pending and empty checks also retain Search focus. All six state tests passed without failures or skips.

Inspected portrait and TV screenshots align the initial hero/rail artwork rectangles and content margins between pending and loaded states. Long titles retain their existing two-line treatment. The phone skeleton still reserves more metadata/action space than the loaded hero, shifting the later continuation shelf. That existing geometry mismatch remains a follow-up. The failed phone state also retains its connection banner above retry content; this phase makes no broader error-layout improvement claim.

The candidate's full Release tvOS native suite passed 271 tests with no failures or skips. Two additional public remote journeys passed Home directional rows and the top-bar/Browse return paths, including Music and Audiobooks destinations. A separate private iPhone test passes Listen Home's music and audiobook shelves, deep Browse reachability, and switching back to Watch. The fixture has no continuing audio items, so it does not cover Listen expansion state.

The independent source review found no production defect. It identified an incomplete horizontal-movement assertion in the new remote regression. The corrected test checks that Right changes focus before reversing and that the selected card remains hittable afterward.

The corrected remote regression passes again on the original and candidate sources, one test in each run with no failures or skips. The independent follow-up review confirms its movement assertion is resolved. Repository tooling, source caps, whitespace checks, and the Player architecture snapshot check pass.

The native-copy generator check fails on both original and candidate Home sources. Expected generated keys add TMDB attribution and remove `Media`; Home's stack substitution changes no copy. This existing generated-copy drift is retained as a separate check failure. It is not reported as a passing check or repaired through unrelated translation churn.

Reconciliation incorporates main's Android overlay change at `2d93bbf68be91746986c28d8d79c092b0343380f`. Native production and test source remain unchanged. The candidate Home hash still matches the counted experiment. Post-commit Player `verify-changed` passes source caps, whitespace, and its selected native-client build stage in 52 seconds. That stage builds Debug iOS and tvOS simulator clients. It does not run hosted browser journeys or physical-device tests. Default changed-commit gitleaks scanning reports no leaks.

## Failed measurement routes and limits

An initial Instruments launch by bundle ID did not present the app or generate fixture traffic. Its trace is excluded. A confirmed `simctl launch` followed by process attachment produced 62 Time Profiler rows and zero potential-hang rows in a short recording. Attachment missed much of startup. Neither count establishes zero hitches or a cold CPU improvement.

A private TV `XCTApplicationLaunchMetric(waitUntilResponsive: true)` probe compiled and passed. Its exported metrics were `[]`. This is missing evidence, not zero launch time. See [Apple's metric documentation](https://developer.apple.com/documentation/xctest/xctapplicationlaunchmetric) for the intended measurement surface. Earlier hitch probes also emitted no usable hitch metric; see [their record](tvos-hitch-metric-2026-09-30.md).

The first phone landscape attachment captured a transition. A stricter probe then incorrectly assumed screenshot pixel dimensions must rotate; its expectation timed out while the accessibility window reported 874 × 402 points. A follow-up draft used an unavailable `XCUIDevice.screenshot` member and failed compilation. These probe failures are retained. The corrected capture uses the SDK's `XCUIScreen` surface and waits for landscape window geometry. It passes, and its inspected full-screen attachment shows the settled 874 × 402-point landscape window. Returning to portrait also preserves the populated hero and continuation content.

The physical Apple TV was offline at the current read-only inventory. The paired phone was reachable but had no Kinosail process. No physical app installation or launch was performed. Device input-to-frame latency, hitches, memory pressure, energy, production networking, iPad behavior, and playback interference remain unmeasured in this experiment.

## Reproduction and retained private artifacts

The owned test devices are recorded in `.verification/native-home-start/initial-manifest.json` and `phone-environment.json`. These contain environment identities and should remain private. Fixtures contain no production account or media data.

The phase directory retains the fixture, exact app bundles, original source, launch drivers, every response event, screenshots, command manifests, logs, and result bundles. It also retains excluded probes rather than silently replacing them with successful runs.

```sh
# From the task checkout, with its recorded owned simulators booted and seeded:
python3 .verification/native-home-start/measure-launches.py
python3 .verification/native-home-start/measure-ios-matched-launches.py
```

The TV driver initially archives the installed candidate and refuses to overwrite it. For a repeat, preserve the previous archive and results first. The iOS driver uses its already bound bundles. Start the recorded fixture with the bundled Python runtime that provides Pillow, and wait for `HOME_FIXTURE_READY` before launching either driver.

A public regression can be repeated with the synthetic seeded session:

```sh
xcodebuild -project apps/player/apps/native/Kinosail.xcodeproj \
  -scheme Kinosail-tvOS-Remote -configuration Release \
  -destination 'id=<owned-tv-simulator>' \
  -derivedDataPath '<task-derived-data>' \
  -resultBundlePath '<new-result-bundle>.xcresult' \
  -parallel-testing-enabled NO -collect-test-diagnostics never \
  -only-testing:Kinosail-tvOSRemoteUITests/RemoteHomeScrollingTests \
  CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- \
  ENABLE_TESTABILITY=YES ONLY_ACTIVE_ARCH=YES test
```

Retained manifests bind the actual revision, command, fixture, environment, and result. Required hosted checks, protected-main ancestry, native distribution, deployment, and physical-device proof remain separate delivery facts. The broader performance goal remains active.

## Protected-main delivery

[PR #400](https://github.com/Kinosail/kinosail/pull/400) merged with a merge commit at `162f7c88766646ad71743105168abbd5a85d2c23`. Its selected [hosted run](https://github.com/Kinosail/kinosail/actions/runs/36814935883) passed repository, security, and native Swift compilation checks. Unrelated Go, Android, browser, and container-publication jobs were skipped. Fetched ancestry confirms source commit `151c00aa7` is included in main. Native distribution, deployment, and physical-device timing remain unverified here.
