# Kinosail deep QA — September 30, 2026

This audit found and fixed two confirmed defects. It does not establish that every product path or device is free of bugs.

## Run record

- Scope: Player, Subtitles, shared Go packages, Apple clients, and Android clients. Dashboard is absent from current main.
- Starting revision: `55c0b765c16eb68a403b1a8f4c76f8a71340bdad`.
- Fix commit: `b6288a48`; reconciled source and final host binaries: `d95bf65a4d4b9f3cd9462ddf489bcbce8ee1bc1f`.
- Reconciled upstream: `77494fdff25d5e97a6d1f8e87a0659eb31dc990a`.
- Environment: macOS ARM64, Xcode 27, iOS 27, tvOS 27, installed Android API 36 images, repository-pinned Playwright 1.63.
- Local evidence: `.verification/deep-qa-20260930/` in the task checkout. This directory is ignored and contains private disposable session state.
- Test data: generated fictional movies, episodes, music, audiobooks, books, photos, and a loopback metadata provider. Native captures use the existing fictional preferences fixture.
- Web instances: task-owned Go binaries on loopback HTTPS ports 38147 and 38148. These are host builds, not container or deployment evidence.
- Final binaries contain the reconciled source revision above, confirmed with `go version -m`; `vcs.modified=false`.
- Primary checkout and unrelated worktrees were preserved.

## Confirmed findings

### QA-001 — Partial metadata results trigger a refresh loop (high; fixed)

1. Configure a local provider with valid movie details and artwork.
2. Return HTTP 503 for optional credits.
3. Start either app with a movie requiring metadata.
4. Observe repeated credits, detail, artwork, persistence, and library-refresh work without another user scan.

The automatic refresh stored unchanged partial records and rescanned the library. That scan queued another metadata refresh. The disposable host fixture received hundreds of thousands of requests and eventually exhausted local connection capacity.

The shared operation now skips unchanged downloads and publication when the record is already reflected in the library. It still publishes repaired artwork and records awaiting index publication. Failed optional work remains eligible for a later real scan.

- Red: `metadata-feedback-red.log` recorded 13 self-triggered credits requests in both repetitions on the original Player build.
- Focused operation red: `metadata-unchanged-red.log`, `metadata-download-red.log`.
- A first implementation exposed a concurrent publication regression in the existing TVmaze web/API test. A baseline overlay passed five repetitions; the regression failed repeatedly in both apps.
- Additional red: `metadata-concurrent-red.log`, `metadata-stale-index-red.log`.
- Final green: `player-metadata-publication-green.log`, `subtitles-metadata-publication-green.log`, 20 repetitions per app.
- Final operation/race proof: `metadata-publication-green.log`, `metadata-final-race.log`.
- Tests cover no repeated publication, no redundant downloads, successful artwork repair, failed artwork, pending index publication, later retry, and provider recovery.

### QA-002 — Failed Supporter artwork overflows badge cells (medium; fixed)

1. Render the production Subtitles Supporter template with twenty badges.
2. Abort the badge artwork requests.
3. Check phone and desktop layouts in light and dark themes.

The implicit grid column allowed image alternative text and descriptions to exceed each badge cell. The fix explicitly constrains the column with `minmax(0,1fr)` and advances the application CSS cache version.

- Red: `badge-failure-red-confirmed.log`; three assertions failed for overflow at 390px and 1024px. A fourth failure was an instance startup race and is excluded from defect proof.
- Green: `badge-green.log`, 15 Chromium checks; `badge-cross-browser.log`, 30 Firefox/WebKit checks.
- Sizes: 1440, 1024, 720, 390, and 320px. Themes: light and dark.
- Each failed-artwork check asserts twenty failed images, child bounds inside every cell, and no horizontal page overflow, then saves a screenshot.
- Existing loaded/empty/conditional fixtures also passed. Their renderer now uses the real origin so relative artwork URLs resolve.
- The new failed-artwork regression is selected by both Subtitles browser launch scripts. The dark phone case is included in hosted smoke checks.

## Verification

