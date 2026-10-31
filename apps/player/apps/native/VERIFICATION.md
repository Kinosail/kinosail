# Swift Apple verification

## Apple follow-up review and deep CI repairs — 2026-10-29

A follow-up review of iPhone, iPad, Apple TV and Apple Watch screens against `DESIGN.md` found these defects. The weekly deep run also had two Apple failures. Each fix has a check that failed first.

| Surface and state | Defect | Covering check |
| --- | --- | --- |
| Episode cards, heroes, resume rows, iPhone player header | An episode titled only with the show name read "Example Show / Example Show · S2 E1". It now reads "Episode 1", as the Apple TV app does. | `ScreenFollowUpTests/testEpisodeCardsNeverRepeatTheShowName`, `ShowSeasonSelectionTests.episodeTitlesNeverRepeatTheShowName` |
| iPhone and iPad details | All episodes and Remove from Continue watching had no icons. Genres used the system secondary color, not the muted role. | `ScreenFollowUpTests/testDetailLinksShowIconsAndGenresUseMutedText` |
| Settings, About | Privacy policy had no icon, so its text did not line up with the other rows. | `ScreenFollowUpTests/testPrivacyPolicyRowAlignsWithOtherAboutRows` |
| Comic reader | Disabled Previous was as bright as enabled Next. | `ScreenFollowUpTests/testDisabledReaderPageButtonLooksDimmerThanEnabled` |
| Apple TV Library hub and Collections | Used `star` and `rectangle.stack` instead of the iOS symbols. The hub has no route on Apple TV today, so no UI test reaches it. | Source review |
| Apple Watch Remote and Heart pages | Supporting text used the neutral system gray `(144,146,148)` instead of the sage muted role. Connecting to iPhone was centered while every other state is leading. | Screenshot pixel sample: `(160,167,156)` = `#A0A79C` after the fix |
| Deep CI, Swift code scanning | Under the CodeQL tracer, `swift-plugin-server` failed with "Bad CPU type", so `@Observable` and `@State` did not expand in the Watch target. The scanning xcconfig now puts each device platform plugin folder first, so macros load in the compiler process. | `scripts/ci/test_security_contracts.py`; deep run `40826859256` Swift code scanning passed |
| Deep CI, Player native contracts | The runner picks an iPhone Duo simulator. Its window control shares rows and fill with the reader pending lines, so the column check measured 63 to 76 points too wide. The check now measures the longest unbroken run per row and ignores groups narrower than a pending line. | `NativePreferencesUXTests` on iPhone Duo: 4 failures before, 8 of 8 after; deep run `40826859256` Player native contracts passed |

**Revision.** Base `617f1f4a` (main). New: the task commits on `mikeo/qa-apple-polish-2`, head `cb08d14c` for the runs below. Xcode 27.1, iOS 27.1, tvOS and watchOS 27 simulators: iPhone 18 Pro, iPad, a new iPhone Duo, Apple TV and a paired watch.

**Test data.** Two isolated copies of `scripts/test-instance.sh` (projects `kinosail-qa-apple-populated` and `kinosail-qa-apple-empty`), each behind `scripts/download-fixture/fault_proxy.py`. The populated copy adds five generated movies with the Drama genre, a long-title movie, `Long Haul (2024).mp4` (SHA-256 `777ce969ebd32f1a05ba3d4087d89da6562ded4cdfccf8dc83b9b6958b6d58e0`) and `Example Show S02E01 Second Season Opener.mp4`, which the Server titles "Example Show". The empty copy has no media. The playback suite used `scripts/ios-playback-fixture.py` with a generated 60-second MP4 (`c77ce1f4fd76a84947ec2993f9ad996054919bf5c2451b82680778d26ea10d7d`). Apple TV runs used `run-tvos-navigation-qa.py` with the 30-second MP4 from `TV-NAVIGATION-QA.md` (`71771b8981828d2663d8ecf3d2b69f6ae8b6492c6f45a90cc855799e0c9323ea`) and `KINOSAIL_TV_PLAYBACK_EXTRAS_FIXTURE=1`.

| Run | Result |
| --- | --- |
| `Kinosail-iOS` unit tests, iPhone 18 Pro | 356 tests in 78 suites passed |
| `Kinosail-tvOS` unit tests, Apple TV | 341 tests in 78 suites passed |
| `NativePreferencesUXTests`, iPhone Duo, before the reader fix | 4 reader column failures |
| `NativePreferencesUXTests`, iPhone Duo and iPhone 18 Pro, after | 8 of 8 passed on each |
| `Kinosail-iOS-Touch`, iPhone 18 Pro | 20 passed, 5 skipped (Other Servers, Plex and artwork outage need their own fixtures) |
| `TouchPlaybackInteractionTests`, iPhone 18 Pro | 6 passed, Picture in Picture skipped |
| `Kinosail-iOS-Touch`, iPad | 17 passed, 5 skipped; `EmptyLibraryTests` and the download outage journey passed on a rerun after a fixture setup error |
| `IPadWindowTests`, iPad | Portrait narrow windows and the landscape tour passed. iPadOS gave a 375-point window for each 507, 375 and 320 point request. |
| Apple TV Home empty, pending and failed states | 3 of 3 passed |
| Apple TV loaded Remote UI classes, groups one and two | 15 and 10 passed |
| Apple TV loaded group three: playback options, crash, layout polish, playback extras | 10 of 10 passed with the 30-second media. Playback extras fails with 12-second media on both main and this branch, because the credits marker starts after the video ends. |
| CI contract tests, `python3 -m unittest discover -s scripts/ci` | 168 passed; the new scanning contract failed first |
| `make max-loc` and `make -C apps/player verify-changed` | Passed: max-loc, diff-check and native-client builds for iOS and tvOS |

Also checked with no defect found: the empty library on iPhone, iPad and Apple TV shows a designed empty state, and no placeholder stays after loading. The Apple Watch Remote page has the black background with the Electric glow, the rounded title and the player picker. iPad landscape uses the sidebar.

Not run: the Heart page with real readings (it needs a Health grant on a device), Apple Watch controls below the first screen (no watch UI automation), the CodeQL tracer locally, physical devices, and the Nox Server. Watch screenshots and the first iPhone and Apple TV runs were lost when the session scratch folder was cleared. The results above are from the reruns.

| File | SHA-256 |
| --- | --- |
| iPhone Duo reader baseline log | `433374e05abfb7f2ca0b0149dbd8adc6bb2ec803d900a33410797fb23e6eabbf` |
| iPhone Duo reader log after | `d92a044e0be36cacc5f0e3b66208eb4bdaf3b29bfdafc58d86847b1f32401cb5` |
| iPhone reader log after | `68e0ffcb582383a832034cf875832039d55f5c8e6cd91176f85183f233adacc5` |
| iPhone unit log | `add37bc5a975e8c7d6f4965f7aebdf55a20c0f4c2625d230c4ae803789bc7e17` |
| Apple TV unit log | `bc7410cfe9f98765816f6bcd95aeacc32c142afd31d329b2fed491bae8914c88` |
| iPhone touch log | `c6c687f71e2714876e94afc7838fe37f91591b45530e29f70d9ac287d15e634c` |
| iPhone playback log | `52380e5a8f34453fe104ef4f97e4b7cbdcee6d99dcc54f2c447e14305aa14b6f` |
| iPad touch log | `54ba60559b31442f731f9948366f2b0be66611c8abea548020f8c630991944bc` |
| iPad rerun log | `d83ca1e2413d86bb7f74c5a9eb9ae46d45f4b08f8716bcc87a465e9bc733e01b` |
| Apple TV extras on main, 12-second media | `d79699108256bccdbff883d0903c7766762d8e1869b766d3163c66e76de09434` |
| Apple TV extras, 30-second media | `58e48a240ca6dee186e3618abc7063b24524b9ac71eb116ccf3b3b06b3a761f4` |
| Apple TV loaded group one | `23b6d2c2fbc52a97af7c7e53c483ab582e0c52f7d72d2dc2dd426bbf324775a4` |
| Apple TV loaded group two | `59a26fe88a64284d9b7c770ffd96f17bd02af8f7e72d974de2b5df6672248fd4` |
| Apple TV loaded group three | `398f17c91c72a464dbbfa06cbe86eb626f71e78936bd9a915c14d771b0a95b59` |
| iPhone empty Home screenshot | `f192fb5721653ccd7656eb0f2f3b373ec4a5f609d6e05d702f32cba8a36181f0` |
| iPhone episode cards screenshot | `78378314662efe237b2357e48f0a83c3261fc460b249c1a83ffd5e15afe90f63` |
| iPad empty Home screenshot | `1c71dbf7379052f7a9f64e7b99856a6dadabfb0820f31876d20b1c7196e5e9c2` |
| iPad landscape Home screenshot | `fb3a8853a7597be5cfd695d92c3b32b7756a905009ad44d9f506b19d2cac487b` |

