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

## Latest checkpoint — October 13

PR 587 remains draft. The campaign goal is active; no complete product or platform acceptance is claimed.
The chronological sections below retain earlier failures, repairs, and exact artifact evidence.

| Surface | Latest verified evidence | Remaining work |
| --- | --- | --- |
| Transcoded HLS recovery | Five public fixtures, nine rendition comparisons, four legacy rebuilds, and staged rejection pass at `657e2ca2`. The full race matrix, shortened-AAC rejection, and 89.2% coverage pass at `07db0b07`. | Current required gates and physical playback. |
| Browser session return | Subtitles Firefox and WebKit each pass 99 cases at `657e2ca2`. Player Firefox and WebKit at `c09c2659`, and Chromium at `8fa923df`, execute the required populated session-return case without retry. Chromium's repaired trace assertion passes at `ea644e55`. | Current gates, the Firefox wrong-worker fixture repair, and a WebKit touch-dispatch stall. |
| Entropy failures | Hosted race passes at `07db0b07`. All three entropy-failure paths retain covered counters in the merged profile. | Current required gates. |
| Apple native contracts | Full hosted native suites and all twelve structured reader probe outcomes pass at `bc33897b`. Reader attachments are retained in xcresult. | The earlier loaded-WebView timeout at `ea644e55` remains unresolved. Failure-state evidence and physical-device proof remain open. |
| Android | Earlier populated phone, tablet, and TV results and exact limits are recorded below. | Caption overlap, intermittent EPUB accessibility, and Wear execution. |
| Publication and deployment | Earlier merged fixes have separate main and container receipts below. | PR 587 merge, current publication, deployment, and physical playback proof. |

Local runtime remains held for the coordinated physical Apple TV window. Current local checks use source formatting, shell parsing, source contracts, and artifact inspection. Hosted gates remain enabled.

## Baseline evidence

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

The browser artifact reconciliation deduplicates `(spec file, full suite/title path, browser project)` across the main and focused reports.
Repeated executions are not additional coverage. The numbers below apply only to this hosted baseline:

| Application / engine | Distinct observed cases | Passed at least once | No passing execution |
| --- | ---: | ---: | ---: |
| Player / Chromium | 1,070 | 791 | 279 |
| Player / Firefox | 1,070 | 787 | 283 |
| Player / WebKit | 1,070 | 785 | 285 |
| Subtitles / each of the three engines | 99 | 99 | 0 |

The Player remainder includes opt-in populated journeys and layout audits, not observed failures for every case.
The corrected private reconciliation is `.verification/ai-qa-campaign/hosted/player-coverage-v2.json`.
The original leaf-title key collapsed phone and desktop direct-retry cases. Both were skipped in this baseline.
The correction adds one unpassed case per engine; passing counts do not change.
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

### Prepared resume: first-fragment recovery retains the original recipe

The public first-fragment eviction case exposed a second prepared-resume boundary.
At a saved position of 12.5 seconds, deleting the existing first fragment left playback unable to restore it.
The staged encoder used the correct media window, but publication validated that window's zeroed recipe against the original offset-bound policy.
Publication now validates the original source recipe. Encoding still uses the same media window.

The new `--prepared --first-recovery` browser case populates all ten fragments, deletes the actual first fragment, and loads the unchanged watch page.
Chromium MSE, WebKit MSE, and native WebKit each match all seven original-file frames after recovery.
The recovered fragment is byte-identical. Initialization, both playlists, the timeline, and its clock certificate remain unchanged.
Audio continuity checks pass before eviction and after recovery. The synthetic source remains unchanged and the Server settles.
The new case joins the required Chromium workflow; the existing tail-recovery command remains in place.

| Private receipt under `.verification/hls-resume-browser/` | Result | SHA-256 |
| --- | --- | --- |
| `20261011T231725Z/receipt.json` | retained failure: first fragment is not restored | `fa4d3fd0d05415b957043977330504e63f7ea2b964ac4b4953005f69824c8969` |
| `20261011T231843Z/receipt.json` | Chromium and WebKit MSE: fourteen frames and complete first-fragment recovery pass | `da506034a97cd03d99d14a39759ed652a5d7a0a4ecc4b639d22d2751a0a8434d` |
| `20261011T231903Z/receipt.json` | native WebKit: seven frames and complete first-fragment recovery pass | `9d84cbe5917028945851381a6c14af8fc53a9b843c14df4b3102372db3e83cd2` |

Outer manifests under `.verification/ai-qa-campaign/resume-first-{red,green,native-green}/` bind the exact task patch and unchanged source.
These checks use base `45a6c6faa293e525e88986a86c537ad30ba43ed5`.
An independent review accepted the original-recipe publication correction.
Cold resume remains separate: the current unprepared Chromium run still fails its frame oracle.
That receipt is `.verification/hls-resume-browser/20261011T231221Z/receipt.json`, SHA-256 `78246610866ac85213a10dccef015c49f12b773d457238c7730627d41f6f0ad7`.

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


### Cold H264 remux resume: bounded source indexing and shared readiness

A cold saved position of 12.5 seconds failed all seven Chromium reference-frame comparisons.
The copied video started at the preceding source key, but playback still used the requested offset as its origin.
The new path certifies the complete remaining packet and key map before encoding.
It applies to plain H264 remux with a positive offset and at most 30 seconds from the actual source key through EOF.
Copied audio conversion is excluded from this repair.

Packet, IDR, and decoder-configuration probes share one eight-second deadline, including workload admission.
Decimal packet timestamps enforce the window while reading; rational timestamps validate the completed map.
A demux seek before the permitted window is rejected before waiting for EOF.
The source and playback policy are checked again before binding and publication.
Failed, incomplete, changed, or timed-out probes do not start a media encoder or publish a certificate.

The source origin and existing mux-clock certificate drive the public playlist and browser timeline.
Concurrent requests share one index and encoder.
A finished encoder can beat the readiness ticker, so completion now preserves valid output for clock binding.
New requests join pending clock certification before deciding to rebuild a cache.
The final job-creation check preserves completed or seekable indexed output under the existing manager lock.
It rechecks cancellation and source policy after metadata verification.
A failed producer's incomplete prefix remains recoverable through a fresh encoding attempt.

Refill jobs retain the original requirement for certified cuts after their seek recipe changes.
Missing or invalid required metadata cannot fall back to legacy mux arguments.
First-fragment recovery is byte-identical; tail recovery preserves decoded video and timestamps, audio continuity, initialization, and metadata.
Whole-file equality is not required for nonzero fragments because container sequence fields may differ.

The private offline matrix passes 41 cases: 34 certified suffixes and seven expected window rejections.
It compares suffix cuts with independently scanned whole-file cuts across MP4, Matroska, AAC, AC3, and video-only fixtures.
It includes irregular keys, near-zero and near-EOF positions, and the tail of a longer file.
A real 95-second source at B=65.1, with preceding A=64, is rejected in 0.079 seconds without encoding.
Its requested remainder fits 30 seconds, but its actual key-to-EOF window does not.

The public fault harness passes twelve cases before the final failed-producer recovery addition.
Those cases cover concurrent requests, held clock certification, whole and fractional-tail recovery, early bounds, non-key packets, source changes, late probe failures, and individual/aggregate deadlines.
The added producer failure first reproduced a persistent 503 after a playable prefix; its corrected replay returns 200 with one fresh producer.
The clock-race replay still passes after that correction.
All recorded Servers and probe wrappers settle. Failure logs retain request/session correlation without authentication tokens.

Chromium, WebKit MSE, and native WebKit match all seven original-file frames for both AAC and video-only sources: 42 comparisons pass.
The retained prepared first-fragment Chromium journey also passes with unchanged initialization, playlists, timeline, and clock certificate.
These browser observations precede the final cancellation and failed-prefix guard refinements; clean-revision replays follow the implementation checkpoint.
Native WebKit uses a capability simulation and does not certify physical Safari or devices.

An experimental extension to cold AC3-to-AAC conversion still mismatched all seven Chromium frames despite reporting source origin 12.
Its displayed timestamps had an approximately 0.021-second shift. The root cause is not yet established.
WebKit chose remux for that source and matched all seven frames; the initial harness incorrectly required conversion in that engine.
The harness now permits either supported mode while retaining the exact frame oracle.
Longer cold suffixes, cold audio conversion, long/nonzero HEVC, physical devices, and production deployment remain open.

The required Chromium workflow includes cold AAC/video-only journeys and the public fault harness.
Local CI contracts pass 169 tests with five environment-dependent skips.
The repository-selected actionlint, source limits, architecture snapshot, whitespace check, and focused retained Go tests pass.
Post-commit affected verification and hosted checks remain required before delivery.

| Private receipt under `.verification/` | Evidence | SHA-256 |
| --- | --- | --- |
| `hls-resume-browser/20261011T233337Z/receipt.json` | cold Chromium failure before repair | `1e3dd830c6d4acaca00e4079ac538048859ffdf8be66187041acd7a5a9cbf0ae` |
| `hls-cold-suffix/20261011T233548Z/receipt.json` | probe-failure control exposes uncertified legacy output | `c8799ccdc3bec2fd3c5098b3019505f7e7c6507b5684dbdb74b9518163bed5c0` |
| `ai-qa-campaign/cold-suffix-matrix-20261011T233806Z/receipt.json` | 41-case offline index matrix | `5ecf0bced3435f943d06ff58ef2c4110a8bedbca60e73123cd695e482685ca64` |
| `hls-cold-suffix/20261011T234306Z/receipt.json` | clock race: 503/200 and duplicate encoding | `8989e2b54f9efbc6ed67c04d5ea0daaa7b08f4ef218f354afc6d2b5a758320c9` |
| `hls-cold-suffix/20261011T234914Z/receipt.json` | twelve public success/fault cases pass | `21eb1fdbe68f37b71406d9f1761558029dd217e18d5e38e5c61c1fc4120aeb48` |
| `hls-cold-suffix/20261011T235310Z/receipt.json` | failed prefix cannot recover before guard correction | `a6214f5b7c54b4a7e8be269644cedbdbe86d1e7d7fc181c25e4de2aee708c3b2` |
| `hls-cold-suffix/20261011T235344Z/receipt.json` | failed prefix recovers with one fresh producer | `418faeaf40da852f244a85c0471444ff5e3645533da1b39c4f872902c82308ad` |
| `hls-cold-suffix/20261011T235358Z/receipt.json` | clock race and zero/tail recovery remain correct | `58ab8ae3dd23c1f04ec49d139a53680d3b214f252bf6b59118eee895fe134e9d` |
| `hls-resume-browser/20261011T235003Z/receipt.json` | AAC Chromium/WebKit: fourteen exact frames | `109e3541f6abb3a28ec50c31597040792eb5e10821a8a8bc24cd42f31b77e84f` |
| `hls-resume-browser/20261011T235124Z/receipt.json` | AAC native WebKit: seven exact frames | `16dc43735a8bd82ad7d381c70dc718e02cc2507d59f024a8c8979dc871978784` |
| `hls-resume-browser/20261011T234914Z/receipt.json` | video-only Chromium/WebKit: fourteen exact frames | `284c713a7d79d1de130d904ccb06f9da1d2039d90a19dc3d80a89aac932eaee2` |
| `hls-resume-browser/20261011T235016Z/receipt.json` | video-only native WebKit: seven exact frames | `0cd45def675af0ebcb3518059e9217ece668462d34fc8212b809bd247d13189a` |
| `hls-resume-browser/20261011T235135Z/receipt.json` | prepared first-fragment regression passes | `b23c7c9d3281365ddb953aa0276eed6d49fa50fdede4e5ac96c48016e3450797` |
| `hls-resume-browser/20261011T234724Z/receipt.json` | retained AC3 conversion timing failure and mode-control correction | `725e711bd1023201603fe96f1f0e51eafce955d60f5cd8b0bdea5ebe6de16a06` |

These runs use base `2a32e9b43826e2d22239a473181cb2bf94f6afb4` with the recorded candidate patches and binary hashes.
The public fault receipts include the patch and harness hashes. Clean-revision browser evidence will replace the candidate boundary for delivery.

### Cold H264 remux: clean implementation checkpoint

All final replays pass on clean revision `b66e30e71c0c7aef60f4d4fd53aa95dda856d6dc`.
The public fault suite passes all thirteen cases, including failed-prefix regeneration and held clock certification.
AAC and video-only sources each match seven reference frames in Chromium, WebKit MSE, and native WebKit: 42 comparisons pass.
These replays include the final cancellation and failed-prefix guard corrections.
Native WebKit still uses a capability simulation; physical Safari remains unverified.

`make -C apps/player verify-changed` passes source limits, whitespace, server compilation, and focused server tests (94 seconds).
Its private log is `ai-qa-campaign/cold-suffix-verify-changed-private.log`.
Independent review found no remaining concrete blocker within the bounded remux scope.
Hosted checks and protected integration remain pending. The audio-conversion and longer-stream boundaries remain open.

| Private receipt under `.verification/` | Evidence | SHA-256 |
| --- | --- | --- |
| `hls-cold-suffix/20261011T235619Z/receipt.json` | thirteen public success and fault cases | `d1cdb21ee05cd3f4761934b1186d17eed621156eba1d7deabffc64e1ca5fe675` |
| `hls-resume-browser/20261011T235620Z/receipt.json` | AAC Chromium and WebKit: fourteen exact frames | `d77de1ec8380a7f9e82d18c5049b1f344e876ad1ccb16e164b0588eaeb717e28` |
| `hls-resume-browser/20261011T235639Z/receipt.json` | AAC native WebKit: seven exact frames | `17f0dd4fbb27e87b0585b428ee84fd1975d5f265346f91696513e3d1922df75a` |
| `hls-resume-browser/20261011T235649Z/receipt.json` | video-only Chromium and WebKit: fourteen exact frames | `d93b8581db4169d854aeb208755252772366e327b8ed73d583fc0aeeda421cb4` |
| `hls-resume-browser/20261011T235700Z/receipt.json` | video-only native WebKit: seven exact frames | `e90e09704c86c051689c15d82730e4e881e1bc85133e0900e7c59ba1bcc147ad` |


### Prepared-resume delivery and copied-audio clock checkpoint

PR #560 merged through protected main as `e25f2a116ad156d7f4fe35d4cb9606ab87f0d5f1`.
A fetched ancestry check confirms task commit `145fce2207ea72904b451da2d62324f47f0fffe2` is included.
Required hosted checks passed. Publication and deployment for that revision remain separate, unverified boundaries.
PR #562 now targets main. Its reconciliation changed an unrelated native test and QA report, with no change to its production repair.
PR #565 contains the bounded cold-remux repair and remains a draft stacked on #562.

