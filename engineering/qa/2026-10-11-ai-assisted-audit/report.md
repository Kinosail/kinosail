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
Run: [38158260089](https://github.com/Kinosail/kinosail/actions/runs/38158260089).

### AQA-008 — Open: intermittent executable-fixture launch failure

The main-push shared-package job failed `TestFingerprintCacheAndToolFailuresAreDeterministic` before its first fingerprint assertion.
Linux returned `text file busy` while launching its newly written, test-owned shell executable.
The required PR checks had passed on the reviewed source. No marker production source changed in this delivery.
The failure is retained as an environment or fixture-lifecycle finding pending reproduction; it is not attributed to localization.
Run: [38156244208](https://github.com/Kinosail/kinosail/actions/runs/38156244208), job `114519746603`.
A retry passed without a source change. Main CI and both production-image promotions subsequently completed successfully.
That establishes publication of `8e937334fbbc0f79915f6f093d8aed268481de64`, not a repair of the intermittent fixture failure or proof of deployment.

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
Hosted Linux navigation checks and the populated container command remain separate delivery boundaries.
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
The validator still lists five isolated cases while the selected test files now declare additional cases.
This remains a proof-tooling finding pending exact collection reconciliation, not passing download E2E evidence.
[Hosted run 38159857532](https://github.com/Kinosail/kinosail/actions/runs/38159857532).
The archive is `q09-download.zip`, SHA-256 `788808aa9b969a61fea2772d7f0703340466cdea06004c6d863580db70bcd3ac`.

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

## Remaining coverage

The campaign remains active.
Retain the earlier audit's unverified journeys until a new passing execution establishes their behavior.
Inspect the deep-run artifacts, repair confirmed failures, and record the final revision and each remaining platform boundary.
Physical devices, real media/receivers, production deployment, store distribution, and unsupported external environments are not certified by these local runs.