## iPhone and iPad screen review — 2026-10-28

A review of every iPhone and iPad screen against `DESIGN.md` found these defects. Each fix has a test that failed on the baseline and passes now.

| Surface and state | Defect | Covering test |
| --- | --- | --- |
| Scrub preview, two Servers | The frame cache was keyed only by the preview template, so another Server's frame could show for the same template. Two frame tests also failed on every run because cached frames leaked between tests. | `PlaybackFramePreviewTests` (unit): `doesNotReuseFramesFromAnotherServer`, `loadsAuthenticatedFrame`, `rejectsInvalidFrameData` |
| Connection banner, iPhone and iPad | The banner covered the navigation bar and the top of each screen, including the screen title and the library filters. | `DownloadJourneyTests/testConnectionBannerKeepsScreenTitleVisible` |
| Connection banner, largest text | The banner filled the whole iPhone screen and pushed the screen off the bottom. It now scrolls inside a fixed height. | `DownloadJourneyTests/testConnectionBannerLeavesRoomAtLargestTextSize` |
| Ebooks & comics, largest text | The fixed-width category menu was wider than the screen, so the filters, count and grid were cut off on both sides. | `ScreenPolishTests/testLibraryFiltersStayOnScreenAtLargestTextSize` |
| Ebook reader, largest text | Long words in the chapter heading ran past the right edge. The reader now breaks long words. | `ScreenPolishTests/testEbookTextWrapsInsideScreenAtLargestTextSize` |
| Library hub | My List and Collections used different symbols from More and the tabs. | `ScreenPolishTests/testLibraryUsesTheSameSymbolsAsMore` |
| Customize tabs | Reset to default tabs was available when the default tabs were already in use. | `ScreenPolishTests/testResetTabsIsAvailableOnlyAfterTabsChange` |
| Playback options and other sheets | Done was at the leading edge, while Audio & subtitles and Playback speed put it at the trailing edge. Done now uses the trailing confirmation position in every sheet. | `ScreenPolishTests/testPlaybackOptionsDoneSitsWithOtherSheetConfirmations` |
| Consistency tour | The tour tapped the first navigation bar button, which is not always Back, and skipped Movies when the tab bar was collapsed. | `ScreenTourTests/testCaptureConsistencyTour` |

**Revision.** Baseline: `365c4bc3` (main). New: the task commits on `mikeo/qa-ios-polish`. Xcode 27.1. iPhone 18 Pro and iPad simulators with the iOS 27 runtime.

**Test data.** A local Kinosail Server built from this checkout, with media from `scripts/generate-test-media.sh` plus generated movies (one with a very long title) and a second season of Example Show. A local static TMDB fixture supplied metadata. The app reached the Server through `scripts/download-fixture/fault_proxy.py` on `127.0.0.1:38492` (control `38493`), which can drop connections. The playback suite used `scripts/ios-playback-fixture.py` with a generated 60-second MP4 (SHA-256 `849526cd78903b5c1521930f792a3b502df55a0503c307a3333b93bee20a19f0`).

**Commands.**

1. `xcodebuild -scheme Kinosail-iOS test` on the iPhone simulator: 355 tests in 78 suites passed.
2. `xcodebuild -scheme Kinosail-iOS-Touch test` with `-only-testing` for `ScreenPolishTests`, the two banner tests and `ScreenTourTests/testCaptureConsistencyTour`. Set `TEST_RUNNER_KINOSAIL_DESIGN_TOUR_SERVER` and `TEST_RUNNER_KINOSAIL_DOWNLOAD_SERVER` to the proxy, `TEST_RUNNER_KINOSAIL_DOWNLOAD_CONTROL` to the proxy control port, and both token variables to a test owner session. Run on iPhone and iPad.
3. `xcodebuild -scheme Kinosail-iOS-Touch -only-testing:Kinosail-iOSTouchUITests/TouchPlaybackInteractionTests test` with the playback fixture, after signing out: 6 of 6 passed on iPhone.

The largest-text tests pass `-UIPreferredContentSizeCategoryName UICTContentSizeCategoryAccessibilityXXXL` at launch, so the simulator setting does not matter.

| Run | Baseline | New |
| --- | --- | --- |
| Frame preview tests | Failed with 4 issues | 4 passed |
| Banner over title, iPhone | Banner `y 75–198` overlaps navigation bar `y 62–168` | Passed on iPhone and iPad |
| Banner at largest text | Navigation bar pushed below the screen | Passed on iPhone and iPad |
| Library filters at largest text | Filter frame `x −35 to 437` on a 402-point screen | Passed on iPhone and iPad |
| Ebook at largest text | Heading frame ends at `x 440` | Passed on iPhone and iPad |
| Library hub symbols | `star` and `rectangle.stack` | Passed |
| Reset to default tabs | Enabled with default tabs | Passed |
| Playback options Done | Done centered at `x 58` of 402 | Passed |

Also checked with no defect found: pending and loaded Home, Movies, Shows, show, episode and movie details, Search (empty, results, no matches), More, Library, My List (empty), Collections, Music, Audiobooks, Books, Photos, Settings and its sub-screens, the player with its sheets, and the comic and EPUB readers, at the default and largest text sizes. With the Server unreachable, the banner offers Try again and Downloads, Settings still offers Change Server, and Try again clears the banner when the Server returns. The app is dark only, so there is no light mode to check.

Not run: Apple Watch screens (watchOS build only), iPad Split View and Slide Over widths, iPad landscape tour, Picture in Picture (not available in the simulator), VoiceOver with a screen reader, physical iPhone or iPad, and the Nox Server.

| File | SHA-256 |
| --- | --- |
| banner baseline log | `3b3fd72b8e3848538a5ba36b15e4164287ca24179a64f4f5f7012015dd3dd5a6` |
| banner baseline screenshot | `fc3dc43c90044f9c53e294a90faffff6cf78a4f6d989d09608c94c20266f6602` |
| banner at largest text, baseline log | `c83666ede8a3378248412b77f58611ad7f7b6aed9c001621c7e82abd39c0d82d` |
| polish tests baseline log | `8520b96383368b5a133e3147ceb96428084f785a3b3e0919f60612a54f87e2ec` |
| library filters baseline log | `28686e6c97e65aa676f0948eb9d26a5f32d2f1a77b4f4ed11f21c82b3418b25e` |
| ebook and symbols baseline log | `3ace0d915b26e4731c9c13bb59d55f9bfa1edc5423d37ffce0f11ed5a04befd0` |
| iPad new log | `ae576cd82897a4dde4c0270a4c77de3ac5f968602314e57d1e02f7cd4b0e6e2f` |
| iPad ebook at largest text | `da14a6805aef30307030a2fcaf7447f9be29ad1245b249136e4dba97a3c61db9` |
| iPhone playback suite log | `1bedd658beeb72058aeaeabd147ba810c41f842a4d42580521159cad8e3789d1` |
| iPhone unit suite log | `d5f8e391d724d86b06858c54536ddede8e03e087c56e8e56437be32fff7242cf` |
| iPhone new log, part 1 | `97b0b9b8f3dee70e619c985de52788f2393035e74c8b8685ff4ca59814d7c7fe` |
| iPhone new log, part 2 | `738fb62b5eba52a86812f41e89e78b98fc645a000772d7f031228d6f6e21cfd3` |

