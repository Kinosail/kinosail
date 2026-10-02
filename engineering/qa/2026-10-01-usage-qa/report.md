# Kinosail evening QA — October 1, 2026

## Scope and run record

The user requested useful QA work to use the remaining weekly Codex allowance before midnight in America/Denver. The initial live meter showed 87% used and 13% remaining. The goal is active. Exact consumption depends on the session; this report does not promise an exact completion time.

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

The playback matrix combines real synthetic-media journeys with controlled browser media and transport failure scenarios. It covers Direct First, blocked autoplay, resume, seeking, duration, quality, speed, buffering, and recovery. This is not a physical-device or codec certification.

The native suites used dedicated iPhone 18 Pro and Apple TV 4K simulators on iOS/tvOS 27.0. They include their own loopback and rendering fixtures. They do not establish an authenticated live Server journey, physical touch, or Siri Remote behavior. The iOS skipped case remains outside passing coverage.

Production sources did not change between baseline `75ebf1d`, report commit `680781773`, and its merge `895f627b4`. Browser servers retain the fresh baseline binaries. Native and source runs record the report revision; the initial two manifests captured the revision at completion. Later manifests capture it at launch. This distinction does not change the tested production sources.

The initial custom setup harness read the enrollment secret at the wrong JSON level. The isolated Player fixture was restarted, and the harness now uses the existing `totp.secret` API response field. This was a harness error, not a product defect.

Initial Chromium runs also exposed incomplete fixture setup: Player's long-show fixture writer and Subtitles' inspector fixture writer were omitted by an anchored test selector. Subtitles' filesystem root was missing. Those runs recorded 20 Player passes and 6 fixture failures, plus 22 Subtitles passes, 13 fixture failures, and 24 serial skips. The corrected writers and isolated filesystem root were installed before the reported browser batches. The failed initial runs remain in the evidence root.

## Findings

### QA-001 — Stale Player Code Atlas blocks deep CI (confirmed)

The manually dispatched [deep CI run](https://github.com/Kinosail/kinosail/actions/runs/36959819107) at `895f627b4248bbcc2138c75885ff2849ea91e394` failed its tooling job because the Player architecture snapshot no longer matched current source.

The existing `python3 scripts/tooling/test-architecture-explorer.py` reproduced that failure locally before changes. Regenerating both app snapshots changed only Player's generated `index.html`. Its `internal/server` data was stale; package and source-file counts remain 68 and 893.

The same regression check then passed, including its invalid-input and no-side-effect checks. No generator or production behavior changed. `architecture-red.json` and `architecture-green.json` retain the control runs. The complete root tooling suite, `make max-loc`, `git diff --check`, and Player's post-commit `make verify-changed` passed before publication of the repair. The affected-app check selected only the generated documentation path and ran its source cap and diff checks; it did not rerun the app suite.

### QA-002 — One offline playback stall (unverified)

One Chromium download journey stopped near 0.099 seconds after the network disconnected. The saved file was verified, its range probe returned the expected HTTP 206 and two bytes, and the offline video reached a usable ready state. The assertion requiring moving playback failed.

The same journey passed in the initial Chromium batch, in Firefox and WebKit, and twice in fresh Chromium repetitions after the failure. The saved trace and screenshot remain under `player-browser-corrected-results/test-instance-production-a-16c56-ter-the-network-disconnects-chromium`. The observation has not reproduced twice from a known state, so no product fix or passing full Player batch is claimed.

## Verification boundaries

- Source tests: both app suites and 60 shared packages passed.
- Populated Subtitles browser batch: all 177 checks passed across three engines.
- Populated Player library/account batch: one unverified Chromium stall; 77 other checks passed. Both fresh stall repetitions passed.
- Player playback/recovery matrix: all 228 checks passed across three engines, within the controlled-fixture boundary above.
- Native simulator suites: iOS and tvOS passed within the boundaries above; one iOS check skipped.
- Physical devices, real receivers, external subtitle providers, and purchases: not run.
- Production containers: blocked by Podman VM storage exhaustion.
- Deep hosted CI: running; tooling snapshot failure reproduced and repaired locally. Other hosted outcomes remain pending.
- Container publication, Nox deployment, and public TLS: not verified by this audit.

Confirmed defects will receive a failing regression before a production fix. Verified chunks will be delivered through protected-main pull requests with merge commits.
