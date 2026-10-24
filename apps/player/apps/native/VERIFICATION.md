# Swift Apple verification

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
