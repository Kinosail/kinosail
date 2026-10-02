# Kinosail evening QA — October 1, 2026

## Scope and run record

The user requested useful QA work to use the remaining weekly Codex allowance before midnight in America/Denver. The initial live meter showed 87% used and 13% remaining. The midnight target was missed, and useful QA continued on October 2. At 06:46 Denver time, the live meter showed 91% used and 9% remaining. Credits and the available reset were untouched.

- Baseline revision: `75ebf1d57bba483e443d288378256d6bd6ec8fe2`.
- Host: macOS ARM64; Go 1.27.1; Xcode 27.0.
- Browser packages: Playwright 1.63.0 and axe-core/playwright 4.13.0.
- Running servers: fresh Go builds with application version `qa-75ebf1d`.
- Fixture: repository-generated synthetic CC0 media and local synthetic metadata.
- Isolation: separate Player and Subtitles media, database, cache, and backup directories.
- Access: loopback HTTPS on ports 38127 and 38128; test Owner MFA enrollment and onboarding confirmed with HTTP 200.
- Local evidence root: `/tmp/kinosail-usage-qa-20261001`.
- Every completed check writes a JSON run record with revision, command, directory, environment, fixture description, duration, exit code, and log path.

Podman's VM has 248 KiB available on `/var`, which is 100% full. Container runs are blocked. The synthetic-media generator ran through a host-only command adapter with the installed macOS FFmpeg. Browser checks use fresh macOS server processes. These checks do not establish production-container behavior.

## Journey checklist

| Batch | Success paths | Failure and boundary paths | Presentation and interaction |
| --- | --- | --- | --- |
| Player library and account | Owner login, browse, navigation, settings, media details | Viewer permissions, empty library, invalid inputs | Phone and desktop geometry, keyboard, accessibility |
| Player playback | Direct playback, resume, seek, subtitles, quality, speed | Autoplay denial, preparation, network stalls, decode errors, recovery | Pending, loaded, empty and failed feedback |
| Player storage | Downloads, offline media, readers, photos | Unsupported browser capabilities, cancelled and failed work | Reachable controls and truthful saved state |
| Subtitles | Overview, wanted inventory, inspector, history, settings | Missing tracks, provider errors, rejected input, safe cleanup | Phone and desktop geometry, keyboard, accessibility |
| Shared server behavior | Existing public API and persistence suites | Validation and authorization failures before side effects | Error responses and privacy boundaries |
| Native clients | Available native tests and simulator builds | Cancellation, recovery and response validation | Simulator interaction where a usable fixture exists |

Chromium is the initial populated-browser batch. Firefox and WebKit follow for relevant journeys. Tests that skip a check are recorded as skipped.

## Worktree cleanup

The user authorized keeping useful work and discarding obsolete checkouts. After fetching remote main, every inspected inactive checkout's HEAD was an ancestor of the baseline revision. Two of the original 19 checkouts had already disappeared before cleanup began.

This task removed 15 obsolete checkouts. Fourteen were clean with all committed work in remote main. The remaining checkout held a superseded Safari draft: its seek completion and buffered-position changes are already in current production sources. Its modified files and binary Git diff were archived and verified before removal.

Keep these two unfinished checkouts:

- `all-app-polish`: Android system-bar contrast change and a focused compositor regression draft.
- `player-end-to-end-speed`: native playback-readiness lifecycle tests, performance probe, and unfinished measurement evidence.

Active leases were preserved. The primary checkout's unrelated changes were preserved. Local branch refs were retained. Relevant ignored verification evidence from removed checkouts was archived before removal. Reproducible build directories were discarded.

Recovery records and archives are under `/Users/mikeo/.codex/worktree-recovery/2026-10-01-usage-qa`. `decisions.json` records each removal, its HEAD, the fetched main revision, and its archive. The post-cleanup audit reports exactly the two retained dirty inactive checkouts.

Available host disk space increased from approximately 13 GiB to 21 GiB. This does not free Podman's separate VM filesystem.

## Verification results

| Check | Result | Local run record |
| --- | --- | --- |
| Player `go test ./...` | Passed: 5 packages | `player-go.json` |
| Subtitles `go test ./...` | Passed: 7 packages | `subtitles-go.json` |
| Shared packages `go test ./...` | Passed: 60 packages | `packages-go.json` |
| Browser bundle contract tests | Passed: 4 tests | `web-bundles.json` |
| iOS simulator suite | 280 passed, 1 skipped, 0 failed | `ios-native.json`; `ios-native.xcresult` |
| tvOS simulator suite | 271 passed, 0 skipped, 0 failed | `tvos-native.json`; `tvos-native.xcresult` |
| Subtitles populated browser batch | 177 passed across Chromium, Firefox, and WebKit | `subtitles-browser-corrected.json`; `subtitles-browser-corrected-playwright.json` |
| Player populated library/account batch | 77 passed, 1 failed across the three browsers | `player-browser-corrected.json`; `player-browser-corrected-playwright.json` |
| Player offline playback reproduction | 2 fresh Chromium runs passed | `player-browser-offline-repro.json`; `player-browser-offline-repro-playwright.json` |
| Player playback and recovery matrix | 228 passed across Chromium, Firefox, and WebKit | `player-browser-playback.json`; `player-browser-playback-playwright.json` |
| Root `make tooling-check` after snapshot repair | Passed | `tooling-green.json` |
| Origin-parser fuzzing | Passed: 131,234 executions in the 30-second fuzz window | `player-origin-fuzz.json` |
| Remote-boundary parser fuzzing | Passed: 110,817 executions in the 30-second fuzz window | `player-remote-boundary-fuzz.json` |
| Backup restore confinement fuzzing | Passed: 49,724 executions in the 30-second fuzz window | `restore-boundary-fuzz.json` |
| Player storage, authentication, controls, and layout matrix | 297 passed; 3 fixture failures; 3 confirmed focus failures | `player-browser-storage-auth.json` |
| Native client against the actual Go Server | 3 passed, 0 skipped, 0 failed | `ios-live-contract-summary.json`; `ios-live-contract-shows-corrected.xcresult` |
| Player skip-link repair | 6 cases passed across three engines, phone and desktop, with 24 route checks | `player-browser-skip-final-green.json` |
| Player complete Go suite after focus repair | Passed: 5 packages | `player-focus-suite.json` |
| Native progress transport regression | Failed for the intended extra request field before repair; passed afterward, with 10 argument cases | `ios-progress-transport-red-body.json`; `ios-progress-and-live-green.json` |
| Native playback and saved-file journeys after progress repair | 2 passed against the actual Go Server | `ios-live-media-moving-green.json` |
| Complete iOS suite after progress repair | 282 passed, 1 skipped, 0 failed | `ios-progress-full-green.json`; `ios-progress-full-summary.json` |
| Complete tvOS suite after progress repair | 273 passed, 0 skipped, 0 failed | `tvos-progress-full-green.json`; `tvos-progress-full-summary.json` |
| Player worker, storage, transfer, startup, seek, and recovery boundaries | 177 passed across three engines | `player-browser-boundaries.json` |
| Actual Hls.js watch-to-library navigation | 2 WebKit runs passed; 6 transitions | `player-browser-real-hls-nav-ready.json` |
| Actual native Dialog Boost and Night Mode HLS | 1 function with 2 argument cases passed | `native-hls-evidence.json`; `ios-real-hls-effects-known-state-fixed.xcresult` |
| Actual tvOS remote browse/navigation | 1 passed; 25 retained screenshots | `tvos-real-remote-navigation.json`; `tvos-real-remote-navigation.xcresult` |
| Subtitles trusted HTTPS disclosure reproduction | 2 WebKit runs passed | `subtitles-browser-trusted-repro.json` |
| Player remote authorization sequence fuzzing | Passed: 671 executions in the 30-second fuzz window | `player-authorization-fuzz.json` |
| Subtitles persisted media-share state fuzzing | Passed: 83,428 executions in the 30-second fuzz window | `subtitles-share-state-fuzz.json` |
| Actual native PDF, EPUB, and comic documents | 1 function with 3 argument cases passed; populated renders inspected | `native-reader-evidence.json`; `ios-real-reader-documents-rendered.xcresult` |
| Actual native EPUB active-content and external-resource isolation | 1 passed; reachable control fetched 1 image; protected chapter fetched 0 external resources | `native-reader-isolation-evidence.json`; `ios-real-reader-isolation-corrected.xcresult` |
| Actual native reader failure and reload | 1 function with 6 cases passed; real 404 and revoked-token 303 resources, with native API 401 control | `native-reader-failure-recovery-evidence.json`; `ios-real-reader-failure-recovery-corrected.xcresult` |
| Actual Server audio effect PCM | 4 modes passed on the single advertised 540p rendition; equal conversion paths and exact preference restoration | `actual-hls-pcm-controlled-evidence.json`; `actual-hls-pcm-controlled-identity.json` |

