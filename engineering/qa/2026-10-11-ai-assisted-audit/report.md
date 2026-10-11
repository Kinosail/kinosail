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
- [Hosted deep CI 38151255345](https://github.com/Kinosail/kinosail/actions/runs/38151255345) is an independent current-main run. The current artifacts confirm all three Subtitles engines passed 99 tests each without skips or retries. Player Firefox passed 857 executed cases and WebKit passed 855 across the main and focused reports. Their main reports also list 355 and 357 skipped cases; those do not count as coverage. The iOS contract job failed one image-encoding check; tvOS passed 300 tests with one skip. Remaining terminal status needs inspection.

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
Its outer receipt is `startup-localization/manifest.json` under the private campaign directory.

### Additional layout and native evidence

The existing `TestWriteShowActionFixtures` exporter and `show-action-layout.spec.ts` passed 12 cases across Chromium and WebKit.
They cover six widths from 320 to 1920 pixels and long, unbroken, watched, and empty states.
Keyboard focus and accessibility assertions passed. The 320-pixel WebKit and 1920-pixel Chromium screenshots were inspected.
These are rendered-fixture checks, not populated-server E2E evidence. Firefox remains a local launch boundary.

The hosted iOS failure is `MediaCardRatingTests/testVideoCardsRenderIdenticallyWithAndWithoutRatings`: rated-image PNG encoding returned nil.
The local focused retry on the reusable Kinosail iPhone failed before tests because simulator boot exceeded 60 seconds.
That infrastructure failure neither reproduces nor resolves the hosted assertion. No native code was changed.

## Remaining coverage

The campaign remains active.
Retain the earlier audit's unverified journeys until a new passing execution establishes their behavior.
Inspect the deep-run artifacts, repair confirmed failures, and record the final revision and each remaining platform boundary.
Physical devices, real media/receivers, production deployment, store distribution, and unsupported external environments are not certified by these local runs.