## Apple TV focus, layout and loading-state pass — 2026-10-28

A remote-only walk of every Apple TV screen against the navigation fixture found these defects. Each one has a failing UI test first, then the fix.

- Home opened pending or empty with nothing focused. `LaunchReveal` started the content at opacity 0, which the focus engine treats as unfocusable, and it never looked again. The reveal now starts at 0.02.
- The Continue watching rail jumped up or down when focus moved between a long and a short spotlight title. The spotlight title line now has a fixed height.
- Shelves under the fixed top bar ended in a hard cut line. The Home scroll area now fades out over 32 pt at its top edge.
- The Home skeleton rails stopped at the content edge, while loaded rails run past the screen edge. The skeleton rails are no longer clipped.
- Show pages: season pills started 24 pt left of the Episodes heading and had no visible focus. They now use the Apple TV capsule (white when focused) and align with the heading. The Episodes shelf was cut at the safe area; it now runs to the bottom of the screen.
- Browse tiles had almost no visible focus. A focused tile now shows a 4 pt off-white outline.
- Settings explanations were list rows. They are now muted section footers.
- The Now playing options button was a tiny icon that VoiceOver read as "More". The top bar now shows a text **Options** button.
- `RemoteMovementTests/testPlaybackMenuReturnsToMovieDetail` failed on baseline because the movie page stays in the tree under the player. It now checks that the player closes.

**Revision.** Baseline: `365c4bc3` (main). New: the task commit on `mikeo/qa-tvos-polish`. Xcode 27.1, tvOS 26 simulator **Kinosail QA TV** (Apple TV 4K, 3rd generation, 4K output).

**Test data.** `scripts/tvos-navigation-fixture.py` with a generated 30-second MP4 (SHA-256 `71771b8981828d2663d8ecf3d2b69f6ae8b6492c6f45a90cc855799e0c9323ea`). With `KINOSAIL_TV_PLAYBACK_EXTRAS_FIXTURE=1`, the four fixture episodes also get chapters, an intro marker (2–12 s), a credits marker (24–30 s), an English WebVTT track and a next episode. Without it, episode playback is unchanged (`scripts/test-tvos-navigation-fixture.py`, 9 tests).

**Commands.** `scripts/run-tvos-navigation-qa.py --device <QA TV> --media <mp4> --evidence <dir> --mode <mode>` for `loaded`, `pending`, `empty` and `failed`. For `loaded`, export `KINOSAIL_TV_PLAYBACK_EXTRAS_FIXTURE=1` and `TEST_RUNNER_KINOSAIL_TV_PLAYBACK_EXTRAS_FIXTURE=1`. The loaded list now includes `RemoteLayoutPolishTests` and `RemotePlaybackExtrasTests`. Unit tests: `xcodebuild -scheme Kinosail-tvOS test` on the same simulator, without `TVQualitySessionSeed`.

| Check | Baseline `365c4bc3` | New |
| --- | --- | --- |
| Loaded suite (`RemoteQualityBaselineTests` … `RemotePlaybackCrashTests`) | 26 passed, 1 failed (stale playback check) | 35 passed (with the two new classes) |
| Pending Home: Search focused, skeleton covers where loaded cards land | Fails (no focus) | Passes |
| Empty Home and failed Home | Empty fails (no focus); failed not run | Both pass |
| Rail stays still across long and short spotlight titles | Fails | Passes |
| Top-bar edge: 10%→90% brightness ramp at least 16 pt | Fails (0 pt) | Passes |
| Season pills align with Episodes, visible focus, Episodes shelf to the bottom | Fails | Passes |
| Browse focus outline; Settings footers; Options button legible and labeled | Fails | Passes |
| Skip Intro, Next Episode, captions and chapters on an episode | Not covered | Passes: Skip Intro shows at the bottom-trailing corner, Select skips, Next Episode shows at the credits and starts S1 E2 with its own Skip Intro; Subtitles Off → English; Chapters lists 3 |
| Unit tests (`Kinosail-tvOS`, includes `TopShelfTests`) | — | 334 passed, 2 failed, 7 skipped. `PlaybackFramePreviewTests` (`loadsAuthenticatedFrame`, `rejectsInvalidFrameData`) fail on this simulator; this change does not touch trickplay. |

AVKit contextual actions (Skip Intro, Next Episode) are not in the accessibility tree while the transport bar is hidden, and accessibility queries during playback are slow. The playback test therefore finds the prompt by screen pixels and uses only screen captures while video plays. The Settings session note under Sign out is visible but is not exposed to the accessibility tree, so the test checks it by pixels.

Not run: physical Apple TV, a 1080p simulator (the layout is in points and matches 1080p), Top Shelf on the system home screen (only `TopShelfTests`), VoiceOver, the Nox Server.

| File | SHA-256 |
| --- | --- |
| loaded `remote.log` | `c44ef814bf1e99eb4b0c4fc52f85572167c9f4072585c665faf7c831b559952e` |
| loaded `fixture.log` | `fc5405816c9caf31a1f8b01ca53fc01e54eb880025e4546e4e9b2cef5597f81d` |
| album focus rerun `remote.log` | `c558672d6556e394b7a16b44fc76af61aebc504c9738cde5f35ac229ebf2cc77` |
| pending `remote.log` | `05aee1fef2876fa99f058262429d8417c854aea32893db8b60cd25b931879e7e` |
| empty `remote.log` | `25687905b735733d68930139f61d27e576b02c46d7fab84f8e31ac8c3efb623e` |
| failed `remote.log` | `b48221b2b7d4534514fb3034e71fa0197b23fcea42d6058220176f05a98f089a` |
| unit test log | `b1b1ffdfec907bc4925a5f134956f436fd2c630fdd37a0967a52eb88b38f241d` |
| top-bar fade screenshot | `6fadb8162fdc264fcb4e9b665ac0ea5ecca2f635df54fa3d077c034bece44efa` |
| pending Home screenshot | `933e504f0728df75d6885d423fce7b149876c2b56d37c02e31c2e28977befd27` |
| Skip Intro screenshot | `55f931b948e4b15890be18ee3162b0b3f615212a8a169fdbece6af484a2e44c6` |
| Next Episode screenshot | `9b709a587d56fb030fdb252157fd986ecb2e3192b942bb6be05328a3760a1692` |

## Posters return after a failed load — 2026-10-26

A poster that failed to load stayed a film icon until the app went to the background. The card retried four times in about 32 seconds, then stopped. It retried again only when the app returned to the foreground or the Server came back after being marked unreachable. A 404, a loader cancellation, or a timeout while other requests succeeded left the icon on screen. A Server log showed `/art/{id}` 404s from the iPhone app while the poster files were present. Now a card keeps retrying every 30 seconds while it is on screen. A loader cancellation that the card did not cause also counts as a failure to retry. The Android app had the same four-retry limit and gets the same change.

**Revision.** Baseline: `aa9928cc` (main). New: the task commit on `mikeo/artwork-recovery`. Xcode 27.1.

**Test data.** `scripts/tvos-navigation-fixture.py` (160 generated titles) with a generated 30-second `testsrc2` MP4 (SHA-256 `aa51ca917d5dc0787a01f7e9e1613b52b23693ffbf3ded60b4ee2154565a6aa2`). The new state `target=artwork&mode=failed&delay=60` returns 404 for every `/art/` poster for 60 seconds. Backdrops still load. `Tests/TVQualitySessionSeed.swift` now also seeds iPhone simulators.

**Commands.** On a new iPhone 18 Pro Max simulator (iOS 27.0):

