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

The initial Player Go suite and the populated Chromium batches for both apps are running. Their outcomes are pending. No passing suite is claimed yet.

The initial custom setup harness read the enrollment secret at the wrong JSON level. The isolated Player fixture was restarted, and the harness now uses the existing `totp.secret` API response field. This was a harness error, not a product defect.

## Verification boundaries

- Source tests: pending.
- Populated Chromium browser checks: pending.
- Firefox and WebKit: not yet run.
- Native simulator tests and builds: not yet run.
- Physical devices, real receivers, external subtitle providers, and purchases: not run.
- Production containers: blocked by Podman VM storage exhaustion.
- CI, container publication, Nox deployment, and public TLS: separate from this local audit.

Confirmed defects will receive a failing regression before a production fix. Verified chunks will be delivered through protected-main pull requests with merge commits.