The authorization fuzzer exercises public approval, revocation, and consumption operations using an in-memory Quick Connect broker. It rejects Owner and weak-session approvals and prevents revoked grants from issuing sessions. The persisted-state fuzzer requires valid media-share state to remain valid after pruning. Neither is an HTTP journey or a complete validation audit.

While this batch ran, PR #418 independently changed persistent browser-cookie lifetime. This task reconciled with that main revision and reran `packages/identitycore` and `packages/servertest`; both passed as `session-reconciliation`. The [fourth deep run](https://github.com/Kinosail/kinosail/actions/runs/36970986771) started at the subsequent evidence merge, `59661fab987baaf27097578c122da84b0b49c3b6`. Its outcome remains pending.

That fourth run completed with failures in tooling and Player's Chromium and WebKit jobs. All three Subtitles browser jobs and Player Firefox passed. The new stale snapshots were repaired through PR #421. A [fifth deep run](https://github.com/Kinosail/kinosail/actions/runs/36972039705) started on its merge, `1a47860f1668fd33664f9e1a9c9ba74e29d8c499`; the final result is recorded below.

The fifth run completed with Player WebKit's same HLS console failure. Tooling and Player Chromium passed. The earlier Chromium interruption remains an unverified observation. Full deep CI remains failed.

The playback matrix combines real synthetic-media journeys with controlled browser media and transport failure scenarios. It covers Direct First, blocked autoplay, resume, seeking, duration, quality, speed, buffering, and recovery. This is not a physical-device or codec certification.

The initial native suites used dedicated iPhone 18 Pro and Apple TV 4K simulators on iOS/tvOS 27.0. They include their own loopback and rendering fixtures. The iOS skipped case remains outside passing coverage.

A later disposable iOS probe exercised production `ServerClient` against a separate fresh Go Server on loopback HTTP, with its own Owner and confirmed MFA. Three journeys passed: real catalog/detail/playback/reader/download-identity response decoding; saved preferences and four rejected writes with unchanged persisted state; and Quick Connect approval, token use, sign-out, and subsequent HTTP 401. HTTP playback descriptors were decoded; this probe does not prove AVPlayer rendered frames. It does not prove TLS, physical touch, or Siri Remote behavior.

The probe source, SHA-256 receipt, server binary hash, and private fixture launcher are retained in the evidence root. The temporary probe was removed from the test target after execution. Initial harness corrections addressed Swift actor access, the video-only download-track operation, and the distinction between an episode ID and its Show ID. Those incomplete runs remain recorded. They are not product findings.

The storage/authentication batch used the baseline production binary. An environment override made Server name intentionally read-only, causing three settings-test failures. Removing that override restored all three settings checks. Its other three failures confirmed QA-003. The subsequent HTMX matrix passed all 75 controlled pending, loaded, empty, failure, and stale-response checks across three engines. An early expanded focus test incorrectly assumed Downloads used `#main`; it correctly uses `#downloads`. That assumption was corrected before the final six-case focus run.

Production sources did not change between baseline `75ebf1d`, report commit `680781773`, and its merge `895f627b4`. Browser servers retain the fresh baseline binaries. Native and source runs record the report revision; the initial two manifests captured the revision at completion. Later manifests capture it at launch. This distinction does not change the tested production sources.

The initial custom setup harness read the enrollment secret at the wrong JSON level. The isolated Player fixture was restarted, and the harness now uses the existing `totp.secret` API response field. This was a harness error, not a product defect.

Initial Chromium runs also exposed incomplete fixture setup: Player's long-show fixture writer and Subtitles' inspector fixture writer were omitted by an anchored test selector. Subtitles' filesystem root was missing. Those runs recorded 20 Player passes and 6 fixture failures, plus 22 Subtitles passes, 13 fixture failures, and 24 serial skips. The corrected writers and isolated filesystem root were installed before the reported browser batches. The failed initial runs remain in the evidence root.

## Findings

### QA-001 — Stale Player Code Atlas blocks deep CI (confirmed)

