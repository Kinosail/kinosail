# Kinosail monorepo agent policy

## GitHub Actions verification

- Required quality and security gates run on GitHub-hosted runners. Select affected apps using `scripts/ci/affected.py`; unknown inputs select every consumer. PRs and main pushes require quick Go tests and populated Chromium smoke journeys. Weekly and manual deep runs add race/coverage and complete cross-browser suites. Do not add `.gates-disabled` or bypass required checks to obtain a passing run.
- Local hooks enforce the source-file cap and the current worktree lease; GitHub Actions runs complete selected suites. Run focused checks locally when changing their implementation.

## Scope and ownership

- This repository is authoritative for Player, Subtitles, Dashboard, and Player native clients.
- Work here or in its Git worktrees. Keep Supporter and Home Assistant in their own repositories.
- Preserve unrelated work. Use a dedicated branch and worktree for concurrent implementation tasks.
- Keep at most one worktree for each active task. Do not let multiple agents change the same checkout.
- Checkpoint coherent work with regular commits. Integrate through a pull request before updating local `main`.
- After the remote merge, fast-forward local `main` and remove the task worktree when both checkouts are clean and the task commit is in remote main. Preserve and report unrelated dirty work that prevents local cleanup.
- Never leave dirty or unique task-owned work in an inactive worktree. Report unrelated inactive checkouts without changing their files or leases.
- Start secondary worktrees with `make worktree-lease TASK=<thread-or-task-id>`; commit and pre-push hooks renew the current lease. Run `make worktree-audit` separately to find inactive checkouts; their state must not block another task's commit.
- Use `make worktree-cleanup` only for expired clean merged checkouts. It must preserve dirty or unique work for explicit recovery.
- When local `main` is clean and current with remote main, use `make agent-finish TASK=<thread-or-task-id>` to lock, verify, fast-forward local `main`, and remove the task checkout and branch. Report a blocker if its preconditions are unmet.
- Commit only task-owned files. Never stage, discard, stash, or rewrite another task's changes.
- Kinosail is source-available, not open source. Do not publish private source, data, URLs, or artifacts without authorization.

## Implementation

- For product and API changes, consider the user experience, developer clarity, and reproducible agent verification. Preserve app-specific behavior; do not add process or abstractions just to satisfy a checklist.
- Before changing behavior, identify the affected public interface and failure path. Report material tradeoffs and verification limits.
- Make the smallest complete production change. Reuse existing seams and avoid unrelated refactors or compatibility layers.
- Preserve independent app binaries, containers, versions, releases, deployment checks, and health evidence.
- Keep each app as one API-driven Go process and supported container. Public HTTPS runs in a separate restricted gateway container with no application state. Web and API adapters call shared application operations.
- Treat values crossing trust boundaries as untrusted. Bound, parse, normalize, and validate them before side effects.
- Add focused negative tests for changed inputs and prove rejection causes no side effects.
- Test observable behavior at public interfaces. Test code is outside production line-count limits.

## Verification

- Run focused regression tests and affected package checks while editing.
- Before publication, run `make max-loc` and, after committing app changes, the affected app's `make verify-changed`. For workflow-only changes, run the CI contract tests and `actionlint` locally. GitHub Actions remains the authority for complete selected suites.
- Once the relevant checks pass, rerun them only after a change or a concrete failure requires it. Fix task-caused local test failures without routine approval stops.
- Run container, populated-browser, cross-browser, performance, race, and release gates only when the changed surface requires them.
- Run root checks serially because the Go linter uses a shared lock.
- For visible changes, follow the app's `DESIGN.md` and `impeccable` skill. Inspect populated responsive renders and accessibility evidence.
- Report exactly what passed, what was not run, and every remaining device, browser, deployment, or environment boundary.

## Delivery

- Unless the user requests read-only work or says not to publish, complete implementation through a pull request into protected `origin/main`. Required GitHub checks must pass; never bypass protection or force-push main.
- Merge the pull request with a merge commit so the reviewed task commit remains an ancestor of remote main.
- Fetch and reconcile current `origin/main`, rerun checks affected by reconciliation, and retry ordinary push races.
- Prove the task commit is included in remote main with a fetched ancestry check. `git ls-remote` alone proves only the ref value.
- GitHub Actions is the CI and container-publication authority. Green main pushes publish affected app containers automatically; version tags and major releases are not required. Keep the local deployment watcher separate from artifact publication.
- Nox deployment requires successful CI and the selected app's production-image promotion for the exact current main commit. Its locally rebuilt ARM image must pass its own scan and remote health check; report that image separately from the published GHCR digest.
- Current CI/CD decisions and evidence live in `engineering/research/ci-cd-reset-2026-09-22.md`; the 2026-09-20 note is historical.
- Treat source tests, browser checks, remote publication, deployed revision, container health, TLS, and physical-device proof as separate facts.
- End implementation delivery reports with `MAIN: YES — <remote main SHA>` after proof, or `MAIN: NO — <specific blocker>`.

## Working style

- Handle the task and verification yourself by default. Do not spawn subagents except for necessary independent reviews or when I explicitly ask you to.
- Read only the files and references relevant to the task. Use `rg` and bounded output.
- Continue authorized local work without routine approval stops. Ask only when missing input materially changes scope, permissions, or behavior.
- The current user request takes precedence over these defaults. Keep working until the requested outcome is verified or a specific blocker requires user action.
- Use concise updates and plain language. Preserve exact product terms, commands, paths, and quotations.

## Documentation language

- Write the root README and user-facing docs in clear, plain English. Use the principles of [ASD-STE100 Simplified Technical English](https://www.asd-ste100.org/) as inspiration, but do not claim formal ASD-STE100 compliance.
- Prefer common words, active voice, and direct instructions. Keep one main idea in each sentence and paragraph. Aim for 20 words or less in instructions and 25 words or less in explanations.
- Use one name for each feature or action. Define a technical term the first time a general reader needs it.
- Preserve exact product names, setting keys, API names, commands, code, license terms, and security limits. Add a short explanation when a term is necessary but unfamiliar.
- Use numbered steps for tasks that need an order. State what the reader needs, what to do, what should happen, and what to do when it does not.
- Prefer US English. Avoid idioms, unexplained abbreviations, vague references, and marketing claims that change the technical meaning.
- Treat sentence-length targets as editing guides. Keep a longer sentence when splitting it would hide a condition, limit, or security rule.
