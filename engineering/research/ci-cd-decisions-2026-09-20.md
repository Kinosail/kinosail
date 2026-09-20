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

Every change gets a redacted secret scan over all introduced commits, including deleted secrets; scheduled runs audit history. CodeQL runs only affected languages, with manual Go extraction using the repository's Go version. Dependency/configuration changes get Trivy and PR dependency review. Selected Go modules get `govulncheck`; lint and SAST detect different classes of problems.

Immutable action pins, read-only PR jobs, enabled-gate assertions, and exact-main publication validation remain. Changing the workflow policy cannot silently disable its own required aggregator.

### 6. Continuously publish deployable containers from green main

Repository Quality hands its validated plan to reusable delivery only for a protected main push and only after its required check passes. Delivery verifies the latest push run of all four sibling workflows for the exact commit. Missing, failed, cancelled, stale, or PR runs cannot authorize publication.

Each selected app builds on native AMD64/ARM64 runners with shared BuildKit caches. The production candidate is tested and scanned by digest, then the two architectures are assembled, signed, and attested. Promotion reuses the assembled digest; it does not rebuild the artifact. Every completed publication gets `sha-<full-commit>`; digests are the immutable deployment/rollback identity. `main` and `latest` are moving production tags.

Production-tag changes are serialized per app. Before advancing them, compare intervening main changes: an older run cannot supersede newer relevant code, while later documentation or another independent app's changes do not strand this app. Commit-image publication happens before this serialized step, so replacing an obsolete pending promotion does not discard its deployable artifact. GitHub's default concurrency queue retains only one pending job; its newer `queue: max` feature is currently rejected by the latest released actionlint (1.7.12), so the implementation uses this compatible separation rather than suppressing workflow validation.

No version tag or GitHub release is required. Existing optional tagged installer/native-release workflows remain available to avoid breaking consumers, but they are not the normal container delivery path. The local Nox watcher remains separate; publishing a container does not prove the live host deployed it or passed TLS/device checks.

Evidence: [DORA continuous delivery](https://dora.dev/capabilities/continuous-delivery/) supports small, automated, continuously deployable changes; its research identifies delivery speed and reliability together, rather than treating release frequency alone as success. [Docker's multi-platform guidance](https://docs.docker.com/build/ci/github-actions/multi-platform/) supports distributing architectures across native runners. [SLSA 1.2](https://slsa.dev/spec/v1.2/) and [GitHub attestations](https://docs.github.com/en/actions/concepts/security/artifact-attestations) support verifying artifact provenance. This implementation does not claim a formally audited SLSA level.

## Evaluation and future changes

Collect at least 20 successful runs per scenario before claiming steady-state latency: prose-only, one server app, UI, native, shared packages, dependencies, and CI policy. Separate cold and warm caches, queue delay, job execution, PR feedback, post-merge publication, and deployment health. Report p50/p95 wall time, runner minutes by OS/architecture, failure/retry rate, and escaped regressions. A faster failed run is not a delivery improvement.

Initial acceptance targets, not measured claims: prose-only feedback under 2 minutes; ordinary warm-cache app feedback under 10 minutes; ready-to-pull containers within 15 minutes of a green main run. Revise targets from data instead of weakening behavior checks to meet a stopwatch.

Revisit selection when app imports/build inputs change or a scheduled run catches a missed regression. Add the missed dependency and a selector regression first. Revisit browser breadth if escaped bugs cluster in other engines. Revisit caching/sharding only after profiling shows they shorten the critical path rather than increase aggregate setup time. Treat flaky tests as defects; do not make retries or quarantines a silent green path.

## Verification record

- Focused selection/aggregation/publication tests cover malformed and oversized inputs, missing objects, renames/deletions, more than 300 changed files, unusual filenames, merge-base behavior, missing/cancelled/skipped jobs, exact-commit authorization, and stale promotion.
- Local workflow validation and actionlint are required before publication. All new source and test files remain below 300 lines.
- Hosted CI results, observed timing, merge ancestry, published digests, and any unresolved deployment boundary will be recorded after the PR run. No speedup or live production deployment is claimed from local tests alone.