1. Start the fixture with the MP4.
2. Seed the session: `xcodebuild -scheme Kinosail-iOS -only-testing:Kinosail-iOSTests/TVQualitySessionSeed test`, with `TEST_RUNNER_KINOSAIL_TV_QUALITY_SEED=1`, `TEST_RUNNER_KINOSAIL_TV_QUALITY_FIXTURE_MODE=loaded` and `TEST_RUNNER_KINOSAIL_TV_QUALITY_FIXTURE_ID=tv-polish-qa-ios-artwork`.
3. Before each run, stop the app and delete `Library/Caches/KinosailMediaCache` and `Library/Caches/com.kinosail.player` in its data container. A cached poster hides the outage.
4. Run `xcodebuild -scheme Kinosail-iOS-Touch -only-testing:Kinosail-iOSTouchUITests/ArtworkRecoveryTouchTests test` with `TEST_RUNNER_KINOSAIL_TV_FIXTURE_QA=1` and the same fixture ID.

The test opens Movies during the outage and samples the **After the Rain** poster.

| Check | Baseline `aa9928cc` | New (two runs) |
| --- | --- | --- |
| Poster at 55 s, during the outage | Film icon (0% poster pixels) | Film icon (0%) |
| Poster after the outage ends at 60 s | Still a film icon at 131 s (0%) | Poster at 73 s and 75 s (55%) |
| Other posters on screen | Film icons | Posters, except the fixture title with no image |

Android: `./gradlew :app:testDebugUnitTest --tests com.kinosail.player.core.ArtworkRecoveryTest` (Robolectric, a local HTTP fixture and virtual time). Baseline: 5 poster requests, then no more, and no poster. New: a sixth request after 30 seconds loads the poster.

Apple TV uses the same Swift artwork view. In the tvOS fixture a 500 outage did not reproduce the failure, because the library refresh marked the Server reachable again and the cards retried. The 404 outage was not run on Apple TV. Not run: physical iPhone, Apple TV or Android devices, and the Nox Server.

| File | SHA-256 |
| --- | --- |
| baseline iPhone log | `37492e3c30a54120bef64b3f3e46a1fc6923ed6c876c111669f12cb8bff78129` |
| new iPhone log, run 1 | `7a7fa427b6044311d1acc3ca43ea929b4a1e395b3da445e4f01dd3964b87622b` |
| new iPhone log, run 2 | `966167bff3a45616c58b29d0d334170a41196774d7e7de0a4b08b9e14968c7b6` |
| baseline after-outage screenshot | `55152a83f0a7a22fbe89c594cefdd6e22032bdc96b5432403c7fec32718b372f` |
| new after-outage screenshot | `efc5294ad1e3c9b65c4ce7c7091e62da23de6c35e38df66aa3535c1ba5c97024` |
| Android recovered capture | `d41ea566d266bfaf50cccb59c4058cb34cb2dda544d1d8f78f1abd4495e7fdda` |

## Apple TV player controls and title preferences — 2026-10-25

The Apple TV transport bar no longer has a separate **Seek with preview** button; scrubbing uses the system scrub bar. **This title’s preferences** now shows each value on its own row, saves changes automatically, drops the Downloads note (Apple TV has no downloads), and shows **Use profile defaults** only when the title has its own preferences. The status sentence is a footer, not a focusable row.

**Revision.** Baseline: `b321cea0` (main). New: the task commit on `mikeo/player-controls-audit`. Xcode 27.1.

**Test data.** `scripts/tvos-navigation-fixture.py` with a generated 30-second `testsrc2` MP4 (SHA-256 `0754f62c3f5e9fd12d7f007fe6d8186d6699bb27377fa233fd087fc50e7dce14`). The fixture now stores per-title preferences for `PUT` and clears them for `DELETE`. Invalid bodies return 400 and change nothing (`scripts/test-tvos-navigation-fixture.py`).

**Command.** The `run-tvos-navigation-qa.py` flow, limited to `RemotePlaybackOptionsTests`, `RemoteDetailMovementTests/testSettingsPlaybackBackRestoresRowAndTopBarFocus` and `RemotePlaybackCrashTests`, on an erased Apple TV 4K (3rd generation) (at 1080p) simulator.

| Check | Baseline `b321cea0` | New |
| --- | --- | --- |
| Transport bar has no Seek with preview | Fails | Passes |
| Speed, Audio language, Subtitles rows show their value | Fails (`Speed` only) | Passes (`Speed, 1×`) |
| No Save button, no Downloads note, no status row, no reset while following the profile | Fails | Passes |
| Choosing 1.5× saves with no Save button | — | Passes; fixture logged one save for `movie-004` at rate 1.5 |
| Use profile defaults appears, resets to 1× and disappears | — | Passes |
| Held remote input and repeated playback | Fails (stale check: the movie page stays in the tree under the player) | Passes (now checks the player closes) |

Not run: physical Apple TV, Nox Server. Direct-play files still show no scrub thumbnails on Apple TV; thumbnails need the Server's I-frame playlist, which only compatible streams have.

| File | SHA-256 |
| --- | --- |
| `remote.log` | `51e1a1b7edf0a6063c1b644ddfa78527bd79d788375503afb693070b696a3dfe` |
| `fixture.log` | `5dcf0b8efe6030e5387f0ab1374de1b729e3dcc09aedd2bfd8c4df93961abd25` |
| title playback preferences screenshot | `90950382f7623bd94827ddba564acfb46a017caf8abc7ab9c32bc55980b46d5b` |
| title preferences saved screenshot | `77c211c4c32e2dd1f6319dc0a7726f81d084fa1c34b2e2d7e80345829db5e8f2` |
| title preferences reset screenshot | `c04334a5a38d5c58d858c48c99ee64ed1884bbedd96dd1f013e2e0386993c77c` |

## Scrub preview speed — 2026-10-25

Scrub previews now keep up with remote presses and slider drags. Frames are cached in memory, the next three 10-second frames in the scrub direction load ahead (at most four loads at once), and the last frame stays on screen while the next one loads. A failed load still shows "Preview unavailable". Starting a new scrub clears the old frame.

**Revision.** Baseline: `6a241d39` (main). New: the task commit on `mikeo/scrub-preview-cache`. Xcode 27.1.

**Test data.** A local Player Server built from this repository, with no accounts, on `http://127.0.0.1:4283`. Media: a generated 20-minute `testsrc2` MKV (H.264, 2 s GOP, E-AC-3 5.1) with a burned-in timecode. Each preview FFmpeg call waited 0.8 s first, to match preview times measured on a slow library disk.

**Command.** A scratch session seed and a scratch Remote UI probe (not committed) on the Apple TV 4K (3rd generation) simulator. The probe opens the title, plays it, opens **Seek with preview** from the transport bar, waits for the first frame, then presses right 12 times, 0.5 s apart. 0.3 s after each press it checks whether the loading placeholder is on screen.

| Press | Baseline `6a241d39` | New |
| --- | --- | --- |
| Placeholder shown | 12 of 12 presses | 0 of 12 presses |
| "Preview unavailable" | Never | Never |
| Frame after the last press | — | Position 2:22 shows the 2:20 frame (burned-in timecode `00:02:20.000`) |

The iOS and tvOS apps build with `./scripts/build-apple.sh ios` and `./scripts/build-apple.sh tvos`. The iPhone and iPad slider uses the same loader; it was not driven by a UI test.

Not run: physical Apple TV or iPhone, Nox Server.

| File | SHA-256 |
| --- | --- |
| `before.log` | `51154938483ea8d51ea1fd7c0cb2b0186bf5987bbaefda6d60c2a3b7b121a050` |
| `after.log` | `393f508dc07df937b64ab4f9c66e99fcd757b0a92802cd0a7700c72786d17cdb` |

## Downloads — 2026-10-24

iOS download recovery and feedback: connection drops, Server outages, background transfer, retry countdowns, queue summary and local notifications. Everything below ran on a Mac with Xcode 27.1 (27A9275).

**Revision.** Baseline: `b9325249` (main before this change). New: the task commit on `mikeo/downloads`. The app and test sources in the final runs match that commit. Only documentation changed after them.

