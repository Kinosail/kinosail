# Kinosail CI/CD decisions — 2026-09-20

Status: implementation under validation. Hosted results and publication evidence must be appended before calling this delivered.

## Objective and invariants

Minimize time from a change to trustworthy feedback and deployable containers. The user explicitly authorized redesigning all checks except the 300-line source cap, requested continuous shipping rather than major releases, and requested current web, industry, and academic evidence.

Preserve the five protected check names, independent app images, full tests within each selected app, race detection, meaningful coverage floors, vulnerability rejection, and main-only publication. A failed detector, missing job, cancelled job, or unexpected skip must fail the required check. A green check certifies only its stated scope; it does not certify physical Apple devices or a deployed production host.

## Observed baseline

Raw evidence: [ci-baseline-2026-09-20.json](ci-baseline-2026-09-20.json), captured at 2026-09-20 23:48 UTC from GitHub's run/job API. The sample is the five workflows for commit `375ce3b1e97e37ea9c88cbad262f2644eb4b4582`, an existing CI-repair branch. It is useful for identifying repeated work and long jobs, not a controlled before/after benchmark of application behavior.

| Workflow | Observed state | Listed jobs, including skipped | Sum of job elapsed time |
| --- | --- | ---: | ---: |
| Player Hygiene | Failed | 9 | 23.7 minutes |
| Subtitles Hygiene | Failed | 8 | 24.6 minutes |
| Dashboard Hygiene | Passed | 7 | 13.2 minutes |
| Repository Quality | Still running | 11 | At least 113.7 minutes |
| Security | Passed | 6 | 7.1 minutes |

Elapsed job time is not billed cost; runner sizes, OS multipliers, setup, scheduling, and unfinished jobs prevent that interpretation. The repository required-check job had not yet started. The sample is unhealthy and must not be described as a successful baseline.

Specific observations:

- Player's standalone race suite took 322 seconds. Its static job took 190 seconds and included another coverage run. Repository Quality ran Player coverage again for 176 seconds.
- Static quality scanned every app again and took 305 seconds. Every app also linted its own code and revalidated repository workflows.
- Mutation was on the protected merge path for every module, with a 360-minute timeout. At capture, package and Dashboard mutation jobs had still not finished after approximately 38 and 35 minutes, respectively.
- A normal application change scheduled Swift compilation, both container architectures, and every app's full browser matrix regardless of which app changed.
- Releases required version tags. Main pushes did not themselves create deployable published containers.

Reproduce the raw evidence with `gh run view <databaseId> --json databaseId,workflowName,headSha,status,conclusion,createdAt,updatedAt,jobs`. Run IDs and URLs are retained in the JSON. Do not compare a warm-cache successful sample with this mixed, partially failed sample and call the difference a measured speedup.

## Decisions and supporting evidence

### 1. Select whole apps conservatively, not individual tests probabilistically

The selector reads the complete NUL-delimited Git diff, includes both sides of renames, and has no GitHub 300-file API/path-filter truncation. PRs use the merge base; main pushes use the complete before/after range; merge groups are supported. Missing Git objects and invalid events fail. Unknown paths and workspace configuration changes select everything. Shared packages select all three consumers. Changing an app `go.mod` selects all modules because workspace dependency resolution is shared.

Prose-only exemptions are explicit. Shipped `LICENSING.md`, notices, and embedded Markdown are not exempt. Native-only changes select native checks rather than server containers. Application Go changes still run the app's entire suite; this is not package-pair or changed-test-only selection.

