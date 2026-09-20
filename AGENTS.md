# Kinosail monorepo agent policy

## GitHub Actions verification

- All quality and security gates are enabled on GitHub-hosted runners. Do not add `.gates-disabled` or weaken thresholds to obtain a passing run.
- Local hooks enforce the source-file cap and worktree ownership; GitHub Actions runs the full suites. Run focused checks locally when changing their implementation.

## Scope and ownership

- This repository is authoritative for Player, Subtitles, Dashboard, and Player native clients.
- Work here or in its Git worktrees. Keep Supporter and Home Assistant in their own repositories.
- Preserve unrelated work. Use a dedicated branch and worktree for concurrent implementation tasks.
- Keep at most one worktree for each active task. Do not let multiple agents change the same checkout.
- Checkpoint coherent work with regular commits. Serialize completed task integration into local `main`.
- Before finishing, merge or cherry-pick completed work into local `main`, then remove its worktree.
- Never leave dirty or unique work in an inactive worktree. Report blockers that prevent safe integration.
- Start secondary worktrees with `make worktree-lease TASK=<thread-or-task-id>`; commit and pre-push hooks renew the lease and audit every checkout.
- Use `make worktree-cleanup` only for expired clean merged checkouts. It must preserve dirty or unique work for explicit recovery.
- Finish with `make agent-finish TASK=<thread-or-task-id>` to lock, verify, fast-forward local `main`, and remove the task checkout and branch.
- Commit only task-owned files. Never stage, discard, stash, or rewrite another task's changes.
- Kinosail is source-available, not open source. Do not publish private source, data, URLs, or artifacts without authorization.

## Implementation

- Evaluate every change across UX, DX, and AX: name the user benefit, keep the implementation easy for developers to understand, and give agents clear ownership and reproducible verification commands.
- Prefer an existing shared operation or contract when it improves all three experiences. Do not add abstractions, controls, or process solely to satisfy this rule; preserve app-specific behavior.
- Define the behavior that must remain unchanged before editing. Verify the affected public interfaces and failure paths, and report any tradeoff or unverified boundary rather than claiming no regressions are possible.
- Make the smallest complete production change. Reuse existing seams and avoid unrelated refactors or compatibility layers.
- Preserve independent app binaries, containers, versions, releases, deployment checks, and health evidence.
- Keep each app as one API-driven Go process and supported container. Public HTTPS runs in a separate restricted gateway container with no application state. Web and API adapters call shared application operations.
- Treat values crossing trust boundaries as untrusted. Bound, parse, normalize, and validate them before side effects.
- Add focused negative tests for changed inputs and prove rejection causes no side effects.
- Test observable behavior at public interfaces. Test code is outside production line-count limits.

## Verification

- Run focused regression tests and affected package checks while editing.
- Before publication, run `make max-loc` and the affected app's `make verify-changed`.
- Run container, populated-browser, cross-browser, performance, race, and release gates only when the changed surface requires them.
- Run root checks serially because the Go linter uses a shared lock.
- For visible changes, follow the app's `DESIGN.md` and `anti-ai-slop-ui` skill. Inspect populated responsive renders and accessibility evidence.
- Report exactly what passed, what was not run, and every remaining device, browser, deployment, or environment boundary.

## Delivery

- Unless the user requests read-only work or says not to publish, complete implementation by committing and pushing to `origin/main` without force.
- Fetch and reconcile current `origin/main`, rerun checks affected by reconciliation, and retry ordinary push races.
- Prove the task commit is included in remote main with a fetched ancestry check. `git ls-remote` alone proves only the ref value.
- GitHub Actions is the CI and release authority. Keep the existing local deployment watcher separate from public artifact releases.
- Treat source tests, browser checks, remote publication, deployed revision, container health, TLS, and physical-device proof as separate facts.
- End implementation delivery reports with `MAIN: YES — <remote main SHA>` after proof, or `MAIN: NO — <specific blocker>`.

## Working style

- Handle the task and verification yourself by default. Do not spawn subagents except for necessary independent reviews or when I explicitly ask you to.
- Read only the files and references relevant to the task. Use `rg` and bounded output.
- Continue authorized local work without routine approval stops. Ask only when missing input materially changes scope, permissions, or behavior.
- Use concise updates and plain language. Preserve exact product terms, commands, paths, and quotations.