**Test data.** `scripts/download-fixture/up.sh` starts the local test Server on `https://127.0.0.1:38127`, generates `Long Haul (2024).mp4` (150 s, 8 Mbps H.264, 152,959,460 bytes, SHA-256 `a152043f2c7f90d7ef9f3a257d65ae422242b8abac70fe340e4bbeaed07ccd48`), scans it and starts `fault_proxy.py`. The proxy serves plain HTTP on `127.0.0.1:38300` and forwards to the Server over TLS. Its control port `127.0.0.1:38301` accepts `/mode/pass`, `/mode/drop` (closes live connections and refuses new ones) and `/rate/<bytes per second>` (one shared limit). `scripts/download-fixture/down.sh` stops both.

The fault proxy is an isolated test piece. It protects against a concrete failure that no other E2E test reaches: a download that stops, or never resumes, when the Server disappears mid-transfer. The simulator cannot cut its own network.

**Command.** After `xcrun simctl uninstall <device> com.kinosail.player`, with `TEST_RUNNER_KINOSAIL_DOWNLOAD_SERVER=http://127.0.0.1:38300`, `TEST_RUNNER_KINOSAIL_DOWNLOAD_CONTROL=http://127.0.0.1:38301` and `TEST_RUNNER_KINOSAIL_DOWNLOAD_TOKEN` set to the owner token from `up.sh`:

```bash
xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-iOS-Touch -destination 'platform=iOS Simulator,id=<device>' -parallel-testing-enabled NO -only-testing:Kinosail-iOSTouchUITests/DownloadJourneyTests test
```

`DownloadJourneyTests` limits the proxy to 2 MB/s, adds Long Haul (Original) to Downloads, drops the Server for 75 s, restores it, backgrounds the app for 30 s, backgrounds it again until the download finishes, and opens the ready download.

| Step | Baseline `b9325249`, iPhone | New, iPhone 18 Pro | New, iPad |
| --- | --- | --- | --- |
| Server dropped | Row kept showing "21 h 3 min left" | "Can’t reach your Server. Kinosail will try again automatically." with a countdown | Same |
| 75 s later | No change | Still waiting, no Resume button, no "interrupted" | Same |
| Server back | Transfer did not resume (proxy bytes unchanged) | Resumed by itself in seconds | Same |
| 30 s in background | 12.6 MB moved | More than 30 MB moved (the 2 MB/s limit) | Same |
| Finished in background | Never finished | "Ready to watch offline · Long Haul" notification | Same |
| Result | Failed (444 s) | Passed (238 s) | Passed (236 s) |

The iPad run first failed at sign-in because the simulator keychain held an old Plex account. `xcrun simctl keychain <device> reset` fixed it; the rerun above passed.

**Unit tests.** `xcodebuild -scheme Kinosail-iOS test`: 353 tests in 77 suites passed. `ResilienceTests.pausedExtentsLeaveRoomForAnotherPreparation` now expects 4 extent requests for 16 blocks, because extents are 4 blocks long.

**Logs.** The device log showed `operation=download-transfer outcome=complete bytes=152959460` and `usernotificationsd` accepted the request. Retries log `warning` with failure class, status code and attempt only. The download log has no title, URL or token.

**Checksums (SHA-256).**

| File | SHA-256 |
| --- | --- |
| `dl-final-iphone.log` | `e08ce28b33ea359956264073b20c627597fcbf5dff30e6434be7152b3187964e` |
| `dl-final-ipad3.log` | `1cca9536715759671d7270b82d446c851c72c212c8602ca55274dba3dd05dfb3` |
| `dl-baseline3.log` | `f8e5768061335a2e00e4a5cfd5bb4878ee7f64526e5869c24aac88680a85f286` |
| `unit2.log` | `75fc7c69edaec8c57417046cb2bc59dff3be918647131e4d9783bebe66624771` |
| iPhone `d3 server unreachable.png` | `fe0790df596f7d31005c2ac85f0f68744aa9c08da27e21ccd8ee827f9e7b0810` |
| iPhone `d5 resumed automatically.png` | `6a86a94e4ad51885a8293181ecd46125f0dbb2e3863a150a06cbbe763df175b1` |
| iPhone `d7 ready notification.png` | `1da1e712b0b122f7e3c24aa36fedf69895bd600f6ea05c000cbbf86f20debec2` |
| iPhone `d8 ready.png` | `909aabd5cddd57b02d00de79429ea8b03020237f0e2df6ff3c693091c5a44fdf` |
| iPad `d3 server unreachable.png` | `c6f07e75964d74d5be2f13069cd19f12751fcf07bb9eec681afaabad7240ffc8` |
| iPad `d7 ready notification.png` | `20e55354ea6137b8ae2dc76271208cfe5a0e7a1b5d8680841ae4a69fc9612b09` |

**Not verified.** Physical iPhone or iPad. Real offline and Wi-Fi only on cellular (the simulator cannot report them; the copy and the cellular button were not seen). Tapping the notification. App termination and relaunch mid-download. Low storage. Downloads from Jellyfin, Emby or Plex, which these servers do not offer yet. Android is unchanged.

## Plex servers — 2026-10-24

ADR 0010 second delivery: Plex sign-in, Plex servers in the combined library, playback, subtitles and progress. Everything below ran on a Mac with Xcode 27.1 (27A9275).

**Revision.** `490ca318`. The app sources in the iPad and Apple TV runs match that commit. Only `PlexJourneyTests` changed after those runs: it gained the From your Apple devices step and two waits.

**Test data.** `scripts/other-servers-fixture/up.sh <work directory>` now also starts Plex Media Server 1.43.5 on `127.0.0.1:32400`, pinned by image digest, and a plex.tv stand-in (`plex_tv.py`) on `127.0.0.1:8099`. The media now includes a sidecar English SRT file for Big Buck Bunny. The Plex server is not claimed. `ALLOWED_NETWORKS` lets the fixture reach it without a plex.tv account, and it still fetches Plex metadata, so it needs internet access.

The plex.tv stand-in is an isolated test piece. It protects against a concrete failure: without it, no test could exercise the plex.tv code flow, connection ranking or server choice, because the project has no Plex account. It answers `POST /api/v2/pins`, `GET /api/v2/pins/<id>`, `/api/v2/user` and `/api/v2/resources` in the plex.tv format. Its resource list puts an unreachable remote connection before the working local one, so the test proves that the app ranks and checks connections. The app reads its address from `KINOSAIL_PLEX_TV` only in Debug builds and only for `http://127.0.0.1:<port>`. Release builds always use `https://plex.tv`.

**Commands.** Each journey ran after `xcrun simctl uninstall <device> com.kinosail.player` and `xcrun simctl keychain <device> reset`, with the Jellyfin and Emby variables from the section below plus `TEST_RUNNER_KINOSAIL_SOURCE_PLEX=http://127.0.0.1:32400` and `TEST_RUNNER_KINOSAIL_PLEX_TV=http://127.0.0.1:8099`:

```bash
xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-iOS-Touch -destination 'platform=iOS Simulator,id=<device>' -parallel-testing-enabled NO -only-testing:Kinosail-iOSTouchUITests/PlexJourneyTests test
xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-tvOS-Remote -destination 'platform=tvOS Simulator,id=<device>' -parallel-testing-enabled NO -only-testing:Kinosail-tvOSRemoteUITests/RemoteOtherServersTests/testPlexCodeOnboarding test
```

| Journey | Device | Result |
| --- | --- | --- |
| `PlexJourneyTests`: Sign in with Plex, Continue with Plex web sheet, ready screen, Home, Movies, direct play of an MP4 with the sidecar subtitle chosen and shown, HEVC/AC-3 MKV played through a Plex remux, MPEG-4 AVI converted by Plex (the test reads `/transcode/sessions` on the server during both), the show page with 3 episodes, Jellyfin added through Add a server with Big Buck Bunny shown once, then sign out and add Plex again from From your Apple devices without signing in | iPhone 18 Pro simulator | Passed |
| `PlexJourneyTests`, without the last step | iPad simulator | Passed |
| `RemoteOtherServersTests.testPlexCodeOnboarding`: focus Sign in with Plex with the remote, code and QR code shown, code linked through the plex.tv stand-in, focus Start watching, reach Home | Apple TV 4K simulator | Passed |
| `OtherServersJourneyTests`: the Jellyfin and Emby journey, rerun for regressions | iPhone 18 Pro simulator | Passed |