Academic evidence: [Ekstazi, ICSE 2015](https://users.ece.utexas.edu/~gligoric/papers/GligoricETAL15EkstaziTool.pdf) tracked actual file dependencies and measured an average 32% end-to-end reduction across its evaluated JVM projects. That result supports dependency awareness, not transferring the percentage to Go or selecting tests by filename similarity.

Recent evidence: [Targeted Test Selection, ICSME 2025](https://arxiv.org/abs/2509.10279) reports selecting 15% of tests and detecting over 95% of failures in its industrial evaluation. It demonstrates a speed/recall tradeoff, not proof that an arbitrary selected subset is safe. [Pipeline-Aware Regression Test Optimization, ICST 2025](https://arxiv.org/abs/2501.11550) distinguishes pre-submit failure detection from post-submit transition detection and discusses feature-collection costs.

Kinosail decision: deterministic coarse selection offers most of the immediate benefit with a small, auditable dependency contract. Do not introduce ML selection, dynamic per-test coverage infrastructure, Bazel, or a remote execution service without local measurements showing the remaining bottleneck warrants their maintenance cost.

### 2. Keep stable gates and prove every skip

All required workflows still start on every PR. Work is skipped at job level from one shared selection implementation. Required aggregators check an exact job inventory: selected jobs must succeed; unselected jobs must be skipped. Detection must itself succeed. A missing or malformed plan cannot pass.

GitHub documents that workflow-level path filters can leave required checks pending, and that diff-based filters have limits. See [workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax) and [job dependencies](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-jobs).

Tradeoff: five short detector jobs preserve existing workflow/check identities and avoid changing branch protection in the same rollout. Consolidate orchestration only if measured scheduling overhead becomes material.

### 3. Execute tests once per selected module, with race and coverage together

`test-go.sh` runs the complete module with `-race -count=1 -covermode=atomic -coverprofile=...`, then inspects that same profile. This removes two repeated non-race app test runs from the normal merge path. Test-support packages are excluded only from standalone coverage accounting; their tests still execute and their consumers still run.

Coverage policy: Player/Subtitles retain 89%; Dashboard retains 100%; shared packages use the established `packages/Makefile` floor of 85%, replacing the conflicting repository-wide 100% policy. This is a deliberate policy reconciliation, not evidence that 85% proves correctness. Full observable-behavior tests, race checks, security analysis, and runtime tests remain independent gates. The 300-line cap and its existing frozen debt policy are unchanged.

### 4. Match runtime breadth to risk; move diagnostics off the merge path

Every selected server app gets an AMD64 production-container test and Chromium browser checks. Server UI, templates, assets, E2E, and shared web surfaces select Firefox/WebKit too. Build scripts, containers, and dependency/build inputs select ARM64 checks. Scheduled/manual hygiene runs select everything. Publication builds and runtime-tests both production architectures regardless of the PR selection.

Mutation and broad complexity/duplication/Halstead/dead-code metrics are scheduled/manual diagnostics, with bounded job timeouts. They remain executable and their failures remain visible. They are not allowed to occupy the ordinary protected merge path indefinitely. Fuzz/performance checks remain in scheduled app hygiene.

[Practical Mutation Testing at Scale, IEEE TSE 2021](https://research.google/pubs/practical-mutation-testing-at-scale-a-view-from-google/) advocates incremental changed-code mutation, filtering low-value mutants, and limiting mutation volume. [Long Term Effects of Mutation Testing, ICSE 2021](https://research.google/pubs/long-term-effects-of-mutation-testing/) provides evidence that mutation can identify useful test gaps. These support retaining mutation as a diagnostic investment; they do not support demanding exhaustive whole-repository 100% mutation success on every PR.

Reconsider a bounded, changed-line mutation PR lane after measuring useful findings, false positives/equivalent mutants, and p95 runtime on this repository. Do not reinstate full-repository mutation as a default merge prerequisite without those measurements.

### 5. Keep distinct security checks, avoid repeated scope

Every change verifies the reviewed browser dependency hashes without installing Go modules, and gets a redacted secret scan over all introduced commits, including deleted secrets; scheduled runs audit history. CodeQL runs only affected languages, with manual Go extraction using the repository's Go version. Dependency/configuration changes get Trivy and PR dependency review. Selected Go modules get `govulncheck`; lint and SAST detect different classes of problems.

Immutable action pins, read-only PR jobs, enabled-gate assertions, and exact-main publication validation remain. Changing the workflow policy cannot silently disable its own required aggregator.

### 6. Continuously publish deployable containers from green main

Repository Quality hands its validated plan to reusable delivery only for a protected main push and only after its required check passes. Delivery verifies the latest push run of all four sibling workflows for the exact commit. Missing, failed, cancelled, stale, or PR runs cannot authorize publication.

Each selected app builds on native AMD64/ARM64 runners with shared BuildKit caches. The production candidate is tested and scanned by digest, then the two architectures are assembled, signed, and attested. Promotion reuses the assembled digest; it does not rebuild the artifact. Every completed publication gets `sha-<full-commit>`; digests are the immutable deployment/rollback identity. `main` and `latest` are moving production tags.

Production-tag changes are serialized per app with `queue: max` and `cancel-in-progress: false`. Before advancing them, compare intervening main changes: an older run cannot supersede newer relevant code, while later documentation or another independent app's changes do not strand this app. Commit-image publication happens before the serialized step. [GitHub documents](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency) that the default single pending slot is replaced by arrival order, not commit order; a slower old build can otherwise cancel a newer promotion. `queue: max` retains up to 100 pending jobs; overflow remains an explicit failed/cancelled delivery boundary.

Released actionlint 1.7.12 does not yet recognize `queue`; [upstream support PR #654](https://github.com/rhysd/actionlint/pull/654) was still unmerged at inspection. `.github/actionlint.yaml` allows only that exact unknown-property diagnostic in `delivery.yml`; the always-required runtime contract test independently asserts the exact supported concurrency block, including `queue: max` and cancellation disabled. Other actionlint errors still fail. Remove this narrow schema compatibility exception when upstream support ships.

No version tag or GitHub release is required. Existing optional tagged installer/native-release workflows remain available to avoid breaking consumers, but they publish version tags only and cannot overwrite `main` or `latest`. The local Nox watcher remains separate; publishing a container does not prove the live host deployed it or passed TLS/device checks.

Evidence: [DORA continuous delivery](https://dora.dev/capabilities/continuous-delivery/) supports small, automated, continuously deployable changes; its research identifies delivery speed and reliability together, rather than treating release frequency alone as success. [Docker's multi-platform guidance](https://docs.docker.com/build/ci/github-actions/multi-platform/) supports distributing architectures across native runners. [SLSA 1.2](https://slsa.dev/spec/v1.2/) and [GitHub attestations](https://docs.github.com/en/actions/concepts/security/artifact-attestations) support verifying artifact provenance. This implementation does not claim a formally audited SLSA level.

### 7. Preserve the consumer trust contract

Continuous-image installers verify exactly `https://github.com/Kinosail/kinosail/.github/workflows/delivery.yml@refs/heads/main`, with the GitHub OIDC issuer. [Fulcio's OIDC specification](https://github.com/sigstore/fulcio/blob/main/docs/oidc.md) binds reusable workflow identities to `job_workflow_ref`. Delivery verifies that same identity immediately after signing, before publishing commit or production tags. Explicit version installs retain their scoped version-workflow identity.

Migration: already downloaded installer scripts/archives must be refreshed before updating to the continuous `latest` channel; source changes cannot update existing copies. Commit tags and digests can be deployed directly through the container engine; the installer version selector still accepts `latest` or semantic versions, not commit tags. No new tagged installer release is claimed here.

Continuous images embed `sha-<40 lowercase hex>` and report `container-managed` through the update API. They do not query or install versioned GitHub releases, and do not falsely claim to be current. Deploy the next signed image to update them. Native release checking remains version-based. Native-only changes currently compile the client; distributing signed Apple applications still requires the native distribution process.

## Evaluation and future changes

Collect at least 20 successful runs per scenario before claiming steady-state latency: prose-only, one server app, UI, native, shared packages, dependencies, and CI policy. Separate cold and warm caches, queue delay, job execution, PR feedback, post-merge publication, and deployment health. Report p50/p95 wall time, runner minutes by OS/architecture, failure/retry rate, and escaped regressions. A faster failed run is not a delivery improvement.

Initial acceptance targets, not measured claims: prose-only feedback under 2 minutes; ordinary warm-cache app feedback under 10 minutes; ready-to-pull containers within 15 minutes of a green main run. Revise targets from data instead of weakening behavior checks to meet a stopwatch.

Revisit selection when app imports/build inputs change or a scheduled run catches a missed regression. Add the missed dependency and a selector regression first. Revisit browser breadth if escaped bugs cluster in other engines. Revisit caching/sharding only after profiling shows they shorten the critical path rather than increase aggregate setup time. Treat flaky tests as defects; do not make retries or quarantines a silent green path.

## Verification record

- Focused selection/aggregation/publication tests cover malformed and oversized inputs, missing objects, renames/deletions, more than 300 changed files, unusual filenames, merge-base behavior, missing/cancelled/skipped jobs, exact-commit authorization, and stale promotion.
- Local workflow validation and actionlint are required before publication. All new source and test files remain below 300 lines.
- Hosted CI results, observed timing, merge ancestry, published digests, and any unresolved deployment boundary will be recorded after the PR run. No speedup or live production deployment is claimed from local tests alone.


### First hosted rollout (before the follow-up fixes)

[Raw rollout evidence](ci-rollout-first-2026-09-20.json) records the head SHA, run/job IDs, URLs, timestamps, and conclusions. This sample is **failed**, not an accepted performance result. The source was main `ae618325cb4d16351cbf01890106ed2dbdb42894` plus the CI change at `146c55d4`.

- [Security](https://github.com/Kinosail/kinosail/actions/runs/35546041347) passed, including Go/JavaScript/Python CodeQL, findings policy, secret scanning, and supply-chain checks. Repository policy/CI contracts and Swift compilation also passed.
- [Player](https://github.com/Kinosail/kinosail/actions/runs/35546041335) had 16 failing top-level Go tests; [Subtitles](https://github.com/Kinosail/kinosail/actions/runs/35546041327) had 49. These application source/tests were unchanged from the base, including update-adapter, authentication, API, playback, and presentation contracts. The combined race jobs took 7m08s and 8m15s respectively, including setup; these are failed-run timings.
- [Dashboard](https://github.com/Kinosail/kinosail/actions/runs/35546041326) passed its Go tests but failed the unchanged 100% coverage requirement at 99.2%. Coverage floors were not lowered to hide this result.
- [Shared packages](https://github.com/Kinosail/kinosail/actions/runs/35546041328/job/106172017099) failed existing identity/session, safe-return, and WebSocket tests. Their failure paths remain on the merge gate.
- The first run also exposed stale or incorrect harness contracts: CSS parsed as JavaScript, partial JavaScript scopes, gateway service count, installer signing expectations, Docker restart port reuse, and a five-second MCP startup assumption. The follow-up repairs these contracts while retaining their observable assertions. MCP startup is now bounded at 60 seconds and captures relay stderr; success still requires twelve single-thread relay processes.
- Local full tooling validation remains blocked by stale generated architecture-explorer links (`apps/player/internal/server/home_assistant_http.go` no longer exists). The separate remote-setup fixture also fails locally. These are recorded rather than silently skipped.

Independent review caught four missing safety details and two delivery/consumer edge cases: invalid Chromium-only configuration could skip all browsers; Dashboard swallowed project arguments; old version workflows could overwrite latest; app-local vendor edits could miss integrity validation; pending promotions could be cancelled in arrival order; and commit versions could falsely report current. Each now has a focused regression or workflow contract check. Hosted publication, OCI signatures, promotion queue behavior, live deployment, and physical devices remain unverified until protected main can pass.


### Follow-up local verification

- Passed: 33 CI selection/aggregation/delivery/runtime-contract tests; 5 dependency-integrity tests; 12 browser-lint harness tests; browser lint (zero errors, two pre-existing unused-variable warnings); full `updatecontrol` race suite; changed `updatecontrol` lint; repository shellcheck; actionlint with the documented queue compatibility assertion; workflow boundary validation; `make max-loc`; and `git diff --check`.
- Player and Subtitles single-container boundary checks and direct `test-installer.sh` tests passed. The enclosing `make installer-test` still fails in the unchanged `test-remote-setup.sh` fixture.
- The required Player/Subtitles `make verify-changed` commands were attempted with working-tree changes included. Shell/installer checks passed; full shared-package lint blocked them with 116 existing findings. The changed update checker itself reports zero lint issues. Dashboard's initial local `verify-changed` ran its Go tests and exposed the shared shell-source lookup issue, now repaired; the hosted 100% coverage gate remains red.
- An independent read-only review verified the six safety/compatibility fixes, reran CI/integrity tests and actionlint, and found no further concrete blocker in those deltas. This does not substitute for successful app suites or hosted delivery.

PR: [#41](https://github.com/Kinosail/kinosail/pull/41). Keep branch protection and publication prerequisites enabled. Do not merge or claim production delivery while any required check fails. Retain the task branch/worktree if these unrelated repair blockers prevent safe integration; the primary working tree contains separate unfinished work and must not be overwritten.


### Dependency integration evidence and browser corrections

[Integration PR #42](https://github.com/Kinosail/kinosail/pull/42) preserves all eight pending PR heads (#33–#40) and records the final required-check, merge, and container-publication evidence. The following is a **failed integration sample**, not an accepted performance benchmark: head `8ea3e321173715440f22455e193f0301367a081f`, captured September 21 UTC. Timings include scheduling/setup and use the final job completion; summed job time is not billed cost.

| Workflow | Result | End-to-end | Sum of job elapsed time |
| --- | --- | ---: | ---: |
| [Repository Quality](https://github.com/Kinosail/kinosail/actions/runs/35552004730) | Passed | 5m28s | 6m03s |
| [Security](https://github.com/Kinosail/kinosail/actions/runs/35552004742) | Passed | 4m16s | 6m54s |
| [Dashboard](https://github.com/Kinosail/kinosail/actions/runs/35552004731) | Passed | 7m26s | 13m20s |
| [Player](https://github.com/Kinosail/kinosail/actions/runs/35552004744) | Passed with one browser retry | 12m08s | 30m30s |
| [Subtitles](https://github.com/Kinosail/kinosail/actions/runs/35552004729) | Failed | 10m51s | 26m13s |
| [Documentation](https://github.com/Kinosail/kinosail/actions/runs/35552004636) | Passed | 28s | 25s |

Subtitles completed 37 browser cases before Linux WebKit rejected a full-page screenshot above its 32,767-pixel limit; serial-suite behavior left ten subsequent cases unrun. The provider test now captures the two relevant sections while retaining its interaction, page-overflow, responsive geometry, and accessibility assertions. This avoids spending rendering time on unrelated settings and preserves useful visual evidence.

An earlier run exposed simultaneous preferred-language edits by three browser projects against one Subtitles Server. One worker now serializes its 48 scenarios. Restore parallelism only with independent Server state per project; retries cannot repair shared mutable fixtures. Player already resets its Server between engines and retains three workers within each engine.

The offline upgrade fixture now uses the complete downloads bundle, version-distinct worker bytes, and an explicit no-script-errors assertion. Eighteen repeated cases passed using CI's browser channel across Chromium, Firefox, and WebKit. Local installed-Chrome stress testing still recorded one activation stall in 30 cases, so that sample is not clean and does not establish a production lifecycle defect. Failed Player hosted browser jobs now retain traces/screenshots for seven days; successful jobs do not upload them. Diagnose future intermittent failures from those traces rather than silently accepting retries.

The stable gates, full affected-module race/coverage suites, vulnerability checks, and 300-line source cap remained enabled throughout this integration. The final accepted run and published image digests belong to the PR's delivery record; this table must remain labelled failed rather than being rewritten as a successful speedup.

Player's accepted job in this failed sample included one WebKit retry: a 2.2-second idle timer plus a 180 ms CSS transition exceeded the test's 3-second wall-clock assertion under hosted load. The test now advances Playwright's [controlled clock](https://playwright.dev/docs/clock) past the idle deadline, then checks the real computed opacity; keyboard focus must keep controls visible beyond that deadline. This retains the behavior assertion while removing several seconds of waiting per browser. The sample's retry must remain visible in any reliability comparison.

The next measured optimization target is Player browser execution: Chromium, Firefox, and WebKit took approximately 1.9, 2.7, and 3.2 minutes in this sample. Independent per-engine Server fixtures could shorten that critical path, but compare added image-transfer/startup cost and retry rate before splitting jobs. Subtitles screenshot work is another measured source of wasted execution; retain behavior assertions while bounding captured evidence.

The next [Player run](https://github.com/Kinosail/kinosail/actions/runs/35552926805) demonstrated the value of retaining failure traces: all 47 recorded supporter requests returned 200, but an outgoing document started its second request during navigation and WebKit reported an access-control error before that request reached the network. Supporter recognition now aborts on `pagehide`, checks cancellation after both JSON reads, and starts a fresh operation after back-forward cache restoration. Twelve lifecycle contracts passed across three engines, and the complete WebKit Owner journey passed against the rebuilt container. Authentication, CORS, and browser-error assertions remain unchanged.

The corresponding [Subtitles run](https://github.com/Kinosail/kinosail/actions/runs/35552926804) exposed a duplicate Settings link and an assertion racing Library navigation. Tests now select the header link and expand a specific Library row after the destination heading appears. All 48 cases passed locally across three engines in 3m30s. Subtitles now retains failed browser evidence for seven days too. These local repairs await a clean hosted integration run; neither failed sample establishes the final merge latency.

Head `7fd37c42` passed the hosted Subtitles browser matrix, but its [race suite](https://github.com/Kinosail/kinosail/actions/runs/35554293171) failed temporary-directory cleanup in a status-only maintenance test. Both apps now use the existing unscheduled Server mode for that test; status/manual-action assertions and separate automatic scheduling tests remain. Cancellation alone does not join background writes, and the backup scheduler starts after 250 ms even with a one-hour interval, overlapping the failing 0.38-second test. The cleanup log does not identify the exact writer.

The same head's [Player trace](https://github.com/Kinosail/kinosail/actions/runs/35554293231) reached its final offline navigation, then exhausted the entire 60-second journey budget after only 76 ms in that navigation. One accessibility scan alone took 10.3 seconds. Only this onboarding/playback/accessibility/offline journey receives a 90-second total budget; individual assertions retain their existing limits. It now has zero automatic retries because Owner/MFA state persists and replaying setup against that state is not an equivalent attempt. This avoids the observed invalid retry without hiding a failure or adding waiting to successful runs.

Head `3ee5cdcf` passed four required gates and all 203 Chromium cases. Its [Firefox trace](https://github.com/Kinosail/kinosail/actions/runs/35555006010) showed the test starting a new navigation 8.4 ms after Cancel reached Home, interrupting the outgoing theme and navigation scripts with `NS_BINDING_ABORTED`; both assets returned 200 on the replacement page. The journey now waits for Home's load and checks its heading before continuing. This verifies the Cancel destination and avoids test-induced cancellation; the service-worker strategy and strict browser-error assertion stay unchanged.
