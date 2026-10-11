# AI-assisted QA campaign — October 11, 2026

## Method and scope

The user requested web research, comprehensive QA, fixes, and repeated delivery through protected main.
The baseline is `655b7d6db39028db19cad3a1e8dd84dbd77fb936`.
The audit covers Player, Subtitles, shared packages, tooling, Apple clients, and Android clients.
Production, hosted browsers, local browsers, simulated platform APIs, simulators, and physical devices are separate evidence boundaries.

Primary-source research informed the existing repository QA skills:

- [Playwright Test Agents](https://playwright.dev/docs/test-agents): explore and plan journeys, then generate executable assertions. Preserve failures; a healer's skipped test is not acceptance.
- [Vercel dogfood](https://github.com/vercel-labs/agent-browser/blob/main/skill-data/dogfood/SKILL.md): reproduce findings and retain screenshots, traces, and exact steps as work proceeds.
- [Browserbase UI testing](https://github.com/browserbase/skills/blob/main/skills/ui-test/README.md): combine exploratory testing with change-focused adversarial checks.
- [Callstack agent-device](https://oss.callstack.com/agent-device/docs/introduction): native interaction needs device tooling and observable outcomes. A compiler result cannot establish native interaction.
- [Anthropic agent evaluations](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents): assess actual outcomes, isolate trials, inspect failures, and prefer deterministic checks where possible.

Application of these methods uses the pinned local Playwright, actual disposable Go Servers, generated media, and existing native suites.
No upstream skill collection or external paid QA service is required.
The existing audit's uncovered cases guide additional execution, rather than treating a collected or skipped test as a passing journey.

## Current evidence

Private artifacts are under `.verification/ai-qa-campaign/`, `.verification/scrub/`, `.verification/startup/`, and `.verification/apple-mobile/` in this task's worktree.
The outer `scripts/ci/e2e-artifact.py` receipts record revision, tracked diff hash, command, environment, and checksums.
Inner runners retain generated-media hashes, browser evidence, and process results.
Raw logs, authentication state, and traces remain private.

- Local `make max-loc` and `make tooling-check` passed. The CI contract subset executed 169 tests with five environment-dependent skips.
- The original scrub journey passed in Chromium and WebKit at 390, 1440, and 1920 pixel widths. Its real media previews decoded and reused frames; seek and accessibility assertions passed.
- Firefox failed before navigation. Two standalone launcher probes reproduced the missing-profile error. Separate temporary directories did not resolve it.
- Baseline Apple-policy runs failed twice at the same final error-collector assertion after the playback actions completed.
- Baseline startup runs failed twice at the source-invalidation preparation assertion.
- [Hosted deep CI 38151255345](https://github.com/Kinosail/kinosail/actions/runs/38151255345) passed on the baseline after the native job was rerun. The coverage reconciliation below separates distinct cases, repeat executions, skips, and the original native failure.

## Findings and regression ownership

### AQA-001 — Apple-policy journey compares a function with an array

The observable contract is real Server setup, launch failure recovery, decoded playback, seeking, captions, and repeated portrait/landscape loads without unexpected browser errors.
`captureHappyPathErrors` now returns an `errors()` method.
The opt-in Apple-policy journey still compares the method itself with `[]`.
Two fresh baseline runs fail at that assertion with `[Function errors]`.

The existing journey is the primary regression owner. Call the collector and retain the empty-error assertion.
The repaired journey passed with its empty-error assertion intact. No extra unit test or production seam is needed.
The fullscreen API remains simulated in Chromium; this is not physical Apple or native Safari acceptance.

### AQA-002 — Startup invalidation depends on the host's HEVC negotiation

Two fresh local runs reach the invalidation phase, then observe `queued` instead of `ready`.
Safe Server projections show repeated preparation cancellation at the existing 12-second work limit.
The fixture is 64 seconds long. On this Mac, negotiation chooses copied HEVC with audio conversion.
That path requires complete output before readiness and reads speculative input at four times playback speed.
Its minimum input time exceeds the preparation budget.

The source-invalidation case should explicitly request the bounded H264 conversion already used by the Linux browser lane.
Keep its readiness deadline, cache invalidation assertions, and real media.
Keep copied-HEVC completion, cancellation, and adoption with the distinct public owner, `test-hls-followon.py --suite hevc`.
The repaired journey passed, including readiness and stale-source rejection. This is a fixture-selection repair, not permission to relax production preparation limits.

### AQA-003 — Local Firefox launch is an environment boundary

The failure matches an [open Playwright report](https://github.com/microsoft/playwright/issues/42768) and [Mozilla's headless startup report](https://bugzilla.mozilla.org/show_bug.cgi?id=2062988).
Those reports describe protected Firefox application data on macOS 27.
The local failure is confirmed; its exact operating-system authorization cause is inferred.
Do not alter browser permissions, personal profiles, or security controls to make a test pass.
Hosted Linux Firefox can provide a separate engine result.

## Reproduction receipts

Commands use `python3 scripts/ci/e2e-artifact.py --output <new-directory> -- <journey>`.
The Apple journey is `python3 apps/player/scripts/test-apple-playback-local.py`.
The startup journey is `python3 apps/player/scripts/test-startup-local.py`.
Each fixture generates media and a fresh Owner account on loopback. No production data is used.
Baseline and repeated failures used the base revision above. Passing receipts bind the pending patch and report unchanged source during execution.

| Private receipt under `.verification/ai-qa-campaign/` | Result | Manifest SHA-256 |
| --- | --- | --- |
| `apple-launch-baseline/manifest.json` | failed | `66bb9db1333e95ddc6564479a794196b88c5d3fc5e75ec73e92a646e78a06ea3` |
| `apple-launch-red2/manifest.json` | failed | `b476fb218d79aade29e2e981e7073577eaf9f7b3beece27cf539c163aac7f529` |
| `apple-launch-green/manifest.json` | passed | `adca4873a9a476a662e8af3b68b413f45204d985d64c061a5b5ea7b289d8c156` |
| `startup-baseline/manifest.json` | failed | `38d2a5751d7d145b45f124e97a91df35abd29d5443564084c84466b06cb27dd2` |
| `startup-red2/manifest.json` | failed | `8185e418fc1cc75598456a215548496229194ebf6816fc816540088fcfe02ea9` |
| `startup-green/manifest.json` | passed | `1f0bee9d62bb1357f2182774fca72312c207510dcd4fcdbdf25a1a1153eddfb9` |

### AQA-004 — Open: copied HEVC projects a nonexistent tail fragment

`python3 apps/player/scripts/test-hls-followon.py --suite hevc` passed complete and cold playback locally.
Interrupted and adopted preparation both failed with `control_fragment_http_404`.
The source container duration is 10.005 seconds; the early public VOD projection advertises six fragments.
The completed physical playlist contains five fragments, ending at 10.091 seconds after audio encoding.
The projected sixth fragment is never produced.

A second playback run reproduced both failures. That diagnostic run allowed 120 seconds for Server health before the unchanged authorization and playback journey.
Two intervening ordinary runs failed before health at the 30-second fixture limit.
A process sample located that delay in localization template initialization under concurrent native load.
No playback deadline or fragment assertion changed.
The second receipt is `.verification/hls-hevc-startup/20261011T154819Z/receipt.json`, SHA-256 `963e86655fe0ea71eef0bab0f1dd8a804d47470b54b05d49da1d28e392378378`.

Hosted Linux passes all four HEVC cases, but its container duration is exactly 10 seconds.
That result does not disprove the fractional-duration failure.
A repair needs a valid media timeline; blindly rounding away a real final fragment or increasing the fragment timeout would hide the defect.

### AQA-005 — Avoid catalog lookups for absent text

Two ordinary HEVC runs exceeded the existing 30-second Server-readiness limit before producing a log line.
A private process sample located CPU work in `Catalog.translateText` during template initialization.
The host was also running other native builds and simulators, so these observations do not establish a general startup-time guarantee.

The repair checks whether the current text contains the source phrase before looking up its translation.
It preserves message order, word boundaries, fallback rules, and replacements that introduce another cataloged phrase.
Existing localization contracts own these behaviors; no duplicate unit test or production seam was added.

A separate private probe applies all 108 supported languages to 35 real Player template literals.
Its 3,780 outputs have the same SHA-256 before and after the change: `7cd9ca4c56b2baa1e43cc2cccb8773ab474621bf86ab7286cfb89bfeb35fc142`.
The measured translation phase changed from 2,512 ms to 531 ms on this host.
The probe is `.verification/ai-qa-campaign/localization-probe.go`, run with `go run .verification/ai-qa-campaign/localization-probe.go "$PWD"`.
This isolated translation probe is not an end-to-end startup benchmark.

`go -C packages test ./localization -count=1` passed.
Both apps passed `go test ./internal/server -run '^Test.*(Lang|Localiz|Spanish|Arabic)' -count=1 -json`: 157 Player cases and 228 Subtitles cases, including subtests.
The real startup journey passed after the repair with unchanged playback deadlines and assertions.
Its outer receipt is `startup-localization/manifest.json` under the private campaign directory, SHA-256 `d2d56b4f76ee4335f10b7a4147c8fefdcf2afb00f57c8173c7951aa7aaa83f84`.
A later ordinary HEVC run timed out before any cases began under extreme concurrent host load. It provides no additional playback result.
The broad local Player `verify-changed` was stopped during input hashing after maximum-line and diff checks passed. Its shared-package stages remain unverified locally. Hosted required checks remain mandatory.

### Additional layout and native evidence

The existing `TestWriteShowActionFixtures` exporter and `show-action-layout.spec.ts` passed 12 cases across Chromium and WebKit.
They cover six widths from 320 to 1920 pixels and long, unbroken, watched, and empty states.
Keyboard focus and accessibility assertions passed. The 320-pixel WebKit and 1920-pixel Chromium screenshots were inspected.
These are rendered-fixture checks, not populated-server E2E evidence. Firefox remains a local launch boundary.

The hosted iOS failure is `MediaCardRatingTests/testVideoCardsRenderIdenticallyWithAndWithoutRatings`: rated-image PNG encoding returned nil.
The local focused retry on the reusable Kinosail iPhone failed before tests because simulator boot exceeded 60 seconds.
That infrastructure failure neither reproduces nor resolves the hosted assertion. No native code was changed.

## Delivery checkpoints

[PR #550](https://github.com/Kinosail/kinosail/pull/550) merged the two QA journey repairs after Player, Subtitles, Repository, and Security checks passed.
A fetched ancestry check confirmed reviewed commit `07d4f5fe528a7bdca19353b3e796c6a4aad764a2` in `origin/main`.
[PR #551](https://github.com/Kinosail/kinosail/pull/551) merged the localization optimization after all four required checks passed.
A fetched ancestry check confirmed reviewed commit `f64bb55a5b1a3c60b7e528381a61425f6de4f65e` in main `8e937334fbbc0f79915f6f093d8aed268481de64`.
The main-push run then encountered the separate executable-fixture failure described below. Merge and publication remain separate facts.

## Hosted coverage reconciliation

[Deep baseline run 38151255345](https://github.com/Kinosail/kinosail/actions/runs/38151255345) completed successfully after the failed native job was rerun.
Its exact source is `655b7d6db39028db19cad3a1e8dd84dbd77fb936`.
The original iOS failure remains recorded; a passing retry does not prove that intermittent PNG failure is repaired.
The retry passed 323 iOS cases with one skip, and 300 tvOS cases with one skip.
Android application assembly and JVM tests passed. These are not Android emulator or hardware interaction results.
Both apps passed AMD64 and ARM64 production-container checks and scans on this baseline.
Scheduled-only checks were not selected by the manual deep run.

The browser artifact reconciliation deduplicates `(spec file, test title)` across the main and focused reports.
Repeated executions are not additional coverage. The numbers below apply only to this hosted baseline:

| Application / engine | Distinct observed cases | Passed at least once | No passing execution |
| --- | ---: | ---: | ---: |
| Player / Chromium | 1,069 | 791 | 278 |
| Player / Firefox | 1,069 | 787 | 282 |
| Player / WebKit | 1,069 | 785 | 284 |
| Subtitles / each of the three engines | 99 | 99 | 0 |

The Player remainder includes opt-in populated journeys and layout audits, not observed failures for every case.
The private reconciliation is `.verification/ai-qa-campaign/hosted/player-coverage.json`.
Later focused executions must be reconciled separately before reducing these gaps.

[Layout run 38155749187](https://github.com/Kinosail/kinosail/actions/runs/38155749187) uses exact source `f64bb55a5b1a3c60b7e528381a61425f6de4f65e`.
Firefox and WebKit passed their quick populated Player and Subtitles journeys with no source drift.
Firefox used loopback HTTP. WebKit used trusted fixture HTTPS inside the disposable Linux runner.
The separate rendered-fixture layer passed 35 Firefox cases and 33 WebKit cases; WebKit skipped two.
Chromium also passed: 95 Player and 111 Subtitles page measurements, plus 11 Player and 26 Subtitles flows.
Its separate rendered-fixture layer passed 33 cases and skipped two.
The narrower Firefox and WebKit runs each recorded 40 Player and 46 Subtitles page measurements with the same flow counts.
These synthetic accounts and media do not prove production TLS, deployment, or physical-device behavior.

Additional authenticated browser evidence on the same source passed ten browse-return safety cases and three Home-return cases.
The safety cases reject external origins, malformed destinations, invalid query state, excessive extents, and cross-profile or cross-tab state reuse.
The Home cases restore the selected action, focus, and scroll after visible or native Back navigation.
Their runs are [38155212904](https://github.com/Kinosail/kinosail/actions/runs/38155212904) and [38155635822](https://github.com/Kinosail/kinosail/actions/runs/38155635822).

The historical Subtitles protocol driver stopped at source preflight before compiling or running tests.
Its pinned protocol inputs no longer match the current source. This is an unexecuted proof, not a product failure.
Current-source Save controls, headers, and body suites passed on `8e937334fbbc0f79915f6f093d8aed268481de64`.
Browse-return primary, cold, HTMX, Shows, and Search suites also passed on that revision without source drift.
All three Restore drivers stopped during checkout preflight before compiling or running their public contracts.
[The bounded run ledger](hosted-gaps.json) records each exact revision, run, outcome, artifact ID, and archive checksum.
Those twelve runs contain eight passing suites and four blocked proofs; blocked prerequisites are not passing coverage.

The BFCache proof reached browser navigation but reported `prerequisite-blocked`.
Chromium reported `response-cache-control-no-store`, `masked`, and `other` reasons for not retaining the page.
The requested persisted-document path was not established. The authenticated response's cache policy was not weakened.
Run: [38156964354](https://github.com/Kinosail/kinosail/actions/runs/38156964354).

### AQA-006 — Open: copied-video resume starts before the requested frame

Two complete authenticated HLS runs on `f64bb55a5b1a3c60b7e528381a61425f6de4f65e` each passed ten of twelve cases.
The MKV and MP4 non-keyframe resume cases both failed `public_full_timeline`, `presented_source_frames`, and `presentation_duration`.
Their requested source offset is 12.5 seconds. The delivered video starts at the preceding 12-second keyframe.
The public timeline is about 20 seconds instead of the expected 19.521 seconds.
The existing frame-identity and presentation-time assertions remain unchanged.
Runs: [38155215927](https://github.com/Kinosail/kinosail/actions/runs/38155215927) and [38155904315](https://github.com/Kinosail/kinosail/actions/runs/38155904315).

FFmpeg documents that input seeking with stream copy preserves frames between the earlier seek point and the requested position.
Its automatic timestamp shifting can then make that earlier content appear at zero. [FFmpeg seeking](https://www.ffmpeg.org/ffmpeg.html), [timestamp shifting](https://www.ffmpeg.org/ffmpeg-formats.html)

A private mux experiment used the existing 30-second scrub fixture and independently decoded its requested 12.5-second suffix.
The reference contains 420 frames. All four mux variants retained 480 frames.
Disabling timestamp shifting, enabling edit lists, and enabling negative composition offsets did not establish correct presentation.
Those variants retained 60 negative-timestamp frames; the oracle did not silently discard them.
No production playback change was made from this experiment.
A follow-up compared ordinary and fragmented MP4 using the same edit-list option.
Ordinary MP4 decoded the exact 420-frame suffix. Fragmented MP4 retained 480 frames, including 60 negative-timestamp frames.
In a private Chrome probe, direct fragmented MP4 matched four independently requested reference images but reported 20 seconds instead of 17.5 seconds.
The production Hls.js library stalled before displaying a frame from the edit-list HLS candidate.
The unchanged HLS baseline displayed different images at all four requested times.
This rules out adopting that mux-option change from decoder output alone; native-client acceptance also remains open.
A direct MediaSource append without Hls.js also timed out before displaying its first frame, even with the duration set to 17.5 seconds.
The byte-stream specification describes single-entry edit-list support, but this candidate did not establish working presentation. [W3C ISO BMFF initialization segments](https://www.w3.org/TR/mse-byte-stream-format-isobmff/#iso-init-segments)
The browser experiment receipt is `.verification/ai-qa-campaign/preroll-browser-probe.json`, SHA-256 `15a04357465a87fc31ebb7a04c4cd0e0375b7c400586a7d1c47d39ce400f43e8`.
The receipt is `.verification/ai-qa-campaign/preroll-probe/receipt.json`, SHA-256 `c1b2332c740d4a7ada6daa1df1ab350a67d8eee2665a23d0e78a54dae8475f7c`.

A separate private interactive timeline prototype compared container-duration rounding, video-duration rounding, and actual segment starts.
Its fractional-tail and uneven-keyframe counterexamples rule out blindly rounding HEVC duration as a general repair.
The HTML was opened in Codex but was not visually verified. It is a design experiment, not playback acceptance.

### AQA-007 — Live browser verification must not reuse a source-only cache hit

The public command `verify-changed.sh` cached `e2e-changed` using source files and command arguments.
An already-running Server can change independently of those inputs, so the next command could report success without running its browser journey.
The new command-level regression reproduced this false success twice before the repair.
It runs the real verifier in a disposable Git repository and changes the stand-in browser target's outcome between invocations.
This is isolated tooling evidence, not a populated product E2E journey.

Live browser journeys now use the required-stage path on every invocation.
Source-only test collection retains content-based caching, including invalidation when bytes change but size and modification time stay unchanged.
The input hasher now reads all stage inputs in one Python process, rather than launching one `shasum` process per file.
The new key format also separates command arguments and starts a new cache namespace.

A bounded local measurement hashed the same first 100 shared inputs before and after the optimization.
The old process-per-file loop took 10.982 seconds; the new process took 0.225 seconds.
All 1,251 shared inputs took 0.720 seconds with the new hasher.
These measurements describe this loaded Mac, not a universal performance guarantee.
The regression passed after the fix; deleted-spec planning and shell lint passed too.
The complete tooling check remains a separate delivery gate.
Its first local attempt failed an unrelated R06 shell-routing probe with empty output.
Fifty direct repetitions of that probe passed without a source change; the original failure is retained.
The second attempt passed all 169 CI contracts with five environment skips, then timed out starting Node in an unrelated gate-pause fixture.
Neither full local tooling attempt is reported as passing.
The final verifier regression passed after the hashing format was finalized. `make max-loc` and `git diff --check` passed.
The implementation checkpoint is `6937787141c50461dc6adbe68d564a85c0fc654e`; hosted required checks remain mandatory.
The first PR run passed the new verifier regression and failed later on stale Code Atlas snapshots.
Both snapshots were regenerated from the current source, including localization and previously merged metadata changes.
The generator's input rejection, canonical platform, and snapshot checks then passed locally.
The corrected run passed all four required gates; [PR #553](https://github.com/Kinosail/kinosail/pull/553) merged.
A fetched ancestry check placed reviewed head `595d2a0cbcaaffc07ed7396b79c47a187bd59891` in remote main `6f8bfa60d46d5c53621d5590ace3e943669d53ad`.
Run: [38158260089](https://github.com/Kinosail/kinosail/actions/runs/38158260089).

### AQA-008 — Merged: executable-fixture launch race

The main-push shared-package job failed `TestFingerprintCacheAndToolFailuresAreDeterministic` before its first fingerprint assertion.
Linux returned `text file busy` while launching its newly written, test-owned shell executable.
The required PR checks had passed on the reviewed source. No marker production source changed in this delivery.
The failure is retained as an environment or fixture-lifecycle finding pending reproduction; it is not attributed to localization.
Run: [38156244208](https://github.com/Kinosail/kinosail/actions/runs/38156244208), job `114519746603`.
A retry passed without a source change. Main CI and both production-image promotions subsequently completed successfully.
That establishes publication of `8e937334fbbc0f79915f6f093d8aed268481de64`, not a repair of the intermittent fixture failure or proof of deployment.

The focused Linux ARM64 reproduction ran the three fingerprint fixture tests 200 times each on `ad990e35c166aeacc254a37b9457f2ab6a28d072`.
It passed 593 executions and failed seven, including the original `text file busy` error.
The other six failures occurred at the first visual-tool invocation, whose production boundary hides the underlying process error.
These tests write executables while parallel tests launch processes. This exposes the descriptor inheritance race documented by [Go issue #22315](https://github.com/golang/go/issues/22315).
The repair uses checked-in executable fixtures. Each test retains its own temporary media, cache, and argument log.
The identical Linux command then passed all 600 executions. The full local markers package also passed with the Go race detector.
No assertion, retry, deadline, or production code changed.
These isolated tests retain dependency-error coverage: empty fingerprint output, malformed visual output, cache reuse, extraction failure, and bounded background-tool arguments.
Populated-server journeys cannot deterministically force each external tool output or filesystem failure.
The private receipt is `marker-fixture-receipt.json`, SHA-256 `759cbfd66ed2314fe19657042ba240bff2254fcf2bf6d0f73d4c06efcd74e4fe`.
It records both binaries, exact commands, immutable image identity, source hashes, and private log hashes.
ShellCheck, the source-file cap, and lint for the changed lines passed.
Full markers lint reports six pre-existing complexity findings in unchanged files; these remain outside this fixture repair.
Hosted CI uses its existing change-based lint policy and remains required.

### AQA-009 — Subtitles phone navigation splits an English word

Independent populated Firefox and WebKit screenshots at 390 pixels show the bottom navigation rendering `Overview` as `Overvie` and `w`.
The other three destinations fit on one line. This remained visible despite passing overflow and geometry assertions.
The Player detail and Subtitles inspector screenshots were also inspected at 390 pixels.
The populated audit gained a text-line assertion before the production change.
Its unchanged-source baseline failed five 320-pixel cases with split navigation labels.
Removing horizontal link padding leaves room for each English label while preserving the existing 48-pixel touch target.
The repaired populated run passed 45 layout cases and 26 interaction flows, including doubled text, delayed assets, search failures, background discovery, and keyboard focus.
It recorded zero unreadable labels and no source drift.
The source was `595d2a0cbcaaffc07ed7396b79c47a187bd59891` plus the recorded navigation assertion and CSS patch.
The command was `KINOSAIL_LAYOUT_APPS=subtitles KINOSAIL_LAYOUT_QUICK=1 KINOSAIL_LAYOUT_ENFORCE=1 KINOSAIL_LAYOUT_VARIANTS=1 KINOSAIL_LAYOUT_FLOWS=1 python3 scripts/testing/test-layout-stability-local.py`.
Its private receipt is `.verification/layout/20261011T172259Z/receipt.json`, SHA-256 `f93cadb57941e6a43d37fa72ad9488008080bca0159e3e61d099e13edcc9c0ee`.
The measurements hash is `423f1702795b6453b4af55d75480de07de535220379fee9602feb8447439d470`.

The existing inspector fixture separately exercised pending, loaded, empty, and failed states across seven widths and two themes.
WebKit passed all 14 cases. Chromium passed 13 and timed out reloading the 390-pixel light case.
A focused rerun passed with unchanged source and the original timeout. The first failure remains retained.
Both fixture runs include accessibility assertions at 390 and 1440 pixels. These use rendered production assets with controlled API responses, not a populated Server.
All four 320-pixel dark-state screenshots were inspected; navigation stayed readable in each state.
The first hosted Linux rerun passed all 95 Player and 111 Subtitles Chromium layout cases plus 37 combined flows.
Firefox and WebKit still split `Overview` at 320 pixels with doubled text; the taller dock also obscured settings headings.
The same Firefox run separately recorded Player home movement and a Subtitles background-discovery navigation timeout.
Those failures remain retained. [Hosted run 38160404212](https://github.com/Kinosail/kinosail/actions/runs/38160404212).
A diagnostic using the container's DejaVu Sans Bold fallback reproduced the narrow label.
Reducing outer horizontal padding to `.25rem` then kept every label on one line in all four width/text-scale combinations.
Four fresh inspector cases at 320/390 pixels and both themes passed with that production CSS.
The final Linux Firefox and WebKit runs passed on `78d8fbb06aeb68646afc5c08bb4c8d2c36e52a1b`.
Each passed 40 Player and 46 Subtitles layout cases, plus 11 Player and 26 Subtitles flows.
Neither reported split dock labels, covered bookmark headings, overflow, or another enforced geometry failure.
The 320-pixel settings screenshots were inspected in both engines.
Chromium also passed 95 Player and 111 Subtitles cases plus the same 37 combined flows.
[PR #554](https://github.com/Kinosail/kinosail/pull/554) merged after all four required checks and all three layout jobs passed.
Fetched ancestry proves the reviewed commit is included in `b38a31a651d0b4c4549799ee8fc5080eb401d84c`.
The container run's separate trust failure is recorded below.
Private screenshots under `hosted/visual-review/` have these SHA-256 values:

- `subtitles-firefox-390-library.png`: `95c3f6a427aad88a48d62e59ae3bb0541935cbcaa4a83ab85b2b7a9c71881313`.
- `subtitles-390-default--view-library-settled.png`: `bb6324ce459fdc90e73f7734411e595ae586ef0e5258e6acad58b92896d42844`.

### Additional bounded campaign execution

Q47's two primary generated-docs deadline cases passed twice on `595d2a0cbcaaffc07ed7396b79c47a187bd59891`.
The proof retained source, dependency, binary, generated-output, and process-cleanup checks.
Its other fixed suites remain unrun. [Hosted run 38159857883](https://github.com/Kinosail/kinosail/actions/runs/38159857883).
The archive is `q47-primary.zip`, SHA-256 `7c8a3d07e3b3e933b53437295cf96192250d61d423403de5a7b0a7237c8426ab`.

Q09 compiled successfully and its isolated browser command exited zero, but result admission failed with `ValueError`.
No group was accepted, and its real Server and phone groups did not run.
### AQA-010 — Download proof rejects its expanded case selection

Fresh collection found 12 isolated cases where the validator expected five, and 47 mixed-group cases where it expected eight.
The existing count contract failed before the title-list repair. Exact collection reconciliation, nine admission checks, and 19 routing contracts then passed.
[PR #555](https://github.com/Kinosail/kinosail/pull/555) preserves strict rejection of missing, duplicate, unknown, skipped, retried, flaky, and malformed results.
The corrected [hosted run 38160742860](https://github.com/Kinosail/kinosail/actions/runs/38160742860) passed all groups on `8989a927cbb763f7c985088d936c600e36a5e9e0` with unchanged source and settled processes.
Its 60 executions comprise 12 isolated controls, nine real-server cases plus 38 isolated controls, and one populated phone-viewport case.
This does not establish media decoding, other engines, or physical-device behavior.
The corrected archive is `q09-download-fixed.zip`, SHA-256 `420a7616450105329be4a6d7d7ad25d02e10de14fb679506b792465698594c1c`.
PR #555 then merged with all four required checks passing.
Fetched ancestry proves its final reviewed head `747166202d3edd17096cd646df1ab0c08414e503` is included in `53d2a6981119be691fbc576ec15d15a7d3b11232`.

The original rejected run remains preserved:
[Hosted run 38159857532](https://github.com/Kinosail/kinosail/actions/runs/38159857532).
The archive is `q09-download.zip`, SHA-256 `788808aa9b969a61fea2772d7f0703340466cdea06004c6d863580db70bcd3ac`.

### AQA-011 — Merged: automated container browser fixture transport

`make -C apps/subtitles test-instance-check` built its ARM64 image and isolated populated Compose instance.
The browser run passed 19 cases, skipped 27, and failed 20. Eighteen failures explicitly report certificate trust errors.
The other two history cases observed missing stylesheet layout; their exact cause remains separate until the transport is repaired and rerun.
Browser certificate validation remained enabled. No host trust or personal browser settings were changed.
The task-owned Compose project had zero containers and zero volumes after cleanup.
The private receipt is `subtitle-nav/test-instance-receipt.json`, SHA-256 `11141efdd155c99f3a60dcfb32b13b4e87ce1d34b6a10593e13ef46a15993e52`.
These passing fixture cases do not establish populated container-browser acceptance.

The reproduced public-command failure precedes the transport change.
The automated Subtitles fixture now selects the app's existing HTTP mode on its fixed loopback bind.
Its URL helper follows the supported TLS environment setting. Manual instances retain their HTTPS default.
Explicit TLS remains available; no browser trust bypass or host CA installation was added.
The rerun must establish browser behavior, including the two history failures, before accepting this repair.
Local HTTP evidence cannot establish TLS or WebKit cookie behavior; hosted WebKit retains disposable-runner certificate trust.

The first HTTP run on `12c5208e` passed 36 cases, skipped 27 serial dependents, and failed three.
The history stylesheet cases passed. The API client rejected Secure cookies on `127.0.0.1`, unlike its supported HTTP `localhost` exception.
A passkey redirect also used the internal port instead of the assigned host port.
Changing the browser origin to `localhost` cleared the first seven dashboard cases on `3c8fe374`.
That run was stopped before the known redirect failure to correct its startup configuration; its 59 unexecuted cases are not acceptance.
Both task-owned Compose projects had zero containers and volumes after cleanup.
The fixture now pins its assigned port and sets its canonical login URL before Owner setup.
Browser assertions, deadlines, and certificate validation remain unchanged.
The complete run on `e39981f01d8d54af4eb75a25a654e44ff174c075` passed all 66 browser cases, with zero failures, skips, or flaky results.
This suite combines populated journeys with controlled rendered fixtures; it is not 66 independent populated-server proofs.
The command was `KINOSAIL_TEST_IMAGE_READY=1 KINOSAIL_E2E_ARTIFACT_DIR=<private-artifacts>/canonical-browser GOMAXPROCS=2 make -C apps/subtitles test-instance-check`.
It reused the verified ARM64 image built earlier in this repair; the app and shared Go sources were unchanged.
The working tree was clean, and cleanup left zero task-owned containers and volumes.
The private receipt is `fixture-transport/canonical-receipt.json`, SHA-256 `cdaa05fe3879bd877d0d0aa9a9f8851be6d41819a57899d31a8888986f031530`.
[PR #556](https://github.com/Kinosail/kinosail/pull/556) merged after required hosted checks passed in [run 38164934817](https://github.com/Kinosail/kinosail/actions/runs/38164934817).
Fetched ancestry proves task head `6889ff9f2e44ae5ebf0140fb44cf591265655ceb` is included in remote main `ad990e35c166aeacc254a37b9457f2ab6a28d072`.
Reconciliation with current main added only the already reviewed Q09 validator and contract changes; Subtitles runtime and fixture inputs were unchanged.

### AQA-012 — Open: intermittent prompt-return bound on main

Main CI on `6f8bfa60d46d5c53621d5590ace3e943669d53ad` observed a legacy admission request taking 336.731493 ms against its existing 250 ms assertion.
The failed test was `TestSubtitleExclusiveOperationBlocksAllLegacyWriteAndAudioPathsWithoutEffects`.
Ten focused local repetitions passed with unchanged test and production code; the failed hosted job also passed on retry.
Neither result establishes a root-cause repair. The original failure and a bounded diagnostic receipt remain private.

### Further copied-resume diagnosis

The direct MediaSource probe buffered only 2.5–17.5 seconds when negative preroll was shifted to zero.
Keeping its timestamps positive restored all four sampled reference images.
This is consistent with [MSE coded-frame processing](https://www.w3.org/TR/media-source-2/#sourcebuffer-coded-frame-processing), which can drop frames before the append window and require a new random-access point.
The actual Hls.js configuration still failed with the experimental edit list.
A separate Hls.js probe using the true 10-second keyframe origin, a 2.5-second relative start, and a 12.5-second global seek matched all four reference images and the 30-second source duration.
The existing requested-offset mapping matched none and exposed 32.5 seconds.
A subsequent probe compared all 420 independently requested reference images; every image matched with the true keyframe origin.
Its receipt is `preroll-hls-full-frame-probe.json`, SHA-256 `20acf22922b55e6a3201f512cd00cde5b2cf242a1ce5b39513ad5bb0bd32fb5c`.
This is private synthetic browser evidence, not a production repair or native-client acceptance.

The failed edit-list candidate contained an unsigned negative decode timestamp in its first `tfdt` box.
Removing `frag_discont` produced a zero decode timestamp and matched all 420 reference images, but Hls.js reported `bufferAppendNoProgress`.
A separate 32-second fixture with B-frames and changing AAC tones matched seven sampled reference images and played through its end.
It still reported a buffer-progress error; parsed fragment times differed from the browser's actual buffered ranges.
Therefore this mux-only candidate remains experimental. It does not establish correct segment scheduling or native playback.
The playback receipt is `preroll-hls-audio-play-probe.json`, SHA-256 `0619ab672ca00642a6e249c48f32d5105e1a1f69d4d996e0f4976e3ed2369d8e`.
[HLS start offsets](https://www.rfc-editor.org/rfc/rfc8216.html#section-4.3.5.2) may express preroll suppression, but client behavior still requires independent proof.

### Prepared copied-resume repair

The retained real-browser baseline confirms AQA-006 on `c5fd62edfd3201fbc15795d3073a658ba1f61c62`.
Chromium and WebKit each display incorrect images at all seven requested positions after a cold 12.5-second resume.
Both also expose the wrong duration. These cold-stream failures remain open.

The prepared repair selects the preceding certified source key while retaining the requested recipe identity.
`X-Kinosail-HLS-Origin` gives the web adapter the actual key origin.
Hls.js reads it before loading the manifest; native HLS reads it through an abortable HEAD request.
Malformed or future origins stop loading before media requests and offer a retry at the retained position.
Native metadata completion clears its temporary clock before applying the real resume seek.
Prepared indexing declines sources whose demux origin is unknown or nonzero.
Cold streams retain their existing behavior; this repair adds no foreground whole-file scan.

Native failure injection reproduced two additional problems: Retry restarted at zero, and the temporary clock suppressed the actual seek.
The corrected native journey passes invalid metadata, a future origin, HTTP 503, a pending seek, and switching to Direct Play.
Failure and loaded screenshots cover 390, 1440, and 1920-pixel widths.
Failure clears pending skeletons and identifies compatible playback correctly.
These are actual Player controls with injected HTTP metadata failures, not unmodified-network E2E proof.

### AQA-013 — Copied refill discards audio preroll and shifts its clock

An independently generated uninterrupted tail passed all seven native frame comparisons.
A regenerated tail retained the preceding image when seeking near EOF, although normal playback reached the correct final frame.
Packet inspection found that the existing refill discarded leading AAC packets, then shifted remaining audio earlier than video.
Removing only that shift repaired EOF but left an 85.334 ms audio gap at a refill boundary.
The independent review caught that gap; passing video images alone did not qualify the candidate.

Refills limit `-copypriorss` to video and keep both tracks on the common mux clock.
Initial preparation still trims both tracks so earlier audio does not redefine the browser's timeline origin.
Applying video-only trimming to initial preparation made both MSE engines display seven incorrect images.
The retained failed receipt records that rejected candidate; the initial/refill distinction repairs it.
The existing prepared suite then exposed a second limit: MKV seeks can return an entire earlier audio cluster.
Unbounded preroll duplicated about two seconds of audio at a refill boundary.
Later fragments now discard only packets before the certified mux delay using an audio bitstream filter.
Retained packets keep their timestamps and payloads; the filter does not shift audio independently.
The retained regression populates all ten fragments, verifies audio continuity, deletes the existing final fragment, then plays the real watch page.
It requires exact original-frame images, correct startup position and duration, unchanged initialization bytes, and a restored tail.
It also checks every audio boundary within the existing 50 ms tolerance and compares final audio and video endpoints.
Chromium, WebKit MSE, and native WebKit each pass all seven images after that correction.
The final remux fixture's boundaries agree within approximately one microsecond, and its final audio and video endpoints match.
The broader conversion suite uses its existing 50 ms tolerance; these results do not establish sample-exact audio continuity.
This uses real native WebKit decoding with MSE capability disabled, not physical Safari acceptance.
WebKit's paused native HLS path emits no frame callbacks; the native oracle uses seek completion and render turns with unchanged image assertions.
The MSE oracle retains timestamp-qualified frame callbacks.

The exact commands are `python3 apps/player/scripts/test-hls-resume-browser.py --engines webkit-native --prepared --tail-recovery` and the same command with `--controls` instead of `--tail-recovery`.
Each uses the outer `scripts/ci/e2e-artifact.py` wrapper.
Receipts below are under `.verification/hls-resume-browser/`; each retains the revision, environment, binary hash, source state, and artifact hashes.

| Receipt directory | Result | Receipt SHA-256 |
| --- | --- | --- |
| `20261011T193629Z` | cold baseline: both MSE engines fail all seven images | `9e48e51abe700123703a744e825c2b8d0082b27900e38e27e80f2ac2fca9eeec` |
| `20261011T200924Z` | native Retry loses the saved position | `70e51623cc26b375e8b8dd7a42bc0b9b6cc6b1c99456a1ecc8b335fca6925faf` |
| `20261011T202716Z` | native frames, existing-tail eviction, and audio continuity pass | `ca0b818efdbf210de9ebe1d5a937817eac033809571b472de67558ae6aa781ed` |
| `20261011T202913Z` | all five native recovery and cancellation cases pass | `70e20c3bf10e22ac9ad18db91f659a1b856aa534ed637ad17f640d73188797ce` |
| `20261011T203144Z` | rejected initial audio-preroll candidate: both MSE engines fail all seven images | `ecccfec4b8aff7c8ede61aac8f37966a6557b2374de5e2a92e05aa84cf88c487` |
| `20261011T212810Z` | corrected initial/refill trimming: both MSE engines, tail replacement, and audio continuity pass | `20258279df626c3d3385c43c9b941065d00ffdaf17e67ac1f8cb1071f796f433` |
| `20261011T212832Z` | corrected initial/refill trimming: native frames, tail replacement, and audio continuity pass | `caa5bcf64c03d704895ee77f250f9466c1c49eed0e8b866080b4620da184b1e6` |
| `20261011T213124Z` | native pending seek exposes a second seek to a transient buffered position | `b54cd3966a3cce99f679ab8d0e69c259d99f2467a932fdab8a9c9edb5f839240` |
| `20261011T213524Z` | bounded preroll: both MSE engines, tail replacement, and audio continuity pass | `ec613cda4af8507e058e2fef65091b2197d0ec7dcb5be32121be7a27fb731515` |
| `20261011T213545Z` | bounded preroll: native frames, tail replacement, and audio continuity pass | `ba2406b867b3637bb97421b23983bb6a9ff470776e521aad2ceea1ecd6ebc095` |
| `20261011T213601Z` | all five native recovery and cancellation cases pass with the seek guard | `1fce5117b5307622df8536a8744d24f81a4e858c449f6410386d218689351eb8` |

The native pending-seek failure was reproducible with a bounded lifecycle trace.
A `canplay` handler clamped to a transient later buffer while the requested seek was still outstanding.
It now leaves an outstanding seek unchanged. The real control journey and diagnostic replay pass without relaxed assertions.

The existing prepared suite's unbounded-preroll failure is retained at `.verification/hls-prepared-timeline/20261011T213034Z/receipt.json`.
Its SHA-256 is `ead617dda9866e4f5577a9f2a3b646e90883835736c42bd0602d142fa5cb0d13`.
The bounded-preroll replay passes eight cases; one stops before preparation with a process-inspection timeout.
That aggregate remains failed. The unchanged serial replay passes all nine cases.
All six existing audio-conversion and legacy controls pass on the bounded-preroll patch.

| Final existing suite receipt | Result | Receipt SHA-256 |
| --- | --- | --- |
| `.verification/hls-prepared-timeline/20261011T214137Z/receipt.json` | nine of nine pass, including sparse, fractional, and resumed timelines | `3d48ddfb06ba60d571caf03a92d15a4e2772d9529163c7180a79d9c3b158309a` |
| `.verification/hls-followon/20261011T213850Z/receipt.json` | six of six audio-conversion and legacy controls pass | `2d4da113d39b010faa5367881cca43a2ba2e10fd2c533131b7419317b321c0fb` |

Phone failure/loaded and desktop pending/failure renders were inspected directly.
Recovery text and controls remain readable, and pending indicators disappear after failure or successful recovery.
These renders do not establish physical-device or assistive-technology acceptance.

The new Chromium frame and metadata-recovery journeys are added to the required Player browser job.
Current local CI contracts pass 169 tests with five environment-dependent skips.
The repository's selected actionlint fork, version 1.17.0, accepts the changed workflow.
The generated Player architecture snapshot was refreshed after the first hosted check identified it as stale.
Architecture contracts, source limits, whitespace, and post-commit Player verification pass.
[PR #560](https://github.com/Kinosail/kinosail/pull/560) carries this repair; hosted checks and publication remain pending.

### Hosted resume failures and native startup correction

[CI run 38177651743](https://github.com/Kinosail/kinosail/actions/runs/38177651743) failed two Player browser checks on `abf740ac`.
The retained native-window test expected replacement playback at 23.25 seconds, but restoration rewound it to 22 seconds.
The native metadata handler now preserves forward decoder progress only when the pending replacement target is still the captured target.
Initial restoration and newer explicit targets retain their own positions.

The hosted Chromium resume test presented the correct seven timestamps, but every RGBA hash differed from the original.
A disposable Linux ARM64 Playwright container reproduced this result.
All seven decoded I420 hashes were identical; the original used SMPTE170M while MSE used BT.709.
The generated source lacked explicit color metadata.
Chromium's [original-file demuxer infers a color space from resolution](https://chromium.googlesource.com/chromium/src/+/refs/heads/main/media/ffmpeg/ffmpeg_common.cc), which explains this diagnostic result.
The fixture now declares and verifies limited-range SMPTE170M on generated frames with FFmpeg `setparams`.
Codec output flags alone failed the new metadata assertion on the Mac; that failed attempt remains recorded.
The same Linux decoder then matched all seven RGBA images without changing the image oracle.
This defines the synthetic fixture's interpretation. It does not repair or certify untagged-media color parity in production.

A subsequent native control replay still exposed an intermittent 10-to-20-second jump during metadata loading.
The browser retained a source-clock seek as the new decoder's default playback start position.
This follows the [HTML media initialization behavior](https://html.spec.whatwg.org/multipage/media.html#dom-media-currenttime).
Player now clears that default only while the decoder has no metadata, then restores the retained source target after metadata loads.
Player also removes its competing saved-progress listener once its native adapter owns restoration.
The shared listener remains available to Subtitles, whose initial native restore depends on it.
The native first-buffer recovery and its outstanding-seek guard remain in place.
All ten retained native window/gap tests pass, including paused fractional gaps and initialized replacement progress.
Real native controls, seven frame comparisons, and a positive resume with certified origin zero also pass.

The browser driver now retains a case before risky work and preserves a browser launch failure when its evicted tail is absent.
An intentionally unavailable Chromium channel leaves the case failed, with `tailRestored=false` and unchanged initialization recorded.
It no longer masks that failure with a secondary packet-count exception.
Mac Firefox still exits before navigation with a missing-profile error, consistent with [upstream Playwright issue 42768](https://github.com/microsoft/playwright/issues/42768).
The upstream explanation is not a locally proven TCC diagnosis, and no browser permissions were changed.
A separate Linux Firefox diagnostic passes all seven prepared-resume frame comparisons, tail recovery, and audio checks.
That diagnostic uses the real cross-compiled Player and an instrumented copy of the browser driver; it is not physical-device proof.
The disposable Linux container was removed after verification.

The final code checks use the commands above and these retained receipts.
Outer `scripts/ci/e2e-artifact.py` manifests record `abf740ac`, the exact uncommitted patch hash, and unchanged source throughout each command.
They establish source-bound local evidence, not a clean-commit or hosted acceptance claim.
The existing timeline and audio suites were rerun after changing their shared fixture; all nine and six cases pass respectively.

| Receipt | Result | SHA-256 |
| --- | --- | --- |
| `hls-resume-browser/20261011T221330Z/receipt.json` | Linux untagged diagnostic: seven identical I420 frames, seven different RGBA frames | `3b68aa04eb0df21d08eedb020a35d27a11d0568a2f150545fbbb69d231a2804f` |
| `hls-resume-browser/20261011T221650Z/receipt.json` | Linux Chromium with FFmpeg 8.1.3: seven RGBA images, tail recovery, and audio pass | `bb11e11a4b4df5b663ea310cf8b8c9180dafccfad4c0b7241822398dd4e70149` |
| `hls-resume-linux/20261011T221807Z/receipt.json` | Linux Firefox diagnostic: seven images, tail recovery, and audio pass | `0b9df2e93761c75471d597dc58298d7ea65f6777ef08f940f73f9a9096a4a3e9` |
| `hls-resume-browser/20261011T221712Z/receipt.json` | Mac Chromium and WebKit MSE: fourteen images, tail recovery, and audio pass | `67508520e784cda4b8bf5b4709965c165ab07e7e2286646a17a269a7b2255292` |
| `hls-resume-browser/20261011T221827Z/receipt.json` | retained failure: native pending seek jumps to 20 seconds | `e55e487e75bae0ea20a8e2653f92548763bbd13fe2536e3e504091d595d5926d` |
| `hls-resume-browser/20261011T222616Z/receipt.json` | final native correction: all five recovery and cancellation cases pass | `318749a8d533ab6dacebb8d054f8f6544cb03d576efa65adbe5c9d149e2ad814` |
| `hls-resume-browser/20261011T222637Z/receipt.json` | final native correction: seven images, tail recovery, and audio pass | `5e1a3146dedb4314c901552e163e88f5890c9205712db66b3aced9b6c6ccd380` |
| `hls-resume-browser/20261011T222651Z/receipt.json` | native origin-zero diagnostic: first playback at 0.5 seconds and seven images pass | `56cbbca97cb8bf528622d10dffdead7e404fb79aafcce898b980fc1297e3a3b7` |
| `hls-resume-browser/20261011T221351Z/receipt.json` | expected browser launch failure retained with missing-tail evidence | `1192afbb927234fdb4db31a002df47f4b0f987c18b0dc3a15963080b9f8a387d` |
| `hls-prepared-timeline/20261011T221925Z/receipt.json` | all nine existing timeline cases pass | `a4ff3132c1fce0bdeb85c310214ba30eeb8b13c826217138fdd648d759b7bddb` |
| `hls-followon/20261011T222154Z/receipt.json` | all six existing audio/legacy cases pass | `55759e1a757c8b97bb99ed9975141e3f1519166454ad909ed66d0e41e4beba5b` |

Receipt paths in this table are relative to `.verification/`.
The native retained-suite manifest is `ai-qa-campaign/native-single-owner-retained/manifest.json` under the same root.
Its SHA-256 is `d308df35c6e97733d185e204bbf5a54aba683899a44498557a48bc2b7787223f`.
The Linux probes and origin-zero diagnostic remain private scripts under `.verification/ai-qa-campaign/`.
The Linux binary, generated source, and result hashes remain recorded separately from the hosted AMD64 job.

Fresh unmodified-server baselines still reproduce cold non-keyframe resume and Mac HEVC fractional-tail failures.
The cold receipt `hls-resume-browser/20261011T214557Z/receipt.json` has SHA-256 `28308557040ce0d2a325d89190f9286404399b09118a13d8bafa1e64a4f2826e`.
The HEVC receipt `hls-hevc-startup/20261011T214713Z/receipt.json` has SHA-256 `1e77b81b23b9ebf50e5f8c20eb2eaf77b4374e9bcb866e8e6107433633dacd57`.
Private clock-header and EVENT-playlist experiments are diagnostics only; neither changes production or closes those findings.
Hosted verification of the corrected PR remains required before merge.

### Seventh delivery checkpoint

[PR #557](https://github.com/Kinosail/kinosail/pull/557) merged the executable-fixture repair after all required checks passed.
Fetched ancestry proves task commit `5b0adeba9e60d0bd2f8dfb2ac4259a24d2ba01e8` in main `c5fd62edfd3201fbc15795d3073a658ba1f61c62`.
[Main run 38168058630](https://github.com/Kinosail/kinosail/actions/runs/38168058630) passed selected checks, published both apps for AMD64 and ARM64, and advanced their production image tags.
This is publication evidence, not Nox deployment or physical-device acceptance.

## Additional private artifact checksums

The archives below are retained under `.verification/ai-qa-campaign/hosted/`.
Hosted artifacts contain the bounded receipts; raw local process logs remain private.

| Archive | SHA-256 |
| --- | --- |
| `hls-all.zip` | `fa9897b02bba7a8fa7bea57177a5893959c37082c04818193612b1286b4cbcfc` |
| `hls-all-repeat.zip` | `b900b6d48b8811451c55b03c10d43d799fd2435722ad2c0bfc383ad989c06395` |
| `browse-safety.zip` | `220fdcabcb3fd76fb642b1ddf1e1cc644ccf7d63164142dd48d52ccc3e149749` |
| `browse-home.zip` | `998853f20c7e500cb8ed29c86237f39282317b099034980785736d041be38c1f` |
| `subtitles-protocol.zip` | `19a40ac27f6bed4355d2a08ee536dc3835d1ebdb32a030dc0bd2a5052b4cfcc8` |
| `layout-firefox.zip` | `080f0c3ce2f6b4c102c9a07c81aacdc29f00bd49edc3c7f831d4fcf1e46040f7` |
| `layout-webkit.zip` | `7b6f8f4617a82f7daa1fbb0a0d45b91a79d6980db5bf2832e1c9a143d926c882` |
| `layout-chromium.zip` | `3fb64ec269a79cf1f085863cf06c59e59a4c1af3df1d1b7f1eb388859afe6807` |
| `nav-final-firefox.zip` | `5368938a1f9b8275e2c2a29e87dbf76893bd4c3c4ff0d93a578af5de5b3d542b` |
| `nav-final-webkit.zip` | `93ab33817d874f85dd3dece0597eea96a9839713782c3d6840f4c174c9c6a46f` |
| `nav-final-chromium.zip` | `7607188bd2328b410afb75f0cf5352c3d386650e59c348d557a4196c7a99da04` |

## Remaining coverage

The campaign remains active.
Retain the earlier audit's unverified journeys until a new passing execution establishes their behavior.
Inspect the deep-run artifacts, repair confirmed failures, and record the final revision and each remaining platform boundary.
Physical devices, real media/receivers, production deployment, store distribution, and unsupported external environments are not certified by these local runs.

### AQA-004 follow-up — complete short HEVC output before publication

The repair covers plain HEVC audio conversion at source offset zero, with a probed duration greater than zero and at most 30 seconds.
Foreground playlist readiness now waits for the actual encoder to finish successfully.
Background preparation keeps its existing size monitoring, but reports ready only after successful completion.
The final playlist therefore lists actual fragments instead of a speculative fractional tail.

A source/policy-bound `.hevc-complete` receipt records successful completion.
Every writer revokes an existing receipt before changing media, including preserved refills.
Admission rejects active producers and requires a matching receipt, an explicit source binding, and all final assets.
An absent `.source` cannot use the older timestamp-only fallback for these short streams.
Old copied-audio cache policies are invalidated once.

The first completion candidate still accepted a failed producer's leftover playlist on retry.
Deleting `.source` did not fix it because the legacy freshness fallback accepted that missing file.
The retained failure returned 503 first and 200 on retry.
The positive receipt repair returns 503 on both attempts and publishes no failed playlist.
An independent review found no remaining concrete blocker after this change.

`python3 apps/player/scripts/test-hls-short-completion.py` uses a real authenticated Server and real FFmpeg media.
A task-owned wrapper holds the producer after actual EOF or returns a controlled failure.
It checks success, repeated failure, the existing 30-second deadline, background readiness, and invalid-cache recovery.
Missing, mismatched, and symlinked completion receipts each start a new encoder.
Removing `.source` while retaining the receipt also starts a new encoder.
The symlink's external sentinel stays unchanged.
Every tested Server and recorded wrapper process settles during cleanup.
Failure responses have matching ERROR records with request and playback-session IDs; the authentication token is absent from logs.
Receipts retain the revision, patch hash, fixture/build commands, source hashes, wrapper hashes, and private-log hashes.

All four existing HEVC journeys pass with five actual fragments, unchanged initialization, complete decoded video, audio content, and packet continuity.
The new completion cases are added to the required Chromium Player job beside those existing media checks.
Only safe receipts are uploaded; raw logs and generated media remain private.
Focused retained startup, cancellation, and policy checks pass locally.

The 29.9-second low-resolution fixture returns its cold playlist in 0.746 seconds on this Mac, including the artificial hold.
Background preparation takes 7.83 seconds and reuses its one producer after release.
These measurements do not establish a speed guarantee for high-bitrate media or other hardware.
Waiting for completion increases startup work for this bounded short-clip path.
Long HEVC, nonzero HEVC resumes, general HEVC refill, and cold non-keyframe H264 resume remain open.
There is no new browser, physical-device, deployment, or publication proof from these HTTP/media cases.

| Private receipt under `.verification/` | Result | SHA-256 |
| --- | --- | --- |
| `hls-short-completion/20261011T225701Z/receipt.json` | retained failure: missing-source fallback accepts failed output | `1203b9398bd9feac643fd9ebbdc4eb330d81defdb774f8bc829c598f5c42f295` |
| `hls-short-completion/20261011T230157Z/receipt.json` | success, repeated failure, timeout, and background completion pass | `7f82a57a6af5a14e6062b279af307af9eaccf957f463dfe392813970d25684e3` |
| `hls-short-completion/20261011T230459Z/receipt.json` | four metadata corruptions force regeneration; sentinel unchanged | `93067dbc70a114952988cb06c73c24217c2f6b027e37c2aa48959e8603aee679` |
| `hls-short-completion/20261011T230516Z/receipt.json` | 29.9-second cold fixture passes | `68afe0540ebfebce43ec2587718a41a81dc29a495df798d3c65e10fce863e48d` |
| `hls-short-completion/20261011T230531Z/receipt.json` | 29.9-second background fixture passes | `005c7897b6af1399bdada18c52ffcd23ba7ad0e80dfd5e1eff8627974647f02f` |
| `hls-hevc-startup/20261011T230327Z/receipt.json` | complete, cold, interrupted, and adopted HEVC media pass | `f50e0c85f44ec1a0758066a4876640e3c050d7296b17e74437d1718dbc7fc515` |

These runs use base `31a061845470b105dabfe9969d3708259b229c32` with the recorded task patch.
The final CI-contract replay passes 169 tests with five environment-dependent skips.
Its first run hit a process-permission error in the unchanged Q09 descendant-cleanup test; the focused retry and full replay pass.
Source limits, whitespace, the architecture snapshot, and the repository-selected actionlint 1.17.0 pass.
PR #560's current CI encountered an artifact-service `ETIMEDOUT` during the AMD64 container evidence upload.
That infrastructure failure is retained separately from the short-HEVC repair and still requires hosted reconciliation.