**Playback start.** The app's `operation=playback-start outcome=ready` log measured 278 ms for the direct MP4, 327 ms for the HEVC remux and 463 ms for the converted AVI.

**Logs.** With the subsystem set to keep `info`, the iPhone run logged Plex sign-in, the share publish, the shared-token adopt, playback preparation and playback start with operation, outcome, kind and playback session. The log has no token, URL or user name. The setting was reset afterwards.

**Artifacts.** `.verification/other-servers/plex/` (git-ignored) holds the four result bundles and logs, 43 screenshots, `environment.txt`, `results.txt`, `playback-ready.txt`, `app-log-iphone.txt` and `checksums.txt` (SHA-256 `23e4743f800425b13a19f6efdc08e3324a50d0cb9505c40005a6c94f6b1fe489`).

**Not verified.** Real plex.tv sign-in, a claimed server, shared servers from another account, remote and relay connections, and Plex Home users. Physical iPhone, iPad and Apple TV. A real camera scan of the Apple TV QR code. Resume positions on Plex: the server keeps a position only after 60 seconds, and the test clips stop sooner. Plex subtitles embedded in MKV files are not shown yet (see ADR 0010).

## Jellyfin and Emby servers — 2026-10-24

ADR 0010 first delivery: the on-device device library, Jellyfin and Emby sources, the new setup flow, iCloud Keychain sharing, iPhone approval of a TV sign-in, and background library loading on the ready screen. Everything below ran on a Mac with Xcode 27.1 (27A9275).

**Revision.** `943366fb93db4f7d0b62ffea8a24cfc52595a6d2`. The next commit, `9b8151a2`, only stops Supporter badge reads for the device library; it was compiled (`Kinosail-iOS`, iOS Simulator) but the journeys were not rerun on it.

**Test data.** `scripts/other-servers-fixture/up.sh <work directory>` builds synthetic media with ffmpeg (three movies: H.264/AAC MP4, HEVC/AC-3 MKV with ASS subtitles, MPEG-4/MP3 AVI; one show with three episodes) and starts Jellyfin 10.11.11 and Emby 4.10.2.0 on `127.0.0.1:8096` and `127.0.0.1:8097`, pinned by image digest. The user is `kino` with password `kino-test-password`. The containers fetch titles and artwork from TheMovieDb, so they need internet access. The script waits until that metadata is present and the show lists its episodes. A fresh Jellyfin 10.11.11 lists no episodes until a second library refresh, so the script requests one.

The table below ran against containers set up by hand with the same steps and 30-second clips. After the script was added, `OtherServersJourneyTests` passed again on a fixture built from an empty directory by `up.sh`. That run uses 90-second clips and a test that taps Close player a second time when the first tap only shows the controls, because a 30-second clip could finish before the test closed the player.

**Commands.** Each journey ran after `xcrun simctl uninstall <device> com.kinosail.player` and `xcrun simctl keychain <device> reset`, with `TEST_RUNNER_KINOSAIL_SOURCE_JELLYFIN`, `…_EMBY`, `…_USER` and `…_PASSWORD` set to the fixture:

```bash
xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-iOS-Touch -destination 'platform=iOS Simulator,id=<device>' -parallel-testing-enabled NO -only-testing:Kinosail-iOSTouchUITests/<class> test
xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-tvOS-Remote -destination 'platform=tvOS Simulator,id=<device>' -parallel-testing-enabled NO -only-testing:Kinosail-tvOSRemoteUITests/RemoteOtherServersTests test
```

| Journey | Device | Result |
| --- | --- | --- |
| `OtherServersJourneyTests`: Jellyfin password sign-in, ready screen with real posters and counts, Home, Movies, direct play, converted play, show page, then Emby through Add a server (single profile goes straight to its password), Libraries in Settings, and one combined Movies row (Big Buck Bunny shows once) | iPhone 18 Pro simulator | Passed |
| `OtherServersHandoffTests`: sign in, open `kinosail://quickconnect?…` for a Quick Connect code started over the API, approve it, confirm the server reports it authenticated, sign out, add the server again from From your Apple devices without a password | iPhone 18 Pro simulator | Passed |
| `OtherServersHandoffTests` | iPad simulator | Passed |
| `OtherServersSpeedTests`: timed install-to-first-frame funnel | iPhone 18 Pro simulator | Passed |
| `RemoteOtherServersTests`: type the address with the remote, Quick Connect code and QR code shown, approve over the API, focus Start watching, reach Home | Apple TV 4K simulator | Passed |

**Funnel timings** (XCUITest clock, so each step includes about 0.5–1 s of test-harness waiting): launch to setup 6.03 s, Connect to sign-in 1.97 s, Sign in to ready 2.02 s, library preview 1.12 s, Start watching to Home titles 1.59 s (3.56 s before background loading), detail 2.2–2.3 s, Play to playing 1.75–1.80 s. The app's own `operation=playback-start outcome=ready` log measured 296–320 ms for direct play and 712–833 ms for converted play on the loopback fixture.

**Logs.** Route timings log at `debug`. Sign-in, share, adopt, Quick Connect approval and playback start log at `info` with the operation, outcome, source kind and playback session. No log line contains a token, password, URL or user name.

**Artifacts.** `.verification/other-servers/final/` (git-ignored) holds the result bundles, 32 screenshots, `environment.txt`, `results.txt`, `funnel.txt`, `playback-ready.txt` and `checksums.txt` (SHA-256 `2aea778730a6e0587131503518404d7bdd530129a6000342762f82cffde779a8`).

`make max-loc` passed.

**Not verified.** Physical iPhone, iPad and Apple TV. Real iCloud Keychain sync between two devices: the simulator stores the synchronizable item but does not sync it. A real camera scan of the TV QR code (the test opens the same link directly). Emby and Jellyfin servers on another host, over HTTPS, or with large libraries. Plex, the on-device engine and Kinosail Server in the combined library are not implemented.

## Library scan retry — 2026-10-17

This change was written in a Linux container without Xcode. **Nothing in this section was compiled or run here.** The Mac results are recorded below when they arrive.

- While the Server's first Library scan runs, Library reads return `503` with `Retry-After`. `BoundedHTTPResponse` now turns a `503` with a whole-second `Retry-After` into `ClientError.retryAfter(seconds)`, clamped to 1–60 seconds. A malformed value, an HTTP date, or another status keeps `ClientError.http(status)`.
- `ServerClient` does not use its quick 250 ms retries for that error. It records the Server as reachable and logs a warning with the request ID, the operation and `retry_after`.
- `ResourceView` (Home, details, shows, collections, music) and `LibraryScreen` keep the loading layout and show "Your Server is still getting ready. Kinosail will try again shortly." They read again after the Server's delay, with no tap needed. A view-scoped task does the waiting, so it stops when the screen goes away. Content that is already shown stays on screen without an error.
- Tests: `RetryAfterTests` (HTTP mapping and bounds) and `LibraryScanRetryJourneys`. The journeys host `LibraryScreen` and a `ResourceView` against the loopback `LibraryRecoveryFixture`, which answers two `503` + `Retry-After: 1` reads before the Library. They require exactly three reads, at least 0.9 s apart, with no extra reads afterward. They write snapshots to `apple-library-scan-retry-evidence`.

Not run here: `./scripts/build-apple.sh ios`, `./scripts/build-apple.sh tvos`, the `Kinosail-iOS` and `Kinosail-tvOS` test schemes, simulator and physical-device checks.

## Apple TV speed pass — 2026-10-16

This pass changed only native Swift sources, the build script and these notes. It was written in a Linux container without Xcode, so **nothing in this section was compiled, run or measured**. The claims below describe what the code does, not observed timings.