The manually dispatched [deep CI run](https://github.com/Kinosail/kinosail/actions/runs/36959819107) at `895f627b4248bbcc2138c75885ff2849ea91e394` failed its tooling job because the Player architecture snapshot no longer matched current source.

The existing `python3 scripts/tooling/test-architecture-explorer.py` reproduced that failure locally before changes. Regenerating both app snapshots changed only Player's generated `index.html`. Its `internal/server` data was stale; package and source-file counts remain 68 and 893.

The same regression check then passed, including its invalid-input and no-side-effect checks. No generator or production behavior changed. `architecture-red.json` and `architecture-green.json` retain the control runs. The complete root tooling suite, `make max-loc`, `git diff --check`, and Player's post-commit `make verify-changed` passed before publication of the repair. The affected-app check selected only the generated documentation path and ran its source cap and diff checks; it did not rerun the app suite.

The fourth deep run found another stale snapshot after the independently merged persistent-sign-in change. The local contract reproduced the failure at `63c42ce00563db25b267ac9b2584fd89fa5eeb9a`. Regenerating both app snapshots changed only metadata for `identitycore` and `servertest`: four production lines, 25 test lines, and one test symbol. Package and source-file counts stayed unchanged. The same contract then passed, including invalid-input/no-write checks. Records are `architecture-second-red` and `architecture-second-green`.

### QA-002 — One offline playback stall (unverified)

One Chromium download journey stopped near 0.099 seconds after the network disconnected. The saved file was verified, its range probe returned the expected HTTP 206 and two bytes, and the offline video reached a usable ready state. The assertion requiring moving playback failed.

The same journey passed in the initial Chromium batch, in Firefox and WebKit, and twice in fresh Chromium repetitions after the failure. The saved trace and screenshot remain under `player-browser-corrected-results/test-instance-production-a-16c56-ter-the-network-disconnects-chromium`. The observation has not reproduced twice from a known state, so no product fix or passing full Player batch is claimed.

### QA-003 — Skip to content changes the URL without moving keyboard focus (confirmed and repaired)

The existing Show keyboard journey failed in Chromium, Firefox, and WebKit. Two fresh Chromium repetitions also failed. The shell supplied the skip-link target but left its `main` element unable to receive focus. The URL acquired `#main` while focus stayed outside the content.

The shell now adds `tabindex="-1"` when missing. It preserves existing IDs and tabindex values. The expanded browser regression checks actual keyboard activation and focused landmarks on Show, Account, Settings, and Downloads pages at 390 and 1440 pixels. Downloads retains its required `#downloads` target. Both widths failed on the baseline before the repair; all six final cases passed across three engines afterward. Populated screenshots are retained with the run.

Focused shell/navigation Go checks, the complete Player Go suite, the regenerated Player Code Atlas contract, the source cap, and diff checks passed. There is no layout, copy, API, or asynchronous-state change.

### QA-004 — Hosted WebKit HLS access-control console errors (under investigation)

The [current-main deep run](https://github.com/Kinosail/kinosail/actions/runs/36961451633) at `eb6d7995d8f1d9edbc6b05acf3e33a3484e2f06f` passed tooling after QA-001, but its WebKit onboarding journey recorded HLS playlist, initialization, and segment access-control console errors. The journey reached moving playback before its final console-error assertion failed. The log remains in `deep-corrected-failed.log`.

This differs from QA-002. The first deep run passed that WebKit journey. No console-error exclusion or product repair has been applied without identifying the cause. A local attempt against the already modified shared fixture could not reach the required initial recent-content state and does not reproduce the hosted observation.

A fresh local Server completed setup and reached playback, then failed an earlier assertion expecting a Continue watching heading. The page instead rendered Arrival as its featured resume title. This local run did not reproduce the hosted HLS console failure. Its separate UI/test-contract observation remains under investigation.

The [third deep run](https://github.com/Kinosail/kinosail/actions/runs/36967414781), at `a35214404d95eb968b8e0c39fe09c1d4ade5f3f8`, retained its failure trace after QA-006. The trace places the HLS errors around watch-page navigation and cancelled requests. This timing does not establish a Server CORS rejection or a lifecycle defect. Production already destroys streaming on `pagehide` and guards recovery callbacks after destruction.

Two local repetitions exercised the actual Go Server and Hls.js, with three watch-to-library transitions per repetition. Both passed with playlist and media-segment requests, a moving video clock, and no HLS page errors. The probe disabled only native HLS capability reporting to select Hls.js on macOS WebKit. This does not reproduce or clear the hosted Linux WebKit observation. An earlier probe ran before the Server was reachable and is excluded from coverage.

Two further repetitions included pause, My List form submission, watched-state form submission, and Library navigation in each of three cycles. Both passed without HLS page errors. `hls-navigation-evidence.json` records the final probe hash and run receipt. A preliminary command used the wrong working-directory prefix and did not update the probe; that run adds no form-navigation coverage.

The existing onboarding journey now records media state in retained traces at `pagehide` capture and in a listener registered after initial load. It records a bounded source path or scheme, readiness, network state, position, paused state, visibility, and Picture in Picture state. Origins and query strings are omitted. Production playback and all console-error assertions remain unchanged.

Fresh local Chromium, Firefox, and WebKit journeys passed as `player-fresh-happy-chromium-18`, `player-fresh-happy-firefox-19`, and `player-fresh-happy-webkit-17`. WebKit and Firefox traces show media readiness resetting to zero and playback pausing after registered cleanup handlers. Chromium's passing trace contains no observer messages. These macOS direct-playback observations do not establish Linux Hls.js cleanup. An initial microtask observer ran before cleanup and was corrected; its apparent after-handler state is excluded. `playback-lifecycle-diagnostic-evidence.json` records the pending source hash, commands, fixture, traces, and this boundary. The hosted error remains unresolved.

The [sixth deep run](https://github.com/Kinosail/kinosail/actions/runs/36978489755) at `da2dd733ae47bb1b74857aa8e7d143fc4df3e247` retained the new observations in its [WebKit artifact](https://github.com/Kinosail/kinosail/actions/runs/36978489755/artifacts/11214946529). Four HLS access-control errors occurred after the Library click began and before that document's `pagehide` capture. The later registered-handler observation shows paused playback and zero readiness. This narrows the timing to navigation before `pagehide`; it does not establish a Server CORS rejection or prove a repair. All three Subtitles browser jobs and Player Chromium and Firefox passed. Player WebKit and its aggregate checks failed; other executed jobs passed, and publication was skipped. `deep-sixth-evidence.json` records the exact revision, job results, trace hash, and timings. Full deep CI remains failed.

### QA-005 — Apple progress updates are rejected by the Server (confirmed and repaired)

Actual iOS playback advanced, decoded a video frame, and sought successfully, but the Server rejected progress updates with HTTP 400. Both independent probe runs failed to observe a persisted position. Phase diagnostics isolated the failure to persistence; the moving clock and decoded-frame checks passed.

The Swift client sent its local `dismissed` flag in both progress snapshots. The authoritative Server `mediaProgressSnapshot` accepts only `seconds`, `watched`, `session`, and `revision`. Strict decoding rejected the extra field before saving state.

The client now serializes those four fields for synchronization. Its local JSON representation still retains dismissal state. Both progress and expected snapshots pass their existing validation before HTTP. No Server schema or validation was weakened.

The new transport regression failed before the production change because both snapshots contained `dismissed`, for both true and false local values. Its eight negative argument cases prove missing required values, malformed controls, excessive lengths, invalid positions, and invalid revisions are rejected without a network request. Unknown fields and malformed persisted JSON retain existing native contract coverage.

After the repair, the transport regression passed. The real iOS journey advanced from its initial position, decoded a frame, sought to six seconds, and observed a saved Server position. A second journey completed and verified a real download, played the saved file with an advancing clock, sought, and removed it. The complete iOS suite then passed 282 tests with one skipped case; tvOS passed 273 with none skipped. The focused Swift Testing suites report test functions separately from their argument cases; the transport suite contains two functions and ten cases.

Evidence includes the before/after result bundles, the disposable probe source, source hashes, and `native-progress-repair-receipt.json`. A preliminary single-method selector selected zero tests and is explicitly excluded from passing coverage. The saved-file journey proves use of a local file; the Server remained reachable during that run, so a disconnected-device journey is not yet claimed.

A subsequent isolated iOS journey shut down its exact task-owned Go Server process after downloading and verifying the file. The authenticated client reported the Server unavailable. Local playback advanced, decoded a frame, and sought to four seconds. The pending position survived a new `ProgressSyncStore` instance. Local download removal worked while the Server was still unavailable. After restarting the same Server state, the pending position synchronized without conflict and the authoritative API returned the saved position. The result bundle reports one passed test, zero skipped, and zero failed. This proves a disconnected Server process on loopback, not a physical network-interface toggle. Evidence is in `native-offline-evidence.json`, `native-offline-control-receipt.json`, and `ios-live-disconnected.xcresult`.

### QA-006 — Hosted Player browser failures lose their evidence (confirmed and repaired)

The failed WebKit job in QA-004 reported that its upload path contained no files. The run retained Docker build records but no browser failure artifact. Player's launcher appends the browser project to the configured output directory; the workflow uploaded only the unsuffixed directory. Subtitles uses the unsuffixed directory.

The upload step now includes both directories. A regression executes each launcher's actual browser command with a recording `pnpm` function, creates representative failure evidence at the resulting path, and checks that the workflow's upload roots cover it. Before the change, all three Player engines failed and Subtitles passed. Afterward, all six app/engine combinations passed within the complete 49-test CI contract suite. `actionlint` also passed.

The retained records are `browser-artifact-red`, `browser-artifact-ci-contracts`, and `browser-artifact-actionlint`. No browser error was ignored or required gate weakened. The third deep run successfully uploaded its [Player WebKit failure artifact](https://github.com/Kinosail/kinosail/actions/runs/36967414781/artifacts/11210517627), containing 18,514,440 bytes. Its downloaded trace and network records were inspected locally. Actual hosted failure-artifact upload is now verified.

PR #416's first package job failed while executing its temporary FFmpeg test script with `text file busy`. The script writer already closes and atomically renames its file. The failed package and aggregate checks passed on one retry; the original log is retained as `browser-artifact-packages-failed.log`. No source repair or universal absence of this intermittent environment failure is claimed.

### QA-007 — The onboarding journey assumes a resume shelf that the approved home design omits (confirmed and repaired)

Two fresh WebKit runs failed because the journey required a Continue watching heading after saving Arrival's position. The populated page correctly showed Arrival as a featured resume title with its saved position and removal action. `DESIGN.md` and the existing compact-home contract require omission of an empty shelf when that title is the only resume item.

The journey now requires a Resume link to the exact watch route, then verifies that no Resume link remains after marking the title watched. This covers both the featured title and the fallback resume row without requiring a particular shelf. The final fresh Chromium, Firefox, and WebKit journeys passed with all existing accessibility, playback, account, backup, installation, offline-shell, and console checks still enabled. Records are `player-fresh-happy-chromium-8`, `player-fresh-happy-firefox-6`, and `player-fresh-happy-webkit-9`.

An initial exact-name Resume assertion missed the fallback row's full accessible name; it was corrected before delivery. Chromium's first local attempt could not reach localhost, and an IP-origin attempt reached the intentional canonical localhost passkey redirect without sharing its origin-bound cookie. The final Chromium fixture bound IPv6 loopback and used localhost throughout. Those preliminary runs are excluded from product regressions and passing coverage. The hosted HLS observation in QA-004 remains separate.

### Additional native runtime evidence

An actual Quick Connect approval issued a new device token for the owned tvOS simulator. The production Keychain and `AppSession` restored its Viewer Profile against the actual Go Server. The existing remote playback crash journey then passed three playback cycles, including options presentation, held seeking in both directions, pause/resume, and return to the movie detail screen. `tvos-live-session.xcresult` and `tvos-real-remote-playback.xcresult` each contain one passed test with no skips or failures; retained screenshots are attached to the latter.

The watchOS companion simulator build passed as `watchos-simulator-build`. No Watch runtime, HealthKit, phone pairing, physical Apple TV, or physical remote proof is claimed.

The existing tvOS remote navigation journey passed against the actual Go Server. It exercised Search, Settings, Movies, Shows, Music, Audiobooks, Photos, and return-focus paths. Its result bundle retains 25 screenshots; populated launch and Movies renders were inspected. This remains simulator remote input.

The actual iOS HLS probe passed both Dialog Boost and Night Mode after resetting authoritative progress to zero before each case. Each case required an HLS source, an advancing AVPlayer clock, a decoded frame, a seek, and a persisted Server position. Original preference overrides were restored. The earlier uncontrolled-position run failed its Dialog Boost frame deadline while Night Mode passed. No product cause is asserted. A prior interrupted run and a harness compilation failure are excluded from coverage. These runs do not measure decoded PCM or establish subjective audio quality. The final source hash, fixture state, command, and result are in `native-hls-evidence.json`.

The native reader probe loaded real protected PDF, EPUB, and comic resources from the isolated Go Server. It verified nonempty PDF text, EPUB body text, and a decoded comic image. Loading indicators started during pending work, cleared after content loaded, and stayed stopped after close. All three argument cases passed. Populated 390-by-844-point captures were inspected. The first UIKit captures of WebKit content were blank despite loaded document state; corrected scene attachment and WebKit snapshots provided the retained visual proof. This does not establish physical-device behavior or the complete reader shell. `native-reader-evidence.json` records the probe, binary, and render hashes. The disposable probe was removed from the test target.

The EPUB isolation journey served an imported chapter containing inline and external script, image, stylesheet, background, frame, and video references to a task-owned loopback listener. An ordinary control WebView first executed its canary and decoded an image from that listener. The actual Server delivered the hostile chapter bytes unchanged to the native client. The protected reader displayed its sentinel text under `kinoreader:`, left the imported script undefined, and made no additional listener requests during a two-second observation or close. Its populated capture was inspected. This proves the combined reader protection in this fixture, rather than attributing it to one redundant guard.

The result bundle contains one passed test, no skips, and no failures. The initial probe compilation failure occurred before runtime and is excluded. Both attempts restored the exact original EPUB bytes, verified by SHA-256 `27e2b863cd2448f92b060e671063c58fd7f3550217d6057c0edd9255b059fb1d`. The probe was removed from the test target. `native-reader-isolation-evidence.json` records the source, command, network events, fixture restoration, and result. Physical-device behavior and exhaustive malicious-archive coverage remain unverified.

A further native reader journey passed six argument cases: missing resources and revoked sessions for PDF, EPUB, and comic. Real missing resources returned HTTP 404. Each separate Quick Connect token resolved its Viewer before sign-out and received HTTP 401 from the native API afterward. The protected web resource then returned HTTP 303 to sign-in; the native HTTP layer refused the redirect. The reader stopped its pending indicator, reported failure without exposing the Owner token, closed its document, and loaded the original document successfully on retry. No original book or Owner credential was changed.

The result bundle reports one test function with six cases and no issues. The first attempt expected HTTP 401 from the web resource and recorded three expectation failures; its observed HTTP 303 matches the Server's web-route contract. That preliminary run is excluded from passing coverage. `native-reader-failure-recovery-evidence.json` records source and binary hashes, command, controlled metadata, and per-case results. The disposable probe was removed. This covers the actual simulator component and HTTP resource boundary, not the complete Reader screen or a physical device.

A separate actual Go Server delivered a six-second MPEG-4 video with a quiet 1600 Hz mono tone. Each preference mode required video conversion, so plain and enhanced outputs used the same H.264/AAC path. The served source bytes matched the generated fixture hash. The fixture advertised one 540p rendition. Each mode decoded 96,000 finite mono samples from seconds two through four at 48 kHz, with peaks below clipping.

| Mode | Decoded RMS | Ratio to plain | Peak |
| --- | ---: | ---: | ---: |
| Plain | 0.017642 | 1.00 | 0.025060 |
| Dialog Boost | 0.030615 | 1.74 | 0.043516 |
| Night Mode | 0.163133 | 9.25 | 0.230092 |
| Both | 0.212388 | 12.04 | 0.300902 |

All four modes passed. Original inherited preferences were restored exactly, and the owned Server process stopped. The running Go binary is the initial task baseline; its audio filter and Server enhancement sources are unchanged through the tested main revision. The initial comparison remuxed plain AAC but reencoded effects, so its small Dialog Boost difference is excluded from filter proof. Two controlled attempts assumed a filename title that the metadata stub renamed; the final probe verified source identity instead. `actual-hls-pcm-controlled-evidence.json` records fixture generation, source and binary hashes, API requests, local playlists, PCM hashes, commands, results, and cleanup. This proves this mono tone and single rendition, not multichannel mixing, subjective speech clarity, or physical speaker output.

### QA-010 — SubSource accepts a longer episode number (repaired)

A real loopback SubSource provider returned `Show.S1E20.srt` for requested S01E02. Both a single release and a season ZIP installed the wrong dialogue. The same acquisition operation also accepted S01E020, S1E200, and combined or conflicting episode names. The substring matcher did not compare complete numeric identities.

The repair parses complete season and episode numbers at the shared provider-selection and ZIP-member boundary. It rejects mismatched, overflowing, combined, or conflicting identities. Exact padded, unpadded, case-insensitive, and attached-show names remain supported. Equal alternative identities and ordinary quality/year suffixes remain supported. Unicode suffixes and episode ranges fail closed. Existing release and ZIP size bounds apply before matching.

The retained regression exercises actual HTTP search and ZIP download responses, sidecar installation, originals, and persisted acquisition state. On the original source, 42 of the final 75 cases failed by installing wrong content; all 75 cases and existing SubSource checks passed after the repair. Independent review found no remaining actionable finding in this scoped change. Rejected single releases caused zero downloads. Season archives required one bounded download, then rejected the member without a sidecar, original, managed record, or added-history event. Existing failed-search bookkeeping remains allowed. `subtitles-episode-repair-evidence.json` records source hashes, commands, controls, and results. External SubSource service behavior and Owner authentication were not tested by this probe.

### QA-011 — Unicode JSON aliases bypass duplicate-field rejection (repaired)

The actual media-share creation handler accepted `maxDevices:0` followed by `maxDeviceſ:1`, returned HTTP 201, and persisted a share. The escaped and reversed long-s aliases also passed. The valid control created one share; an ASCII duplicate returned HTTP 400 without persistence. These were isolated loopback and application-handler probes. Owner authentication was not exercised, and no authentication bypass is claimed.

The shared decoder now compares each key using Unicode simple-fold equivalence, then preserves the existing lowercase duplicate policy. It computes the comparison once and leaves the original JSON and final unknown-field decoder unchanged. Literal, escaped, and reversed aliases return HTTP 400 with no share persistence. Existing recursive decoder cases cover nested long-s/final-sigma aliases. Unambiguous map keys retain their original spelling, including distinct composed/decomposed and full-fold forms.

The regression failed before the repair and both affected package suites passed afterward. Complete shared-package, Player, and Subtitles Go suites passed. Changed-code lint passed. Both local app `verify-changed` attempts stopped on 114 existing full-package lint findings outside this change; these attempts did not pass. An independent enumeration checked all 1,112,064 Unicode scalar values for decoder-fold equivalence, preservation of existing lowercase collisions, and idempotence; the largest orbit contains four runes. `unicode-json-repair-evidence.json` retains commands, source hashes, controls, and the enumeration packet. A preliminary negative matrix reused one store and contaminated later no-write assertions; the corrected matrix isolates each case and supersedes that run.

The [Unicode repair PR](https://github.com/Kinosail/kinosail/pull/431) merged after required checks passed. Its first hosted shared-package run failed `TestProbeDurationRunsAndLoadsBoundedResults` with a zero duration. That unrelated check passed 100 local repetitions, and the failed hosted job passed on one retry. The original failure is retained in `pr431-packages.log`; no media-probe repair or explanation of that failure is claimed.

### QA-012 — Ambiguous provider JSON overrides a rejected production type (repaired)

Actual loopback provider responses supplied `productionType:machine` followed by a duplicate, case alias, or escaped identical key with `retail`. All three responses installed a subtitle and retained an original, acquisition record, and history entry. The machine-only control rejected acquisition without downloading. Retail and retail with unknown metadata installed exact bytes. Existing validation saw only the final decoded value.

The shared external decoder now checks unambiguous keys, valid UTF-8, bounded nesting, and a single complete document before typed decoding. It reuses the existing key comparison and nesting policy. Unknown provider metadata and supported scalar, null, array, and object shapes remain accepted. Unicode-equivalent aliases fail before destination mutation. This stricter policy also applies to unknown nested objects; distinct composed/decomposed and full-fold map keys remain distinct.

Both parser and actual acquisition regressions failed before repair. All shared-package, Player, and Subtitles Go suites passed afterward, along with relevant provider checks, changed-code lint, and root tooling. Independent review found no actionable finding. Both local app `verify-changed` attempts stopped on the same 114 existing full-package lint findings outside this change. `external-json-repair-evidence.json` records the source hashes, red and green commands, and scope. Rejected responses cause no download, sidecar, original, acquisition record, or acquisition-history write; existing search and provider-health bookkeeping remains allowed. External provider service behavior and Owner authentication were not exercised.

### QA-013 — Invalid SubSource candidate blocks valid fallback (repaired)

A complete HTTP 200 response containing an invalid ZIP marked SubSource unavailable for about one minute. The acquisition loop then skipped a valid, lower-ranked candidate. CRC failure, a fully transferred truncated ZIP, wrong or ambiguous episode members, and a body above four MiB each reproduced this behavior. The lower-ranked candidate installed exact bytes when returned alone.

The adapter now records availability after the bounded HTTP read and treats ZIP/member rejection as a candidate failure. A private overflow error distinguishes a complete bounded prefix from a transport or non-200 failure. The four-MiB bound remains enforced. HTTP 429, 503, and incomplete HTTP transfers still stop further provider requests. A reachable provider can report connected even when an individual candidate is unusable; acquisition still rejects that content.

The final regression checks real loopback requests and persisted effects. Five fallback cases failed before repair; all nine cases passed afterward. Before the valid second download, no sidecar, original, acquisition record, or history entry exists. Successful fallback creates exactly one byte-preserved sidecar and original, one record, and one history entry. Focused existing provider checks and the complete Subtitles Go suite passed. `provider-fallback-repair-evidence.json` retains commands, source hashes, data, environment, and results. Live SubSource behavior and deployment were not exercised.

The [fallback repair PR](https://github.com/Kinosail/kinosail/pull/433) merged after required hosted checks passed. Fetched ancestry proves the reviewed source commit is included in remote main.

### QA-014 — Ambiguous playback diagnostics and credential text in logs (repaired)

The loopback playback trace endpoint accepted duplicate or case-aliased error codes. A later zero replaced an invalid five before validation. Arbitrary diagnostic detail and quality strings could also enter structured logs. Synthetic markers reproduced both findings; an Authorization header marker did not enter these trace logs. These paths predate the fullscreen logging change.

The endpoint now uses the shared strict, bounded JSON decoder before session or log effects. It omits arbitrary detail and quality. Five exact fullscreen error names remain at warning level. Eleven current Play triggers retain six fixed error names. Unknown triggers, names, extra components, and unrelated event types do not retain detail. Numeric diagnostics and session correlation remain available. Free HLS and rendition text is omitted; outer request logging and session validation remain separate contracts.

Five of nine initial loopback cases failed before repair and all passed afterward. The first complete Player run caught a task-caused loss of `control:NotAllowedError`; the repair was extended after nine new positive cases failed. Two additional streaming triggers failed before inclusion. The final seventeen-case grammar matrix also covers unknown and conflicting forms. Focused trace checks, complete shared and Player Go suites, changed-code lint, and root tooling passed. Initial post-commit app checks found 115 full shared-package lint findings: 114 existing findings and one task-caused complexity finding. The unchanged detail grammar was extracted into a helper, removing that new finding without suppression. Both final app checks stopped on the remaining 114 existing shared-package findings, outside the repair. Subtitles was compiled; its complete local suite was not rerun for this trace-only change. `trace-privacy-repair-evidence.json` retains source hashes, commands, controls, and results. No current deployed-handler, Owner-authentication, physical-device, or universal log-redaction proof is claimed.

The first hosted diagnostics run failed an unchanged temporary FFmpeg trickplay fixture with `text file busy`. That test passed 100 local macOS repetitions; the Linux cause is not established. CodeQL also flagged the detail log flow after extraction. The existing CR/LF sanitizer was restored around the allowlisted detail at the logging boundary, with no suppression. Focused trace checks, complete shared and Player Go suites, changed-code lint, and root tooling passed on that source. The [diagnostics repair PR](https://github.com/Kinosail/kinosail/pull/434) merged after all required hosted checks passed, including security. Fetched ancestry proves the final source commit is in remote main.

### QA-015 — Delayed background restore can revive locked download authorization (repaired)

An isolated iOS probe invoked the unchanged stock background callback with synthetic credentials in a private Keychain namespace. After its saved-session read began, detached public clear-and-lock operations completed during a controlled MainActor barrier. The callback then restored the old download scope despite an empty Keychain. This is a verified callback race, not an executed full `AppSession.disconnect` flow or a natural timing measurement.

The callback now captures the engine's authorization revision before the Keychain await. The engine checks it on its serial queue before mutation. Successful authorization and every applied lock, including an already locked state, invalidate earlier revisions. Stale authorization and fallback locks leave newer credentials and pending restores untouched. Authorization commits after all throwing state updates. Ordinary foreground calls retain their existing defaults.

Tests were written before each repair. Ten new public-operation executions cover cold restore, nil and existing locks, newer profiles, same-profile token rotation, stale fallback locks, later restoration, and journal failure. Intercepted outgoing headers prove the current token. A real filesystem write failure reproduced an ineffective fallback lock before the ordering correction; its writable control and both arguments passed afterward. The final focused run passed 15 test functions. The complete iOS suite passed 288 checks with one skip, tvOS passed 273, and the watchOS simulator build passed.

A separate stock callback replay passed four controls and five clear-and-lock attempts. Its added revision await means the controller cannot prove that stale saved credentials reached authorization in those green attempts. The deterministic public-operation regressions establish that guard contract. `native-auth-repair-evidence.json` retains exact sources, commands, fixtures, results, reviews, and exclusions. One nonexistent test selector and two tvOS command setup errors ran no tests and are excluded. Deferred initialization, physical devices, and full natural AppSession/background lifecycle behavior remain separate boundaries. The [native authorization repair PR](https://github.com/Kinosail/kinosail/pull/435) passed all required hosted checks and merged. Fetched ancestry proves its reviewed source commit is in remote main.

A later isolated iOS 27 replay verified actual OS delivery through the exact stock delegate. After eight durable verified blocks, the owned process was killed during its second download extent. iOS relaunched it in the background, restored the saved synthetic session, completed verification, and admitted the 75,497,516-byte file before any manual reopen. The automatic harness performed no authorization, Keychain save, or manual callback. The manifest and two byte ranges used exactly three HTTP requests; reopening added none. The retained executable, exact sources, chronology, hashes, and cleanup are sealed in `agent-native-download/ios-stock-background/verification.json`. Full AppSession disconnect, jetsam, reboot, force-quit behavior, and physical devices remain unproved.

### QA-016 — Subtitle providers follow unbounded or malformed redirects (repaired)

Actual credential operations reproduced sixteen failures across SubSource, SubDL, reconfigured SubDL, and OpenSubtitles. Their custom callbacks replaced Go's default hop limit and admitted oversized full URLs, user information, and malformed query escapes. Forbidden foreign origins received no provider requests; this finding does not establish credential leakage.

The shared callback now rejects the tenth redirect attempt, full URL strings above 4096 bytes, user information, and malformed raw query escapes before contacting the target. Existing provider origin and domain policies remain authoritative. Valid escaped queries and semicolons stay unchanged on the wire. The helper is used by initial clients and configuration replacement.

All 56 public-operation cases passed after repair. Tests were written first and retained reachable sink controls, credential headers or query values, exact target request counts, size boundaries, and closed listeners. `provider-redirect-repair-evidence.json` retains source hashes and receipts. The full Subtitles Go suite and changed-code lint passed. Lint initially caught a test cleanup dial without context; the context-aware correction passed the final matrix and lint. Root tooling, source caps, and whitespace checks passed. Post-commit Subtitles checks passed. The [provider redirect repair PR](https://github.com/Kinosail/kinosail/pull/436) passed all required hosted checks and merged. Fetched ancestry proves its reviewed source commit is in remote main. Real provider accounts, Owner-authentication adapters, deployment, and TLS remain separate boundaries.

### QA-017 — SAML metadata redirects bypass hop and URL bounds (repaired)

The public `NewSAML.Begin` operation followed twelve redirects and admitted a 2049-byte full target URL. Both reached valid synthetic metadata and created a private test key. Two failures and twelve controls were reproduced before production changes. The callback now applies Go's ten-request redirect history bound and the existing 2048-byte URL text limit before its unchanged trusted-URL check.

All fourteen cases passed after the two-line repair. Rejected targets receive no request and create no key, files, login destination, or POST result. Controls retain nine followed redirects, the exact 2048-byte limit, valid metadata, and another allowed IPv4 loopback origin. An independently reachable IPv6 HTTP origin remains rejected by the existing policy. All fixture listeners closed. `saml-redirect-repair-evidence.json` retains exact sources, commands, fixtures, results, and no-effect observations. The complete shared, Player, and Subtitles Go suites passed. A test listener lint finding was corrected with a context-aware ListenConfig call, and the final fourteen-case matrix passed. Changed-code lint and independent review passed. Root tooling, source caps, and whitespace checks passed. Both post-commit consumer checks stopped on the same 114 existing full shared-package lint findings, with none in the changed SAML files. Changed-code lint and complete Go suites passed. The [SAML redirect repair PR](https://github.com/Kinosail/kinosail/pull/437) passed all required hosted checks and merged. Fetched ancestry proves the reviewed source commit is in remote main. Real identity providers, deployed authentication adapters, full SSO, and physical devices were not exercised.

### QA-018 — Reader archives admit ambiguous resources and malformed EPUB spines (repaired)

An isolated actual Player HTTP probe made 35 GET requests against 17 synthetic archives. Duplicate ZIP image and chapter names select entry-order-dependent bytes. Duplicate EPUB manifest IDs choose the last declaration. A chapter path escaping the archive root produces a manifest containing an unusable empty asset path. Source archives and outside-directory sentinels stayed unchanged; no filesystem escape was observed. Native manifest rejection was compared against source policy, not executed through Swift in this probe.

Shipping regressions were written before production changes for both app handlers. Thirty-two negative cases failed in each consumer; eight valid controls passed. The first selector omitted positive owners and is excluded from complete-control coverage. Corrected runs include the original matrix, a duplicate TAR member beyond the metadata window, and public progress writes followed by fresh-server reads. Rejected writes leave persisted progress unchanged. Split fixture files meet the 300-line cap.

All forty cases passed in each consumer after repair. Archive listing rejects collisions against admitted names before resource selection. ZIP reads reject duplicate names before size filtering or opening members. EPUB parsing rejects ambiguous IDs, missing references, and invalid selected chapter paths. Valid case-distinct identities, parent-relative chapters inside the archive root, unsupported media skips, and distinct comic folders remain supported. Unique generic assets remain readable when only EPUB metadata is invalid. The complete shared, Player, and Subtitles Go suites passed. Independent review found no blocking issue. Changed-code lint passed with zero findings. Test-only complexity comments retain the exact executed statements. Root tooling, source caps, and whitespace checks passed. Both post-commit consumer checks passed their focused Reader checks, then stopped on the same 114 existing shared-package lint findings. No new Reader lint finding was reported. The [Reader archive repair PR](https://github.com/Kinosail/kinosail/pull/438) passed all four required hosted gates and merged. Fetched ancestry proves its reviewed source commit is in remote main. Independent hosted review confirms exact Reader source/test blobs and passing consumer suites on Ubuntu with libarchive-tools. Nonverbose logs do not expose individual case results or skip counts; forty-case totals are certified locally. Evidence is `reader-trust-repair-evidence.json` and its linked red, green, source, fixture, and review receipts. Literal wildcard matching in CBT command operands is a separate confirmed issue. Live Subtitles TCP, physical clients, external libraries, and deployment remain separate boundaries.

### QA-019 — Direct-only playback cannot retry a failed media request (confirmed)

A frozen actual Player binary at `14e106520` ran populated Chromium journeys at 390 by 844 and 1440 by 900. Twenty-six checks passed; two retry checks failed. A single induced media HTTP 500 displayed Retry Direct Play, but clicking made no new media request in either viewport. An ordinary reload restored real decoded playback. The client obtains the retry source only from `data-direct`, which the direct-only renderer omits.

The private packet retains exact source and binary hashes, raw console records, eight screenshots, and a restart script. Picture tap preserved playback; desktop mouse click invoked Chromium's native pause. That difference is not reported as a product defect. A two-viewport shipping regression draft lists successfully in Playwright but has not run yet. Evidence is `agent-webkit/native-controls-current-browser/review-packet.json`. Safari, physical devices, and QA-004 resolution are not claimed.

### QA-020 — Backup manifests accept conflicting fields before replacing state (repaired)

The exported Restore operation accepted conflicting manifest format and mediaIncluded declarations when the valid value appeared last. It then replaced existing synthetic destination state. Shipping tests were written first. Ten negative cases failed; six valid manifest controls and three existing rejection controls passed. Duplicate spellings include escaped, ASCII, and Unicode aliases. An unknown-field case also reproduced acceptance before repair.

The existing 2 MiB manifest boundary now uses the shared uniqueness-aware JSON decoder before destination writes. All nineteen cases and the existing legacy/autodetect owner passed after the import-and-call repair. Rejected manifests preserve every destination filename and byte, including synthetic database, WAL, and SHM files, and invoke no database adapter. Valid single escaped and case-alias keys remain supported. Unknown manifest fields are intentionally rejected; legacy archives without a manifest retain their existing branch. Complete shared, Player, and Subtitles Go suites and changed-code lint passed. Independent review found no blocking issue. A stale architecture snapshot stopped the initial tooling run. Both snapshots were regenerated, and root tooling then passed. Source caps and whitespace checks passed. Both post-commit consumer checks stopped on the same 114 existing shared-package lint findings, with no new backup finding. The [backup manifest repair PR](https://github.com/Kinosail/kinosail/pull/439) passed all four required hosted gates and merged. Fetched ancestry proves its reviewed source commit is in remote main. Evidence is `backup-manifest-repair-evidence.json`. New cases directly exercise plain Restore; no transient filesystem or live SQLite integrity certification is claimed.

### QA-021 — CBT and CB7 resource names act as patterns (repaired locally)

The exported ReadArchiveAsset operation passed admitted member names to bsdtar as inclusion patterns. Wildcards concatenated distinct resources, anchor characters selected another member, and recursive inclusion added descendant bytes. Real ordered TAR and compressed 7z fixtures reproduced twenty-eight failing fixtures. Tests were written first with thirty-seven fixtures and 146 public reads. Ordinary, leading-dash, slash, and existing backslash-rejection controls remained valid. No source archive changed, no file was extracted, and no outside-directory read was observed.

The existing admitted command branch now quotes six matcher metacharacters and disables recursive inclusion. All 146 reads passed locally. Complete shared, Player, and Subtitles Go suites passed. Independent production review found no blocking issue. Test lint identified duplicate flows, formatting, and complexity. A shared matrix and preservation helper retain every fixture, byte, and order without suppressions. The final focused matrix, full shared suite, and changed-code lint passed. Root tooling, source caps, and whitespace checks passed. Production changes remain confined to two lines. Repository and container paths always use bsdtar; Linux package families share the reviewed matcher and option behavior. Evidence is `archive-literal-repair-evidence.json`. Local runtime evidence uses installed macOS bsdtar. Hosted Linux, HTTP/native clients, and deployment remain separate boundaries.

### QA-008 — Hosted Subtitles WebKit disclosure does not open (unverified)

The third deep run failed the trusted HTTPS guidance journey: after clicking How trusted HTTPS works, its explanatory text remained hidden. Chromium and Firefox passed that check. The retained WebKit trace records layout instability and scrolling before the click. This does not identify a product cause. Two local WebKit repetitions passed with the original assertion unchanged. No repair, retry-based exclusion, or fully green hosted deep run is claimed.

The fourth hosted Subtitles WebKit job passed with that assertion unchanged. This observation remains unverified; passing repetitions do not erase the retained failure.

### QA-009 — Hosted Chromium native HLS stops after its first moving frame (under investigation)

The fourth deep run failed the onboarding journey before its explicit playback action because Retry playback remained visible. Its [retained failure artifact](https://github.com/Kinosail/kinosail/actions/runs/36970986771/artifacts/11211827312) was downloaded and inspected. Player's Chromium job reported 310 passed checks, one failed, and 238 skipped cases.

The playback events show the unsupported direct source selecting native HLS, receiving metadata, presenting frames, and advancing to 113 milliseconds. It then reported media error code 4 and offered retry. Playlist requests returned HTTP 200; initialization and segment requests returned HTTP 206. The master correctly advertised video-only H.264 variants. This is an initial native-HLS interruption, separate from QA-002's disconnected saved-file stall. The trace does not establish a malformed stream, decoder defect, or recovery cause. No product repair or browser-error exclusion is claimed. Records are `deep-fourth-chromium.log` and `hosted-fourth-chromium`.

## Verification boundaries

- Source tests: both app suites and 60 shared packages passed.
- Populated Subtitles browser batch: all 177 checks passed across three engines.
- Populated Player library/account batch: one unverified Chromium stall; 77 other checks passed. Both fresh stall repetitions passed.
- Player playback/recovery matrix: all 228 checks passed across three engines, within the controlled-fixture boundary above.
- Player worker/storage/transfer/startup/seek/recovery boundary batch: all 177 checks passed across three engines. Controlled browser transports and media states remain separate from actual Server playback.
- Native simulator suites after repair: iOS 282 passed and one skipped; tvOS 273 passed. Live probes additionally establish decoded playback, saved progress, verified transfer, disconnected-Server local playback, seek, pending-position persistence and reconnect synchronization, and local removal against the actual Go Server on loopback HTTP. The tvOS remote playback journey passed three cycles; watchOS simulator compilation passed.
- Physical devices, real receivers, external subtitle providers, and purchases: not run.
- Production containers: blocked by Podman VM storage exhaustion.
- Hosted CI: the first deep run passed both app suites, all six browser jobs, native Swift/Android compilation, and all four production-container builds. Its stale tooling snapshot failure was repaired through merged PR #413. The next deep run passed tooling but failed the Player WebKit happy-path console assertion described in QA-004. The third run retained the Player failure artifact and failed both WebKit jobs, as recorded in QA-004 and QA-008. Other third-run jobs passed; publication was skipped after the aggregate gate failed. Full current-main deep CI is not claimed green.
- Container publication, Nox deployment, and public TLS: not verified by this audit.

Confirmed defects will receive a failing regression before a production fix. Verified chunks will be delivered through protected-main pull requests with merge commits.