| Surface | Result | Evidence and limits |
| --- | --- | --- |
| Shared Go packages | Passed full source tests, race suite, tidy, consumer compilation, and 97.7% coverage | `packages-baseline.log`, `packages-remainder.log`, `packages-final-coverage.log` |
| Changed Go code | Zero new lint findings in packages and both apps | `packages-lint-reconciled.log`, `player-lint-changed-final.log`, `subtitles-lint-changed-final.log` |
| Vulnerabilities | No reachable vulnerabilities | Scanner also reported one unused required-module advisory; see `packages-remainder.log` |
| Player Go suite | All packages passed | `player-source-final.log`; server suite 231 seconds |
| Subtitles Go suite | All packages passed | `subtitles-source-final.log`; server suite 187 seconds |
| Reconciliation | Catalog/metadata/operations checks and both app HTTP race regressions passed | `reconciled-packages.log`, `reconciled-player.log`, `reconciled-subtitles.log` |
| Source cap and shell scripts | Passed | `max-loc.log`, `subtitles-shellcheck.log` |
| Badge and conditional browser matrix | 45 passed across Chromium, Firefox, WebKit | Loaded and failed artwork; 320–1440px; saved renders |
| Player populated Chromium journey set | 61 passed, one failed | Playback, seeking/startup/bandwidth, navigation, keyboard, themes, MFA, Quick Connect, checkout and responsive views; `player-populated-final.log` |
| Player failed masthead assertion | Repeated twice; unresolved fixture isolation | Test expects Example Movie as the feature; page selects Arrival from previously populated progress. Four adjacent checks passed. See `player-masthead-isolated.log` |
| Subtitles populated host set | 24 passed, two failed, 24 skipped | `subtitles-populated-final.log`; failures are missing test-root configuration and a hard-coded canonical port |
| Subtitles dashboard follow-up | Two passed, one failed, 22 skipped | `subtitles-dashboard-final.log`; cleanup expects two files, but the generated host library also contains a Spanish VTT. Container fixture parity remains unverified |
| Exploratory web routes | 34 valid route/viewport observations without overflow, axe violations, broken images, or page exceptions | `browser-exploration.json`; two invented `/settings/subtitles` routes returned 404 and are excluded |
| iPhone simulator | 270 tests in 54 suites passed; 47 captures | `ios-baseline.log`, `ios-baseline.xcresult`, `iphone-gallery/` |
| iPad simulator | Five focused lifecycle/preference tests passed; 47 captures | `ipad-baseline.log`, result bundle and `ipad-gallery/`; not a separate full-suite pass |
| tvOS simulator | 257 tests in 52 suites passed | `tvos-baseline.log`, `tvos-baseline.xcresult`; includes rendered focus regressions, not a full remote playback journey |
| Android source | Phone, watchcore and Wear unit tests passed; debug and instrumentation APK builds passed | `android-baseline.log`, `android-emulator-build.log` |
| Android emulator | Blocked at startup | `android-phone-emulator.log`: insufficient host disk space; a build is not emulator QA |
| Containers | Blocked before build | Podman VM full; test-instance build failed creating a temporary builder directory |
| Local changed-app gates | Both stopped at the full shared lint gate | `player-verify-changed.log`, `subtitles-verify-changed.log`; 113 repository-wide findings. No gate override was used |
| Design detector | Two unchanged border-style findings retained | `badge-ui-lint.log`; owned recognition and error status use these accents |

The initial broad browser runs are diagnostic evidence, not passing suites. They included copied Player routes in Subtitles, stale numeric CSS expectations, already-initialized setup state, and connection failures during the metadata storm. Their raw logs are retained separately.

Native loaded, pending, empty, error, large-text, reader, photo, preference, approval, and casting views were inspected from populated captures. Some snapshots labelled `supporter-failed` were taken during request retries and still show pending geometry; they do not prove the terminal failure render. Physical interaction and media decoder behavior remain separate boundaries.

## Repeatable commands

Run these from the repository root or the indicated app directory. Evidence artifacts record the isolated environment and result; synthetic credentials stay local.

```sh
go -C packages test ./... -count=1
go -C apps/player test ./... -count=1
go -C apps/subtitles test ./... -count=1
go -C packages test -race ./operations ./metadata -count=1
make -C packages coverage test-race vuln consumer-test
make max-loc
make -C apps/player verify-changed
make -C apps/subtitles verify-changed
```

From `apps/subtitles`, generate fixtures with `KINOSAIL_UI_FIXTURE_DIR=<directory> go test ./internal/server -run TestWriteUIStateFixtures -count=1`. Set the local HTTPS URL and fixture directory, then run:

```sh
KINOSAIL_BROWSER_MATRIX=full pnpm --dir e2e exec playwright test \
  supporter-badge-layout.spec.ts conditional-states.spec.ts --workers=1
```

Apple runs used task-owned simulators, `xcodebuild test`, isolated DerivedData, saved result bundles, `-parallel-testing-enabled NO`, and `-jobs 2`. The iPhone gallery used `TEST_RUNNER_KINOSAIL_POLISH_GALLERY=1`; iPad used the same build with `test-without-building -only-testing:Kinosail-iOSTests/NativePreferencesUXTests`. Task simulators and rebuildable DerivedData are removed after captures are preserved.

## Remaining work

- Reclaim storage with authorization, then run container startup/media/API checks and the supported populated browser matrix against exact images.
- Complete Android phone, TV, tablet and Wear emulator journeys, plus Apple TV remote and watchOS interaction journeys.
- Reset host fixture state and resolve the remaining masthead/cleanup/canonical-origin assertions against supported isolated data.
- Exercise physical codecs, HDR, hardware acceleration, AirPlay, casting, PiP/background playback, long playback, and paid activation on suitable devices.
- Record required GitHub checks, merged source ancestry, image publication, and any later deployment as separate evidence.

The requested deep QA goal remains active. Container and Android runtime work await storage recovery; this report is a delivery checkpoint for verified fixes.