- Apple TV cards now decode posters at 400 pixels on 1080p output and 500 pixels on 4K output, and landscape cards at 800 pixels instead of 1600. The Apple TV decoded-image budget is 64 MiB with 160 entries; iPhone and iPad keep the previous 32 MiB and 96 entries. The retained artwork tests construct `ArtworkLoader()` with those defaults.
- `ArtworkMemoryIndex` mirrors the loader's decoded cache behind a `Mutex`. `Artwork` reads it synchronously, so a card that scrolls back into view shows its image on the first frame instead of after an actor hop. Prefetch requests now keep the raw artwork path so prefetched images are found by the same lookup.
- The full-bleed detail and show hero decodes at 1600 pixels on 1080p Apple TVs and still at 4096 pixels on 4K output.
- The launch cover now waits 120 ms instead of 250 ms before it can fade, and the fade lasts 0.3 s instead of 0.45 s.
- Top Shelf publishing waits 4 s after the app becomes active, runs at utility priority, requests artwork through the background lane and encodes JPEGs off the main thread. It still clears the snapshot before every rebuild, when sharing is disabled and on sign-out.
- Playback start no longer waits for the fresh progress baseline before installing the player. `ProgressWriter` receives the baseline request and applies the fresh value before its first write. Player readiness uses `AVPlayerItem.status` observation instead of a 100 ms poll. The show page prepares the featured episode, and Home prepares the focused Continue watching title after 400 ms of settled focus.
- `ServerClient` allows six connections per host instead of four so catalog requests are not queued behind four artwork fetches on HTTP/1.1.
- `scripts/build-apple.sh` accepts `KINOSAIL_BUILD_CONFIGURATION=Release`. The Debug default compiles with `-Onone`, which is a large share of Apple TV sluggishness when a Debug build is installed on hardware.

Checks run here: `make max-loc` and `git diff --check` pass. Not run: both simulator builds, the Swift test schemes, the tvOS Remote UI tests, populated simulator or physical Apple TV inspection, and any timing measurement. Run `./scripts/build-apple.sh tvos`, the `Kinosail-tvOS` test scheme and a populated Apple TV 4K and Apple TV HD session before treating this pass as verified.

## SDK 27 readiness follow-up — 2026-09-19

Toolchain: Xcode 27.0, build 27A266a. Deployment minimums remain iOS/tvOS 26. Source includes authenticated foreground Search library / Play title / Continue watching App Intents, opt-in dynamic Top Shelf, per-Apple-TV-user storage, scoped media navigation and the app-only UserDefaults privacy declaration. The existing native design guidance kept these additions in system navigation and progressive settings rather than adding a new app shell.

Evidence from this follow-up:

- Both simulator targets and both unsigned device targets build with SDK 27 using the four documented `build-apple.sh` commands. The iOS bundle contains metadata for all three App Intents. These are compilation and packaging results, not executed intent results.
- Release signing and physical-device installation evidence are intentionally kept outside this public repository. Simulator builds do not establish store distribution or physical-device readiness.
- iPhone 18 Pro / iOS 27 and Apple TV 4K / tvOS 27 simulators install and launch to the live setup screen, including nearby Server discovery. Runtime inspection caught and corrected a missing environment around the Top Shelf modifier and a Keychain query option rejected by the TV simulator. The final TV setup screenshot has no secure-storage alert.
- URL-opening requests reach the system confirmation dialog. Device Hub computer-control requests repeatedly time out, so confirmation, populated browsing, playback, Shortcuts execution and Top Shelf activation were not completed through the UI in this follow-up. The historical intermittent TV black screen has no new stable reproduction or established root cause; it is not marked fixed.
- `make max-loc` and `git diff --check` pass. Added Swift regression sources cover malformed, duplicate, unknown, oversized and cross-profile links, invalid intent input before restoration, and rejected shelf data before filesystem effects. **No test suites were run** because `.gates-disabled` remains in force.

Local build logs: `/tmp/kinosail-27-{ios-build,tv-build,ios-device,tv-device,ios-signed,tv-signed}.log`. Local screenshots: `/tmp/kinosail-{ios27,tv27}-current.png`; the `*-link-rejected.png` filenames contain the system confirmation dialog, not proof of app-level rejection.

Remaining acceptance work: sign in to the developer account in Xcode and provision the TV capabilities; approve Icon Composer's first-use license before producing the layered iOS icon; run the enabled suites only after explicit authorization; exercise real-user switching, Top Shelf play/detail and privacy changes, authenticated Shortcuts, real media/HDR, interruptions, PiP, downloads, VoiceOver/Siri Remote and App Store distribution. Build success does not make this a 100% readiness certificate. Existing icons remain unchanged pending the Icon Composer decision.

## Historical implementation evidence — 2026-09-12

## Scope and source

This work resumes the interrupted `finish-swift-apps` checkout. Its three existing commits implement sessions/catalog, protected playback/progress, and verified downloads/reading. The final batch completes receiver contracts and controls, captions, photos, artwork request coordination, session cleanup and the native blue/neutral design direction. The active `Sources` tree has no unimplemented service or platform operations and no preview gallery. The previous JavaScript client was retained as reference material during this implementation and removed in the subsequent repository cleanup.

Only native Apple files were changed. Existing server APIs, web apps and `.gates-disabled` were preserved. iPhone and iPad share the iOS target; tvOS is independent. The user subsequently authorized signed-device installation while leaving tests disabled; both Info plists now permit the configured deployment helper.

## Builds

Using installed Xcode 26.6 (17F113), these commands succeeded during this run:

- `./scripts/build-apple.sh ios`
- `./scripts/build-apple.sh tvos`
- `./scripts/build-apple.sh ios device`
- `./scripts/build-apple.sh tvos device`

All four initial builds succeeded at `08aa9a01` after reconciliation with main. Subsequent verification-note edits do not change the built production sources. Simulator builds use ad-hoc signing for Keychain. Those initial device builds were unsigned and were not installations. Logs are retained locally under `.verification/swift-finish/`. The App Intents metadata tool reports that extraction was skipped because there is no AppIntents dependency; this is not an application compile failure.

## Manual simulator observations

The disposable loopback fixture is outside production sources and uses synthetic credentials and media. Its responses do not prove compatibility with a deployed Server.

- iPhone 17 Pro, iOS 26.5: restored the fixture Keychain session, loaded populated Home/artwork, opened Resume and played a real MP4 through the authenticated transport. Native AVKit exposed full screen, volume, skip, pause, seeking and playback speed. Timed external captions appeared. Playback progress requests reached the fixture.
- iPad, iOS 26.5: entered a Server address, displayed the QR/six-digit pairing screen, completed fixture Quick Connect, loaded populated Home, opened title details and download options, selected the supplied audio track and queued a compatible download. Request evidence includes download identity, preferences, preparation, manifest and media-file retrieval. A wrong fixture track route was corrected to the authoritative `/download-tracks` route; production code already used the correct route.
- Apple TV, tvOS 26.5: installed/launched, displayed setup, discovered nearby Servers, and opened the system text-entry UI with the remote Return action. Inspection exposed dark text on the dark setup surface; the final batch supplies explicit TV foreground roles and a readable native toolbar heading.

The Mac locked during interaction, and Computer Use reported that automatic unlock was paused after physical input. No attempt was made to bypass the lock. Final UI interaction and populated TV playback were therefore not completed. Final headless screenshots at build revision `08aa9a01` show the blue/neutral populated iPhone Home in dark appearance, populated iPad Home in light appearance, and the corrected readable TV setup heading and labels. These are visual evidence only; they do not establish remote focus or final interactive behavior.

## Follow-up fixes and device delivery

The follow-up uses the installed Impeccable skill and its Apple native guidance. The shared blue/neutral palette is retained, with increased-contrast roles, a horizontal Home feature when enough width is available, larger native audio/reader controls, adaptable audio options, contextual recovery headings and dark photo presentation. The final audio transport uses separate track navigation to avoid fitting five large controls into a narrow row. These changes compile on both targets; the final responsive and accessibility interaction sweep is blocked by the locked Mac.