The converted-audio frame mismatch has a confirmed cause.
Hls.js 1.7.3 anchors muxed media to the earlier AAC priming timestamp.
For the reproduced source, that shifts copied video by 21.333 milliseconds.
Dropping the first AAC packet repaired video but removed approximately 16 milliseconds of decoded startup audio; that candidate was rejected.
The implementation preserves every startup packet and supplies the certified video clock with the certified source origin.
Its buffer-controller subclass changes the append offset while preserving the pinned adapter's append queue.
This uses the pinned adapter's internal append handler and requires real-browser regression proof when that dependency changes.
See the upstream [buffer controller](https://github.com/video-dev/hls.js/blob/v1.7.3/src/controller/buffer-controller.ts) and [passthrough remuxer](https://github.com/video-dev/hls.js/blob/v1.7.3/src/remux/passthrough-remuxer.ts).

Both certification headers must be present together or absent together.
Malformed, negative, excessive, duplicate, or incomplete clock metadata settles into the existing retry state before requesting media.
The failure trace uses a fixed safe class and retains the requested playback position.
Eight injected controls pass in both MSE engines. These malformed-header controls are isolated because the valid Server cannot emit those responses.
Real watch-page journeys force AAC conversion through a documented user-agent capability simulation.
Near-zero playback, exact-key playback, and first/tail recovery match every reference frame and reach the ended state.
Native WebKit remains a capability simulation, not physical Safari acceptance.

Refill verification found an additional 58.667-millisecond audio gap.
Disabling accurate input seek reduced it to 10.667 milliseconds, which was still incorrect.
The strengthened oracle rejects every positive converted-audio gap beyond two microseconds of timestamp rounding.
Retaining two AAC packet durations covers decoder warm-up and the new packet grid without changing packet timestamps.
Independent low-rate review then reproduced mux clock shifts of 3, 5, and 24.938 milliseconds when that allowance admitted negative output audio.
The final filter also bounds output audio to nonnegative timestamps; all sixteen reviewed low-rate/phase cases preserve the video clock and initialization.
A cache-policy tag invalidates earlier converted-video refill output, including zero-offset recipes.
The [FFmpeg seek documentation](https://ffmpeg.org/ffmpeg.html#Advanced-options) and [noise bitstream-filter variables](https://ffmpeg.org/ffmpeg-bitstream-filters.html#noise) define the relevant behavior.

Chromium PCM capture independently confirms that retaining the startup priming packet avoids the silence introduced by the rejected packet-drop candidate.
Actual watch-page noise captures also retain audio energy across startup/refill and regenerated-tail boundaries.
However, the regenerated-tail overlap contains a brief low-correlation waveform transition; seamless decoded splicing is not yet certified.
A 44.1 kHz Matroska probe matches all seven video frames and compatible playback ends, but its synthetic direct-file reference times out at EOF.
That probe is retained as incomplete evidence, not a passing full journey or a confirmed product defect.
The separate WebKit PCM control produced silence even for its baseline, so it provides no WebKit waveform proof.
These limits remain open; packet continuity and video-frame equality do not close them.

The retained Chromium playback suite passes 101 tests.
Extracting the pending-HLS-loader fixture preserves the source-file limit; its focused replay passes.
CI contracts pass 169 tests with five environment-dependent skips after a process-group permission failure on the initial local attempt.
Source limits, whitespace, architecture snapshot, and repository-selected actionlint pass.
The required Chromium workflow adds converted-audio near-zero, first-fragment, and tail-recovery journeys.
These are candidate observations on base `a2ad2bea`; post-commit verification and clean-revision replay follow this checkpoint.
The final candidate patch is private at `ai-qa-campaign/audio-clock-final-candidate.patch`, SHA-256 `a44b73df2f4df17e7ee6c6a8e46cddd81ed3c49a2ed60d2b04f6f1f9449a2e1e`.

| Private receipt under `.verification/` | Evidence | SHA-256 |
| --- | --- | --- |
| `hls-resume-browser/20261012T002320Z/receipt.json` | stricter positive-gap rejection before overlap repair | `48204878556ef0302df64d91f775eff055daab7e4d731bf5c003e9f8e9cb1d7f` |
| `hls-resume-browser/20261012T003047Z/receipt.json` | near-zero converted audio: fourteen frames and ended | `1a8b042d8dfd2c560cce39d7fa59a8f6783b2d6a70978d804959eb8eb71d7050` |
| `hls-resume-browser/20261012T003102Z/receipt.json` | exact-key converted audio: fourteen frames and ended | `ad54704286521f4ea61c211e2f59164bace43ea6bfbf98fb720ad9fcf023b659` |
| `hls-resume-browser/20261012T003125Z/receipt.json` | native WebKit converted tail: seven frames and ended | `8713891597fe4fab0fe4efd4cba932c184218a9dc0219c2642955582a0e8f467` |
| `hls-resume-browser/20261012T002135Z/receipt.json` | eight malformed-metadata controls per MSE engine | `c5355aeae606751947e280d42ff94c7cbc56678e2c16e5b4ed4044e4f83dbc82` |
| `hls-resume-browser/20261012T003658Z/receipt.json` | final bounded overlap: both MSE tail recoveries | `98de8391b2d490c92fb44c5b8427d8521d84f75d3c3f752f180a2b5ab7f5efa6` |
| `hls-resume-browser/20261012T003719Z/receipt.json` | final bounded overlap: both MSE first recoveries | `5475de358ea7ba744eb34d19be5b10b9f996c40a22a947b1efe33ebd2659e455` |
| `ai-qa-campaign/review-low-rate-preroll-20261012T003437Z/receipt.json` | low-rate clock-shift reproduction | `427a1b4daef6c7878a67336994cc73c88c30d0c9dc5957f3cbc58cd08289880f` |
| `ai-qa-campaign/review-low-rate-preroll-20261012T003437Z/clamp-receipt.json` | sixteen nonnegative-bound counterfactuals | `1878e68308d1ff711ce72fe0c18646daa3144ec8c5ed417a8af9c871032a5e70` |
| `ai-qa-campaign/audio-clock-noise/control-capture/warmup-comparison.json` | isolated Chromium startup PCM and packet-drop control | `d2b9ac919b5e1cad56197a62a8a784d78ffc95dd102f9f1e89e6591bfecca24a` |
| `hls-resume-browser/20261012T003415Z/decoded-splice-comparison.json` | actual Chromium waveform; unresolved brief tail transition | `4d18366bb2f1f8ee7cad4e26ac3b375319baeb43493e6bf6cccd8c2ed0a0de0c` |
| `hls-resume-browser/20261012T003305Z/receipt.json` | 44.1 kHz Matroska probe; reference EOF limit | `adf24109d0e332b53439bc0e47db628c23d95bafb92b435ae7150a7433292ac4` |


### Copied-audio clock: clean checkpoint verification

Clean revision `9cd7192c87f1bcf468f167e351f0f908622cbb51` passes all eight final public browser runs.
Converted audio covers near-zero and exact-key resume, first-fragment and tail recovery, plus native WebKit tail recovery.
AAC and video-only regression journeys also pass. Together these runs match 91 reference frames and reach playback ended.
The malformed-header run passes eight controls in each MSE engine.
Every harness receipt records its exact command, generated source, binary hash, environment, unchanged source, and settled Server.
`make -C apps/player verify-changed` passes source limits, whitespace, server compilation, focused server tests (89 seconds), and E2E discovery.

Independent low-rate decoder proof reproduces the rejected clock shift: only one of six Chromium frames matches without the clamp.
The committed clamp matches all six frames, supplementing the sixteen-case offline clock and initialization matrix.
This low-rate browser test is isolated decoder evidence, not public Server acceptance.

The clean Chromium PCM replay confirms the unresolved brief waveform transition at the regenerated tail.
There is no zero-energy gap in its measured windows, but that is insufficient to claim seamless decoded audio.
The private diagnostic receipt now records its wrapper command, input sample rate, container, and driver hashes.
Its original-file frame journey passes independently of the waveform comparison.
The draft audio repair is not ready for protected integration until this transition is characterized or repaired.
The [MSE audio splice algorithms](https://www.w3.org/TR/media-source-2/#audio-splice-frame-algorithm) describe overlap processing; they do not prove this implementation's decoded output.

| Private receipt under `.verification/` | Evidence | SHA-256 |
| --- | --- | --- |
| `hls-resume-browser/20261012T003901Z/receipt.json` | near-zero converted resume | `9207c2287bd422d0c959b41c4a512bba1437ddfd24fa249ccac0cc5ed04f3f7d` |
| `hls-resume-browser/20261012T003918Z/receipt.json` | exact-key converted resume | `8f6aed46f5d5c25ca1dd3f5c886ab230b9acc4551730c77bf017a567ed9f514f` |
| `hls-resume-browser/20261012T003935Z/receipt.json` | converted tail recovery | `a74a6a519c47f35414ac90eb9927a3da9418663dd9413cad16326a21147cf0d6` |
| `hls-resume-browser/20261012T003953Z/receipt.json` | converted first recovery | `1827626ff872f6cd3dfeb668a31d0987832df561edd8cfff4507e31a5b249711` |
| `hls-resume-browser/20261012T004015Z/receipt.json` | native WebKit converted tail | `8f34cad6225106594b8af7f9fbeb3dd7ebebb07864bf8ea603a80a6947e77b2f` |
| `hls-resume-browser/20261012T004030Z/receipt.json` | paired metadata controls | `2211736ce3a13172e31f18d2841c26d6c1e40045ca16ef9c17ce306ea6359ae1` |
| `hls-resume-browser/20261012T004054Z/receipt.json` | AAC regression | `9153f4489d083bb842a45e2e5c5c1685a2a84466c58e87a7c2aa584a682bc069` |
| `hls-resume-browser/20261012T004112Z/receipt.json` | video-only regression | `ccf48906b34be6ec4b34d222066422a3a6c836bfb6e42c66c247f9c609044759` |
| `hls-resume-browser/20261012T004133Z/receipt.json` | Chromium PCM diagnostic and frame journey | `e8f168e88ed887e68094a3146181d327feb842d5c9df6a952720b0aad2c78129` |
| `ai-qa-campaign/review-low-rate-preroll-20261012T003437Z/verification-summary.json` | isolated low-rate video counterfactual | `08f7b8ebbeb9a0d2ad723f5067162375143f6413ea640a0f48b6f386118f681a` |
| `hls-resume-browser/20261012T004133Z/decoded-splice-comparison.json` | clean decoded-splice diagnostic; boundary open | `dd379853afe755dddfe78b5d61d5bc41db16e606f6be64c4772c0fccf6de1988` |


### Short converted-audio recovery: preserve the original encoded content

PR [#562](https://github.com/Kinosail/kinosail/pull/562) merged as `87428823d71d43f612319511377f154795d1ebf3`.
A fetched ancestry check proves task commit `2a32e9b43826e2d22239a473181cb2bf94f6afb4` is included.
Required checks passed; latest container publication and deployed revision remain separate, unverified boundaries.

The copied-audio checkpoint had a real decoded-content defect.
The same deterministic noise source gives minimum tail correlation 0.22353 after eviction, versus 0.98544 without eviction.
These measurements establish waveform damage; they do not measure audibility.
Independent packet analysis also confirms a changed AAC grid and encoder history after the old refill seek.

For indexed converted-audio windows up to 30 seconds, recovery now replays the original encoder history from the certified source origin.
Bitstream filters discard packets before the requested video key and its decode timestamp.
Only the requested suffix is written. The original AAC payloads, timing, and initialization remain identical to uninterrupted encoding.
The cache policy invalidates earlier converted-audio output. Longer windows retain their existing path and remain outside this repair's proof.

A new populated public journey compares every AAC packet's PTS, DTS, duration, and payload hash with an independent uninterrupted encode.
The oracle fails before this repair and passes the repaired tail in Chromium and WebKit.
A Chromium watch-page PCM capture improves the recovered-tail minimum correlation to 0.975704 with zero sample shift throughout the measured window.
It also preserves startup and intermediate refill audio. Physical listening and WebKit waveform proof remain open.

Independent review caught a later-origin rounding error, and the expanded public journey caught the same class at the final fragment.
A strict floating-point seconds comparison discarded one 21.333 ms AAC packet at an exact boundary.
The repair compares integer audio sample ticks with the relative source decode time.
It rounds the submicrosecond input-seek remainder to the encoder's sample clock.
The retained 25 fps journey resumes at 64.5 seconds and exercises the earlier numbered refill and evicted tail.
Near-zero recovery and 32/44.1/48 kHz inputs expand the boundary matrix.
Candidate receipts below are based on `205871963ba445806a241fdcfbdb5bcec60336db` plus the recorded implementation changes; clean checkpoint replay follows.

| Private receipt under `.verification/` | Evidence | SHA-256 |
| --- | --- | --- |
| `hls-resume-browser/20261012T004133Z/decoded-splice-comparison.json` | evicted-tail Chromium PCM counterexample | `dd379853afe755dddfe78b5d61d5bc41db16e606f6be64c4772c0fccf6de1988` |
| `hls-resume-browser/20261012T004631Z/decoded-splice-comparison.json` | same-source uninterrupted Chromium PCM control | `209b47b75d33b42e4d9ab22c02da5ee1b9c65098f1b05154678d8a20f9e557c9` |
| `ai-qa-campaign/review-aac-splice-20261012T004712Z/verification-summary.json` | independent AAC encoder-grid diagnosis | `4065d9bfa7908b6daecfe1d7e03c84fd0d7a48a72fb03d32768b100ae96d2b8d` |
| `hls-resume-browser/20261012T005837Z/receipt.json` | new public exact-AAC oracle fails before repair | `ba876b38ef806bd7cb8ac6d925952b62fa2875fb72333931ee657f9ab5dec9bf` |
| `hls-resume-browser/20261012T005932Z/receipt.json` | candidate exact-AAC tail recovery in both MSE engines | `b1a42646fc6d6881df9dc18cd6bab96d736f314bb39f73c4cfbb8750d05d971d` |
| `hls-resume-browser/20261012T005955Z/decoded-splice-comparison.json` | candidate actual-watch Chromium decoded audio | `7bca32d50aceaefde99f2f6667fba0bbd43adb31cf11bbeabb07cd27c95f0b4e` |
| `ai-qa-campaign/review-aac-rounding-20261012T010314Z/receipt.json` | 25 fps absolute-clock rounding counterexample | `4d18a35c9e22101462a3adbe7fd37cd0101cd55e4a4c50ba533f4f1e68313597` |
| `ai-qa-campaign/review-aac-residual-20261012T010854Z/receipt.json` | Matroska input-residual counterexample | `d18f62490f18b121d3aab4f8d6d4ac7543e94800133572502c05153c78a7b050` |
| `hls-resume-browser/20261012T010758Z/receipt.json` | 25 fps late-tail floating comparison counterexample | `7ddfa7eab2b229328f190f22ec2f7d652b74db8aa755ade5eb3db9c9f7a989a1` |

The final sample-tick candidate passes the following populated journeys.
Source limits, whitespace, architecture generation, actionlint, and 169 CI contract tests also pass (five environment skips).
The remaining five local lint findings belong to the stacked cold-resume change and will be repaired before integration.

| Private receipt under `.verification/` | Evidence | SHA-256 |
| --- | --- | --- |
| `hls-resume-browser/20261012T011041Z/receipt.json` | final candidate: resume 64.5s, 48000 Hz | `174afe6b753c10d743b08e145184df6deda37f76ec10967a4429b4b60c193f6c` |
| `hls-resume-browser/20261012T011103Z/receipt.json` | final candidate: resume 0.5s, 48000 Hz | `ec2271ce974870b83cac2da198b4e6d1da6cabaddf39dd883b480cb5605a3de5` |
| `hls-resume-browser/20261012T011125Z/receipt.json` | final candidate: resume 12.5s, 44100 Hz | `bfd248a196432ed040b0c439dd12e6fbffcf72f876f7138c08ec5eccfcf5d073` |
| `hls-resume-browser/20261012T011148Z/receipt.json` | final candidate: resume 12.5s, 32000 Hz | `1ca2ff44fa6f6aeeeabb7fca35e7469ca27a328a2fbec9bbffa47605c979f56b` |
| `hls-resume-browser/20261012T011208Z/receipt.json` | final candidate: resume 12.5s, 48000 Hz | `496c3a003fb8bb74e6499d139ac7bd40fe05fdb3cb63421afb2cfcb4fe420dde` |


### Fractional copied-video cutoff

Checkpoint `583355bb` passes `make -C apps/player verify-changed`, including focused Server tests (81 seconds).
Further independent review found the equivalent video cutoff defect in a Matroska source with a 130 ms video offset.
At source origin 2.05 seconds, flooring the input seek creates a fractional remainder.
Comparing seconds then drops the next copied keyframe. The retained public regression reproduces the resulting incorrect tail video clock.
Copied video now compares its original packet ticks and rounds that remainder to the video time base.
The independent counterfactual restores every audio/video packet timestamp, duration, payload, and initialization.

The public candidate passes Chromium's seven exact frames and playback-ended check, including recovered tail and exact AAC reference packets.
WebKit compatible playback ends and preserves the audio reference, but its original Matroska reference cannot load.
That WebKit comparison remains incomplete; it is not counted as a passing full journey.
The Chromium fractional-origin case joins the required hosted workflow.

| Private receipt under `.verification/` | Evidence | SHA-256 |
| --- | --- | --- |
| `ai-qa-campaign/review-aac-residual-ticks-20261012T011136Z/receipt.json` | sample-tick AAC counterfactual | `bf09597b1c1927015f0e0f6614b9f33472ba20640f5a75e6171e700a10788bca` |
| `ai-qa-campaign/review-video-cut-20261012T011220Z/receipt.json` | fractional video-key loss | `b1a057b3e69a33bf9231569a4819f0a6e145d126d34405accfd4ea8b84d04e65` |
| `ai-qa-campaign/review-video-cut-ticks-20261012T011352Z/receipt.json` | exact audio/video packet counterfactual | `62a5a7fc994b6f274483637038156438f354202ec3baf4cee4afab6bc41bf995` |
| `hls-resume-browser/20261012T011717Z/receipt.json` | public pre-fix tail clock failure | `dfdcf433204406ed3146ebd59698ab1ce2fa64ecea0e1c3de32b071cf2cdccc4` |
| `hls-resume-browser/20261012T011748Z/receipt.json` | Chromium passes; WebKit original reference cannot load | `77bd6d16c9564dd9967149f5c351eecc67ddaaacfcbe66ff622c3a31da380ed2` |


### Clean converted-audio checkpoint

Revision `b8460a8192ad5b6574763c2e2d09a6a642b07a3b` passes the retained fractional-origin Chromium journey and later-origin journeys in both MSE engines.
All 21 reference frames match, playback ends, and recovered AAC packets equal uninterrupted encoding.
`make -C apps/player verify-changed` passes source limits, whitespace, compilation, focused Server tests (82 seconds), and cached E2E discovery.
Independent review closes the bounded packet-tick repair with no remaining concrete implementation blocker.

A clean actual-watch PCM replay preserves the repaired tail: minimum correlation 0.97349 and zero sample shift.
Its no-eviction control gives 0.97570 and zero sample shift at that tail.
Both captures show a separate 128-sample alignment transition around 20 seconds.
The earlier passing candidate and this clean checkpoint produce byte-identical initialization and all ten media fragments for the same source.
That capture/runtime transition still needs characterization; it does not establish changed encoded media or audibility.
Full decoded-audio acceptance remains open while this separate diagnostic boundary is investigated.

| Private receipt under `.verification/` | Evidence | SHA-256 |
| --- | --- | --- |
| `hls-resume-browser/20261012T011922Z/receipt.json` | clean fractional-origin Chromium | `5f0b5d5fefad635956cdfca2f7f1c6834c6d546a60ffa08f801067dbc5e14f69` |
| `hls-resume-browser/20261012T011941Z/receipt.json` | clean later-origin Chromium and WebKit | `31ecf7c0c78b3e45b1d3341090cf6042a9471e74d50ca18340794d82cec5c1ec` |
| `hls-resume-browser/20261012T012037Z/receipt.json` | clean actual-watch PCM capture and packet oracle | `9a5063452b5ae96618dd2a2c68022ac1641627b495d8e458fe37e6f78f2dd576` |
| `hls-resume-browser/20261012T012037Z/decoded-splice-comparison.json` | recovered tail fixed; earlier runtime transition open | `3788d31dd516d3121f7938b7aa2c2a0b553dc814e15f7de0731b8119b41f5310` |
| `hls-resume-browser/20261012T012211Z/decoded-splice-comparison.json` | no-eviction PCM control shares earlier transition | `c0b5508ce7138d09e3e2545d58f77dd0f97b21226ca074ae09a83dc9198ffd40` |

### Cold-resume hosted complexity findings

The refreshed #565 checks rejected four functions for cyclomatic complexity and one readiness function for cognitive complexity.
These are task-caused findings, not the historical lint baseline.
The repair extracts recipe eligibility, untouched-cache validation, suffix bounds, and readiness waiting into focused functions.
It preserves the existing source-policy checks, certification order, cancellation errors, and probe deadline.
Independent review found no admission or certificate defect, and requested preserving the one-time HEVC completion decision.
The final extraction retains that decision once per readiness wait, as before.

The initial candidate passes all thirteen public cold-suffix cases and both browser engines' seven-frame resume journeys.
The public suite covers source replacement, invalid key/configuration proofs, lookback bounds, producer failure, timeout, aggregate deadline, clock race, and fractional recovery.
The branch reconciles current `origin/main` at `a695f71a9b78971559dadd6d355b2338fa219e90`; only the unrelated merged native changes arrive from main.
Focused verification and clean readiness replays follow the checkpoint.

| Private receipt under `.verification/` | Evidence | SHA-256 |
| --- | --- | --- |
| `hls-cold-suffix/20261012T012547Z/receipt.json` | thirteen public suffix cases with source patch receipt | `11d7a7fe867c1fa74feb9622dc97badc0492de0d464cfc6d06023a141dd9b0d2` |
| `hls-resume-browser/20261012T012557Z/receipt.json` | both MSE engines: fourteen frames and ended | `a63605315f52ac75bf7e3be0041e0c1231ef8d30a4cb9d6788c3f860c734171e` |


### Cold-resume complexity repair: clean verification

Clean revision `ce729cfda45cafeae0ec501aa8110c833b11f102` passes the clock-race and producer-failure public controls.
All five short-HEVC completion controls also pass, covering the readiness path shared with the cold-resume change.
Both MSE browser journeys match seven reference frames and reach playback ended.
The local linter reports zero issues against current main.
`make -C apps/player verify-changed` passes compilation and focused Server tests (83 seconds), source limits, and whitespace.
Required hosted checks must rerun on this updated branch; protected integration remains pending.

| Private receipt under `.verification/` | Evidence | SHA-256 |
| --- | --- | --- |
| `hls-cold-suffix/20261012T012834Z/receipt.json` | clean clock-race | `a6e7f2e5f4ca82a3777c5b473fdc8d10aa7860b16e285e5b458b4d086eacde63` |
| `hls-cold-suffix/20261012T012844Z/receipt.json` | clean producer-failure | `51d6c629019ed02bc55c73d15715f5aba86497a587d945e7f7634ce85bc3aacc` |
| `hls-short-completion/20261012T012854Z/receipt.json` | clean short-completion | `e315b5a2a61f8e66d1aedb540082cacf2e5b68e716d7f0988d1b4bf893a5f6fe` |
| `hls-resume-browser/20261012T012959Z/receipt.json` | clean browser | `f84f15fb423dace217f53c6c83f5f86ff02f0e61060283d3ecbe2056cc18a3c4` |


### Stacked audio repair reconciled with the cold-resume checks

Merge revision `7876881369f6ef026840b98cf0b6c778911396fa` preserves both converted-audio eligibility and the extracted cold-resume validation.
The architecture page is regenerated, and both branches' evidence remains in this report.
Cold converted-audio resume and prepared converted-audio tail recovery pass in Chromium and WebKit: 28 exact reference frames, EOF, unchanged source, and settled Servers.
Prepared recovery also preserves every reference AAC packet.
The combined branch has zero new lint issues, passes 169 CI contracts (five environment skips), and passes `make -C apps/player verify-changed`.
Focused Server tests pass in 82 seconds. Required hosted checks and the separate decoded-audio diagnostic boundary remain open.

| Private receipt under `.verification/` | Evidence | SHA-256 |
| --- | --- | --- |
| `hls-resume-browser/20261012T013141Z/receipt.json` | cold converted-audio resume, both MSE engines | `c3e1b5e6f09c187cc37436471d05b6687c8e6012873171da730751835fb528ab` |
| `hls-resume-browser/20261012T013200Z/receipt.json` | prepared converted-audio recovery and packet oracle, both engines | `af9a1171bfaaba5798c320e027a052a91a9eeb987e146731e33949625b9fc68e` |


### Audio diagnostic closure: main-thread recorder lost render quanta

The earlier 128-sample transition is a recording artifact, not a changed media splice.
A simultaneous diagnostic taps the same `MediaElementAudioSourceNode` with both the old `ScriptProcessorNode` and an `AudioWorkletProcessor`.
The worklet records every render-frame index. Its two captures have zero frame gaps and zero source alignment shift.
Their minimum source correlations are 0.95299 around the intermediate boundary and 0.96596 around the recovered tail.
The old recorder diverges after 44,032 and 35,840 identical samples, then loses multiples of 128 samples.
All 129 measured windows are byte-identical between recorders after accounting for those discrete missing samples.
There are no playback seeks, stalls, or rate changes during the measured transitions; only the initial diagnostic seeks are recorded.
The [Web Audio rendering model](https://www.w3.org/TR/webaudio-1.0/#rendering-loop) defines 128-sample render quanta, consistent with the measured losses.
The simultaneous capture provides the evidence for this diagnosis; the specification alone would not establish it.

This private diagnostic uses a browser-context CSP override solely to load its local AudioWorklet module.
It does not verify security policy or physical audibility. The retained populated packet and browser journeys use the normal policy.
Two attempted intercepted module loads and a CSP-rejected blob load are instrumentation failures, not product playback defects.
Their private receipts are retained separately. No production change is needed for this recording artifact.

The bounded audio repair is ready for required hosted checks. Long converted windows, physical devices, and current deployment remain separate boundaries.

| Private receipt under `.verification/hls-resume-browser/20261012T014019Z/` | Evidence | SHA-256 |
| --- | --- | --- |
| `receipt.json` | exact revision, driver hashes, public packet/frame result | `aa9bc37659efd57afcdc632b103b82d5dc99ca02ad77e61124d26286fb70d74e` |
| `decoded-worklet-splice-comparison.json` | continuous audio-thread capture at both boundaries | `dd18d9ba48f77d7d0344fbd8e1142a6463feadb1adc78b62013f778aedb31db6` |
| `decoded-splice-wide-comparison.json` | simultaneous old recorder loses samples | `9660a1f615413852ef6e986ceb181d91f07894f0cafcf121407444535d54c631` |
| `simultaneous-capture-audit.json` | byte-exact windows and continuous render-frame indices | `639160ab0dbee00f579a707c03c1a6110102bd9c5aa205102f32f3b8092fdc56` |

### Cold-resume reconciliation with atomic recovery publication

Required checks passed for cold-resume head `fcfe4e40789ce93f248c5e29373bf4c2889a0c54`.
Main advanced before merge, adding the independently delivered temporary-file replacement repair.
Merge checkpoint `ffbdbb8e06c47e3cbc4e8ea66ee27fe691d3c590` incorporates main `8307ea47fbc51408548e8e50b996b75ad7a0b561`.
Both populated Chromium recovery journeys pass: first fragment and tail fragment.
Each matches all seven original-file frames, reaches EOF, preserves its source, and settles its Server.
`make max-loc` and `make -C apps/player verify-changed` pass after reconciliation.
The private command manifest is `cold-reconciled-20261012T0200/receipt.json`, SHA-256 `927ff83f9ae6fb04a36aafe582f53c1c5df541a180df9c6b7386a66fac1fdff3`.
Required hosted checks must pass again for the reconciled head; no protection was bypassed.

### Audio reconciliation with atomic recovery publication

Merge checkpoint `6d476b60d8f6c00aecc8585cdf781443f5935807` incorporates the reconciled cold-resume branch and main recovery-stage repair.
First- and tail-fragment converted-audio recovery pass in Chromium and WebKit.
The four journeys match 28 reference frames, reach EOF, and retain every uninterrupted-reference AAC packet.
Sources remain unchanged and all test Servers settle. Source limits, whitespace, compilation, and focused Server checks pass.
`make -C apps/player verify-changed` passes, including 87 seconds of focused Server tests.
The private manifest is `audio-reconciled-20261012T0200/receipt.json`, SHA-256 `cdd5c362b01da1b864b0f3b5d4f5982fe4479d22c46d58c1b2e7a80dd05ea5c8`.

### AQA-014 — Open: longer unprepared H264 resume lacks a certified clock

At clean revision `3f6e9e26920e4f021a0bfa86d561311336264594`, a generated 74.5-second H264/AAC source reproduces the longer cold-resume boundary.
Saved progress is 12.5 seconds. The original-file reference presents the expected 12.541667-second frame and reaches EOF.
Unprepared compatible playback presents 12.596028 seconds for the first requested sample, then the frame oracle times out.
Its duration is 74.596027 seconds, and its responses contain neither certified clock header.
The identical prepared control matches all seven frames, reports 74.5 seconds, and reaches EOF.
Both probes preserve the source and settle the Server. Private driver changes extend the fixture and final sample only.
The production watch page and media pipeline are unchanged. This is a confirmed open defect outside the short-window repair.
An independent design review recommends reusing the complete suffix certificate within existing time and output budgets.
Indexing budgets must remain separate from the 30-second converted-audio replay bound.
Incomplete scans must fail explicitly; an origin-only certificate cannot establish future fragment cuts or EOF.
The private comparison is `long-resume-comparison.json`, SHA-256 `ffd1fdea390c61d78582425d95108d29f86e5e3556e95dca99bef702adfc88e2`.

### AQA-012 follow-up — traced repetitions do not reproduce the delay

Twenty unchanged repetitions pass with `GOMAXPROCS=2`, including all sixteen rejected mutation routes per repetition.
The retained Go trace captures scheduler activity, but contains no failing request and cannot establish the hosted failure cause.
The original replacement-policy request log also records 336 ms before its final log emission.
This rules out attributing the entire observed delay to that final log write alone. No threshold or production behavior changed.
The private manifest is `subtitle-admission-trace-20261012T014453Z/receipt.json`, SHA-256 `e08cc02b23ac7a4dd1ab6644eb430f0f31040277ede388b22e986c912e6ef872`.

### Physical device discovery and automation boundary

Fresh CoreDevice queries reconnect the physical iPhone and Apple TV on OS 27.0.
Both report Developer Mode enabled and developer disk-image services available.
Installed Player builds are 3617 on iPhone and 3713 on Apple TV; these numbers do not certify the current task revision.
No Android hardware or physical Apple Watch appears in this inventory.
The campaign uses [Callstack agent-device](https://oss.callstack.com/agent-device/docs/installation) version 0.21.25 from an isolated private tool directory.
It opens the existing iPhone Player, but UI inspection fails with `devtools_security_developer_mode_disabled`.
This reports a Mac developer-tool permission boundary, separate from the enabled device Developer Mode.
Permission to enable the Mac setting was requested; the setting remains unchanged pending an answer.
The automation session closes successfully. No physical UI or playback acceptance is claimed.
The safe private receipt is `physical-readiness-receipt-20261012.json`, SHA-256 `d06e3d19d0778708b35199bcb024bb3d7c52ab62438e2aa0cbf931fb83564fdf`.

### Reconciled tooling repair: refresh the architecture snapshot

The reconciled cold-resume hosted run rejects a stale generated Player architecture snapshot.
The newly merged recovery-stage source file changed that snapshot. The affected-app check alone did not catch it.
Regenerating the Player architecture page repairs the mismatch without a runtime change.
`python3 scripts/tooling/test-architecture-explorer.py` passes for Player and Subtitles; source limits and whitespace also pass.
The original failed tooling job is retained privately from run `38194183334`, job `114633193650`.

### AQA-014 candidate — certify longer cold H264 remux suffixes

The retained 74.5-second irregular-keyframe fixture reproduces the defect before production changes.
Original-file playback reaches EOF. Cold compatible playback has the wrong clock and fails the frame oracle.
The public Server regression also fails its certified-origin assertion on the original implementation.

Plain H264 remux resumes now require a complete suffix certificate within the existing aggregate eight-second deadline.
The existing 8 MiB, 100,010-line, and 4,096-key limits still bound the scan.
IDR, codec configuration, source identity, and rational packet timestamps must all certify before publication.
Converted audio retains its separate 30-second encoder-history bound and exact arithmetic checks.
Oversized or incomplete remux indexes reject safely; this change does not promise arbitrary movie-size acceptance.

Independent review found two candidate regressions. Both received public red cases before correction.
Unsupported duration metadata originally bypassed certification and returned an uncertified HTTP 200.
It now rejects before workload admission, suffix probing, or encoding, inside the same deadline.
The second case uses format duration 74.460 seconds and observed video EOF 74.5 seconds, with origin zero.
It originally rejected a valid source. The repair preserves the existing 0.1-second endpoint tolerance.
Both cases pass after repair. The reviewer reports no remaining concrete code blocker.

All eighteen public Server cases pass. They cover concurrent cold requests, clock publication, fractional and later resumes,
first/middle/tail fragment eviction, warm restart, damaged metadata, source changes, failed probes, and each resource limit.
Warm cache reuse starts no fresh scan. Corrupt or missing metadata rebuilds with an identical verified certificate.
Rejected indexes publish no playlist and start no media encoder. Safe failure logs retain request/session correlation.
Every owned Server and probe settles. Sources remain unchanged except in the deliberate source-change case.

Chromium and WebKit MSE match all fourteen reference frames and reach EOF.
WebKit native HLS also matches seven reference frames and reaches EOF with MSE capability deliberately disabled.
That last result exercises the native decoder policy; it is not physical Safari or Apple-device proof.
The scan-span calculation was extracted without changing its policy to satisfy the existing complexity gate.
The final extracted source passes both MSE browser journeys and the endpoint-tolerance public recovery case again.
Local lint reports zero issues. Architecture snapshots, source limits, and whitespace checks pass.
The shared CI workflow now retains the longer remux browser journey in the required Chromium job.

The private manifest `long-remux-reviewed-receipt.json` records commands, fixture receipts, revisions, and artifact hashes.
Its SHA-256 is `6e9460838b4abbaf391da3caf2a9e606d5e4274c8b1fb45464a7cc4a0e3c0e48`.
Each public receipt also captures the tested patch and helper hashes, so uncommitted candidate runs remain reproducible.
Long converted-audio recovery, physical-device acceptance, production deployment, and larger real-world sources remain open.

### Canonical Player container browser check — certificate trust boundary

The campaign invokes `make -C apps/player test-instance-check` with private image tags and an isolated Compose project.
The production image, populated fixture image, health verification, and browser launch succeed.
Browser requests then reject the fixture's self-signed certificate with `ERR_CERT_AUTHORITY_INVALID`.
No TLS check is disabled. These failures do not count as UI coverage or a passing canonical container journey.
The separate authenticated HTTP media regressions above retain their own passing evidence.
Private evidence is under `long-remux-test-instance/`; the canonical browser failure remains under investigation.
The completed run has five passes and 82 failures, all classified as certificate trust failures.
Its private receipt SHA-256 is `6bd6a6b4a5aee978bb097821ccfae190379c438bf8302f2e6c0f43b9d5c6e124`.
The canonical cleanup removes its isolated container and volumes; unrelated running containers remain intact.

### Longer-remux delivery checkpoint

Implementation checkpoint `3c2d803127ab98a3ccb9ddf75280af9ffdea9e09` contains the reviewed repair and retained regressions.
Checkpoint `3e5d1731` removes one trailing blank line caught by the post-commit whitespace gate.
`make -C apps/player verify-changed` then passes, including compilation and 85 seconds of focused Server checks.
All 169 CI contract tests complete successfully, with five existing skips. Local actionlint passes.
Main advances only in native test coverage and its separate report during this checkpoint.
PR #565 incorporates main `013d5247` with merge `13099b04`; PR #569 incorporates that base with merge `07ba1885`.
Longer-remux merge checkpoint `9f8d2facc0d77e03602ebb501bd9928a44849975` includes both updates.
The affected-app verification passes again after reconciliation, reusing unchanged source-check results.
Required hosted checks still control integration. No branch protection or certificate verification was bypassed.

### Combined resume integration and current main reconciliation

PR #570 now targets main and includes the reviewed cold-resume, audio-clock, and longer-remux commits.
PRs #565 and #569 remain open until fetched ancestry proves those commits reached main through #570.
Independent review of `1ba568395b41fabe4f6c285ddc7a551bc41c2fae` finds no concrete integration blocker.
That checkpoint includes main `394736cb740fc6873aa6e94b41e22dc797155529`.
The hosted tooling job identifies stale architecture metadata after main changed the Show handler.
Regenerating the Player snapshot repairs the mismatch; both architecture checks, source limits, and whitespace checks pass.
This repairs generated documentation only. Required hosted checks still control the merge.

### Canonical Player fixture inventory and sign-in pacing

The canonical cleanup check still expected six retired media names. The generated Example media files are present and nonempty.
The assertions now name those actual fixtures. The test Server continues to use HTTPS.
A disposable Linux browser trusts only the validated public fixture CA; Mac trust remains unchanged.
Both browser and API requests reject that CA before installation and succeed after installation.
The first trusted run has 82 passes and five failures. Four failures come from unavailable Git metadata while saving evidence.
A read-only mount of the worktree's common Git directory repairs those receipt failures.
The fifth trace records `POST /login` returning HTTP 429 with `Retry-After: 60`.
The suite shares one IP and can exceed the production limit of 30 credential attempts per minute.

An unchanged assertion rerun passes all 87 tests, with zero retries, skips, or failures.
It also verifies all eight expected fixture files and removes its owned containers, volumes, temporary fixture root, and environment file.
This rerun confirms the Git evidence repair, but does not resolve the timing-dependent sign-in collision.
The private receipt is `player-fixture-trusted-final/receipt.json`, SHA-256 `87494e9a9ec0aff3c97a98e9b10265b59ae2d583a6fab5ac1f62d32f08d3e507`.

The test repair waits 2.1 seconds before each ordinary populated-fixture credential submission.
It generates rotating codes after waiting, submits once, records the status, and requires the real HTTP 303 response.
All selected local and shared Owner/Viewer helpers use that operation. Existing destination and form assertions remain.
The canonical runner rejects multiple workers; parallel runs require separate fixture instances.
Production rate limits and dedicated negative-authentication checks remain unchanged.
Independent review identifies an isolated readiness control that returned HTTP 200 with a meta refresh.
That control now uses the production HTTP 303 redirect contract. Pacing applies only to populated-instance tests.
The completed paced run passes all 87 tests, with zero retries, skips, or failures.
All 78 recorded sign-ins return HTTP 303. The two isolated readiness controls also pass.
Both public-CA controls and all eight inventory checks pass. Owned containers, volumes, fixture directories, and private environment files are gone.
The private paced receipt is `player-fixture-paced/receipt.json`, SHA-256 `c56a902f55ca735e73d6fe8fa31f4215e53da083fbfc199cab6d8237030a4e60`.
That run uses test checkpoint `9d0a382dab0be59749ecc9106cb7ec9b406cab35` and the unchanged production source from main `394736cb`.

### Resume integration reaches protected main

PR #570 merges into main `e0a9536c6a04198a2eff9f5306c7c658fcdf9406` after its required checks pass.
A fresh fetch and four ancestry checks prove that the cold-resume, audio-clock, longer-remux, and final reviewed checkpoints are included.
GitHub also marks the earlier PRs #565 and #569 merged. No protection was bypassed.
The private ancestry receipt SHA-256 is `7c9774543ea3c121fac670bea95b2bfd162eda1b9be9094c1dc72fd12d529e52`.
The fixture repair now incorporates that main revision. The fresh production-container result follows below.
Hosted main publication, deployed-image acceptance, and physical devices remain separate boundaries.

### Fresh integrated container and native Core verification

Clean checkpoint `09bc16a7cdcee188ec55fcc6f82816e0163cf945` builds a fresh production image containing the merged resume repairs.
Its image revision label matches that exact checkpoint. The canonical trusted-HTTPS Chromium suite passes all 87 tests.
There are no retries, skips, or failures. All 78 credential submissions return HTTP 303.
Both CA controls and all eight fixture checks pass. Owned containers, volumes, fixture directories, and private environment files are removed.
The private integrated receipt is `player-fixture-integrated/receipt.json`, SHA-256 `9a578c5e8bf4e56016867a6fd749fd981d1ca1b24e9c27585d6849e3d6880083`.

The existing Swift Core journey also passes all sixteen response-validation cases against a real loopback Player handler.
It fetches media after successful validation. This is native Core and Server compatibility evidence, not AVPlayer or device interaction.
Its outer receipt is `native-core-integration/evidence/manifest.json`, SHA-256 `9dcdcad22e8e5600479045a7da9bd6a235dfcbcda085e0e0beee3b25c2ee1bea`.

### Login-readiness fixture: contain the complete redirect chain

Hosted run `38200899240` passes 345 smoke cases but fails the delayed-script readiness control twice.
The intercepted `POST /login` returns HTTP 303. Its redirected `GET /` reaches the unrelated configured Server.
That Server redirects back to `/login`, so the helper correctly refuses to complete the sign-in journey.
The missing-script case passes. The fresh populated-container result above remains a separate passing boundary.

A disposable Linux peer reproduces the unexpected redirect before the fixture repair.
The readiness fixture now serves its complete same-origin HTTP redirect chain and the actual production theme script.
It retains empty credentials and zero submissions while initialization is pending, one successful submission afterward,
and zero submissions when the script is missing. Cleanup closes the peer even if page creation or closure fails.
Independent review finds no concrete blocker. Both cases pass in Linux Chromium, Firefox, and WebKit at the normal timeout.
Every outer-peer request count is zero, and all owned browser containers and peers close.
This is an isolated native-form fixture; it does not establish populated Server authentication or TLS behavior.
The private receipt is `login-readiness-redirect-receipt.json`, SHA-256 `1f24d60d0bff7a0c1129a98c0c64c8768316f221a278230c78325156bd1ff2d9`.

### Longer converted audio: repeatable decoded splice difference

Two fresh 74.5-second H264/AC3 experiments pass all seven Chromium video-frame comparisons and reach EOF after tail regeneration.
Both use deterministic nonperiodic audio and retain actual watch-page AudioWorklet captures before and after eviction.
Render blocks are contiguous, and source alignment shows no sample-clock shift.
The captures are identical through sample 42,495, then differ at source 72.885333 seconds, matching the regenerated audio onset.
Two short windows have markedly lower correlation with the control before recovering. A second run repeats the same result.
This establishes a localized decoded difference; audibility and the precise encoder/decoder cause remain unproven.
AAC packet identity is recorded diagnostically because the longer path restarts the encoder. Different bytes alone are not a failure.
No production audio behavior changes in this checkpoint. A bounded warm-up experiment remains the next investigation.
The first successful capture receipt is `.verification/hls-resume-browser/20261012T035748Z/receipt.json`, SHA-256 `cb97bd7ff35e3c0daf99a27a588b568691409a7a9266b8c2b9822f98cb6ca030`.
Its decoded comparison SHA-256 is `a416029719a52f5550f81a67d6a1e459f554028b0b18002f5905661b180869d0`.
An earlier private recorder syntax failure is retained separately and provides no audio acceptance.

### Architecture selection reaches protected main

PR #574 merges after all required checks pass. A fresh fetch proves both reviewed selector checkpoints in main `a95f2cbb2ed8a1a27eb39e311be1c8f71a95c50d`.
The selector includes architecture-generating Go sources, module inputs, and architecture documentation without losing app isolation.
The private ancestry receipt SHA-256 is `ec4fb4879d0b94cfc3d7df7c1db5a3265f16ec525690e8953c0f1f32e2a978a7`.

### AQA-015 — Long converted-audio refill changes its packet grid

The retained `test-hls-audio-splice.py` journey confirms decoded damage before any production repair.
It uses a real disposable Player, a 74.5-second nonperiodic AC3 source, and the unchanged watch page.
An AudioWorklet records render-thread samples. One source alignment remains fixed across each capture.
The initial refill reaches correlation 0.7395; tail recovery reaches 0.7013. The untouched tail control remains above 0.9797.
All three captures have contiguous render blocks and zero sample lag. Seven video reference checks and EOF pass.
The source remains unchanged and the Server settles. Perceptual audibility and physical devices remain separate.
The recorder's CSP exception permits instrumentation; this journey does not establish CSP security.

The failing receipt is `.verification/hls-audio-splice/20261012T043453Z/receipt.json`, SHA-256 `581b619c061e6f398d2e106bc7dba04fe015842ce1fd3d3ddbd55055c678eb40`.
The outer manifest is `long-audio-retained-red/manifest.json`, SHA-256 `9d9909c2d1f745f3e8b4fbcf9fb581bfb5fe7ac96ab6b7537a41de80b98b8133`.
Both bind production base `a7623d3335e95ab86157218f5c78f77cab2b6242` and the test patch.

Private experiments separate encoder warm-up from packet alignment.
Warm-up alone either leaves a mismatch or introduces a 5.3 ms gap. Those variants were rejected.
Warm-up aligned to the cached packet grid removes the tail mismatch without shifting source content.
A second experiment uses the initial fragment's grid for population and middle-fragment recovery.
Both decoded joins pass with the old neighboring fragments preserved; minimum source correlation is 0.9804.
That experiment is `.verification/hls-resume-browser/20261012T042906Z/`, with helper snapshots and exact environment in its receipt.
These are bounded generated-media experiments. The production repair is still pending.

### Publication and deployed-revision boundary

Main CI `38200496460` passes for `e0a9536c6a04198a2eff9f5306c7c658fcdf9406`.
Both immutable app images are published, scanned, signed, attested, and verified.
The green production-tag jobs explicitly retain existing tags because newer main changes affect the apps.
They do not prove promotion of that revision. The private receipt is `e0-publication-receipt.json`.

A fresh read-only Nox inspection finds both apps running and healthy.
Player reports `394736cb740fc6873aa6e94b41e22dc797155529`; Subtitles reports local snapshot `6f41117dfa618ad65479f91463d431ab2f500996`.
Neither proves deployment of current main. Historical local-watcher receipts also do not establish current remote state.
The safe receipt is `nox-readonly-20261012.json`, SHA-256 `d4d691056d7fe7fe3447686720ecfe4602abe381e6737085c6b1e157933c72c9`.

### AQA-015 expansion — Source timestamps and fragment interleaving

The first production candidate passes 48 kHz tail recovery and 44.1 kHz MP4 middle recovery.
It fails the 44.1 kHz Matroska case with a 130 ms video offset. The candidate remains unpublished.
The retained journey now selects sample rate, early or middle recovery, and the fractional video offset.
Its middle and early cases restore the old neighboring fragments after recovery, representing an already buffered client.
The source and disposable Server settle after every completed run.

A private counterfactual regenerates identical AC3 packet payloads in MP4 and Matroska.
Both containers retain the same 256-sample priming information. Their audio clocks differ in precision.
Before AAC encoding, MP4 seeks have zero sample error. Matroska seeks have errors of 1, 10, and -4 samples.
The errors persist through each five-second decode. This separates source sample selection from the output AAC packet grid.
The exact commands and packet hashes are retained in `long-audio-exploration/input-clock/receipt.json`.

A fixture-specific prototype disables timestamp trimming and reconstructs the AC3 sample clock before trimming.
It accounts for 1,536-sample packets, first-frame priming, and the final partial frame.
The real watch page then passes initial population, middle recovery, and both retained-neighbor joins.
All four captures have zero sample lag; minimum correlation is 0.9584. Packet grids, seven video references, and EOF also pass.
The passing receipt is `.verification/hls-audio-splice/20261012T045910Z/receipt.json`, SHA-256 `5c4cc4802b1f8c15f4b07265782d2d7e57df25d0d697672950c9998ef3f8e767`.
This wrapper assumes known synthetic input properties. It is not a general production repair.
A production implementation must validate packet continuity, priming, clock precision, and genuine gaps before correcting sample selection.

The 8 kHz ALAC case exposes a separate fragment partitioning defect during early recovery.
Audio timestamps remain contiguous, but audio needed for one video fragment appears in the following fragment.
The browser jumps from source 16.46947 to 18.1 seconds. The missing capture duration correctly fails the journey.
An earlier attempt to create 8 kHz MP3 in MP4 fails before Player starts; that run provides no product evidence.

A private outer-muxer interleaving experiment removes the 8 kHz jump.
All four watch-page captures pass with one sample of initial lag and minimum correlation 0.9847.
The passing receipt is `.verification/hls-audio-splice/20261012T050004Z/receipt.json`, SHA-256 `2117d485b147834ce0adb6aa38ecbc3d91f362f3872f6be830ee3c3231c9828b`.
Unbounded buffering with sparse or delayed audio remains a production design concern.
The private experiment bundle includes the candidate patch, wrapper sources, exact commands, and checksummed passing and failing receipts.
Its index is `long-audio-exploration/counterfactual-checkpoint/index.json`, SHA-256 `ee3ee53d980b7da94d1d9c0d074bdb90ff1f74e0ae91416375a61179cc28317c`.
Neither prototype establishes physical-device playback or perceptual audibility.

The retained packet oracle now checks fragment assignment as well as continuous timestamps and AAC phase.
Each fragment must contain audio through the following video's decode boundary, within one AAC packet.
A retrospective check rejects the preserved failing 8 kHz cache and accepts the interleaving prototype and both 44.1 kHz controls.
This prevents later preloaded fragments from hiding misplaced audio. The PCM oracle also requires contiguous render blocks.
The private projection is `long-audio-exploration/fragment-partition-counterfactual.json`, SHA-256 `07766a823d17c0f218e8894155e239dad03faf134d8af84ed6b8c32c4d6d688f`.

Independent full-suffix replay confirms the interleaving cause without changing encoded audio.
The default and inner-muxer-zero variants retain identical init and all 474 AAC packets but assign them to the wrong fragments.
Outer 20-second, 30-second, and zero variants produce identical corrected fragments for this fixture.
A debug replay records 1,211 forced flushes just above the default ten-second threshold.
Output queue sizes of eight and sixteen packets leave the defect unchanged.
These measurements do not establish a general safe interleaving buffer limit.
The private full-suffix receipt SHA-256 is `1f38dc5b04ad239092860e483728a4a569d019029bd1feed06d872d208ae5bc6`.
The output-queue counterfactual receipt SHA-256 is `0adfbd36dbae561866bdf4258b4c0ec4541f4fb2832c529fbb695f730fee1c51`.

A separate two-hour synthetic AC3 scan decodes 206,719 frames and exactly 317,520,000 samples.
Every frame matches one continuous sample clock under nearest-millisecond quantization; channel configuration remains stable.
The local scan takes 3.95 seconds and peaks at 17,219,584 resident bytes.
This makes decoded-frame validation worth testing within the existing preparation budget.
It does not establish end-to-end preparation time, high-bitrate behavior, Linux performance, or encoder buffer limits.
Its exact generation and probe commands, source checksum, and measured limits are in `long-audio-exploration/movie-clock-probe/receipt.json`.

### Current main publication after the fixture repair

Main `4d8ddd2b3e64279ee41f58db90ba0ebf393e9914` passes all required checks in run `38204654259`.
Player's AMD64 and ARM64 builds pass testing and scanning; publication and production-tag advancement complete successfully.
Both publication and promotion push digest `sha256:abb3c637fcabe1a05314f4f862bbf94659917daddaf706f8379766e17506bfb8`.
A fresh fetch also proves reviewed fixture commit `a7623d3335e95ab86157218f5c78f77cab2b6242` is in that exact main revision.
The safe evidence is `main-4d-publication-receipt.json`. This does not establish Nox deployment or include the pending audio candidate.

### AQA-015 production candidate — Certified audio clocks and bounded refill

The candidate now validates the complete decoded audio clock before correcting coarse timestamps or warming an AAC refill.
It checks one timestamp quantizer, channel configuration, sample counts, priming, and final-frame boundaries.
The optional proof has a six-second budget and reserves two seconds of the existing preparation deadline.
Unsupported or discontinuous sources keep the legacy seek behavior. Incomplete proof never certifies a clock.
Expected opt-outs produce one correlated warning. Genuine process failures retain their error evidence.
The shared probe ownership and settlement checks remain intact.

Certified refill aligns input samples with the observed initial AAC packet grid.
It reconstructs coarse AC3 timestamps before trimming and uses a finite 30-second interleave window.
Independent review found two rounding boundaries in fractional-frame-rate sources.
Input trimming and mux shifts require separate rounding; input time must first match FFmpeg's microsecond seek.
Both boundaries have retained real watch-page failures and passing repairs.
The submicrosecond case improves minimum initial-join correlation from 0.7282 to 0.9868 without relaxing the oracle.
Its four captures retain one fixed sample lag. Packet placement, seven video references, and EOF pass.
Nine forged clock records also return 404 without encoding, source changes, or cache asset changes.
Their diagnostics contain one bounded, correlated warning without fixture credentials or source paths.

The real HTTP source controls cover a gap, delayed audio, optional timeout, parent cancellation, and a genuine probe failure.
The timeout previously consumed 12.23 seconds and left preparation unavailable.
It now falls back to ready playback in 9.35 seconds. Cancellation settles all owned children in 0.46 seconds.
Gap and delayed sources remain uncertified and ready. The genuine probe failure retains an error and a fallback outcome.

A two-hour, 180p source reaches ready playback in 9.17 seconds and certifies exactly 7,200 seconds of audio.
Sampled peak resident memory for the Server and its process tree is 576,815,104 bytes.
This is local preparation evidence, not full-movie browser playback, high-bitrate buffering, Linux, or physical-device proof.
Long cold converted-audio behavior and current-main deployment remain separate boundaries.

| Private receipt | Result | SHA-256 |
| --- | --- | --- |
| `hls-audio-splice/20261012T053126Z/receipt.json` | Fractional frame rate, middle refill, old neighbors and nine corruption controls pass | `1c72a0e20d29b87ddbff8ac617b52d707340c1005326e233d188e3598f613c45` |
| `hls-audio-splice/20261012T053728Z/receipt.json` | Submicrosecond origin and the same complete checks pass | `9a7ca8a4327529520d022146cd7a18a0cec963f0c5fb7560455dc4124ba71e18` |
| `hls-audio-source/20261012T053502Z/receipt.json` | Five real HTTP source controls pass | `ebf7c3511e54d6149f40cb96a47d37e3c2be89adbc88b78f41d51ae2c0fa3653` |
| `hls-audio-source/20261012T054025Z/receipt.json` | Two-hour preparation and owned process settlement pass | `799ac75da2e1f02eefbdb796329c129149931db624a3b5cc97a64005e426b82e` |

These candidate receipts bind their dirty patch and binary. Final revision checks and protected delivery remain pending.

### AQA-016 — Small video dimensions produce an invalid HLS quality label

The original two-hour fixture uses valid 32-by-32 H.264 video and continuous 44.1 kHz AC3 audio.
Player certifies its source clock, then fails preparation with `HLS quality is invalid`.
The source and owned processes settle unchanged. This is a distinct open defect in rendition naming or validation.
The failing receipt is `.verification/hls-audio-source/20261012T053815Z/receipt.json`.
Resizing only its video to 320-by-180 produces the passing preparation above; the audio packets are copied.
The exact fixture conversion and hashes are retained in `long-audio-exploration/movie-clock-probe/supported-generation.json`.

### AQA-015 precision review follow-up

The source filter now shares the integer microsecond-to-sample conversion used by refill alignment.
At a coarse 11.075-second origin, the retained failure has minimum initial correlation 0.6579.
The repaired journey reaches 0.9590; all four captures have zero lag, and video, EOF, and packet placement pass.
Its receipt is `.verification/hls-audio-splice/20261012T055221Z/receipt.json`.

Refill now derives the initial audio shift from the measured AAC phase.
The correction must fit the source video quantum plus bounded timestamp rounding.
This handles independent mux rescaling and precision lost in the printed video clock.
A real 96 kHz source with a 1000 Hz video track confirms the two-sample boundary identified by review.
The reported video clock is 0.010688 seconds; the AAC phase identifies an exact 1,024-sample shift.
Four captures, both fragment grids, seven video references, EOF, and ten corrupt-metadata controls pass.
The added control rejects an otherwise well-formed phase shifted by half an AAC packet.
Its receipt is `.verification/hls-audio-splice/20261012T055409Z/receipt.json`.

A separate 400/29 fps diagnostic does not reproduce the proposed audio-clock rounding failure.
All four decoded-audio captures and both packet grids pass before phase calibration.
Its compatible HLS reaches EOF and all seven video hashes match, but the original-file reference times out near 30 seconds.
This remains an unresolved reference-playback boundary, not evidence that the audio repair passed the whole journey.
The complete failed receipt is `.verification/hls-audio-splice/20261012T054923Z/receipt.json`.

The worktree audit also reports ten unrelated inactive checkouts: nine contain unique commits and one is dirty.
Their files and leases remain unchanged. The active campaign checkout has a renewed lease.

### AQA-015 final recorder boundary and revision checks

At committed revision `159e5c14b2a248c3f358e6621200b9a7d7d4db28`, the quantized mux-clock journey passes.
The 32 kHz first-fragment journey exposes a recorder startup gap before playback begins.
Its recovered and retained-neighbor captures have identical PCM hashes and zero alignment lag.
The retained capture records one 128-sample paused block, then a 1,280-frame gap before continuous playback.
All decoded correlations exceed 0.985; video, EOF, both packet grids, and ten metadata controls pass.

The recorder now suspends its audio clock during setup and resumes only after the media element accepts play.
The continuity assertion still checks every recorded render block, with no gap exemption or PCM realignment.
The complete 32 kHz journey then passes all four captures with zero lag and no render gaps.
The failed receipt is `hls-audio-splice/20261012T055854Z/receipt.json`.
The passing receipts are `hls-audio-splice/20261012T060030Z/receipt.json` and `hls-audio-splice/20261012T060442Z/receipt.json`.

Independent review finds no remaining blocker in the eight changed production Go files.
At revision `159e5c14`, affected Player verification, focused Server lint, source limits, and both architecture snapshots pass.
All 170 CI contract tests pass with five existing skips; pinned actionlint also passes.
Protected PR checks and delivery remain pending.

### AQA-016 regression owner and reproduced boundary

The existing resumed-playback browser journey now accepts a bounded 32-by-32 synthetic fixture.
It checks unchanged source dimensions, original-frame equality, saved position, EOF, and invalid quality-path rejection without cache writes or encoders.
Existing fixtures are at least 180 pixels high, so they miss the lower-bound mismatch. No production seam is added.
The new fixture reproduces the earlier two-hour failure before browser playback: preparation never becomes ready.
The Server records `HLS quality is invalid`; source files and owned processes settle unchanged.
Its private receipt is `hls-resume-browser/20261012T061029Z/receipt.json`.
The public interface is the HLS master and rendition path. Valid small source dimensions must retain their original dimensions.
The current path validator rejects every quality below 144p although rendition generation uses the source's actual height.
The intended repair admits positive heights while preserving the existing 2160p ceiling and file/path checks.

The one-line lower-bound repair passes the complete small-video journey in Chromium and WebKit.
Each engine preserves 32-by-32 dimensions, matches all seven reference frames, resumes at the saved position, and reaches EOF.
Six malformed or out-of-range quality paths return 404 without starting encoders or changing cache assets.
The source remains unchanged and the Server settles. The passing Server log contains no HLS warning or error.
Receipt `hls-resume-browser/20261012T061139Z/receipt.json` retains the browser results and checksums.
The required Chromium workflow now retains this case with the existing prepared-timeline artifacts.
The production change only lowers the allowed positive rendition height; the upper bound and path confinement remain intact.

### Original-file EOF observation resolved

An isolated Chromium reference probe reproduces the earlier 400/29 fps behavior using the same synthetic media bytes.
After the final paused seek, `currentTime` is 74.48 seconds and `ended` is already true, despite a 74.53-second duration.
Calling `play()` then restarts at zero. This explains the earlier 30-second timeout without a Player change.
The private source-bound probe is `reference-eof-diagnostic/receipt.json`; it is isolated browser evidence.

After collecting the seven frame references, the browser oracle now seeks half a second before the end.
It requires `ended` to be false before playing and still waits for the real `ended` event.
The complete 400/29 fps public journey passes all four audio captures, packet grids, video comparisons, EOF, and ten metadata controls.
Its receipt is `hls-audio-splice/20261012T061514Z/receipt.json`. The workflow retains this low-frame-rate case.

Independent review finds no blocker in the small-video repair or its regression coverage.
Focused playback package tests and changed-code lint pass. CI contracts pass 170 tests with five existing skips.
The required Player local verification stops at the existing 95-issue shared-package lint baseline.
That failure does not establish a passing complete local verification suite; hosted checks remain required.

The final small-video browser repetition passes Chromium and WebKit at revision `e195724b`.
Receipt `hls-resume-browser/20261012T061651Z/receipt.json` binds the completed EOF correction.
Subtitles local `verify-changed` encounters the same 95-issue package-lint baseline.
One later CI-contract run encounters an intermittent permission error while polling an owned process group.
The unchanged nine-test Q09 subset passes immediately afterward; the original error remains retained for diagnosis.

### AQA-017 — Subtitles compatible playback differs from the original frame clock

A separate real Subtitles process plays the 32-by-32 source at the saved position and reaches EOF.
Both Chromium and WebKit preserve dimensions, but only one of seven paused frame references matches the original.
Chromium repeats the same six mismatches in a second fresh process. This is separate from the repaired quality-label rejection.
Compatible duration is 32.083332 seconds in Chromium and 32.166667 seconds in WebKit; original duration is 32 seconds.
The exact timing cause remains unverified. The strict browser comparison stays failed.
Six invalid quality paths still return 404 without encoders or changed cache files. Sources and Servers settle unchanged.
The private receipts are `subtitles-small-video-2/receipt.json` and `subtitles-small-video-3/receipt.json` in the campaign directory.
An earlier harness attempt called a Player-only settings endpoint and received 404 before browser playback; it is not product evidence.

### AQA-015 Linux rounding phase and coarse-clock boundary

[PR 575 CI](https://github.com/Kinosail/kinosail/actions/runs/38211142029) rejects the 44.1 kHz Matroska source clock.
The 48 kHz and 32 kHz journeys pass before that failure.
A disposable Linux Noble ARM64 Server and Chromium reproduce the failed audio with Jellyfin FFmpeg 8.1.3.
Hosted CI uses the same media-tool version on AMD64; these are separate execution environments.
The reproduction retains the exact fixture, binary, commands, browser data, source hashes, and patch in private receipts.

Every one of the 2,139 decoded AC3 frames fits one fixed timestamp quantizer.
Its rounding phase differs from the nearest-to-zero rule after the container normalizes negative encoder priming.
The full-frame intersection is `[30600,30899]` in units of 1/44100 of a container tick.
The certificate now intersects that one phase across the entire source.
It never assigns a separate tolerance to each frame. Sample-clock streams retain their existing exact check.
The integer comparison bounds untrusted timestamps before arithmetic.

Independent review also identifies an ambiguous-clock boundary: a one-second timestamp cannot identify an AC3 block after seeking.
Coarse certification now requires a timestamp quantum no larger than one millisecond.
A selected-probe metadata fault reproduces the previous incorrect certification before this guard.
This fault is an isolated proof boundary; it is not a real one-second source or browser acceptance result.
The cache policy changes so retained generations must obtain the current certificate.

On Linux, the repaired browser journey improves initial minimum correlation from 0.7178 to 0.9897.
Recovered and retained-neighbor captures improve from -0.0621 to 0.9804.
All four captures have zero sample lag and uninterrupted render blocks.
Both packet grids, seven reference frames, EOF, and ten metadata rejection controls pass.
Seven source controls also pass: gap, delay, a real one-millisecond gap, ambiguous metadata, timeout, cancellation, and probe failure.
Rejected clocks retain one correlated warning and ready playback without a guessed clock.
The source and owned Server processes settle after every case.

| Private receipt | Result | SHA-256 |
| --- | --- | --- |
| `hls-audio-splice/20261012T064047Z/receipt.json` | Linux browser reproduction fails | `34083254128c3a6738001e296e89ac7da9345262a62be0cc9bc7ed0851aac7b0` |
| `hls-audio-splice/20261012T064324Z/receipt.json` | Fixed quantizer: four captures, grids, video and ten metadata controls pass | `dba88aefb6c51f114066f7408c87034141f61ddf1ab1dd4cfb73fd3f12da60cc` |
| `hls-audio-source/20261012T064448Z/receipt.json` | Ambiguous one-second metadata is incorrectly certified before the guard | `b64b7af70f8f1526a8b87b388476ffa9a8aadc8385295da605d80b59e8ff03b5` |
| `hls-audio-source/20261012T064529Z/receipt.json` | Seven source and fault controls pass after the guard | `3e04c11cd9cf45f9654fb2d204aa7e387e91b68f6832c4347eeb0cd13c4aa6e1` |

The Firefox native-layout job in [run 38211141937](https://github.com/Kinosail/kinosail/actions/runs/38211141937) passes one unchanged retry.
The original 844-by-390 navigation timeout remains an intermittent observation with its trace retained.
No timeout, layout assertion, or navigation behavior was changed for that retry.
Protected delivery and current-main deployment remain pending.

At commit `72ed3bda`, the final guarded Linux 11.075-second-origin journey passes all four audio captures, both grids, video, EOF, and ten metadata controls.
Its maximum absolute lag is one 48 kHz sample; minimum correlation is 0.94446.
The receipt is `hls-audio-splice/20261012T064719Z/receipt.json`, SHA-256 `ec5bfd4ebe367f0e0d86d80a3ddf73f2fbac187da42e5c3512ca0ae30919d545`.
The final Mac coarse-origin regression also passes, including all four captures and the same checks.
Its receipt is `hls-audio-splice/20261012T064823Z/receipt.json`, SHA-256 `7fc84f81f12c994b89373dbdde5fbddaf92fc79bca73ef1ffb764d73af79db1d`.
Player `make verify-changed` passes compilation, affected Server tests, source limits, and diff checks at that commit.
The regenerated Player architecture snapshot and source limits pass. The final independent review reports no concrete blocker.


### AQA-017 copied-clock investigation checkpoint (not accepted)

The retained Subtitles watch-page regression now covers Chromium and WebKit frame hashes, converted AC3 audio PCM, and an explicit Direct First to Compatibility switch. The local candidate fixes initial source-clock normalization. Receipt `20261012T071404Z` passed both browsers and both strict decoded-audio captures. This is not a completed repair.

A keyframe-cut prototype passed completed-file frame checks but introduced invalid speculative VOD timelines while encoding. A held real encoder had one four-second fragment; the HTTP playlist advertised eight. Completed real cuts were 4, 6, 4, 7, 6, 4, and 1 seconds. The keyframe-cut production change was removed. Its rejected harness remains private under `.verification/ai-qa-campaign/subtitles-mux-probe/rejected-keyframe-prefix.py`.

An offline packet matrix identified FFmpeg fragment-index PTS adjustment. With fixed-time cuts, 600 copied packets had only 594 distinct PTS. `movflags=+skip_sidx` preserved all 600 timestamps; `frag_every_frame` did not preserve composition offsets. FFmpeg documents the relevant adjustment in [movenc.c](https://github.com/FFmpeg/FFmpeg/blob/n8.1.3/libavformat/movenc.c#L6581-L6597). Independent review confirmed that HLS does not require SIDX, and Player already omits it.

The remaining real-browser boundary still fails: receipt `20261012T072647Z` matches six of seven samples in both browsers but times out seeking the reordered final frames after an irregular 64.5-second resume. The raw window starts at source keyframe 58 while the requested window is 60 seconds. Native WebKit capability simulation `20261012T072740Z` starts at 67 rather than saved 64.5 and matches four of seven samples. Do not publish this candidate or claim native acceptance. The exact runs, source patches, media, packet probes, and private logs remain in `.verification`.

The explicit MSE direct-switch case passed at `20261012T072447Z`; no red proof is claimed for that adapter branch. On this Mac, native HLS must be capability-disabled to reach lazy Hls.js loading. The first direct-switch probes timed out because the test did not open Playback behavior or incorrectly assumed native HLS could not be selected.

The workflow contract suite currently identifies a recurring macOS `killpg(...,0)` EPERM in Q09 owned-descendant cleanup. This is retained as an environment or harness investigation; assertions and cleanup boundaries remain unchanged.


### Hosted audio regression proof and bounded CI runtime

PR 575 run `38214459101` completed all continuous-audio sample-rate/offset cases and the source rejection controls successfully on Ubuntu. The job then reached its 30-minute job deadline during source-version verification. That cancellation also prevented its final proof and upload steps from completing; the required Player check remains failed.

The browser job budget is now 45 minutes. Per-test and per-step deadlines, required selections, assertion thresholds, and artifact requirements are unchanged. Local validation passed all 170 CI contract tests (five skips), pinned actionlint, source-file limits, and whitespace checks. This is a runtime allowance supported by the observed job duration, not a passing hosted result. The previous unchanged Firefox layout rerun for PR 577 passed; the original pre-document navigation timeout remains retained as intermittent evidence.

### AQA-017 edit-list and early-playlist reproduction

Linux Chromium also fails the retained 64.5-second Subtitles resume at `hls-resume-subtitles-linux/20261012T073619Z`. Its first two reference frames match; the next seek presents the following IDR instead. This is separate from the Mac browser result.

The bounded private Hls probe at `hls-resume-browser/20261012T073551Z` shows actual buffered media ending at 74.48 seconds while fragment metadata ends at 76.56. The initialization has a 2.08-second MP4 edit in both tracks. The browser applies it; the pinned Hls sample timing does not. An append-only offset cannot reconcile these two clocks.

A private pre-consumer experiment accounts for that measured edit in all track PTS/DTS, keyframe timing, initialization timing, and append timing. Both browsers pass seven reference frames and EOF at `hls-edit-list-complete-prototype/20261012T074833Z`. Its edit value is fixture-specific. It is diagnostic evidence, not a general implementation or native acceptance.

The new retained `test-subtitles-copied-prefix.py` fails before the planned keyframe repair. Receipt `hls-copied-prefix/20261012T075127Z` proves the real encoder is stopped with one fragment while HTTP advertises eight. Future cuts differ from the source keys and from the eventual completed playlist. The source remains unchanged, the owned encoder joins, and the Server exits. This assertion requires a stable, accurate full-duration playlist during progressive encoding; removing future seeking would not pass it.

The native repair needs certified source cuts and generated-media clock evidence. Independent design review identifies the existing Player H264 packet, IDR, configuration, and joined-process scanner as the reusable boundary. Player cache admission, recovery, and audio replay policy must remain app-owned. Full-source scanning cost and distant-segment delivery still require verification.


### Shared media proof and per-track clock controls

Commits `3e1e295e` and `83150af9` extract the existing joined probe runner and immutable H264 source index into `packages/mediaprobe`. Player retains source validation, cache certificates, workload admission, audio policy, and recovery. The process tests move with their standalone fixture. Independent review found no concrete blocker in either extraction.

Focused ownership, copied-HLS, and recovery checks pass. The moved process tests compile for Linux ARM64 and Windows AMD64; they were not run on those systems. New-code mediaprobe lint and source limits pass. Player's populated prepared irregular-resume journey passes Chromium and WebKit at `83150af9`.

`make -C apps/player verify-changed` passes all selected Go tests, vulnerability checks, source limits, and whitespace checks. Its shared-packages stage still reports the established 95 lint findings, with no mediaprobe findings. Later verification stages were not reached. This is not a fully passing local gate or hosted proof for the extraction.

The source-cut manifest renderer and prefix validation are now shared through `packages/hlsmanifest`. Player still requires its source and generated-clock certificate before calling them. The helper validates bounded, sequential observed cuts and preserves the existing final-GOP tolerance. It does not certify source identity, media bytes, or future-segment readiness.

The private per-track experiment replaces the fixture-specific 2.08-second adjustment with bounded initialization parsing. It uses the selected timing track's edit for PTS/DTS and the video edit for first-keyframe timing. The real bytes are unchanged. This is still an injected, version-specific Hls controller, not a production repair.

The later-first control proves a materially different case: the first media request is segment 3 and Hls selects audio track 2 (`-2560/48000` initial timing). The video and audio edits are 83.333 and 21.333 milliseconds. Seven reference frames, backward seeks, and EOF pass in both browsers. The earlier `082042Z` attempt had an incorrectly serialized test variable and never reached transmuxing; it is a harness failure, not a product regression.

| Private receipt | Result and scope | SHA-256 |
| --- | --- | --- |
| `hls-resume-browser/20261012T080401Z/receipt.json` | Player prepared irregular resume after shared source-index extraction; Chromium and WebKit pass | `d22ba363c76f433c0215da9c4441bff605e72aef8de2fceb1652d076eed1b07a` |
| `hls-track-edit-prototype/20261012T081732Z/receipt.json` | Per-track edit diagnostic; irregular 64.5-second resume passes both browsers | `4f12890870d769a7ea31373db472a8c23c01ebeb5e97a375824b62e2d4366d3f` |
| `hls-track-edit-prototype/20261012T081825Z/receipt.json` | Per-track edit diagnostic; converted AC3 video, EOF, and strict PCM pass both browsers | `3bc1f093f89b7fac134b3f1f58a2c349b1206644a28e4bf2e59205e4ab9f175c` |
| `hls-track-edit-prototype/20261012T081948Z/receipt.json` | Per-track edit diagnostic; video-only passes both browsers | `90e70ea939ebae88958b6bcbb873aca80c196d184476abf8377dfdd94c7d1d4e` |
| `hls-track-edit-late-prototype/20261012T082248Z/receipt.json` | Per-track edit diagnostic; first media request is fragment 3, audio timing anchor, backward seeks and EOF pass both browsers | `40ea5f8ffe65d372514c250dcd5939332e81b31cc45acfc8a9dce2243c23bc05` |

The prospective native repair is limited to certified, unedited H264 streams starting at source zero. It requires one generation-bound cut map throughout each response and identical early, completed, and warm playlists. Partial encoding must not fall back to fixed four-second guesses when its producer exits. Long-source indexing cost and distant segment delivery remain open. Partial transmux results, retry behavior, physical Safari, and deployment acceptance remain separate boundaries.

PR 575 passed every required check at `5de9428e`, but strict protection blocked its merge after PR 571 advanced main. GitHub updated it to `7aea60a4`; its new checks are running. PR 577 passes required checks at `c67fb0a4` and awaits base reconciliation and protected delivery. Neither repair is claimed merged by this checkpoint.

The shared cut-helper review found two extraction regressions before commit: malformed trailing `EXTINF` could be ignored, and the observed final-cut bound rejected a previously allowed 60.083333-second value. Both are corrected. Parse errors are rejected immediately, and the certified expected cut retains its 60-second bound while observed final durations keep the existing tolerance. Exact generated media names are now checked directly. The final focused copied-HLS/recovery checks, new-code hlsmanifest lint, source limits, and whitespace checks pass.

The retained prefix regression now also checks exact warm and restarted VOD bodies and absence of another source encoder. At `hls-copied-prefix/20261012T083342Z`, the early map still changes after completion and after restart. No source re-encode occurs; source preservation, both Server exits, and encoder joining pass. SHA-256: `b3637ec133b387e9bd021ce295f0f91dddd3a112db3ca0142269f8a06ba7e401`. The prior `083137Z` encoder-count control included installation capability probes; the wrapper now targets only the generated test source.

The shared-cut extraction at `05af4d28` passes the real Player prepared irregular-resume journey in Chromium and WebKit. The clean-revision artifact is under `ai-qa-campaign/shared-cuts-browser`; the browser receipt is `hls-resume-browser/20261012T083500Z/receipt.json`.

The prefix regression also supports `--native`. It releases the held source encoder only after native WebKit requests a future segment. The browser still uses the real Server and media; MSE is capability-disabled to select native HLS. The run at `hls-copied-prefix/20261012T083817Z` remains red before the repair. Its source is unchanged, both Server processes settle, and the source encoder joins. Receipt SHA-256: `b1b9e6cf67d8dade08009e01d40ecc603f381fba570acf7dc5dd690b444e3ca8`. This is simulator-browser evidence, not physical Safari acceptance.

### Probe failure classification before optional Subtitles indexing

Before changing probe behavior, the failure inventory is: unsupported or bounded-out metadata, ordinary nonzero exit after cleanup, cancellation or deadline, failed process observation, rejected termination, and failed or over-budget settlement. An optional caller may fall back only after proven cleanup and while its own job and source policy remain valid. It must stop on ownership failure. Existing Player callers continue rejecting every failed index.

Before implementing the MSE clock adapter, its failure inventory is: malformed or oversized boxes, unsupported edit versions, negative or multiple edits, nonunit playback rate, duplicate tracks or edits, missing video/timing tracks, invalid timescales, nonfinite anchors, stale initialization after retry, and media without its timing initialization. The existing real-browser irregular-resume regression is red. An isolated pre-consumer test covers malformed initialization that real FFmpeg cannot generate and deterministic selection of either timing track. It must reject before calling the media consumer and leave bytes unchanged. This test is not populated-server E2E proof.

The retained completion-certificate seam checks OS failures that real-media E2E cannot reliably induce. Its new pre-change assertions require a distinct lifecycle error. Existing real configured-process tests now require cancellation to survive both pre-start rejection and joined-descendant cleanup. This closes a concrete E2E gap without introducing a test-only production hook.

PR 575 now combines the reviewed audio and small-video repairs at `f7b4688f`, including current main `a2d5f2b1`. Fresh required checks are running. PR 577 remains open until inclusion is proved. The unaccepted Subtitles candidate is excluded from that delivery branch. Local combined-head source limits and whitespace checks pass; affected verification stops at the established 95 shared-package lint findings before later stages run.

The new pre-change classification checks fail as expected, including cancellation in all five index entry points. After the repair, focused ownership, copied-HLS, recovery, and manifest checks pass. New mediaprobe code has zero lint findings. Source limits and whitespace checks pass. Linux ARM64 and Windows AMD64 test binaries compile; no runtime proof is claimed for those platforms. The independent read-only review found no concrete blocker.

- Private `probe-classification-red-private.log` SHA-256: `5ccfe5d6987085895a8522c835f663a52c9ae5a61dae4759c3626f46e351bc0b`.
- Private `probe-phase-red-private.log` SHA-256: `9a84c42ec8f49d1580e9dcf55a037e29c322eae794953496a5c155249a6b27c8`.
- Private `probe-classification-green-private.log` SHA-256: `8895d38be6a6f948538496826ec282b7c7537c5aff8ae14e9f61556d547fdf6a`.
- Private `probe-classification-focused-private.log` SHA-256: `1ac7b848e61d02a0a6b1ce20d67156eafc6ad8ddb6b62ff42afc4827483702d4`.

### AQA-017 production MSE adapter checkpoint

The production adapter now parses bounded per-track MP4 edits before Hls consumes fragment timing. It changes metadata only; initialization and media bytes remain unchanged. It applies only to local copied-HLS modes. The normal buffer controller retains append ordering. The parser supports one nonnegative, unit-rate edit per track; unsupported timing fails before the media consumer and enters the existing fatal media-error path. Native HLS remains unfixed.

The pinned Hls callback review found stale-context handling, init-only callbacks, and a keyframe field containing video DTS ticks divided by the selected timing track scale. Each failure was reproduced before correction. The adapter now delegates discarded callbacks before changing cached edits, accepts valid initialization before samples, and restores keyframe units using the video timescale. A follow-up empty-initialization check also failed before correction. Present-empty data rejects even when earlier edits are cached. All 20 retained controls pass, including both fresh and cached malformed inputs. Final read-only review reports no remaining concrete blocker in this path.

The initial production-browser candidate passes irregular 64.5-second resume and converted AC3 with strict browser PCM in Chromium and WebKit. These runs preceded the final callback fixes; final clean-revision browser proof is pending.

- Private `hls-resume-browser/20261012T090229Z/receipt.json`: passed; SHA-256 `fa9fcea474968258c31a91f3cf5f7c18b3e3eec29a18e9a62e13edd229650b42`.
- Private `hls-resume-browser/20261012T090318Z/receipt.json`: passed; SHA-256 `dc5a1a10c0073d055f8675f28f4bd2058459a64abc972ea37209dd3a7ec765c1`.
- Private `hls-resume-browser/20261012T090624Z/receipt.json`: passed; SHA-256 `855c1e2f00f6f727cad6857d49ab279da6bc461cafd94569315540790c2e02cd`.

The existing projection fixture built an outdated cache policy by hand and panicked after the copied-clock policy change. It now uses the actual settings operation for that fixture identity. No production cancellation guard was weakened.

Primary references: [pinned Hls stream controller](https://github.com/video-dev/hls.js/blob/v1.7.3/src/controller/stream-controller.ts), [pinned passthrough remuxer](https://github.com/video-dev/hls.js/blob/v1.7.3/src/remux/passthrough-remuxer.ts), and [MSE ISO BMFF edit-list requirements](https://www.w3.org/TR/mse-byte-stream-format-isobmff/). This version-specific adapter requires replaying the retained browser and parser checks on Hls upgrades.

### AQA-017 clean-revision MSE verification

At `641498c1eeb1c9e76b3d94db8e358fa03d83af2d`, all five final runs pass with clean, unchanged source receipts. Chromium and WebKit pass the irregular 64.5-second resume, converted AC3 strict PCM, and Direct First to Compatibility journeys. Linux Chromium independently passes the irregular resume with the cross-built ARM64 binary and Jellyfin FFmpeg. The retained Linux driver only substitutes the prebuilt binary and container paths. It does not inject a browser controller or change media assertions.

The canonical Subtitles container command passes all 66 Chromium cases, with zero failures, skips, or flakes. The suite combines populated journeys and controlled rendered fixtures; it is not 66 independent populated-server proofs. Its production and fixture images use task-specific tags. The image revision labels match the tested commit. The Compose project has zero remaining containers and volumes. The older task-owned Linux browser container is removed after its successful final run.

| Private artifact manifest | Result | SHA-256 |
| --- | --- | --- |
| `copied-clock-final-long/manifest.json` | Pass; exact revision | `60880d543da4e2cb6dfaeb19f76325a73a059f4f5def3ff04d85d2f00d880d4d` |
| `copied-clock-final-audio/manifest.json` | Pass; exact revision | `4f71f6e9a4dfb595206ea21ec76944d7f56362ec6a708d6e5048f5165412c648` |
| `copied-clock-final-linux/manifest.json` | Pass; exact revision | `34d9adc1aa61911c397d60744eddcf818059598c916dcee36c975377efefe64a` |
| `copied-clock-final-switch/manifest.json` | Pass; exact revision | `779b138b90f37552f4bdef53a4352927f02165f5b1152f6cd885a0352832eaeb` |
| `copied-clock-container/manifest.json` | Pass; exact revision | `b0a569a4201023d6d35f8a315d995a91144a6b3e1edba233d3c0042b4280da0f` |

The combined private receipt is `copied-clock-final-receipt.json`, SHA-256 `d2abdbf3334ad3843c559205d83f998f4636a69ba56bbbffcfa1c710cdd84f46`. It records private image IDs, wrapper hashes, the Linux driver/binary hashes, and cleanup. These local images are not GHCR publication or Nox deployment proof.

Subtitles affected verification passes source limits, whitespace, compilation, and focused Server checks. Full shared-package lint found two new test-complexity findings above the established 95-finding baseline. The assertions were reorganized without changing production code; focused ownership and cancellation checks pass afterward. No assertion was removed or weakened.

Native HLS still fails the retained irregular prefix/seek journey. Its source-cut and generation-lifetime repair remains open, along with physical devices and production acceptance. The MSE passes do not close those boundaries or establish 100 percent QA.

### Protected delivery and remaining Subtitles clocks, 2026-10-12

PR 575 merged at `5f169ee28472b99c416f0b7376641cefc835f573` after required run `38225253679` passed.
PR 577 was included and GitHub marked it merged. Fetched ancestry checks prove both reviewed heads, `f7b4688f` and `c67fb0a4`, belong to remote main.
Main run `38229443046` was still running at the last observation. This does not prove container promotion or deployment.
The primary checkout remains dirty with unrelated Apple project and localization edits; it was not fast-forwarded or cleaned.

Native WebKit HLS remains open. The unmodified `a2d5f2b1` binary fails all seven long-resume comparisons in `hls-subtitles-native-baseline/20261012T092127Z`.
Candidate `5ed82339` matches four in `hls-resume-browser/20261012T092222Z`; both reach EOF. This is a pre-existing native boundary, not native acceptance.
The exact `5ed82339` long Direct First-to-Compatibility journey passes both MSE engines (`copied-clock-final-long-switch/manifest.json`).

The fractional 2.5-second case exposed a separate leading-empty-edit failure.
Chromium falls back to the original; WebKit cannot decode the original Matroska reference. The tightened oracle now requires the selected decoder adapter.
A separate MP4 reference is certified against every source video packet hash and presentation timestamp; it makes WebKit's reference playable.
With that correction, both compatibility journeys still fail before the repair.

A leading empty edit supplies a per-track delay. Chromium MSE ignores it rather than applying the following media edit.
The adapter now copies initialization and fragment buffers, replaces the leading empty edit with equivalent `tfdt` offsets, and keeps one media edit.
It preserves parent box sizes with a `free` box, every sample payload, and the original cached Hls inputs.
Exact integer conversion, bounded boxes, duplicate timing fields, and version-specific overflow are checked before consumption.
Native HLS still receives the original server bytes.
Primary references: [Chromium edit handling](https://raw.githubusercontent.com/chromium/chromium/main/media/formats/mp4/track_run_iterator.cc) and [MSE single-edit requirements](https://www.w3.org/TR/mse-byte-stream-format-isobmff/#iso-init-segments).

| Private receipt | Result | SHA-256 |
| --- | --- | --- |
| `hls-resume-browser/20261012T093446Z/receipt.json` | Corrected reference; both compatibility cases fail | `ca2e4064d47cd32068d13e22d8a316d350db51ad3b0bafd9df0c61d09789bcc2` |
| `hls-empty-edit-prototype/20261012T093632Z/receipt.json` | Fixture-specific byte-normalization prototype passes both engines | `7fb883f0fb83115300820f2d5dce2e88cdacd3c3ea3b94f5fdf4f076dd9f308d` |
| `hls-resume-browser/20261012T094124Z/receipt.json` | Bounded production candidate passes both engines, all seven frames and EOF | `9dee26c703de20968dc9889af934db477ebf3f8cb65d64073036d989b055d3ca` |

These three runs include an uncommitted patch and are not exact committed-revision proof.
Thirty-three isolated controls pass after reproducing cached-input mutation and leading-edit failures before implementation.
Independent review found a caption clock mismatch when audio supplies the Hls anchor. A retained control failed before changing text cues to the video edit.
Malformed init data also exposed a recovery loop. A retained browser control fails before terminal rejection handling and passes afterward.
The error path uses the bounded `copied-media-timing` diagnostic class, makes no automatic direct retry, and retains its visible error through late readiness events.
Fifteen rendered direct-playback controls pass. They are isolated fixture evidence, not populated-server proof.

The shared-probe package now has zero local lint findings. Whole shared-package lint still has 95 established findings.
One retained Mac process-ownership check failed with EPERM during settlement; an unchanged retry passed. Its assertion remains strict.
This remains an intermittent ownership boundary; cross-compilation is not Linux or Windows runtime proof.

### Isolated MSE delivery verification

Commit `25f34f766ad16c2bd07e31e9118fda5fab1e0dfc` contains the MSE repair without the unfinished shared-index/native-HLS work.
Both apps' `make verify-changed` commands pass. Subtitles Go compilation and focused checks use their validated input cache.
All seven real-server browser scenarios pass Chromium and WebKit: default and fractional resume, long resume, long direct-to-compatible switch, video-only, converted audio, and fractional converted audio.
All fourteen browser cases match seven reference frames, resume correctly, and reach EOF. Four decoded-audio captures report zero sample lag.
Every source remains unchanged and every Server process settles.
The exact-revision manifest is `mse-delivery-browser-proof/manifest.json`, SHA-256 `328494a99e830582aac4b047da8b68f3da33e0ccfa07913bb852530d028a459c`.

The exact-revision canonical `test-instance-check` also passes all 66 selected checks at `25f34f76`.
Its receipt is `mse-delivery-container-proof/manifest.json`; the suite mixes populated journeys and rendered fixtures.
The task-owned Compose project `kinosail-test-78590-27681` leaves no containers or volumes.
A fresh worktree audit reports eleven unrelated inactive worktrees: nine clean with unique commits and two dirty. They were left untouched.

A malformed-response rendering probe exposed queued autoplay after terminal rejection. The retained test fails before adding the Play-event guard.
With that guard, rejection remains paused and visible after late readiness events. Both engines pass at 390×844, 1440×900, and 1920×1080.
The probe uses separate contexts with service workers blocked so the deliberately malformed network response reaches each browser.
Earlier reused-context probes sometimes played an unmodified response; those are unsuccessful fault-injection attempts, not passing rejection proof.
Phone, desktop, and TV-sized pending/error screenshots were inspected. Failure removes the loading skeleton and clears `aria-busy`.
This is isolated network-fault evidence using the production page, not unmodified-media E2E or physical-device acceptance.

Post-merge main run `38229443046` failed Player's long-lookback tail restoration with `fragment_not_restored`.
It restored segments 0 and 8 before the final missing fragment failed; process joining and source preservation passed.
The unchanged Player production code passes that case locally in `hls-cold-suffix/20261012T100144Z`.
The failed hosted jobs were restarted once. The original evidence remains retained, and the intermittent failure remains open.
Container promotion and deployment for `5f169ee2` are not confirmed.

### Final terminal-state verification

At `76a7696204dd73ecaa0d0e7227f277d32b8489d2`, fractional converted-audio playback passes Chromium and WebKit, including decoded PCM comparison.
The retained malformed-init journey passes both engines at all three viewport sizes.
Late readiness and Play events leave playback paused, the error visible, the skeleton hidden, and `aria-busy` false.
The harness blocks service workers only for these isolated malformed-response cases.
The canonical Subtitles container suite passes all 66 checks again.
Both apps' affected verification passes; unchanged Go checks use the validated input cache.
The task-owned Compose project `kinosail-test-89861-7836` leaves zero containers and volumes.

| Exact-revision manifest | Result | SHA-256 |
| --- | --- | --- |
| `mse-terminal-browser-proof/manifest.json` | Pass; source unchanged | `d0606ef32aa37a126658e1b2e18349711ed61f685d0bfd3440b4c52552b0aedd` |
| `mse-terminal-container-proof/manifest.json` | Pass; source unchanged | `bf68a9f5c80a3f538007b6cbb562d105a6f2a2644e7abcb6f17d2fce8da35575` |

These results supplement the fourteen successful playback cases at `25f34f76`.
Hosted Linux, native HLS, physical devices, image publication, and production acceptance remain separate boundaries.
### AQA-018 completed refill leaves a missing segment waiting

Main run `38229443046` returned a missing tail fragment after recovering the first and middle fragments.
A controlled real-Server replay at `4e0fbb21` reproduces the failure by holding the middle-refill process after its media output finishes.
The final fragment is removed and requested while that producer still appears active.
The request returns 404 after waiting; no replacement encoder starts.
Source preservation and process joining pass. The receipt is `hls-refill-race/20261012T101415Z/receipt.json`.

Before implementation, the retained `refill-completion` public journey covers this timing boundary.
Failure inventory: an active producer may still create the file, may finish without it, may fail, or may be canceled.
A replacement must preserve the original initialization and timeline, source frames, audio continuity, source identity, and shared encoder ownership.
The successful producer-completion case must retry admission under the existing lock; canceled or failed requests must remain bounded.
This process-lifetime injection is isolated timing evidence around a real Server and real media.
The ordinary cold-suffix cases cover unmodified encoder execution.

The first candidate passed the public regression but failed review: waiting inside `prepareSegment` also blocks background preparation monitoring.
An existing live-producer adoption control hung, confirming that contract violation. That placement was withdrawn.
Review also identified canceled-producer readmission and permanent-filesystem-error risks.
The next candidate must keep background admission nonblocking and limit retries to successfully joined producers after a missing-file result.

The revised HTTP wait keeps the exact producer selected under the existing admission lock.
It permits at most one new admission after successful producer completion leaves the fragment missing.
A distinct completion cause preserves prior cancellation atomically. Failed, canceled, or unobserved producers do not authorize another refill.
The original nonblocking preparation operation remains available to background monitoring.
Permanent filesystem errors do not enter recovery. Re-admission checks the current source and policy again.
Independent review found no remaining concrete blockers in this version.

Four retained public-handler controls prove inactive, seek-replaced, identity-replaced, and context-canceled producers do not start another encoder.
Their E2E gap is precise ownership ordering while the producer join is held. The real-media suite does not deterministically arrange that ordering.
The focused race run, including existing live-producer adoption checks, passes. Changed-code lint reports zero issues.
The successful real-Server regression records one INFO recovery event with the request ID, playback session, and bounded `producer-joined` outcome.
Credentials do not appear in its private log.

At clean commit `5398b5a4647ba41cb7f3598d755cb206465deede`, all nineteen public cold-suffix cases pass.
The exact-revision manifest is `refill-completion-exact-proof/manifest.json`, SHA-256 `268ac49f2e63d3e9ceb30f775169bb5fe3fbfff9eeb2a0d0f14995eca80d2275`.
Source remains unchanged. Affected Player compilation and focused Server verification pass.
The Player repair is in PR 581; the separate Subtitles MSE repair is in PR 580.
At this checkpoint, required hosted checks are still running. Neither pending PR is main or deployment acceptance.

### Native cut-map and frame-oracle investigation

At `e96ef3de`, native long resume and the held-prefix test still fail.
The prefix exposes inaccurate future cuts and changes its map across completion, warm reads, and restart.
Receipts are `hls-copied-prefix/20261012T103455Z` and `hls-resume-browser/20261012T103455Z`.

A throwaway Go overlay uses known fixture keyframe cuts and independent fragments.
It passes native long resume and produces an accurate immutable prefix, including warm reads and restart.
This hard-coded overlay is not production code or certified source-index proof.
Its configuration and source hashes are in `ai-qa-campaign/native-cut-prototype/configuration.json`.

The paused native frame oracle was unreliable: it could pause before `playing` and sample the preceding frame after `seeked`.
The driver now waits for initial playback and a timestamped video-frame callback at every sample.
Native seeks briefly play muted, then pause at the requested frame timestamp. Expected pixel hashes do not control the wait.
With the overlay, this stronger oracle matches all seven frames and resumes at 12.500001 seconds while the initial producer was held.
Receipt: `hls-native-frame-prototype-prefix/20261012T103849Z`.
The callback semantics follow the [WICG video-frame callback specification](https://wicg.github.io/video-rvfc/).
The retained driver is replayed against unmodified production behavior before implementing the cut-map repair.
The full hosted Go suite at `5398b5a4` found a regression in existing rendition-publication recovery.
`TestCopiedHLSHTTPWaitsForValidMediaDuringRenditionPublication` reproduces it locally before the fix.
A temporarily missing playlist prevents producer selection; that must retain the existing bounded file wait.
The HTTP helper now waits under the request deadline when no producer was selected.
This path cannot authorize another admission. Permanent errors and cancellation still stop it.
The expanded focused race run, including copied HTTP and cancellation controls, passes in 27.107 seconds.
A second independent review found no concrete blocker. Hosted checks must pass again after publication.

### Native generation-plan failure inventory

The stronger native frame oracle fails both baseline journeys at `e96ef3de`.
Receipts `hls-copied-prefix/20261012T104033Z` and `hls-resume-browser/20261012T104034Z` retain unchanged-source and joined-process evidence.
The production repair will persist one source-bound generation plan before encoding.
Eligible source-zero H264 streams use certified keyframe cuts; unsupported sources retain an explicit legacy plan.
Cancellation, unsettled probes, changed source or policy, and invalid persisted records must stop generation or delivery.
A certified prefix must match its immutable plan during active, completed, warm, and restarted delivery.
Public malformed, missing, mismatched-policy, and mismatched-cut controls are added before implementation.
These inject private-cache damage around real media; they are isolated fault evidence.
The plan cannot exceed 4,095 segments because the playlist also references initialization data.

PR 580 merged through protected checks into `9e8d3bbc8a00cca88ab8a9dc6ec000586597c2ae`.
Fetched ancestry confirms reviewed commit `4e0fbb21` is included.
Main `5f169ee2` passed its unchanged hosted retry and both production promotions.
Player published digest: `sha256:decb7e5d26a1df423921fc5fd85469367a564020b73c4fddef4e14cdecd3de21`.
Subtitles published digest: `sha256:eb0a7ebfb316077900b56807d0d4a25cfc1c98c91d02b9f430f30910dc1fcfbd`.
Those digests precede PR 580 and are not current-main deployment acceptance.

The first production candidate passes held-prefix native playback, all seven presented frames, far seek, warm reuse, and restart.
Receipt: `hls-copied-prefix/20261012T105259Z/receipt.json`.
Long native resume also passes in `hls-resume-browser/20261012T105431Z/receipt.json`.
The malformed-plan control changes from HTTP 200/200 before implementation to 503/404 afterward.

Independent review found two further gaps, each reproduced before its repair.
A canonical certified-to-legacy downgrade returned 200/200 (`hls-copied-prefix/20261012T105523Z`).
A source-version change during a held fragment wait still returned 200 (`hls-copied-prefix/20261012T105606Z`).
The published master now binds the exact plan bytes by SHA-256.
Delivery revalidates the captured source and policy after waiting and opening the requested asset.
Both controls then pass: downgrade at `20261012T105735Z`, held source replacement at `20261012T105733Z`.
The source-change fixture intentionally changes source modification time; content remains unchanged and all owned processes join.
The affected Go controls pass. Final lint, complete fault matrix, MSE regression, and exact-revision browser proof remain pending at this checkpoint.

At `1635c0ac`, nine real-Server browser/fault cases pass with unchanged tracked source.
The manifest `native-generation-browser-proof/manifest.json` has SHA-256 `13e87d9b826f19e906fbdd87b7959e6d3b0ebcec5451cafbca80246c7f430dd8`.
It includes native prefix/far seek, long resume, both MSE engines with decoded audio, five persisted-plan faults, and held source replacement.
The canonical Subtitles container suite passes 66 checks; its disposable project `kinosail-test-42482-23307` leaves zero containers and volumes.
The shared mediaprobe and hlsmanifest race suites pass. Whole shared-package lint still stops affected verification at 95 established findings.

The complete Subtitles Server suite then exposed three existing compatibility regressions on Darwin.
A rejected probe response exited successfully, with successful observation, Wait, and group settlement.
Darwin's zombie-group EPERM was still incorrectly promoted to an uncertified lifecycle because metadata parsing failed.
Existing public-handler controls reproduce the regression; a retained failure-class control fails before repair.
Independent safety review confirmed metadata rejection and process settlement must be evaluated separately.
The exception remains Darwin-only and requires an active context, successful exit observation and Wait, and proven settlement.
Metadata remains rejected. Cancellation, uncertain observation, failed Wait or settlement, and other termination failures remain fatal.
The three affected public-handler controls and the complete mediaprobe race suite pass after this distinction.

A real-Server unsupported-index fault additionally exposed a false ERROR after safe fallback (`hls-copied-prefix/20261012T111030Z`).
The source remains on an explicit legacy plan and its probe is settled, but completion logging incorrectly reports a lifecycle failure.
Probe completion now emits that ERROR only for an actual `ErrProbeLifecycle`.
The parent retains one correlated WARN with the bounded `legacy-before-encoding` outcome.
Final fallback, full-Server, and refreshed exact-revision evidence remain pending for this final adjustment.

The complete Subtitles Server suite passes at `e5414ae5` in 143.278 seconds.
Unsupported metadata and bounded probe timeout both retain a settled legacy plan with one correlated WARN and no false lifecycle ERROR.
Timeout receipt: `hls-copied-prefix/20261012T111259Z/receipt.json`.
Two additional real-Server faults hold a future-fragment request while changing transcoder settings or killing the fixture encoder.
Both reject delivery with HTTP 404, start no replacement encoder, and preserve the source and process ownership.
The policy fault records one correlated WARN; the producer fault records one correlated ERROR.
Receipts: `hls-copied-prefix/20261012T111801Z` and `hls-copied-prefix/20261012T111826Z`.
These are injected-fault controls with real media and HTTP, not physical-device acceptance.
CI now retains the immutable-map, source and policy changes, producer failure, optional-index fallback, and persisted-plan fault controls.
The artifact upload includes safe receipts only. Native WebKit capability simulation remains separate local evidence.



### Android clear-search synchronization repair

Main CI run `38238653057` failed `phoneHasOneExplicitClearActionAfterSubmission` at `SearchFeedbackJourneyTest.kt:50`.
The unchanged local search suite passes, confirming the failure depends on scheduling.
A temporary fixture delay before accepting the clear request reproduces the same assertion on both phone and TV: expected an empty query, observed `Fictional`.
Cached rows can make the screen idle before the fresh network response completes.
The two retained journeys now wait for the response-backed empty `resultQuery`, while preserving clear-input, outgoing-query, and TV-focus assertions.
This follows [Android's guidance for conditions that depend on external work](https://developer.android.com/develop/ui/compose/testing/synchronization).
No production behavior changes, retries, or sleeps were added. The temporary delay was removed after red/green proof.

All eight search journeys pass with the delay. Without it, the complete Android gate passes: 161 app, 8 watchcore, and 5 wear tests, plus both debug builds.
Local proof: `ai-qa-campaign/android-search-clear-proof.json`, SHA-256 `2a77981bf1839226b1ccb3640563af9f95a121b72ac03edb446eeb7e5e50aded`.
The proof records main `9e8d3bbc` plus the exact uncommitted test patch; it is not an exact committed-revision claim.
These JVM journeys use an HTTP stand-in and do not prove populated production Server or physical-device behavior.


### Recovered-audio recorder boundary

PR 581 hosted run `38239833223` found a 575-sample starting offset in the 96 kHz coarse-video-clock recovered capture.
At 48 kHz capture rate, this is about 12 milliseconds. Correlation stays above 0.965 and render blocks remain contiguous.
The offset appears before the missing fragment's boundary; the preceding fragment is unchanged.
This suggests recorder startup timing, but the hosted AMD64 failure is not reproduced or dismissed.

The unchanged local macOS and Linux ARM64 journeys pass.
A paired Linux experiment on identical source/cache bytes passes four original-start and four settled-start captures.
The recorder now requires advancing initial presentation and an exact paused seek before recording post-startup recovery.
It retains bounded playback/audio checkpoints in the already-uploaded receipts and comparisons.
The first worklet checkpoint records main-thread message receipt and the reported render frame; it does not retime PCM.
The 1-millisecond starting-offset, correlation, and continuous-block checks remain unchanged, with no discarded samples or automatic retries.

A fresh settled-start recovery passes all four captures, both grids, video, EOF, and metadata rejection controls.
A premature unrelated `seeked` event is ignored in all four captures until the requested target settles.
A 576-sample shift injected at the splice lowers minimum correlation from 0.9657 to -0.1097 and is rejected with the original pre-splice anchor.
Proof: `ai-qa-campaign/pr581-recorder-controls/proof.json`, SHA-256 `9160a4b355e32f00c9ed8ce87946c4e886f95f9521fb45bba137b6fdb86aa4c3`.
These are Linux ARM64 and isolated recorder controls. Immediate-start seek timing and the hosted AMD64 failure remain open audit boundaries.

PR 583 merged through protected checks into `e9baa35a14ea74e85e5a20225a88e98b940b71c1`.
Fetched ancestry confirms reviewed Android test commit `3ae56d24` is included.

### Hosted verification follow-up: login and fixture shutdown

Hosted WebKit layout run `38243395191` completed Subtitles login with HTTP 303, then timed out filling the previous document's authentication field.
A controlled native WebKit commit reproduces the same `layout-stability-local.mjs:115` failure before repair.
Subtitles submits the password and second factor together. The audit now submits once and waits for the existing redirect condition.
The retained delayed-redirect and rejected-factor controls require exactly one POST and no library access after rejection.
Linux ARM64 passes all 11 applicable controls across Chromium, Firefox, and WebKit; four Firefox-specific controls are skipped on other engines.
Native macOS Chromium and WebKit pass. Firefox fails before browser launch on this workstation and is not counted as local coverage.
A real Subtitles Server with populated synthetic data passes login and six responsive WebKit views over trusted fixture HTTPS.

Hosted source-change rejection returned 404 with the correct diagnostic and no replacement encoder, but failed the fixture's join assertion.
Linux ARM64 reproduces that result: immediate Server shutdown kills the wrapper before it can reap FFmpeg.
The fault harness now waits, within its existing bounded deadline, for the exact child's join receipt before stopping the Server.
Source change, settings change, and encoder failure all pass with the strict rejection, diagnostic, unchanged-content, and join assertions intact.
This verifies settled faults. Early Server shutdown with an installation wrapper that spawns descendants remains a separate ownership boundary.

The combined proof binds revision `5921a073` and the exact uncommitted harness hashes, not an exact clean-commit run.
Proof: `ai-qa-campaign/pr582-harness-proof.json`, SHA-256 `20c957e11d51e78adc482872bddf3a583f6541324ba15819a0055c6b09952d9c`.
Independent review found no blocker. Both generated architecture snapshots were refreshed and their contract passes.
Script checks, TypeScript quality controls, source limits, and whitespace pass. Fresh hosted checks remain required.

### Native campaign follow-up on 2026-10-12

The Android production tree is unchanged between `e9baa35a`, `51a5fcc2`, and PR 582 head `8b6011b2`.
Phone, TV, and tablet runs used API 36 read-only emulator overlays. Shared AVD data was preserved.
The test APK added private, failure-only EPUB diagnostics. No product assertion or timeout was weakened.

| Surface | Evidence | Boundary |
| --- | --- | --- |
| Phone native journeys | 14 passed in the full follow-up; four focused EPUB repetitions also passed | Earlier intermittent EPUB failure remains open |
| Phone real Server playback | Two portrait and two landscape journeys passed | Actual Media3 decoder and isolated main `51a5fcc2` Go Server; generated media and HTTP media-fault bridge |
| Android TV native journeys | 11 passed; two EPUB assumptions skipped | Native focus and D-pad against in-process synthetic data |
| Tablet native journeys | First 11 checks passed before a 600-second outer timeout; remaining three passed separately | Incomplete original run retained; not represented as one green suite |
| Tablet real Server playback | Two journeys passed in 98.075 seconds | Loading, failed media, retry, recovered frame, captions, speed, and action contrast |
| Wear | Application and instrumentation APK builds passed | Emulator execution deferred during coordinated physical tvOS playback |

Phone proof: `android-phone-diagnostic3-proof.json`, SHA-256 `78562325e264ebdc598f7d21e48cf3b812d758d3b960d014f97ad611ce91da14`.
Phone portrait proof: `android-phone-live-proof.json`, SHA-256 `a15b934f2dbb8e0735c2ffaa7a4bd113376255deeec987d6162cc45bb187bcde`.
Phone landscape proof: `android-phone-landscape-live-proof.json`, SHA-256 `04b756836cccdd3068219fd1327a6a1e8650bfd4f5cac6d2fd08f284a42cd3cb`.
Tablet proof: `android-tablet-proof.json`, SHA-256 `84461e0c2b3f39d999593be202504d3d2950045c9baf44a44c3c429dc031a828`.
Tablet real Server proof: `android-tablet-live-proof.json`, SHA-256 `f03c1881c3a7b13f9b5cd0aca13fc9485f610285838032395d6939e50c0fefa8`.
These files are private under `.verification/ai-qa-campaign`; raw logs and credentials are not publication artifacts.

Tablet review confirms matching pending and loaded catalog geometry, readable controls, and visible EPUB chapters.
The recovered video screenshot also shows captions intersecting the gesture area. Existing action checks do not assert caption placement.
That finding needs an observable failing check before a production repair.
No Cast receiver, physical Android device, native TLS, deployment, or paired Watch acceptance is claimed.
All task-started phone, TV, and tablet emulators were shut down normally. The isolated Server and HTTP bridge also stopped.

### EPUB accessibility reproduction remains open

A validated diagnostic control deliberately waits for an absent heading and retains the original 20-second failure.
Its service and direct WebView trees both contain the real chapter, proving both observation paths work on a loaded document.
The following normal tablet journey fails after reopening saved Chapter 2. `Harbor lights` is visibly rendered.
Both the service tree and the direct app query contain an empty WebView virtual child.
The WebView remains attached, shown, and important for accessibility; system accessibility is enabled.
Clearing the service cache and ten bounded refreshes do not recover the text.
The runtime is WebView `133.0.6943.137`. No newer-runtime or real screen-reader comparison has run.
Both queries still use Chromium's node cache. They do not distinguish stale Chromium cache from an empty renderer accessibility tree.
This narrows the investigation beyond a framework-only cache problem; it does not establish an application lifecycle defect or TalkBack failure.
The existing assertion still fails. Diagnostics are preserved privately, and tracked source was restored unchanged.
Proof: `android-epub-diagnostic5-proof.json`, SHA-256 `0a2d0079a74b9fea766b7c187f539ac1db16fe86def81acb7cd2cbdbf840c61c`.

### Protected publication and shared probe diagnostics

PR 581 merged with reviewed commit `b8b9de74` included in fetched main `51a5fcc28f84e84c8c651529fe6416353c802149`.
Main run `38250393619` passed its selected checks and both Player container architectures.
It published and promoted `sha256:ab5036e417b3749d47bbe3847e59b38e1ba72b213946feeb5e39235b941989da`.
This is GHCR publication proof; no Nox deployment or physical acceptance is inferred.
Publication proof: `player-51a5-publication-proof.json`, SHA-256 `52ecebd2b280007516722b38719f9f204fe9d7fc6da4c24fb7df8f9e2274d06d`.

PR 582 head `8b6011b2` preserved the exact source tree tested at `18602`.
Its three layout engines passed. The Player browser gate then found a shared probe logging regression.
All ten strict converted-audio splice cases passed. The injected exit-23 source-probe case failed its required ERROR assertion.
Safe fallback, source preservation, correlated fallback WARN, and process settlement still passed.
The extracted probe code logged lifecycle failures but omitted ordinary nonzero exits after a successful scan.
One condition now restores that ERROR while keeping expected metadata rejection, cancellation, and optional timeout quiet.
Return values and fallback eligibility are unchanged. Independent review found no blocker.
The original hosted failure remains recorded; no assertion was weakened and auto-merge was disabled during repair.
The failed receipt has SHA-256 `bc4e9291f2cae991bc08d16a64818fbf293313e1f36f4e98ab3a8ed73dfc7566`.

The focused Player source-error, timeout, cancellation, and coarse-metadata controls pass after repair.
The Subtitles unsupported-index fallback control and the shared mediaprobe package checks also pass.
Error logging remains correlated and bounded; the existing checks reject secrets in diagnostics.
All fixture Servers settled and source media remained unchanged. Source limits and whitespace pass.
These local runs bind the changed source hash over base `8b6011b2`; fresh hosted Linux verification is still required.
Proof: `pr582-probe-diagnostic-repair-proof.json`, SHA-256 `2e85efefb90838878178c8cd4618d264c19d531145e6bbbdfcafcca7df513bdd`.

The first diagnostic condition introduced one new complexity finding during post-commit verification.
The equivalent guard now lives in the existing reporting helper and receives the probe deadline context.
Independent review found no blocker. Player four-case fault replay, Subtitles unsupported-index replay,
mediaprobe checks (44.912 seconds), changed-package lint, source limits, and whitespace pass again.
Final local proof binds both changed source hashes over `e14406b9`; hosted verification is pending.
Proof: `pr582-probe-final-proof.json`, SHA-256 `7ab72c6f460e36e5252d81aa951f3c58c2b5a87d944b94f40d692c83cd2bdbf8`.

Fresh head `52e7886a` passed changed app lint but exposed stale generated Code Atlas metadata in tooling.
Both snapshots now reflect the three added mediaprobe lines; dependency, symbol, and file counts are unchanged.
The complete local `make tooling-check`, architecture checks, source limits, and whitespace pass.
The source repair is unchanged. Both post-commit app wrappers stopped at the established 95 shared lint findings.
Snapshot proof: `pr582-architecture-refresh-proof.json`, SHA-256 `35f1ca1f1420cf8c42f6cc79ddc571792a9b61a9b783a1752751632e95191465`.

### Merged main publication and fresh deep verification

PR 582 merged reviewed commit `8e816c7187fb08669b80638e78d3052223a33965` into main `b1b2c00e961519b193dbef546683422d4bc5186a`.
A fetched ancestry check proves inclusion. Required main run `38266522591` passed and published both applications.
Each app passed AMD64 and ARM64 build, production tests, and scans. Hosted signature and provenance verification passed.
At verification on October 12, the commit, `main`, and `latest` tags resolved to the same manifest for each app:

| App | Published digest |
| --- | --- |
| Player | `sha256:4f17925107fb0c76cb82707851797d02a16e3361c36216225094c90c12e6c05b` |
| Subtitles | `sha256:323bb62a7b0fd96f8787544ecb0dd66118e0e820182515dca60ef127baa8ff8d` |

Both architecture configurations contain the exact main revision. This does not prove a Nox deployment or physical-device acceptance.
Publication proof: `main-b1b2-publication-proof.json`, SHA-256 `e414db5d686b95f469273cf6d6d7d62114579be90b1f375146860ac9a8c8d266`.

Manual deep run `38266559451` tested the same revision and failed. Original failures remain retained.
All six browser archives prove unchanged exact source. Their 2,215 listed artifact hashes match.
Counts below reconcile full suite/title/project identities across the base and focused reports.
They include isolated browser fixtures and populated Server journeys; they are not all populated E2E cases.

| Application / engine | Observed cases | Passed at least once | No passing execution |
| --- | ---: | ---: | ---: |
| Player / Chromium | 1,074 | 794 | 280 |
| Player / Firefox | 1,074 | 790 | 284 |
| Player / WebKit | 1,074 | 786 | 288 |
| Subtitles / each engine | 99 | 99 | 0 |

Subtitles has no skipped, failed, or flaky cases in these reports.
Player has two WebKit failures. Its other unpassed cases need explicit runners or have browser-specific exclusions.
Proof: `deep-b1b2-hosted/browser-artifacts-proof.json`, SHA-256 `9f1ab45554cb69d895330a3f542e8c027a2cc86fd786c31c14d209b1bc702bbb`.
Coverage reconciliation: `deep-b1b2-hosted/player-coverage-v2.json`, SHA-256 `2c63ac0ed125cb33c4d5cfdce466471272f5f9470055ffdceb8e9a1f6a07d25c`.

The Player Go job rejected 86.7% coverage against its unchanged 89% threshold.
Its terminal log contains no test failure or race report. Unmerged PR 579 uses different coverage accounting and reports 89.5% separately.
The deep iOS run passed 331 cases, failed one, and skipped one. The failed card comparison captured a blank raw pixel buffer.
Its rated and unrated PNG attachments are byte-for-byte identical and visibly nonblank. The raster comparison needs diagnosis; no visible rating defect is proved.
The deep tvOS run passed 312 cases, failed one, and skipped one.
Its endpoint-scrub test did not establish the required EOF, loading, and attached-AVKit prerequisite. Later scrub and watched-state assertions did not execute.
These Xcode case counts differ from Swift Testing's parameterized console counts.

Both WebKit failures accepted the real Mark unwatched response, then waited for a native ended event during held navigation.
All four attempts sought to 11.9 seconds of a 12-second clip with no buffered media at the target and readyState 1.
The accepted-departure handler intentionally stops HLS loading. The fixture therefore needs to buffer the ending before accepting departure.
A private candidate preserves real EOF, the real 303 response, and all no-overwrite assertions. It has not run or been published.
The ordinary watched/unwatched navigation journey and both late-pause variants passed.

Local builds, tests, and emulators remain held for the coordinated physical Apple TV window.
Caption placement, EPUB accessibility, Wear execution, BFCache admission, scheduled-only checks, shared lint, and device/deployment boundaries remain open.
No 100% product or platform acceptance is claimed.

### Compose recovery, supersession, and input contracts

Three additional hosted Q47 suites tested exact revision `b1b2c00e961519b193dbef546683422d4bc5186a`.
These close the previously unrun Q47 recovery, supersession, and contracts suites.
Each fixed case passed twice without a retry. All eight distinct cases passed across 16 executions; none failed or skipped.

| Suite | Hosted run | Distinct cases | Passing executions |
| --- | --- | ---: | ---: |
| Recovery | [38273036654](https://github.com/Kinosail/kinosail/actions/runs/38273036654) | 2 | 4 |
| Supersession | [38273252807](https://github.com/Kinosail/kinosail/actions/runs/38273252807) | 2 | 4 |
| Contracts | [38273786454](https://github.com/Kinosail/kinosail/actions/runs/38273786454) | 4 | 8 |

Recovery proves that a lost request and an HTTP error preserve inputs, show recovery actions, and accept a successful retry.
Both cases complete exactly one new request after the failed request. The resulting download and clipboard contents match the expected Compose output.
Supersession proves that changed inputs cancel held requests. Late headers or body data do not replace the current output.
Contracts verify preview, browser download, and clipboard contents for Player, Subtitles, and Both.
Each validation execution rejects seven invalid path inputs and four invalid port inputs with zero template requests and no output.
The disposable peer observes no credentials, cookies, request bodies, or query strings in these requests.

Each artifact contains a receipt, results, source manifest, and artifact manifest. All nine listed file hashes match.
All 7,463 tracked source records match the tested Git tree. Before and after source fingerprints match in all three runs.
The receipts confirm unchanged binary, dependencies, and generated files. All owned processes settled.

| Suite | Downloaded archive SHA-256 |
| --- | --- |
| Recovery | `cf1bed29bb4b2fc112b1cb5fd35a08fb5a7dd5534777da530890ae59d0922a91` |
| Supersession | `7b3d480a1fdb019f91109d25e92cf8a7cb2425aedbe40d9d4fd26459a92b782b` |
| Contracts | `723021ca64342569ab706cb27e610c98f0e7cc875f0a4214ccb988777079acca` |

The private ledger is `.verification/ai-qa-campaign/q47-supplementary-hosted-ledger.json`.
Per-suite receipts and reconciliation proofs are in `.verification/ai-qa-campaign/q47-supplementary-hosted/`.
These are Chromium journeys against generated documentation and disposable HTTP peers.
They do not prove a production installation, populated app Server behavior, Nox deployment, or physical-device acceptance.
The separate Player browser totals above remain unchanged.

### PR 587: download focus, media fixture admission, and native capture

A populated Downloads refresh removed keyboard focus from the Play offline link.
The Server response hides that link because downloaded-file readiness belongs to the device.
The repair preserves an already visible link for the same job before replacing markup and restoring focus.
Production changes total two added lines in `packages/webassets/static/downloads-ui.js`.

The regression holds the canonical refresh at a disposable Go peer with native service workers enabled.
It proves replacement of the old DOM node, retained focus, Enter navigation, stored-byte checksums, and zero removals.
All three engines passed at `4a2594c03cec7bcfd9c76f1d43c0fd7eee290165` in [deep run 38324888163](https://github.com/Kinosail/kinosail/actions/runs/38324888163).
Pending and loaded screenshots are byte-identical within each engine. Visual inspection confirms the focus ring remains visible.

| Player browser | Download-focus result | Verified checksum entries | Archive SHA-256 |
| --- | --- | ---: | --- |
| Chromium | Passed without retry | 521 | `27825e928748b4039f254b091d02c0d7f768495f14f53e92555f120786817dea` |
| Firefox | Passed without retry | 561 | `2f105194dd9bda70057e284d3c3ed9f22b6037c5f9d09379cc49c219e93d9404` |
| WebKit | Passed without retry | 579 | See private `player-webkit-4a2594c0.proof.json` |

Each archive also proves clean, unchanged source at the exact revision and matching fixture inputs.
Chromium and Firefox completed their full Player browser jobs successfully. Chromium's HLS follow-on checks also passed.
All three engines passed seven caption-recovery cases and 47 download-pause cases without retries.
Subtitles passed all 99 cases in each engine, with no skips, failures, or flakes and 198 verified checksum entries per archive.
These observations do not replace the earlier whole-campaign coverage inventory.

The watched-state fixture now buffers a natural ending before accepting the real form POST.
It measures three fresh advancing presented-frame timestamps after resume, rather than a counter that WebKit can reset.
All five watched cases passed without retry in Chromium and Firefox at the revision above.
WebKit failed four cases and passed one only on retry. Its traces failed during startup or rejected an earlier acknowledged progress save.

Revision `ebd56df70c39d712567ba9fed9f76c3224ecb4e6` corrects the latter fixture assumption.
It freezes the baseline request set when the watched form is intercepted, then requires each baseline save to finish with HTTP 204.
A five-second deadline and route abort preserve failure during cleanup. Every subsequent progress request remains forbidden.
The accepted watched state must remain unchanged after real native pause or ended events and released queued callbacks.
This fixture no longer guarantees that form acceptance precedes every instance of progress ownership.
The historical guard-mutation check remains pending for this revision.
Shared startup and coverage-accounting changes remain owned by PR 579.

Native deep run [38327020555](https://github.com/Kinosail/kinosail/actions/runs/38327020555) tested `2609d636ae37323f9c61bcab0b45ed65755f8afd`.
iOS passed 331 test cases, failed the card-rating comparison, and skipped one. tvOS passed 319 and skipped one.
The first rated RGBA capture contained 366,160 zero bytes. Its PNG and the unrated PNG were identical and visibly nonblank.
Both PNGs have SHA-256 `27afffddcedda61aefb36e2eda1b945ad590f20dea3beb1da18edabe1788f6ad`.
The native artifact archive has SHA-256 `c0345b74914490aec3e453eb3e233f237dfadcef86cd2993b2673eb7ebf77b99`.
Keeping the bitmap context alive during copying did not resolve this intermittent capture failure.

The next capture implementation draws `ImageRenderer` content directly into an owned RGBA context.
Assertions and image attachments use that same completed raster. Strict dimensions, pixel equality, and independent nonblank checks remain.
A separate control changes only the opacity of an overlaid rating label and requires unequal pixels at equal dimensions.
This protects against a capture that draws backgrounds but omits text. Both control images are retained.
The implementation follows [Apple's context-rendering API](https://developer.apple.com/documentation/swiftui/imagerenderer/render(rasterizationscale:renderer:)).
Independent source review found no blocker. Swift syntax parsing, source limits, and whitespace checks passed.
At `9dbee4b88f11a1c14e33bd5dad0a1267d5aea9c4`, [deep run 38330038654](https://github.com/Kinosail/kinosail/actions/runs/38330038654) passed the native contracts.
iOS passed 333 cases and tvOS passed 320; each platform skipped one case. Neither platform reported a failure.
Both card comparisons and visible-text controls passed. All 20 images were inspected, including 18 unique images.
Each visible control contains readable PG-13 text. Hidden controls match the normal video poster, without rating text.
The archive SHA-256 is `5d411a803d1567833857adbbdecc29d6bc4da82354bc527e15d6d712381bbb88`.
These are hosted simulator rendering contracts; physical-device acceptance remains separate.
The earlier background-authorization count failure now retains safe timing diagnostics after freezing the original failed assertion.
That test passed at `2609d636`; the intermittent cause remains unproved.

At `9dbee4b8`, [layout run 38330042136](https://github.com/Kinosail/kinosail/actions/runs/38330042136) passed in Chromium, Firefox, and WebKit.
Each engine completed 11 Player flows and 26 Subtitles flows without failed stability or focus flags.
Chromium recorded 95 Player measurements and 111 Subtitles measurements. Firefox and WebKit each recorded 40 and 46.
The isolated layout checks passed 35 cases in Chromium and WebKit, with two Firefox-only skips each. Firefox passed all 37.
Exact clean source and seven script input hashes were verified. Measurement hashes are recorded in the private proof files.
The layout artifacts do not contain a complete upstream checksum manifest; their measurement hashes were computed after download.

At `ebd56df7`, WebKit passed ordinary watched/unwatched navigation without retry and verified 580 artifact checksums.
It also passed download focus, pause, caption recovery, pagination, and queue loading.
All four controlled watched cases failed. Seven attempts reached the held response but could not establish fresh video motion.
One attempt failed the older shared startup prerequisite. The first held-response failure screenshot still displayed "Seeking...".
Its trace reported only the generic motion failure, so it does not distinguish a media stall from missing compositor callbacks.
The new diagnostic captures safe media state, bounded buffered ranges, and callback counts before play and at the existing deadline.
It records the first three raw callback times, qualified times, visibility, and an allowlisted play rejection class.
The original failure is retained even if attachment writing fails. No assertion, deadline, playback action, or transport behavior changes.
Historical mutation verification and controlled WebKit late-callback acceptance remain pending.

The same revision's Chromium watched-state cases all passed without retry.
Its separate older-progress ordering case failed once with `NotSupportedError`, then passed on retry. The job correctly remained failed.
The trace places native `play()` at 163,300 ms, before the HLS library request began at 163,400 ms.
The library, playlists, and media segment subsequently returned HTTP 200. The trace archive SHA-256 is `da68f172184e5d8bbacfbf78314a81c741e742dcd9dfebc85127ee261a50128a`.
That fixture now waits for decoded data before its single native playback request.
Decoded-frame, held-progress, request-order, and stored-state assertions remain unchanged. Runtime validation of this prerequisite remains pending.

The `9dbee4b8` deep Go job failed the unchanged 89% coverage gate at 86.7%, without test-failure or race markers.
The independent coverage-accounting repair remains in PR 579. No coverage threshold was reduced.

Private repeatable evidence is under `.verification/ai-qa-campaign/pr587-hosted/`.
Browser archives contain command receipts, fixture inputs, environment details, traces, screenshots, and checksum manifests.
Native archives contain the hosted iOS/tvOS result bundles and image attachments; read-only extraction used `xcrun xcresulttool`.
Local affected verification plans still select held runtime work. No local build, browser run, emulator, or deployment was started for these deltas.
The earlier full affected checks stopped at 95 existing shared-package lint diagnostics; no threshold or required gate was bypassed.
PR 587 remains unmerged. Device, deployment, skipped-case, and complete campaign acceptance remain open.

### Held-response motion diagnostics and main reconciliation

At `7bafe6ed`, [deep run 38332811290](https://github.com/Kinosail/kinosail/actions/runs/38332811290) completed Chromium, Firefox, and native jobs successfully.
Chromium verified 521 artifact checksums; Firefox verified 561. All five watched cases passed without retries in both engines.
iOS passed 333 contracts and tvOS passed 320; each skipped one. Neither platform failed.
The iOS skip requires supported picture-in-picture hardware. The tvOS skip is an opt-in disposable navigation-fixture seed.

WebKit verified 582 artifact checksums and failed four controlled watched cases.
Six held-motion failure snapshots contained zero raw frame callbacks while native play fulfilled.
All snapshots reported visible, focused, in-viewport media with no native error code.
Two watched attempts advanced approximately eight seconds; both unwatched snapshots reached native EOF.
Other watched attempts exhausted the partial buffer. Two attempts failed the older startup prerequisite before collecting this diagnostic.
This rules out timestamp filtering as the reason for the missing callbacks. It does not establish the browser's underlying cause.
The WebKit archive SHA-256 is `2bf020e25a01d040de7a70dc4c59e90584d097915c8e79a03b9f752a6d0adf79`.

For head `e67181c4`, [required CI 38333249448](https://github.com/Kinosail/kinosail/actions/runs/38333249448) passed every selected job.
Its browser archive tested merge revision `e269bb00`, whose parents are `ae91fe70` and `e67181c4`.
[Layout run 38333249158](https://github.com/Kinosail/kinosail/actions/runs/38333249158) also passed in all three engines.
Exact clean source and seven script hashes per layout archive were verified. No measured stability or focus flag failed.
The Chromium progress-ordering case passed on its first populated execution; its general fixture execution was correctly skipped.
Exact clean merge source, ten fixture inputs, and all 418 artifact checksums were verified.
The archive SHA-256 is `e2e8d540433ce9f50b280a82442ad53724fb5720ff565b6eb13cade96ce93d45`.

PR 579 merged into `7131f22a`. This task merged that main revision in `64ca7ff3`.
The reconciliation retains initial callback settlement, shared startup handling, safe diagnostics, and acknowledged baseline writes.
It uses one natural buffered-ending preparation. It removes the redundant second tail seek introduced by the automatic merge.

The next trial uses three increasing non-dropped frame counts with increasing media time.
It samples total and dropped counts together, rejects invalid counters, and discards credit after a counter or time regression.
The [Media Playback Quality specification](https://w3c.github.io/media-playback-quality/) defines displayed frames as total minus dropped frames.
These samples replace individual frame presentation timestamps. They do not claim equivalent per-frame timestamp evidence.
The eight-second deadline, native playback, actual pause/ended events, positive callback release, and stored-state assertions remain.
Raw frame callbacks remain diagnostic only; there is no fallback path or extended timeout.

Ten isolated arithmetic controls cover stale, dropped-only, reset, invalid, and advancing counters.
They were written before the helper. Missing-module discovery failed first; that is wiring evidence, not an executed negative control.
Four additional controls mutate only the served browser script's departure guard after motion admission.
They require one exact replacement, original and mutated script hashes, released native callbacks, and an actual forbidden late POST.
Earlier startup, transport, or motion failures must still fail these controls. No blanket expected-failure marker is used.
These are phase-targeted mutation controls against a populated Server, not historical pre-fix replay.
The original four cases still require zero late requests and unchanged stored state. Runtime validation of this trial remains pending.

### Displayed-motion trial and listener isolation

At `4f27d376`, [deep run 38337038045](https://github.com/Kinosail/kinosail/actions/runs/38337038045) passed Firefox, native, and Go checks.
Firefox passed ten isolated arithmetic controls and all nine watched cases on their first attempts.
Each mutation initiated a late POST only after its guard bypass was armed. Each normal case preserved accepted Server state.
Exact clean source, ten inputs, and 571 artifact checksums were verified.
The Firefox archive SHA-256 is `11ca1dcd835a6fc3ed91f6a92e6ecb620383a1a8f3b8accf886bfe3cd9b9b1e4`.

WebKit passed the arithmetic controls, both watched variants and their mutation controls, and ordinary navigation without retries.
Its four unwatched cases failed initially. One passed on retry; three still failed. The run correctly remained failed.
Five failed attempts stopped at the ending-buffer prerequisite. Two stopped at initial or resumed playback admission.
These attempts never reached the held watched response. Their failure does not test the departure guard.
The trace reports `bufferAppendNoProgress`, followed by media-source resets. All captured media requests returned HTTP 200.
The downloaded fragments have an 80 ms raw presentation overlap between segments zero and one across three qualities.
Producer provenance, edit lists, and effective append offsets still need reconciliation before assigning the timestamp cause.
Exact clean source, ten inputs, and 569 artifact checksums were verified.
The WebKit archive SHA-256 is `c62dbb7b321a76b5f427aff28e5c17a14d100481b5de165e5bef794b331578b8`.

Independent source review found a separate fixture defect. The global barrier also delayed HLS controller callbacks.
Its anonymous wrappers prevented removal by the original listener identity during recovery.
The revised barrier delays only callbacks installed synchronously by the same-origin application script.
Other registrations retain native dispatch. The wrapper registry preserves listener identity and the capture flag.
One isolated DOM control covers immediate external callbacks, removed listeners, duplicate registration, and one-shot dispatch.
This control was written first. Missing-module discovery failed before implementation; that is wiring evidence, not executed failure proof.
The existing populated cases retain moving media, real native events, positive callback release, accepted state, and mutation assertions.
The listener repair does not claim to resolve the independent fragment-clock finding. Hosted runtime proof remains required.

iOS passed 333 contracts and tvOS passed 320, with the same two documented conditional skips and no failures.
Their archive SHA-256 is `784cff92626c158445fefccc83646128b4b3a365ffc3cbea4ab9b537a804415f`.
Player deep Go coverage passed at 89.7% against the unchanged 89% threshold. No test-failure or race marker was found.
Subtitles passed all 99 cases in each browser without skips, failures, or retries. Each archive verified 198 checksums.
All three [layout jobs](https://github.com/Kinosail/kinosail/actions/runs/38337038964) passed with unchanged source and seven verified inputs each.
Local runtime and physical-device boundaries remain unchanged.

### Restore coverage reconciliation

The earlier R06 preflight failures remain historical evidence. Later exact-revision results close three selected restore proofs.
At `8e816c7187fb08669b80638e78d3052223a33965`, [restore controls](https://github.com/Kinosail/kinosail/actions/runs/38264506250) passed four real HTTP cases.
[Restore headers](https://github.com/Kinosail/kinosail/actions/runs/38264907457) and [inspection-body checks](https://github.com/Kinosail/kinosail/actions/runs/38265395148) each passed desktop and phone cases, including all twelve assertions.
All three archives were rechecked against their source and artifact manifests. Each verified 49 source inputs and three evidence files.
Their archive SHA-256 values are `9f6ddddd1df8fe8205f0e98566d41ccd1d50efe7708ffc9d0b232d2274e31c1e`,
`14a572536feb62ea91ee0051cc3593e57127cfa9a305a0ef989e3421ae6a493a`, and
`b60ce2f7cb04e6ca1c4e10390bc1b44a2e45fd660fa8281fd227409626695a28`, respectively.
Source pins were not relaxed. These are selected historical Chromium and HTTP proofs, not current-revision or device acceptance.


### Chromium baseline acknowledgement repair

At `4f27d376`, required and deep Chromium jobs failed all eight controlled watched cases on both attempts.
Every failure occurred at the five-second baseline-progress deadline, before the actual watched POST was replayed.
The first trace recorded three baseline POSTs with HTTP 204 response headers, received within four milliseconds each.
Their `response.finished()` promises did not resolve during the held navigation. The underlying browser cause remains unconfirmed.
The trace SHA-256 is `248845465b2defb7f5bebe6c847e0b326e8eca6b3d87722034222b960659140f`.
The deep archive SHA-256 is `5974d85776dffdbf8a5083091199ee80b6f0722e1017e1722d451f36bb39ae94`; all 721 checksums were verified.
Required CI tested merge revision `61b4b64b`, with parents `7131f22a` and `4f27d376`. Its archive verified 617 checksums.

The progress handler emits 204 only after the store operation returns successfully.
[RFC 9110, section 15.3.5](https://www.rfc-editor.org/rfc/rfc9110.html#name-204-no-content) defines 204 as complete at its headers.
The revised prerequisite records the received response status and explicitly labels its boundary `http-204-headers`.
It does not claim a browser request-finished event. Null responses, non-204 responses, and missed deadlines still fail.
The five-second deadline, native motion, late-POST checks, mutation controls, and final stored-state assertions remain unchanged.
Historical receipts still require their original request-finished field when rechecked.

The isolated listener control also checks duplicate registration from outside the application script.
The fixture resolves existing wrapper identity before checking registration scope, so that duplicate cannot bypass the barrier.
Source checks and hosted runtime results for these repairs are recorded separately. Runtime acceptance remains pending.

Independent review found no remaining material source blocker in the listener and acknowledgement repairs.
Whitespace, source-file limits, and production browser-script lint passed. Discovery selected 60 cases across three engines and three files.
These source checks did not launch a browser or application. Hosted validation remains the runtime acceptance step.

### Listener trial and public transcode refill diagnostic

At `dda33cd1`, [deep run 38341639360](https://github.com/Kinosail/kinosail/actions/runs/38341639360) passed the WebKit listener control and all four unmodified controlled watched cases on their first attempts.
The ordinary navigation case and two mutation cases also passed immediately.
The watched mutation failed initial playback admission, then passed on retry.
The cancelled unwatched mutation failed the ending-buffer prerequisite twice. Neither attempt reached mutation admission.
The downloaded WebKit archive verified clean source, ten inputs, and 540 checksums.
Its SHA-256 is `c38a4477a275cecb905e8743776ff7ce7c84f078fef16f9ac3a56cea4179c954`.
These failures remain open; the fixture repair is not proof of a media repair.

The next diagnostic was written before any production timestamp change.
It covers these concrete failures: a refill shifts frame timestamps, changes frame count, corrupts decoding, mutates retained media, or silently avoids regeneration.
The real HTTP test generates an eight-second FFV1 source at 25 fps, forcing video transcoding at two source sizes.
It requires one actual rendition at 320×180 and multiple actual renditions at 1280×720.
Both exercise `encodePresentation`; copied-media `encodeVariant` is outside this test.
After the initial encoder exits, it saves initialization and three fragments, then deletes one owned middle fragment.
A public GET must start a real refill. An exec-only adapter records joined producer identities and safe clock arguments.
The mixed decode uses the original neighboring snapshots because clients may already have buffered them.
Forward cache files may legitimately be replaced. Source bytes, initialization, and the preceding fragment must remain unchanged.

The control must decode 150 contiguous frames. The regenerated sequence must preserve each frame's exact timestamp and duration.
The oracle retains the demux timebase and uses `-copyts` and `-fps_mode passthrough`, as described in the [FFmpeg documentation](https://ffmpeg.org/ffmpeg.html).
It records media hashes, decoded-clock hashes, exit status, and bounded diagnostic classes without printing raw codec output or source paths.
Pixel hashes identify evidence; lossy reencoding is not required to reproduce identical pixels.
This test can establish a Server timeline defect. Browser append offsets and the cause of WebKit resets require separate proof.
Hosted execution is pending. Local runtime remains held for the coordinated physical-device window.

The test-only regression was committed as `bcc888a0` and dispatched in [deep run 38344855812](https://github.com/Kinosail/kinosail/actions/runs/38344855812).
Source review found no admission or clock-oracle blocker. Runtime results remain pending.
Its first tooling check exposed a stale Code Atlas test-file count, 374 instead of 376.
Only two untagged Server test files were added. The snapshot count was refreshed; the canonical hosted generator must verify it.

The completed `dda33cd1` Chromium and Firefox archives verify clean source and ten inputs each.
Chromium verifies 535 checksums; Firefox verifies 571. Both passed all ten arithmetic controls, the listener control, and nine watched cases on their first attempts.
Their archive SHA-256 values are `199ee42141e440e20dcea6130da91401faf056c6e86286a10d4a384012b45cdf`
and `7a05d6a7dba4070c20c9917438db2f79c9d5ee764d74530ac2a3bc5556b15974`, respectively.
Their general suites retain 357 and 361 conditional skips. These results do not imply complete product coverage.

All three `bf39db6e` layout artifacts verify unchanged source and seven script inputs each.
Each reports eleven stable Player flows and 26 stable Subtitles flows, without a failed stability or focus field.
The isolated fixture suites passed 35 Chromium, 37 Firefox, and 35 WebKit cases, with two conditional skips in Chromium and WebKit.
Measurement hashes were recorded locally. These artifacts do not provide an upstream full checksum manifest.

### Confirmed transcode refill failure and output-policy trial

The `bcc888a0` public HTTP regression failed all four actual renditions: 180p, 432p, 540p, and 720p.
Each original sequence decoded 150 contiguous frames. Each mixed sequence decoded only 148 frames with the same 1/12800 timebase.
All eight decodes exited successfully and emitted no diagnostics. The source, initialization, and preceding fragment remained unchanged.
Each fixture recorded exactly one joined initial producer and one joined refill producer using `libx264`.
The three-rendition refill recorded three output offsets and start numbers, proving that it exercised the multi-output path.
The hosted Go log SHA-256 is `740eb0f139a9740dae8819bfdebaa3d630ad7629e21ee185486187a44457d505`.
The regression completed in 10.46 seconds. This confirms lost decoded frames after public regeneration; it does not establish the cause of WebKit resets.

The production trial explicitly sets `-avoid_negative_ts disabled` for every HLS video-transcode output, both initially and during refill.
The former flag appeared before `-i`, in the input option group. It did not explicitly configure each output muxer.
The output offset, keyframes, segment duration, and decoder assertions remain unchanged.
A video-transcode-only cache marker forces older presentations to rebuild before they can mix with new output.
Copied streams and audio-only cache identities retain their existing policy.

Before this production change, the public regression gained a synthetic legacy-policy migration check.
It removes the marker from all three owned cache bindings, reopens through HTTP, and requires another joined producer.
The rebuilt master and durable bindings must restore the policy. Decoded clocks and source bytes must remain unchanged.
This tests invalidation of simulated legacy metadata; it does not replay a retained historical media cache.
Safe JSONL receipts and checksums now persist for both passing and failed deep jobs without repeating the test.

Independent review found no source blocker in the trial. Seventeen workflow contracts and the pinned parallel-aware actionlint passed.
The system actionlint rejected a pre-existing parallel step; it does not support the workflow syntax used by this repository.
Source limits and whitespace checks passed. The Code Atlas was refreshed from unchanged canonical package inventory using the generator's source parser.
Full canonical generation and runtime verification remain hosted checks. No local app runtime was started.
These fixtures are video-only software H.264 at 25 fps. AAC timing, browser/native startup, and hardware encoders remain separate boundaries.

### Output-clock trial: refill passes, migration assertion corrected

At `b2d5ae37`, [deep run 38346543067](https://github.com/Kinosail/kinosail/actions/runs/38346543067) passed the unchanged original/mixed clock assertions for all four actual renditions.
All eight decodes produced 150 frames with identical timestamps and durations, zero diagnostics, and successful exits.
The clock SHA-256 is `8fc7ba932ab62e939b596e1d5ffb756fe56173405fe1196abb68f8a63788af0b` in every case.
This repairs the demonstrated 150-to-148 frame loss. It is not yet full migration or browser acceptance.
The evidence archive SHA-256 is `aec1ba44b8a1202366bf5501c29cfc013ed670551320c7a9f9635c77dc87a1c4`.

Both migration fixtures started exactly one additional joined producer and restored the master policy.
The test then incorrectly required `.seekable` after a full rebuild. That marker belongs to a seek-generated cache.
A full rebuild can remove it; the completed master and `.source` bind the resulting cache.
The corrected assertion still requires the new `.source` policy. It accepts an absent `.seekable`, but rejects other read errors or a retained stale policy.
Independent review confirmed this lifecycle distinction. The exact legacy decoded-clock assertions remain unchanged and await hosted execution.

The trial also identified two test-helper lint findings: a complex migration function and the trusted evidence-directory boundary.
The binding check now has a focused helper. The test-only directory creation explicitly documents its trusted runner input.
No production validation or quality threshold changed. Canonical Code Atlas generation passed at `b2d5ae37`.

The earlier `bcc888a0` Subtitles deep job received HTTP 404 in `TestCommandLineCanDisableTLS`.
The unchanged Subtitles deep job passed at `b2d5ae37`; this does not establish the earlier failure's cause.
The CLI test now retains private child output and emits only a bounded failure projection.
It reports the ready-log observation, server failure, bind-conflict class, child 404 count, exit status, and truncation.
The ready log precedes binding and does not prove that the child served a request.
The test still fails the first unexpected HTTP response. No status retry, timeout relaxation, or production seam was added.

### Browser clock setup race

The `b2d5ae37` Firefox archive verifies ten source inputs and 518 checksums.
Its sole failed first attempt was the isolated scrub-preview fixture; the retry passed.
`clock.pauseAt` failed before any preview request because its target equaled the earlier installation time.
The clock had already advanced between those calls. [Playwright documents a future pause target](https://playwright.dev/docs/api/class-clock#clock-pause-at).
The fixture now pauses one hour after its fixed installation time, beyond its unchanged 60-second test deadline.
The shared HTMX fixture had the same setup race against wall time and now uses the same forward-only initialization.
Preview coalescing, the 32-millisecond step, HTMX's 119/120-millisecond threshold, screenshots, and error-state assertions remain unchanged.
These are isolated browser fixtures, not populated-server media proof.
The Firefox archive SHA-256 is `4b9993a63648ba3ccc22df0df8d5b3774cbbe5ab8da7bb8e6ef1bcc888729222`.
The WebKit `bcc888a0` archive also verifies ten inputs and 525 checksums; one mutation prerequisite required a retry.
Its SHA-256 is `fce75a1b810dc66c3defe6c698553ae162843eab4f5da7d63b3a4faeac7b7cad`.
That WebKit result predates the timestamp repair and remains separate from its current validation.

### Refill and migration accepted; native and navigation failures retained

The exact refill and simulated legacy-cache journeys pass at `a8bf2a55` and `9e220a9e`.
Each archive verifies seven checksum entries and 30 safe receipts against its exact revision.
All twelve original, mixed, and rebuilt decodes contain 150 frames with identical clocks across four actual renditions.
Both legacy reopens require exactly one additional joined producer. No decoder reports an error or diagnostic output.
The archive SHA-256 values are `b99aced794a6b11952eaf1a813af8de93bd2c523f5a91b49dfe56740bd0b639c`
and `56e419017d5d23d7bd1a72d52b61d850b8ab09e25f6cc3e27c69c4da3d672fe7`.
The corresponding Player Go jobs passed. The video-only software-encoder boundary remains unchanged.

At `b2d5ae37`, WebKit passes all nine watched-state journeys without retries, including the mutation controls.
Its artifact verifies ten source inputs and 499 checksum entries, with zero failed or flaky cases.
It also passes the focused caption, Downloads, queue, pagination, and settings cases.
The archive SHA-256 is `9f07f83e1120bec14c488ae4ce173cce2ee95db0c16ed3d89861b7cffe366cfe`.
This is browser acceptance at that revision, not physical-device or current-main proof.

Required CI at `9e220a9e` identifies one remaining migration-helper complexity violation: 11 against a maximum of 10.
The legacy-binding mutation now has a separate helper. Mutation order and every public migration assertion remain unchanged.
Independent review finds no blocker. Source limits and whitespace checks pass; hosted validation remains pending.

The same revision's native job fails one stale-pairing authorization case and three assertions in the TV top-bar focus journey.
The authorization fixture records no request admission or arrival during the original ten-second deadline or the extra diagnostic window.
The new receipt also reports whether the public snapshot remains authorized, whether the job exists, and its allowlisted status.
It never reports credentials, identifiers, error messages, or request contents. The original request observation remains frozen before diagnostics.
The TV receipt records four fixed phases, observed and requested focus, window state, and elapsed time.
The existing waits and assertions are unchanged. No native production defect or repair is claimed from these failures alone.
The failed native log SHA-256 is `a4c58551846bad9c3457285a60bb99cb73b101dde9536d41081bcd01aa14c809`.

Firefox layout completes 40 Player measurements and 11 flows, then 46 Subtitles measurements before its first interaction journey times out.
The original navigation receives HTTP 200 for its document and eight assets. The failure screenshot shows the populated Library.
Playwright still reports `about:blank`, and its navigation reaches the unchanged 30-second deadline without a commit event.
This evidence suggests an automation navigation failure, but does not make the incomplete Subtitles flows pass.
The archive SHA-256 is `301fca9016b7ab711008df7bebfb452e852d68e46b085b65993c8e2c31fbeec5`.
The screenshot SHA-256 is `fc564c78d9b1cffd0dcc052fd5837a44b292f47d0ac61ec3550f7a951f31b779`.
These failures remain open. Local runtime stays held, and PR 587 remains unmerged.

### Video-clock coverage with AAC present

The refill matrix now adds an audio-bearing version of both synthetic sources. The original video-only cases remain unchanged.
The new inputs contain lossless 48 kHz audio, which the actual video-transcode path converts to AAC.
They retain the exact 150-frame original, mixed, and legacy-rebuilt video assertions for every rendition.
This targets video-clock changes caused by muxing audio, including encoder priming.
The original, mixed, rebuilt, and standalone refilled audio fragment must decode to nonempty PCM without diagnostics.
The standalone check prevents retained neighbors from hiding a missing refilled audio track.
Video-clock results are recorded before audio acceptance, so audio failures cannot conceal that diagnostic.
Independent source review finds no blocker. Runtime validation is pending; audio sample alignment and waveform equality remain untested.


### AAC-bearing clocks pass; audio duration and native timeout remain open

At `e15932f843a53e7dd11ac2cd372bd347aae9f116`, the expanded [deep run](https://github.com/Kinosail/kinosail/actions/runs/38351956804) passes Player Go tests.
The public-media artifact verifies 13 checksum entries and 76 receipts for four fixtures and eight actual renditions.
All 24 original, mixed, and legacy-rebuilt video decodes contain 150 frames with identical clocks.
All four legacy reopens start exactly one additional producer. The sixteen audio decodes produce nonempty PCM without diagnostics.
The archive SHA-256 is `d82b37a5545437794e6dd668f9c013346919d05cb4297d926fd7bf0e7ae028ca`.

The audio receipts expose an unresolved difference in every audio-bearing rendition.
Original and rebuilt sequences each decode 1,142,784 PCM bytes; the mixed sequence decodes 1,130,496 bytes.
The 12,288-byte deficit is repeatable. Clean decoding alone does not prove audio continuity.
The next diagnostic records original and regenerated fragment packet clocks, including gaps, discard flags, and skip/padding metadata.
Missing clock fields fail explicitly. The existing video and audio acceptance checks remain unchanged.
These are demuxed packet observations; they do not by themselves prove raw encoded-packet loss or waveform equality.
The projection follows [FFprobe's selected-entry interface](https://ffmpeg.org/ffprobe.html#Main-options) and its [packet side-data definitions](https://www.ffmpeg.org/doxygen/5.1/group__lavc__packet.html).
Independent review requested timestamp-presence checks and trimming metadata; both are included.
A hosted gofumpt receipt-format finding was also corrected using source formatting only.

The same revision passes all three [layout jobs](https://github.com/Kinosail/kinosail/actions/runs/38351962381).
Each artifact verifies seven exact source inputs and a clean, unchanged revision. Both app commands exit successfully.
Chromium records 95 Player and 111 Subtitles measurements; Firefox and WebKit each record 40 and 46.
Each engine completes 11 Player and 26 Subtitles interaction flows without failed stability or focus checks.
Isolated layout fixtures report 37 Firefox passes and 35 Chromium/WebKit passes each, with two skips in each latter engine.
There are no failed or flaky isolated cases. The earlier Firefox navigation failure remains retained as intermittent evidence.
Archive SHA-256 values are `b6121f66b735ed829866d9248ac1f7c66cec5df644c4984223a65bfa2b64305b` (Chromium),
`6ac02ba2a7810d1b805ad6c1761e7503c4f740563830cf1f92ec1c1625f67a18` (Firefox), and
`4f71b3dde01c6d23ef3a4a311e1d76ce7c256d32d064bf153329d5de8c496296` (WebKit).

The native run passes the previously failing pairing and TV top-bar cases but fails extensionless AAC playability on iOS.
The safe probe receipt reports a deadline at 20.025212 seconds in `start-reading`, with zero samples and no reported error.
This locates the stall in AVFoundation startup; it does not establish a corrupt fixture or a production remedy.
The native log reports 334 iOS tests with failure and 316 tvOS tests passing. These reported counts are not a full test-discovery audit.
The log SHA-256 is `9c479738653de99f6669ab9bc751889e716b841822c7b4773f7db0baac9906e6`.
The next deep run also selects the existing isolated AAC suite to compare full-suite and isolated behavior.
No native deadline, assertion, or production behavior has been relaxed. Local runtime remains held for coordinated device work.


### Confirmed transcode refill audio gap

The `07a6e06779ec11fc5d986f1c4826606e2925af0b` public-media artifact verifies 116 receipts and 13 checksum entries.
The archive SHA-256 is `6dfc50e52a59a2c426844a272e363b64c2a3c6da9e78066f65f26eea32e9cf32`.
All 24 video decodes and four legacy migrations still pass. Packet diagnostics explain the audio deficit in every AAC-bearing rendition.
The original middle fragment contains 94 packets, starting at tick 92,160. The regenerated fragment contains 91, starting at 94,976.
Every packet lasts 1,024 ticks in the verified 48 kHz clock. Neither sequence reports discard flags or skip/padding metadata.
The mixed sequence has gaps of 2,816 and 256 ticks at the two splice boundaries: 3,072 ticks, or 64 milliseconds, total.
It contains 276 packets versus the original 279. PCM is correspondingly 12,288 bytes shorter in stereo 16-bit output.
This is missing delivered audio time, not merely a different AAC payload hash or decoder-trimming receipt.

Before production changes, the existing public journey now requires matching AAC packet counts, PTS, DTS, and duration.
It also requires equal original/mixed decoded PCM lengths. Both full mixed sequences and standalone regenerated fragments are compared.
The old receipts violate these new assertions in all four audio renditions. Execution of the strengthened regression is still pending.
The audio helpers are separated from video/cache evidence to preserve the 300-line file limit.
A fixed manual HLS selection runs the existing complete Player race suite with the same codecs and evidence format.
It rejects conflicting remaining-playback selections and does not change required PR or main gates.
Seventeen CI contract checks, pinned actionlint, source limits, and whitespace checks pass locally. No local application runtime was started.

The repair must preserve bounded seeking and real audio content. Padding or timestamp-only changes are insufficient.
Failure cases include a restarted AAC sample grid, missing audio before the first video's decode time, and millisecond seek rounding.
Other boundaries include nonzero recipe origins, differing sample rates, omitted ranges, listening filters, missing initial fragments,
per-rendition decode offsets, stale cache metadata, and source replacement. These guide the next bounded experiment and review.
The initial design review favors a separately sought audio input with real encoder history, preserving the existing video input and filters.
That design is not yet implemented or accepted. Exact-clock assertions alone do not establish waveform continuity.

The same revision's native artifact passes iOS (335 passed, one skipped), tvOS (320 passed, one skipped), and isolated AAC (two passed).
The isolated receipt verifies exact source, revision, test discovery, and exit status. The native archive SHA-256 is
`f1f1a1bd549e988379ea10b4695563599fe985a709dc88609d058c21c125801e`.
The iOS skip requires Picture in Picture support absent from that simulator. The tvOS skip is an opt-in disposable-session seeding helper.
The earlier AVFoundation timeout remains intermittent evidence with no claimed production repair or relaxed deadline.

### Strict public audio regression is red before the repair

The [focused hosted run](https://github.com/Kinosail/kinosail/actions/runs/38357506323) at
`3206083a4687113c2120f0da51cf84b1173550d7` executes the stricter public regression and fails as expected.
Its artifact verifies 13 checksums and 94 receipts. Both video-only fixtures pass; both audio-bearing fixtures fail.
All four audio renditions report 279 original versus 276 mixed packets, and 94 original versus 91 regenerated middle packets.
Every mixed sequence retains the same 2,816- and 256-tick gaps. PCM remains 12,288 bytes shorter.
The archive SHA-256 is `9a4123c8a1ee0e0c6062a4c198aace40e11b308d02775d5eab1f4cb83f329f79`.
The safe error projection binds to job log SHA-256 `f81a4bf3b8c26584faa687c9d10d4105d098ae36e903d5c77a48ccaae5d228d1`.
This is executed regression failure before any production audio repair, not a prediction from older receipts.

The next public fixture uses two changing-frequency chirps instead of a periodic sine tone.
It compares decoded content at fixed positions across both splice boundaries, with 100 milliseconds on each side.
Acceptance requires correlation of at least 0.995 and normalized RMS error of at most 0.1.
The same windows must reject silence and a 64-millisecond displacement. Packet clocks, counts, and PCM lengths remain exact.
These content assertions are written before production changes and await hosted execution.

A separate hosted codec experiment tests bounded sample-aligned audio preroll at 48 kHz and 44.1 kHz.
Each source is 18 seconds long. The experiment refills at two and fourteen seconds, retaining both original neighbors.
It keeps video seeking and encoding unchanged, adds a separately sought audio input, and drops only encoded AAC preroll.
Audio timestamps move inside the user filter, after FFmpeg's automatic accurate-seek input trimming.
Negative input `-itsoffset` can discard warm-up before encoding; the pinned implementation explains that boundary:
[demux timing](https://raw.githubusercontent.com/FFmpeg/FFmpeg/n8.1.3/fftools/ffmpeg_demux.c) and
[input filtering](https://raw.githubusercontent.com/FFmpeg/FFmpeg/n8.1.3/fftools/ffmpeg_filter.c).
The experiment reads the original target's timing. Production still needs durable timing metadata before master publication.
It is isolated codec evidence, not public HTTP proof or an accepted repair. Omitted ranges and listening effects remain untested.

Independent review corrected timestamp precision, command journaling, timeout checkpoints, codec provenance, and interrupted checksums.
The experiment uses the public oracle's demux timebase and requires exactly 150 contiguous decoded video frames.
It records resolved codec binary hashes and runs only after successful pinned-codec installation.
An interrupted run preserves incomplete status; an always-run checksum step seals the available receipt and command journal.
The public content helper shares one threshold predicate to remain within the existing complexity limit.

The earlier [full deep run](https://github.com/Kinosail/kinosail/actions/runs/38354552694) at `07a6e067` finishes green.
That green result predates the stricter audio assertions and does not close this defect.
Its latest layout artifacts verify clean revision and all 11 Player and 26 Subtitles flows in each browser.
Archive SHA-256 values are `320fbcc5c42557ba484021ae51065670c08135fdbee168af0934c0f2fd4c6587` (Chromium),
`ffd7bcf97855e5a7f4272ce59b60612d3f43daae65a020136239b83626ff99b6` (Firefox), and
`6726e57ab9ce63fe21d374b8cb327736a815448364990527333092fe37a83351` (WebKit).
The isolated layout skip counts remain two each for Chromium/WebKit and zero for Firefox.
All three Subtitles browser artifacts verify 198 checksums each and 99 passing cases, with no skips, failures, or flakes.
No local application, browser, simulator, emulator, codec, or device runtime was started.

### Bounded preroll restores clocks but still shifts later audio content

The [next hosted run](https://github.com/Kinosail/kinosail/actions/runs/38358977537) at `c4a027f338b62491ead806621fa8c396d69a7e90`
executes the nonperiodic public regression. All four audio renditions fail both content joins while rejecting silence and displacement controls.
Entry correlations range from 0.7138 to 0.7151; exit correlations are about 0.4500. Both video-only fixtures pass.
The public artifact verifies 13 checksums and 102 receipts. Its archive SHA-256 is
`00118718a2c1890f03e2d5eb188de5b331825239b331d54575fb1d06e3fb339e`.

The separate codec experiment completes all four cases and 48 commands without codec errors or diagnostics.
Every case preserves exact AAC packet clocks, decoded video clocks, and PCM length.
Both two-second refills match the control waveform exactly at both joins.
Both fourteen-second refills fail the unchanged content thresholds despite matching all timing checks:

| Sample rate | Entry correlation / normalized RMS | Exit correlation / normalized RMS | Result |
| --- | --- | --- | --- |
| 48 kHz | 0.2530 / 1.2247 | -0.0284 / 1.4329 | fail |
| 44.1 kHz | 0.9471 / 0.3240 | 0.9216 / 0.3972 | fail |

The artifact verifies both checksums, exact script source, revision, and installed codec binary hashes.
Its archive SHA-256 is `ad623934544b9f181dc0e522423ffa5d0b0e44b42364fc7b50ff71a54136bb3c`.
This rejects the proposed mechanism as a complete repair. Equal timestamps and packet counts cannot establish correct audio content.
The next diagnostic compares the lossless input seek against known source samples within a fixed 64-sample search range.
That search reports a possible source offset only. It never realigns the splice comparison or changes acceptance thresholds.
An explicit manual `AAC` selection runs this codec-only experiment; the public HLS and required CI suites remain unchanged.

Required CI also exposes an overbroad artifact contract: it scans all later workflow jobs for JSON wildcards.
The repaired check isolates its upload step and requires the exact sixteen paths for four approved JSON files per campaign.
The prior unrestricted-tail assertion is retained in the failed hosted log. Sixteen architecture and eighteen workflow checks pass locally.
The branch also incorporates protected main `a1c7a1dc8396be32ac66851f098f2f23a8018c6a`, including the native preference-read repair.
No production audio repair, local media execution, or comprehensive pass is claimed.

### Accurate input seeking loses the sample origin on coarse clocks

The [codec-only diagnostic](https://github.com/Kinosail/kinosail/actions/runs/38360904835) at
`2d06a296c70e711fc76084decb889c7d6c69b1c6` verifies both artifact checksums, exact script source, and pinned codec hashes.
All 52 commands complete cleanly. Early refills still match exactly; both late content comparisons still fail.
The lossless input diagnostic finds the 48 kHz seek starts 16 samples late and the 44.1 kHz seek starts three samples early.
Both comparisons use exact PCM equality against the generated source. Neither diagnostic changes the fixed splice acceptance.
The artifact archive SHA-256 is `c11e1b1663cf2d95e56dc346f222ff53f18293fb57bfbe182a3d7886e3e8b233`.
This identifies a sample-origin error before AAC encoding. A timestamp-only repair would preserve that content error.
The next isolated experiment will reconstruct the certified fixture's decoded frame clock before trimming.
General source clocks, discontinuities, priming, resampling, and production publication remain separate unresolved contracts.

Required CI at the same revision exposes two stale campaign-route assertions after adding the manual AAC selector.
Their exact allowlist and manual-only job condition now include AAC; the general campaign route explicitly excludes it.
Required PR/main gates are unchanged. The failed repository job log SHA-256 is
`df23f146e1689cdb8ecad275ef455cd64943e78071bf1dee07f39a68fc4f7afc`.
The [hosted source-metadata run](https://github.com/Kinosail/kinosail/actions/runs/38360999608) passes at the same revision.
All three approved metadata file hashes match. Exact revision, tracked tree, source pins, and both rendered HTML hashes verify locally.
The canonical reproduction updates only Player's architecture snapshot after added Go regression files.
Its rendered SHA-256 is `0194dcd9536a6f5c867c3456b49145d8e53163cbef592448e83df39ea1e65080`.
No local Go, application, browser, native, codec, or device execution was used for this refresh.

The isolated reconstruction trial has its failure model before implementation: wrong fixed block size, partial interior frames,
format changes, nonzero origin, unfinished scans, excessive clock uncertainty, and persistent timestamp discontinuities must reject.
The closed fixture scan must reach EOF and account for exactly 18 seconds of samples.
A persistent one-millisecond timestamp step is the negative admission control.
The normalized lossless seek must match the source exactly at offset zero, independently of the unchanged AAC splice comparison.
Independent review confirms the formula only for the certified fixture and exact formatted microsecond seek.
[FFmpeg seek behavior](https://ffmpeg.org/ffmpeg-all.html#Main-options) requires disabling automatic accurate-seek trimming before reconstruction.
[FLAC fixed-block framing](https://www.rfc-editor.org/rfc/rfc9639.html#section-8.2) permits a shorter final block.
This experiment is not a general source repair and does not alter any production media path.
The conservative uniqueness bound includes two coarse timestamp ticks plus two sample-rounding steps.
Nineteen campaign-route checks pass after the stale assertions are repaired. Python source parsing and source limits pass.
The next codec experiment awaits hosted execution; its helper source hash is included in the receipt.

### Certified sample reconstruction passes the isolated splice experiment

The [hosted trial](https://github.com/Kinosail/kinosail/actions/runs/38362489818) at
`5fa2d1ba5c938205031814ca8c7ac61fa4fbd075` passes all four cases and all 58 codec commands without diagnostics.
Both artifact checksums, both script hashes, exact revision, and codec binary identities verify.
The archive SHA-256 is `25737cdc0ebc324f26bb1eb4ef9a251a6973eaccef81c558201af7a67e8af216`.
Both rates have 4,608-sample FLAC blocks. The scan reaches EOF with exactly 864,000 and 793,800 samples respectively.
The one-millisecond discontinuity control mutates the measured frame metadata; it is not a separately muxed faulty-media test.

All four corrected input seeks match their source PCM exactly at offset zero. The uncorrected late offsets remain +16 and -3 samples.
AAC packet clocks, 150 decoded video clocks, and PCM lengths remain exact in every mixed sequence.
Both early waveform joins match exactly. Late results pass the unchanged thresholds:

| Sample rate | Entry correlation / normalized RMS | Exit correlation / normalized RMS |
| --- | --- | --- |
| 48 kHz | 0.999971 / 0.007690 | 0.999995 / 0.003164 |
| 44.1 kHz | 0.999970 / 0.007768 | 0.999993 / 0.003702 |

All silence and displacement controls reject. No alignment search, padding, or threshold relaxation contributes to acceptance.
This accepts the bounded reconstruction mechanism for these certified sources. Production HTTP refill remains red.
Production still needs original rendition timing, durable generation identity, bounded staging, and publication without replacing retained neighbors.
The branch incorporates protected main `5cf9ba06`, which includes PR 590's native recovery and deployment verification changes.

The next isolated boundary is obtaining exact original cuts efficiently. Independent source review finds pre-mux records
insufficient because they precede mux success, timestamp offsets, and HLS cut selection.
The smaller alternative uses one FFprobe pass over initialization plus finalized fragments, assigning each packet by byte position.
The experiment must prove exact agreement with isolated-fragment clocks, unique packet ownership, and bounded input/output.
A second pass omits the actual middle fragment bytes; all surviving clocks must remain unchanged.
This delegates edit-list and negative-DTS interpretation to FFmpeg and avoids a new MP4 clock parser.
The [MOV demuxer](https://raw.githubusercontent.com/FFmpeg/FFmpeg/n8.1.3/libavformat/mov.c) records each packet's sample byte offset.
Production completion scans would add media I/O. Interrupted-generation recovery and long-film cost remain unverified.
Hosted Repository policy and tooling pass at `5fa2d1ba`, confirming the route-contract and architecture snapshot repairs.

Before production integration, the public HTTP regression also needs a later recovery boundary.
The added journey will close the first server after its encoder joins, remove the first and seventh fragments,
start a fresh server with the same cache, and request fragment seven using the original playback plan.
It must preserve the original init and both retained neighbors, leave the evicted first fragment absent,
and satisfy exact video/AAC clocks, PCM length, waveform joins, and negative controls.
Existing short-clip journeys do not cover durable metadata, nonzero audio warm-up, or a missing initial fragment after restart.
No production seam is added; only the owned cache files are evicted between two real HTTP server lifecycles.

### Joined fragment timing matches independent demux

The [hosted joined-scan trial](https://github.com/Kinosail/kinosail/actions/runs/38364261264) at
`b171808c110e7400293699d6ccf2366d475e92c2` passes all four splice cases and 94 clean codec commands.
The artifact verifies both checksums, all three script hashes, exact revision, and pinned codec identities.
Its archive SHA-256 is `36bbfa1900d381fa907e76face12059a96e45d6ee7ebed2cefae3c21da04bb81`.
At each rate, all video/audio packet clocks in nine fragment intervals exactly match isolated-fragment demux.
Removing the actual fourth-index fragment from joined input preserves every packet clock in the eight surviving intervals.
Each packet's byte position and size belongs to exactly one fragment. The refill now derives its original cut and AAC phase from this result.
All four splice cases retain exact packet clocks, video clocks, and PCM lengths, with unchanged waveform thresholds and negative controls.
This establishes closed-fixture packet association. It does not establish durable metadata or production restart recovery.
The new public restart journey also checks bounded pre-input seeks; an output trim after replaying the source origin cannot pass.

Independent review tightened the restart journey before execution. It reads retained cache files before HTTP reads,
then joins and recounts producers after all requests. This prevents repair-on-read from hiding a deleted neighbor.
The final check requires fragment zero to remain absent. Fragment seven is a late middle fragment of a 20-second source,
not an EOF-tail case. Encoder exit and ENDLIST do not certify any future metadata worker; that handoff needs explicit proof.

Production integration failure model, recorded before implementation: a missing or modified proof must not select a legacy writer;
changed init/source/encoder identity must reject; incomplete metadata must never appear committed; pending media must not be served;
retained fragments must not be replaced; cancellation must join owned workers before cleanup; a competing writer must not be overwritten.
The minimum new public counterexamples remove the committed proof, corrupt it, and change the retained initialization bytes.
Each must return 404, start no encoder, leave the requested fragment absent, and preserve the master and retained neighbor.
The completed master will bind the proof hash. Initial or interrupted presentations must remain distinguishable from committed certification.

Independent review found two draft integration gaps: certified fragment zero must remain recoverable, and joined input probing
must use the existing process-tree owner. A bare command wait is insufficient when a descendant retains a pipe.
Before changing that shared owner API, configured-process controls now cover blocked stdin plus a descendant under deadline and cancellation.
Real media journeys cannot reliably create this process-lifetime failure. The existing bounded two-second settlement assertion remains unchanged.
The first streaming-input implementation will explicitly decline platforms whose existing launcher cannot own stdin; no unowned fallback is allowed.

### Restart regression finds retained-neighbor overwrite

The [hosted public run](https://github.com/Kinosail/kinosail/actions/runs/38365950074) at
`a10b55dd20e4e5bf87c4ea487d80cc19b357dda4` retains 107 receipts with 14 verified artifact checksums.
Both video-only cases pass. Both early audio cases retain the 64 ms defect.
The restarted late case fails because recovery replaces retained `segment-00008.m4s`.
It still joins exactly two encoders, observes a bounded input seek, and leaves fragment zero absent.
Artifact archive SHA-256: `2c0d45302570d84cc1670ad5210a516f0f9686ff289ccbd8348eec189254b320`.
Private job log SHA-256: `adf0dac1200f51e3311249d79ecc7eae5f02bb6058a50fc23eb9bc595a867847`.

### Draft production integration awaiting hosted verification

The first production candidate records original packet clocks after encoder completion, then binds the record hash in the master.
The publication loop accepts only the completion signal from its own worker. It does not adopt an arbitrary existing record.
Recovery admission rechecks certification after joining a prior producer. Damaged committed records reject before creating an encoder.
Regeneration uses separate nominal video and sample-aligned audio inputs. It stages all output and publishes only the requested missing fragment.
The candidate validates original init, exact per-track packet-clock digests, source policy, encoder identity, and canonical cache ownership.
A no-replace link preserves another writer's destination. Retained neighbors never become encoder output paths.

Initial certification is limited to software H.264 and closed, zero-origin, fixed-block FLAC without omitted ranges or history-dependent audio filters.
Every cut must have an exact audio sample and millisecond video seek. Its actual playlist offset must agree with the observed cut.
A common validator bounds persisted arithmetic and rejects incompatible cuts before the stream opts in.
Certification reads at most 16,384 fragments per rendition, 64 GiB of generated input, and 256 MiB of packet metadata within 15 seconds.
Source certification has a six-second budget. These are explicit optional-certification limits, not established long-film performance results.
Unsupported or interrupted streams retain their existing behavior and remain outside the audio-repair claim.

Joined fragment input uses the existing process-tree owner. Its first stdin extension supports Linux and macOS;
Windows explicitly declines streamed-input certification before process launch. No bare subprocess fallback is used.
Configured-process cancellation controls and public first-fragment recovery are added alongside the restart and damaged-proof journeys.
The candidate has only source formatting and line-cap checks so far. No runtime, browser, native, physical-device, or production acceptance is claimed.

### Production transcode recovery: first hosted acceptance boundary

The full Player race run [38369723795](https://github.com/Kinosail/kinosail/actions/runs/38369723795) tested revision `2ef831ae631bf9258de2015ffae87374cbd804fa`. Four initial refill cases passed: 320x180 and 1280x720, each with and without audio. Each case used one initial producer and one real HTTP refill producer. Every tested audio rendition retained the exact decoded PCM bytes: 1,142,784 bytes for the six-second comparison. This closes the earlier 64 ms loss for these early-fragment cases. It does not prove the restarted late case.

The 20-second restart fixture failed before cache eviction or restart. Its completed master never committed a recovery certificate. Certification reported an ordinary-stream fallback without budget expiry. Artifact `11798873690` binds 14 checksum records to the exact revision. Its ZIP SHA-256 is `511d013d1f9ce49ad7a70d9f9ecff4a021e2302b814fa66ab4ac4b829d5a4d76`. The private job log SHA-256 is `4a473fbbdde2131031a376e9182f5b16574fa426974d8749defa3a1cb6e90d07`.

Independent source review found that certification rejects every AAC packet whose duration differs from 1024 samples. A final 512-sample packet necessarily fails this rule. The 20-second fixture ends halfway through that grid; the 8-second fixture ends on it. The generic hosted warning does not independently identify that packet as its rejection point. The candidate now accepts a short AAC packet only in the actual final fragment, rejects any following AAC packet, and retains the exact duration in the clock digest. A single-fragment recovery probe receives final-fragment authority from the certified original record, never from its local EOF. Hosted confirmation remains required.

The candidate also records fixed safe certification operation names. Underlying probe errors are retained for lifecycle handling but never logged. New public rejection controls rehash proofs with oversized source sample counts and video timebases, then require HTTP rejection, no extra producer, absent target fragments, and unchanged retained media and source bytes. These controls remain unaccepted until the restarted journey reaches them.

Hosted lint failures prompted extraction of parsing, validation, and publication phases. No linter thresholds or required gates changed. Local validation remains limited to formatting, source-file limits, whitespace, and verification plans under the coordinated runtime hold. The architecture snapshots were reproduced from the verified metadata artifact for revision `2ef831ae`; the additional source changes require a fresh canonical projection.

### Restart admission after terminal-packet correction

Run [38373152008](https://github.com/Kinosail/kinosail/actions/runs/38373152008) tested `1052a420a6d0dea9a748318612b6b7fd73aa39b2`. All four early cases passed again. The longer case passed the committed-certificate handoff and reached its first missing-fragment request after restart. That request returned HTTP 404 in the 1.94-second case. No restarted recovery acceptance is claimed.

Artifact `11800855019` verifies 14 checksum records and 125 receipts. Its ZIP SHA-256 is `f2ada3ce15229579708fe88fdfb034bb8230ca5e9db44d7f296b58584dffa450`. The private job log SHA-256 is `9a371ba97cccdb88329f3b5cc2c7ac2ed4515e7477711f89b812b7c17af7395d`. The next public run adds bounded failure-only log projections and observed producer arguments. Fixed operation names distinguish certificate admission, bounded encoding, staged validation, and publication. Raw logs and underlying probe errors are excluded from these receipts.

The required lint pass at `17671d3f` reports only one remaining complexity failure, in the public damaged-proof test. Restoration is now a separate helper; every rejection and unchanged-byte assertion remains. Verified architecture metadata from run [38374195572](https://github.com/Kinosail/kinosail/actions/runs/38374195572), artifact `11800456992`, reproduced canonical snapshots. Its ZIP SHA-256 is `fcc6df8b08c5e2dbad52ecae286aba22a18c923f71fa2fa494ed8de06e36c775`. Additional source changes still require a refreshed snapshot.

### Native browser-cache proof boundary

Fresh primary-source research identifies a limit in the existing Q14 approach. [Playwright documents that native BFCache restoration is unsupported](https://playwright.dev/docs/navigations#backforward-cache-bfcache), including explicitly enabled configurations. Its navigation tracking can lose synchronization because restoration bypasses normal document loading events. The retained Q14 proof already reports prerequisite failure, with browser reasons including `response-cache-control-no-store`, `masked`, and an unclassified reason. This evidence does not identify a product defect. Native browser-level validation remains open. Existing cache policy and required gates remain unchanged.

### Restart failure reaches staged clock validation

Run [38375493942](https://github.com/Kinosail/kinosail/actions/runs/38375493942) at `47c890664bafa78828a10d59343029da022b0717` passes the four early cases. The late case starts exactly two producers. Its refill seeks video to 14.000 seconds and audio to 10.986666 seconds, then rejects at `refill-stage-clock`. Admission and bounded encoding therefore precede the observed rejection. The evidence does not yet distinguish a failed probe from an initialization or packet-clock mismatch. Validation thresholds remain unchanged.

Artifact `11801317449` has 126 receipts and 14 verified checksum records. ZIP SHA-256: `cba01697b3fc3d551eeccce58884bb029144526dbe93659c3e5736168816c146`. Private job log SHA-256: `310289bf304a29c5d4ea6f5b96fdbe83af16693fc7918467f7449a69e31ff2a3`. The next diagnostic projection attaches only measured flags, equality flags, bounded clock values, and packet counts to the existing encoder error event. It emits no additional error event or raw probe text. Failure receipts independently allowlist those fields.

Player lint passes at this revision. The tooling failure identifies a stale architecture snapshot. Source-metadata run [38375497472](https://github.com/Kinosail/kinosail/actions/runs/38375497472), artifact `11801500833`, verifies all three file pins and unchanged source. Its ZIP SHA-256 is `a267e7a339a2309a80527f61ef4a4260278dbcfdcdde2dbe3c4e00f18c43bedc`. Metadata SHA-256: `3d6f983a1be5a38672cc9361eddf2be7acbb9243907bff116eaee31495a5cf0e`. Canonical reproduction updates Player's snapshot without local compilation or runtime. The diagnostic source additions require a fresh projection before delivery.

### Staged probe rejects before clock comparison

Run [38377349179](https://github.com/Kinosail/kinosail/actions/runs/38377349179) at `39833be279ac32f082cbbee55c03050a27e35fcb` again passes all four early cases. The late failure has `measured=false`: its staged probe returns no completed clock. An encoded-frame-order mismatch is therefore not established, and no hash semantics or playback acceptance rule changed. Artifact `11802640482` verifies 126 receipts and 14 checksum records. ZIP SHA-256: `f9a2c432c71521a9c80a5e447d7fc12feece668b935f652abf38c7b55ee90be3`. Private log SHA-256: `107cf97e2c8a3591fe6bba8cb6b8ac46576db79bf9a2566393e4259bf93b1d1f`.

Source inspection identifies why a metadata rejection can lose its specific cause: the owned probe joins the scanner and process, but omits the scanner error when returning an interrupted completion. Before changing this path, the existing configured-process rejection control was strengthened to return a distinct sentinel and require its identity after complete process settlement. The one-line repair retains that cause in the joined error. Existing context and lifecycle errors remain present. Fixed packet/parser stage names and bounded numeric fields now survive into the existing encoder failure event. Raw probe output remains excluded.

Player lint passes at `39833be2`. Architecture artifact `11800864320` verifies all three file pins, exact revision, unchanged source, and canonical rendered bytes. ZIP SHA-256: `5671570192e2fdd299cd8ec66f436e0206730d4c78e1894fb179af1ecc5fb149`. Metadata SHA-256: `a193194b40083267fca78350a30218b5332f26c8706bdc33a94cd71438f1036b`. This snapshot refresh is included with the diagnostic correction. New source changes still need their own hosted projection.

### Faster failure evidence without replacing complete acceptance

The manual HLS route now runs the fixed restarted-recovery case under the race detector before the full Player race suite. A separate preflight artifact records its exact command, revision, and one completed fixture. Missing or duplicate completion records fail the checksum step. Failed preflights remain failed jobs. A successful preflight still requires the unchanged full race and coverage command, with separate complete-suite receipts. Required PR and main gates are unchanged.

The new CI contract was written first and failed before the workflow change. The focused CI, workflow, and campaign contracts then passed 38 tests. Installed actionlint, source limits, and whitespace checks pass. A broader campaign-contract run passed 57 tests and exposed an existing macOS `PermissionError` in Q09's Python process-group settlement fixture. No matching owned fixture process remained after its fixed 60-second lifetime. This local fixture boundary is not playback acceptance and is not fixed by the preflight change.

Architecture metadata at `60051bb8` verifies through artifact `11802507312`: ZIP SHA-256 `379cd7ae2c61d57a59efde0e5b5c3b8492fe13fb41a620998d24211afe277c9d`, metadata SHA-256 `e205b45df3428127c4b5c76e3f62dd17542d0161a8aa9fcbc7c582ecec11436a`. All three file pins, exact source identity, unchanged-source flags, and rendered bytes verify. This refresh uses no local Go, codec, application, browser, or native runtime.

### Audio warm-up consumes the original refill duration

Run [38379454658](https://github.com/Kinosail/kinosail/actions/runs/38379454658) at `60051bb8e949e837aff0efab34ce08a7a724313c` retains the specific `aac-packet-duration` rejection. The staged seventh fragment has 50 video packets. After 72 full AAC packets, its last packet has PTS/DTS 742400 and duration 960 at 48 kHz. Audio ends at 15.486667 seconds, before the required fragment ends at 16 seconds. This equals the 10.986667-second audio warm-up origin plus the old 4.5-second output budget. The four early cases remain green.

Artifact `11802219775` verifies 126 receipts and 14 checksum records. ZIP SHA-256: `3318c18fed80bf5a5c535dd0ed8d7f389f81abfb97c3b763343434b9380b1fdf`. Private job log SHA-256: `ed413407aeec0349f3e23ba614583c75e509397403df0627ea4c043ec3a44359`. The observed truncated packet remains a rejection; short-packet authority and every media comparison remain unchanged.

The candidate now measures the output budget from the audio warm-up origin. For this fixture, that allows 7.513333 seconds. Both completion admission and restart construction retain the 12-second limit, using the same warm-up calculation. The video encoder can process up to about three more seconds inside this limit; only the requested missing fragment can be published. The existing public journey now also retains the duration and requires enough budget for discarded warm-up plus the full two-second requested fragment. The real decoded clocks, AAC packet sequence, PCM length, waveform joins, retained files, and negative controls remain the acceptance criteria.

Canonical metadata at `cb328995` already matches the committed snapshots: artifact `11802423760`, ZIP SHA-256 `9b8f132dd394c94d35b9898e1285bea44f668b41b6c6e11c9bbce52b9501273e`, metadata SHA-256 `7c9f44bdbf143c18e106dd95277c2c46a0b780c659c9fede0ace2f44efc1cd88`. All three pins and exact source identity verify. The new duration helper still requires a fresh hosted projection.

### Restarted recovery preflight passes with the warm-up budget

The focused race preflight at `2328a9494d6a1faf1f17dfc44bcbe4e7c1cd9d73` passes the previously failing restarted seventh-fragment journey. Run [38382072123](https://github.com/Kinosail/kinosail/actions/runs/38382072123), artifact `11804715048`, contains 39 receipts and eight verified checksum entries. ZIP SHA-256: `97d6c8625017150e5398cc38f953b74a6dd25f72e929e1133ae594c46b38ea74`. This artifact contains one focused fixture, not the full recovery matrix.

Independent receipt inspection verifies matching decoded video clocks, exact AAC packet clocks and durations, matching PCM lengths, unchanged waveform thresholds, and silence/displacement negative controls. The requested refill uses two bounded inputs and enough output duration for warm-up plus the complete fragment, within 12 seconds. Retained neighboring files remain unchanged. Fragment zero stays absent during late recovery. All five damaged-proof variants reject with no extra producer or published target. Subsequent first-fragment recovery starts exactly one producer and preserves exact PCM, AAC, and decoded video clocks. The full race and coverage suite is running separately.

Architecture artifact `11803572324` verifies the exact same source revision, all three file pins, and unchanged-source flags. ZIP SHA-256: `3dfc0178517134209f1680880b6872e71a04845baed304f78a1e5360ddffe36e`. Metadata SHA-256: `f8c609b68fe2b48f28ed2fe0bfe754da12642f6bf948afc7647842832585f318`. Its canonical Player snapshot is included here. No local compiler, media, browser, simulator, or device runtime was used.

### Complete recovery matrix passes; coverage and Firefox remain red

Run [38382072123](https://github.com/Kinosail/kinosail/actions/runs/38382072123) passes every test but fails the unchanged 89% coverage gate at 88.8%. Public artifact `11804217239` independently verifies all five fixtures, nine rendition comparisons, four legacy rebuilds, five damaged-proof controls, and exact first-fragment recovery. It contains 163 receipts and 20 checksum entries. ZIP SHA-256: `6eba20d62215d8c795d9f7643b707de9ab6d155acf445f94d1ea292d8f053aed`. Private log SHA-256: `1ec884177523a202f43e39933ee472ba35ce8c576c9ae9ffc9dff946e0fb58e1`. Successful media receipts do not make the failed coverage gate pass.

Deep CI [38382608424](https://github.com/Kinosail/kinosail/actions/runs/38382608424) at `7fa9cf0f` retains an exact coverage profile. Artifact `11804039670`, ZIP SHA-256 `1bd4f6a894c5eaf0dcb7a48ffbb491085ff21e8b0648e0e44e6e4e6c7e409ebf`, reports 88.9%. Merging duplicate instrumented blocks gives 8,738 covered statements of 9,829. The staged-clock rejection diagnostics are unexercised in this passing media matrix.

Before extending the public journey, its missing failure model is explicit: a structurally valid, rehashed clock record may pass admission but disagree with real regenerated media. That request must reject after joining one producer. The missing target and fragment zero must stay absent. Retained source, initialization, playlists, neighbors, and the supplied proof must stay byte-identical. Owned staging must be removed. One correlated ERROR must identify the measured clock mismatch without raw paths or URLs. Restoring the valid proof must allow subsequent first-fragment recovery. This is a real HTTP and codec rejection journey, not a direct validator test.

The same deep run records 98 passing Subtitles Firefox cases and one flaky saved-session case. Artifact `11804377988` verifies 200 checksums. The failed first attempt reports navigation from `/login` interrupted by the expected same-origin `/account` navigation; its retry passes. Both app journeys now await navigation commit, then require the visible Account heading before checking the authenticated API and absence of a passkey prompt. Explicit step-up remains required. The receipt now identifies the actual browser project. Both Firefox tests pass discovery only; browser confirmation remains pending. [Playwright navigation documentation](https://playwright.dev/docs/navigations#navigation-events) distinguishes commitment from document loading. No product authentication behavior changed.

### Measured rejection control passes without changing production

At `657e2ca21a5361b987d5474b62ba58833f2d3ab1`, the focused race preflight passes the complete restarted journey plus the new staged-clock rejection. Artifact `11805765350` verifies 41 receipts and eight checksum entries. ZIP SHA-256: `fd045cf032476a6c1595820223d0b4ed152f69ce35ca287d2f0527acfb94f6b7`. The forged clock starts exactly one additional joined producer, returns 404, preserves retained bytes, leaves target and zero absent, and removes staging. Exactly one ERROR matches the response request ID and reports measured video-clock disagreement with matching initialization and audio clocks. The private-path and URL checks pass. Restored valid proof then permits first-fragment recovery with exact PCM and clocks. The full race suite is running; this focused result does not establish coverage acceptance.

Independent review prompted the exact response-ID correlation and the count of all ERROR records for that request. The revision changes only tests and evidence. Existing production media comparisons and coverage thresholds remain unchanged.

Fresh architecture artifact `11805441047` verifies all three pins and unchanged source at the same revision. ZIP SHA-256: `033597c2a11d62f226b7fcbbdb79059c973b5f4bb041eca41282430bb1f110a9`. Metadata SHA-256: `17a761177ddc55fe7b0f2ee32598f3cd104bf3a249129401a70352f603b102f8`. Canonical reproduction refreshes the Player snapshot included here.

Native artifact `11804164646` at `7fa9cf0f` verifies against GitHub's archive digest `3e8537242d733e5c3bfead36c269c230a9b1b4025dadd27c77cddf4277e78f0d`. Read-only xcresult summaries report iOS: 336 passed, one skipped, zero failed; tvOS: 320 passed, one skipped, zero failed. Summary SHA-256 values are `ed607cd7f8f816852b3a48b23a6fea3b327735f1e856f153081e394c70f1d19c` and `ecc19c6c8fe5dca0f8c04b48a796e8f035f9069ee7b0e0f9a9083972200aea78`. Private native log SHA-256: `2d925f742cb73aac17698bef78479e0a47c33ebde21f0ed3edfa35ea9ba8beca`. These are hosted simulator contracts. Physical-device and deployment boundaries remain open. No local native runtime or compilation ran.


### Isolate process-wide entropy failure controls

The full race run [38385075783](https://github.com/Kinosail/kinosail/actions/runs/38385075783) reports a race in `TestBookmarkEntropyFailureDoesNotSave`. Its assignment to `crypto/rand.Reader` overlaps an earlier HTTP/2 TLS connection cleanup. Private job log SHA-256: `34fc16c950b342a6d46fd44351ae25da43d887296bd600593d2cf0748a4a6f96`. This is a test isolation failure; the trace does not establish a product concurrency defect.

Before changing these controls, their failure model is explicit. Global entropy replacement must not affect unrelated requests or TLS cleanup. Each selected control must execute and pass, preserve race instrumentation, and retain its existing state and response assertions. The child must terminate within a fixed timeout. Coverage from isolated execution must remain in the parent profile. A successful child exit without the selected test is insufficient.

The three existing isolated HTTP controls cover unavailable cryptographic entropy that a populated browser cannot deterministically force. They verify no cast grant, no saved bookmark, and a usable fallback request ID. Running the same instrumented test binary in a fresh process confines their global replacement. Forwarding Go's test coverage directory retains child counters for the parent's final profile. Hosted race, coverage, and lint results remain required.

The deep profile at `657e2ca2` has 8,741 covered statements of 9,829 and still reports 88.9%. Its checksum is `629b14765a3ab0526103c3c405b06421eba6c512dfaaca1cfdff44e9d1714864`. The measured-mismatch control now covers its intended diagnostics. The malformed-packet rejection remains uncovered. The next control repeats the observed original defect using the test-owned encoder adapter: shorten only the recovery output to 4.5 seconds. It must generate real truncated AAC, reject without publication, join its producer, preserve retained bytes, remove staging, and emit one correlated safe packet-duration diagnostic. This protects rejection behavior if a codec returns incomplete media despite a successful exit. The valid first-fragment recovery must still pass afterward.


### Shortened AAC rejects; Player session selection is now explicit

The focused race preflight at `07db0b0798c3b88ae8e1a349156af0d50929f7d8` passes the real shortened-AAC control. Artifact `11807597988` verifies 44 receipts and eight checksums. ZIP SHA-256: `31c1f551ba5db013e2e51da49eaa15504c3c22ae292587dfaf882f0877e9d823`. The controlled encoder receives exactly one 4.5-second output budget. The actual short AAC packet produces one response-ID-correlated duration diagnosis. The request returns 404, retains presentation bytes, removes staging, and leaves both missing fragments absent. Restoring the adapter permits exact first-fragment video clocks, AAC clocks, and PCM. The full suite remains separate.

Architecture artifact `11807298010` verifies all three pins, exact source, and unchanged-source flags. ZIP SHA-256: `a586143ded73a1cd4df233f6864debadbf7e48bff01827314301fc32925b7431`. Metadata SHA-256: `56fd9cd0c404462409e9e4b7763c64f6052e627b5e990ccad3a05c7b606e375b`. Pure source reproduction refreshes the canonical Player snapshot included here. The earlier tooling failure at this revision reports that stale snapshot; its private log SHA-256 is `8ade546ba9dc6bf661a6a820dace2f4f33596dacb285ec9f7375efb8279fb6c5`.

Browser artifact inspection also finds an execution gap. The Player saved-session case is skipped in the general suite and absent from the populated runner's selected files. The runner now includes its existing test file and requires its exact title. Its existing result verifier rejects missing, skipped, failed, retried-failure, and duplicate required journeys. The three focused verifier contracts pass. Shell parsing, ShellCheck, source limits, whitespace, and discovery of the selected case in all three browser projects pass. Hosted execution is still required; no new browser test or product authentication change was added.

At `657e2ca2`, Player Firefox artifact `11806348473` verifies 573 checksums with no failures or flakes. Its seven reports contain 877 passing executions and 361 skips. Those skips require separate populated fixtures, isolated transport controls, or unsupported native UI; they are not passing evidence. ZIP SHA-256: `f2a31ccf558df26eb4324069ea17003bb67ef755f6a66005751506395ca757f7`. Subtitles WebKit artifact `11806990789` verifies 198 checksums, 99 passes, and zero skips, failures, or flakes. ZIP SHA-256: `a5eaa3736f49486454cbaf355016df90dfa9f8cee631888dc5a47392329387ec`.


### Complete race and coverage pass; native reader failure needs a precise stage

The entire HLS job [38388024799](https://github.com/Kinosail/kinosail/actions/runs/38388024799) succeeds at `07db0b07`, including integrated Player verification. Full artifact `11807995728` verifies all five fixtures, nine rendition comparisons, four legacy rebuilds, five damaged-proof controls, both staged-rejection controls, and exact first-fragment recovery. Its 168 receipts and 20 checksum entries verify. ZIP SHA-256: `339cadc96ba65556524ea689bb92bd711a217e1d378c3366cb96c24795e04534`. Private job log SHA-256: `74831d6f91a3957a39e854fce0a2f2ef248f6e8eef9db2ff4e3d6e03cc4fd825`. No test failure or race is reported.

The separate deep Player job also passes. Go's authoritative function summary reports 89.2%, above the unchanged 89% floor. Coverage artifact `11808355507` has archive SHA-256 `5774a96fb36c902a9d93e6697ac945b1f36f5ca3408804149254e4aadbb7e821`. The merged profile retains positive counters for cast entropy rejection, bookmark entropy rejection, and fallback request-ID generation. All three isolated child controls therefore retain their failure-path coverage. Private deep job log SHA-256: `e616e7a66a3b283fdb0942d175d2f8bab47cb3c66f9a1685f16240663ee60785`.

The earlier `657e2ca2` native job reports one failed parameterized iOS test: the pending-to-loaded reader journey times out for light mode at font sizes 20 and 32. Its generic wait message does not identify which operation stalled. The xcresult reports 335 passes, one failed case, and one skip. Artifact `11806284270` has ZIP SHA-256 `87a20243e66bc77b7f79f3cc4e44e01161cfc49a572c3bbdb89c5b7d121e9280`. Private log SHA-256: `adef86ed568dd5a57bff1cea1ebfccb670b4828734fb0d7789d671a0dbd975d3`.

The existing native diagnostic file stays in the simulator's temporary directory and is absent from the hosted artifact. Failure-only diagnostics now retain the fixed stage, light-mode flag, font size, and fixture request counts in the captured test log. No request bodies, tokens, paths, or URLs enter that record. The original exception, ten-second waits, and all geometry and terminal-state assertions remain unchanged. This is diagnostic work; no product reader defect or repair is established yet.

The `07db0b07` deep native job repeats the same two reader wait failures while tvOS passes its reported 316 Swift tests. Its private log SHA-256 is `4ccc602493f4c15a56d063c76da5bd5ecbb881174025a2d832802058ed32a5f3`. The failure remains open; a later pass will not replace investigation of these retained failures.


### Populated Player saved-session execution verified

Hosted deep run `38388921845`, Firefox job `115267783872`, passes at `c09c265954bb2db846f0f69a44d46a51e4d3dba4`.
Artifact `11809696098` includes the saved-session case in `settings-firefox/results-firefox.json`.
It executes once with status `passed` and retry zero. The general report still skips this fixture-dependent case.
The authenticated return receives `/me` status 200, starts no ordinary passkey login, and retains explicit step-up.
Seven reports contain 878 passing executions, 361 skips, zero failures, and zero flakes; these are not 878 distinct cases.
All 574 checksum entries, 573 manifest evidence hashes, and ten source-input hashes match.
Archive SHA-256: `07de5203c322b502f383910796167de90ab6486031f970ff00252d7b4df1a490`.
Session receipt SHA-256: `db1655631ec722b5275cc998dcbf95d950fea8c86607a8b5e684d4387d49d341`.
The private verification is `pr587-hosted/player-firefox-c09c2659.session-proof.json`.

The same revision's native job `115267783222` passes both iOS and tvOS contracts.
Its Swift Testing summaries report 335 and 316 tests respectively, without recorded issues.
These console counts exclude separately reported XCTest cases; they are not full xcresult totals.
Log SHA-256: `33164e3ae08362371057e77ca6c6c24a071d156e1ce108f4cc2e2142122078c1`.
This pass does not resolve the two preceding reader failures. Revision `8fa923df` retains the failing stage if it recurs.

### Retain reader screenshots and measurements in hosted native evidence

The reader journeys wrote their PNG and JSON evidence only to simulator temporary directories.
The hosted workflow retains `.xcresult`, so those files were missing from the downloadable result.
Existing writes now also use Swift Testing data attachments. This preserves pending, loaded, empty, and failed renders,
font geometry, localized footers, concurrent preference-save results, and failure-stage records.
The attachments contain only synthetic fixture content and measurements. Assertions and deadlines are unchanged.
[Apple's Swift Testing attachment documentation](https://developer.apple.com/documentation/testing/attachments)
describes saving data with `Attachment.record` and inspecting it through Xcode test reports.
Independent source review found no blocker. Source-file limits and whitespace pass locally.
Compilation and attachment retrieval require the next hosted native run; local runtime remains held.


### Bounded reader repetition and a Chromium trace synchronization failure

Native job `115274022816` passes iOS and tvOS at `8fa923df24067a02b528e87b6ae3f81977cf148d`.
The Swift summaries report 335 and 316 tests, without issues or reader failure-stage records.
Log SHA-256: `9b6e957f5b058cebd6052093556543aa5e4c9f307aa40f7be4f94c72b800de8f`.
Two failures followed by two passes establish intermittency; they do not identify its cause.
The manual `reader` probe selects the exact parameterized reader function and requests three fixed iterations.
It runs after the full native suite and keeps every failure. Missing, zero, incomplete, or wrong test counts fail discovery verification.
The four light/font argument pairs remain unchanged. Actual iteration counts and attachments require hosted inspection.
[Apple documents fixed Xcode test repetition](https://developer.apple.com/documentation/Xcode-Release-Notes/xcode-13-release-notes).
The probe selector and discovery contracts failed before implementation and now pass. All 19 workflow contracts pass.
The repository's pinned actionlint 1.17.0 fork, workflow validation, source-file caps, and whitespace checks pass.
The system actionlint rejects pre-existing native parallel steps; the repository fork supports those steps without normalization.

Player WebKit job `115267783757` also passes at `c09c2659`.
Artifact `11809396503` verifies 502 checksums, 501 evidence hashes, and ten source-input hashes.
Its populated settings report executes the saved-session case once with no retry; the authenticated return and explicit step-up assertions pass.
Archive SHA-256: `faa4652f80eb522d76def40de6d699a3cab0bf2566b6d2e6d17b5556b16ca243`.
Session receipt SHA-256: `cb0f760b427a79d899b5e0fc39bd9f8d56faf19af3335ebff1d785e9d2843717`.

Chromium job `115267783810` fails because the isolated playback-trace case flakes before populated settings execution.
The assertion expected capability, play, waiting, stalled, and playing. It instead observed session-start, capability, play, and playing.
The existing wait accepted any four events, then synchronously asserted five specific events.
It now polls for those five event names. The same-session and absent title/source/URL assertions remain unchanged.
This is an isolated browser fixture repair, not a populated-server or real-decoder claim.
The general report records 721 passing cases, 357 skips, and one flake that passed on retry. The gate correctly remains failed.
Artifact `11810471375` verifies 482 checksums; archive SHA-256 is `dcb1b905c94e21a6b30d95935555980d34130608abfda114c0c64c0f4fc28019`.
The Chromium saved-session case still needs execution. The repaired trace case needs hosted browser validation.

### Repeated reader probe identifies the failing operation

At `ea644e550f998ba7628999456e06f0832004ffb8`, hosted job `115282916048` passes compilation and both full native suites.
The subsequent fixed reader probe fails, preserving exit 65. Eleven parameter runs pass; light mode at font 32 fails on repetition three.
Its stage is `loaded-webview`: the original ten-second wait never observes a WebView with a URL and completed loading.
Metadata is released; the diagnostic reports no held reader or preference requests and one preference read.
This narrows the investigation but does not establish a product cause or repair.

Artifact `11812116542` retains the probe result bundle, log, command, and receipt.
Archive SHA-256: `5f1da7338553fefa4057b3d18c755437ef22ed260b33edecbb9ad37af3583a28`.
Probe log SHA-256: `2681aa86a0f7ba8dbd78893f84266c22cf1c02525baa492ecabc395fd64c0853`.
Read-only `xcresulttool` inspection verifies all four argument pairs with three recorded repetitions each.
It also confirms attached pending/loaded/empty/failed screenshots and geometry for the eleven successful runs.
The failed run retains pending screenshots and its operation diagnostic, but lacks the WebView state at timeout.
The next diagnostic captures that state and a synchronous window screenshot before teardown, without changing the predicate or deadline.
Its bounded fields contain window visibility, lifecycle state, WebView readiness and dimensions, and isolated fixture counters and event ages.

The probe also exposed a verifier defect: Swift emits one summary for all repetitions, while the previous verifier expected three summaries.
The existing discovery contract now reproduces that format and fails against the old verifier.
The repair reads structured xcresult test details and requires the exact selector, four distinct argument pairs, and three distinct passing repetitions per pair.
Missing, duplicate, skipped, failed, malformed, or unavailable results reject. A nonzero native process exit remains fatal.
All 19 focused workflow contracts pass, including these negative controls; pinned actionlint, workflow validation, source limits, and whitespace pass.
This isolated verifier check protects the evidence boundary; it is not native app execution.

Chromium artifact `11811690752` at `8fa923df` closes the saved-session execution gap across all three Player browser engines.
Its populated settings report executes the case once without retry and verifies authenticated return with explicit step-up.
All 538 checksum entries, 537 evidence hashes, and ten source-input hashes match.
Archive SHA-256: `ea422d6f40626e50ea17c9f42ddc719d336028f9928087bf433b86ecd2b7fc22`.
Session receipt SHA-256: `47cf84e23e9ee8fa43caa139890ebb39225ef423ce38d6de4fd8c7ec7c05c658`.
Local app runtime remains held. No deployment or physical-device result follows from these hosted checks.

### Browser trace passes; bounded macOS cleanup no longer rejects an exiting group

At `ea644e55`, Player Chromium job `115282916149` succeeds.
Its JSON report confirms one passing execution of `records one private trace across playback events`, without retry.
The populated saved-session case also passes once without retry; its authenticated return and explicit step-up assertions remain intact.
Artifact `11814415128` verifies 538 checksum entries, 537 evidence hashes, and ten source-input hashes.
Archive SHA-256: `1473900fb6daea64e589c7d2f0c897275055c17d3467820bac29e80e48ba3003`.
This closes the trace synchronization regression; the trace case remains an isolated browser fixture.

Independent reader-verifier review found that multiple console summaries ending with the expected count could still pass that part of admission.
Duplicate and mixed-summary controls failed before the repair. Revision `2b0c224f` requires exactly one aggregate summary, alongside all twelve structured passing outcomes.
All 19 workflow contracts pass. The real reader timeout remains open, with richer diagnostics running at `bc33897b`.

The existing Q09 owned-descendant control again reproduces the macOS cleanup error.
TERM succeeds, the immediate group probe returns `EPERM`, and the next signal reports `ESRCH`.
A process-table projection taken between those calls contains no group members.
Apple's [XNU process-group signal implementation](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_sig.c) can return `EPERM` when a retained group has no signalable members.
That behavior is consistent with the observed transition; permission denial alone does not prove the group is absent.
Reproduction log SHA-256: `16e68f8455a41e19f1c45da281da529cf7df6f16cbb4970ae40161dcc34231d3`.

The cleanup helper now retains its existing bounded polling after `PermissionError` and accepts absence only after `ProcessLookupError`.
Persistent denial or a still-present group returns false. The TERM/KILL sequence, ownership boundary, and deadlines remain unchanged.
The new kernel-result controls were written first and failed before the repair.
All ten Q09 contracts then pass, including the existing real Python descendant control and persistent-denial/live-group rejection.
Red log SHA-256: `4b4234c23c66633f2515bbeb852b08c9ef7c05dcc3b0ef0348af20e866cf6e23`.
Green log SHA-256: `31b2bfe589b18fc24d4c9e2a9238cc23789f14ea232b47dc9b07d6932031689c`.
These are Python runner contracts. No local Go process, browser, media decoder, simulator, or device ran.

### Native repetition passes; Firefox invalid-worker setup needs synchronization

At `bc33897b7c0553f5e1c7fa2b62791d11aa75a06c`, native job `115297806099` passes both complete suites and the reader probe.
Structured test details contain all four expected argument pairs, each with repetitions one, two, and three passing.
The receipt reports exit zero and verified discovery. Artifact `11812834824` retains the result bundle and probe details.
Archive SHA-256: `d10d7b1ae7addb714a776ee7ec607500c8f77d202762b64f9e61477b344fade3`.
The earlier reader timeout remains unresolved. A later pass is not a repair, and this run has no timeout-state capture.

Player Firefox job `115297806413` fails the existing wrong-worker negative control.
The injected controller initially uses the wrong worker. About 17 milliseconds later, the admission snapshot sees the expected active worker and controller.
The original document has one successful response, no page error, and no side effect. Recovery correctly accepts that observed state.
The trace therefore proves that the negative prerequisite did not survive until inspection; it does not establish an admission-policy defect.
Artifact `11814841529` verifies 132 checksum entries.
Archive SHA-256: `58ba3a86dafbdbd572a6e7b0d2f421dc08fcc14b026c4b160ef8c253a4d2a420`.

The existing wrong-worker and wrong-scope cases now await the public offline `source()` operation before replacing the worker.
That operation awaits the app's initial worker registration. Setup then requires an activated controller and registration with no installing or waiting worker.
The wrong-worker case requires both active and controller script URLs to identify the injected worker; the wrong-scope case verifies its scope.
A setup-complete flag is asserted outside the caught navigation result, so prerequisite failure cannot count as successful rejection.
The flag is retained in the existing receipt. Admission checks, navigation counts, and side-effect assertions are unchanged.
Independent source review approves the repair for hosted validation. No product code or extra test case is added.

The TypeScript source-policy check also identifies seven broad annotations in this campaign's watched-playback evidence code.
They now name the recorded native-event fields, queued callback functions, and native playback error objects.
The source-policy check passes. Three-engine discovery lists 141 cases across the changed browser files; discovery is not execution.
Source-file limits and whitespace checks pass. Hosted runtime validation remains required.

### WebKit Apple launch failure stalls during input dispatch

At the same revision, WebKit job `115297806494` reports 715 passes, 363 skips, and one flake.
The initial `ready Play opens Apple, dismissal pauses and next Play reenters` execution fails; its retry passes.
The gate remains failed. Artifact `11814764004` verifies 447 checksum entries.
Archive SHA-256: `9b4dd8fe9c938411ab804267283dfec2159fba988c843b52f6f20f713b064b37`.
Job log SHA-256: `8e6863084a38b227d4cfa3d4a81e7280adca12ef7e934c8825778aec2f2151da`.

The trace shows the final Play button is visible, enabled, stable, and reached by the tap action.
The interval from performing the tap to its completion takes about 29.3 seconds, before the navigation wait.
That interval includes browser input dispatch and hit-target interceptor cleanup; the trace does not distinguish which stalls.
The following fullscreen-property assertion ends almost immediately. The report gives a 60-second test limit and 33.54-second duration, so this is not an established test-deadline failure.
Its reported `undefined` value is not evidence that the application deleted the simulated property; the trace records no completed property observation.
The cause of the input stall is not established. Keep the original gesture, deadline, and fullscreen assertions for the next hosted run.
The snapshots show Settings already hidden and the Play control visible; they do not support adding a Settings-close readiness workaround.
This case simulates Apple fullscreen and media state; it provides no physical iPhone or real-decoder result.