Manual iPad inspection found a completed, hash-verified download stuck at “File verified, but local playback failed.” AVFoundation could play the MP4 but could not infer its type from the private `.media.part` name. Verification and playback now use the same bounded file-header identification and forbid external asset references. Unknown formats, playlists and nonlocal/symlink paths remain rejected. Focused negative regression sources were added without running them.

With the repaired simulator build, Resume completed verification and exposed Play offline. The fixture server was then stopped, and Play offline visibly played the saved video through native AVKit. A screenshot is retained at `.verification/apple-device-finish/ipad-offline-playback.png`. This proves this saved-file path, not long-running background recovery or every original media format.

Both simulator build commands succeeded again after the audio adjustment and reconciliation with the new icon assets. Physical-device installation evidence is intentionally omitted from this public record; installation is not a claim that every physical-device flow passed.

## Verification boundaries

No test or quality suites were enabled or run, as explicitly requested. Added Swift Testing sources are not passing results.

After the user unlocked the Mac, the follow-up below completed the listed simulator flows. Remaining interactive checks include split-view layouts, increased contrast, PiP, session interruptions and long-running background download recovery. Real Server pairing and the Nox flows listed below are now verified. Physical media/receiver behavior, VoiceOver/Siri Remote, HDR/codec coverage, battery/performance and store distribution remain unverified. Test suites remain excluded until the user separately enables them.

Google Cast has no native sender SDK and is not offered. External captions do not render in system PiP. Offline book resources/artwork and playback audio effects are not claimed. See IMPLEMENTATION.md for retained feature limits and persistence contracts.

## Unlocked runtime follow-up

Both simulator builds succeeded after these final production fixes:

- Audio previous/next controls use accessible symbols so large Dynamic Type does not break their labels across lines. Empty Up next sections are omitted, and playback failures expose retry.
- Audio/video screens respect preparation already owned by the player coordinator, preventing duplicate requests when advancing the queue.
- TV navigation headings use readable dark-appearance colors through the supported legacy navigation-bar API. Detail artwork leaves room for title actions.
- TV video hides app navigation and fills the screen. Playback options are available as an item in the native AVKit transport menu.

Manual loopback-fixture observations: iPhone Home adapted to accessibility text sizes; audio playback, next-track transition and sleep-timer selection worked. iPad EPUB chapter navigation, sepia preferences, reading bookmarks, native PDF rendering/scrolling, comic paging and photo zoom worked. Populated TV pairing, catalog focus, title details, full-screen video, captions and native pause/resume worked. The AVKit options item was visible; activating that item was not established by the available simulator input controls. Screenshots are retained under `.verification/apple-runtime-finish/` in the main checkout.

The Mac was kept awake with temporary display/system idle assertions. iPhone Mirroring could not find the phone during this review; physical installation and launch are tracked separately by the deployment receipts. These observations do not replace real Server, VoiceOver, physical Siri Remote or receiver checks. Tests and quality suites remain disabled.

## Real Nox follow-up — 2026-09-12

Computer control completed ordinary passkey sign-in and Quick Connect approval against the real Nox Server. Its advertised HTTPS origin passed normal certificate validation. Both iPad and tvOS simulators restored the resulting sessions after installing updated builds and loaded the populated library.

The live checks exposed and fixed these production failures:

- Chaptered movie responses include chapter indexes. The native contract now accepts sequential bounded indexes and rejects malformed or conflicting values. The real movie played on iPad and TV; native pause/resume, forward seeking, captions and persisted progress were observed.
- Expired Quick Connect requests now report an expired/cancelled code with connection recovery, instead of a missing-title error. The old misleading error was reproduced; the new 404 mapping has regression source but was not separately exercised after expiry.
- Compatible download preparation incorrectly passed encoder-only profile options to FFmpeg video copying. The exact Nox command failed with exit 234; removing those options produced a decodable file. This necessary server dependency fix is in `packages/downloads/transcoding.go` and was deployed to Nox.
- Resuming a failed native preparation now repeats the idempotent preparation request before requeuing the same job. The previously failed Compatible sample completed after Resume, passed local verification and visibly played through Play offline. The Original sample also completed and played. Network isolation was not applied during this real-Nox check; the earlier fixture check separately established playback with its server stopped.
- TV Change Server now uses an opaque full-screen cover. The whole approval code, QR, instructions and cancellation action remained visible; cancellation returned to the existing Nox session.
- A real audio sample had a saved position of 4,771 seconds despite a 30-second file. Bounded stale progress now restarts at zero. The sample played, advanced and paused at four seconds, and the detail view reflected its new saved position. Invalid numeric encodings and excessive positions remain rejected.

Real Nox EPUB reading, photo rendering and photo zoom also worked on iPad. Final iOS and tvOS simulator builds succeeded at the recorded production revision. Physical-device installation and pairing evidence are intentionally omitted; simulator interaction remains a separate boundary.

Tests and quality suites remain disabled. Regression source additions are not passing test results, and these concrete live flows do not certify all media formats, background behavior, accessibility or release distribution. Private physical-device evidence is not tracked here.

## Launch sequence — 2026-10-31

Branch `mikeo/tvos-launch-zoom` at `2971a583`; baseline `main` at `084ce98d`. Xcode 27.1, tvOS 27.0 simulators created for this check (Apple TV 4K 3rd generation at 1080p and at 4K), loopback fixture from `scripts/tvos-navigation-fixture.py` with a generated six-second MP4, session seeded by `TVQualitySessionSeed`. Each Remote UI test ran under `xcrun simctl io recordVideo`.

- iOS: `./scripts/build-apple.sh ios` succeeded.
- tvOS 1080p, loaded: `testLaunchAndLibraryStartOnTopLeftTitle`, `testMovieDetailBackRestoresGridAndBrowseFocus`, `testTopBarAndBrowseReturnPaths` and `RemotePolishTests/testFocusedContinueWatchingCardsKeepTheirNeighborsClear` passed.
- tvOS 1080p, pending fixture (slow network): `RemoteHomeStateTests/testPendingHomeKeepsNavigationReachableAndRemovesSkeleton` passed. The mark breathes, then the drop lands on the shimmering placeholders.
- tvOS 1080p with `ReduceMotionEnabled`: the launch test passed; the reveal is the 0.25-second fade.
- tvOS 4K, loaded: the launch test failed at `e614f9cb` (launch focused Search because arriving rows started at opacity 0) and passed at `2971a583` (rows wait at 0.02).

Frame intervals while the picture changes, from the reveal start through the next two seconds (simulator rendering on the Mac, not Apple TV hardware):

| Run | Mean | Max | Over 20 ms |
| --- | --- | --- | --- |
| Before, 1080p | 17.4 ms | 30 ms | 6 |
| After, 1080p | 17.6 ms | 30 ms | 5 |
| Before, 4K | 45.5 ms | 303 ms | 7 |
| After, 4K | 31.3 ms | 167 ms | 4 |

Not verified here: frame time on Apple TV HD (A8) and Apple TV 4K hardware. That needs a physical-device check.

| Clip (720p export of the reveal window) | SHA-256 |
| --- | --- |
| `before-1080p.mp4` | `bc68cc8913598e9fcfc5e76dbff4dd583ab3d6b2c0ff797c266519a19845bf83` |
| `before-4k.mp4` | `66015ffb854148169e0e6654db25db33bbb46dd6bfd72a2099bfe9b9c4eaf91d` |
| `before-1080p-slow.mp4` | `9524b28333344bb074769ac5e04587e664139adeb28a5cf6ae898ea31761e36f` |
| `after-1080p.mp4` | `5b8b4f6aef9b6780575b8419f04b411c128c4a648400a3f64908e6409bc930b6` |
| `after-4k.mp4` | `8036a6b4e5831bd805338e660fcf22032c04aa32b3c85147dfa068d764b11583` |
| `after-1080p-slow.mp4` | `df6cd2854f116eee259e7b2aee156b1e892807128a437be23da6695552b46bf2` |
| `after-1080p-reducemotion.mp4` | `e29f2cdf91c13826e3a0d8ec706c9e424d7f1673379d8c7ec0dd6b5ff04694b5` |
