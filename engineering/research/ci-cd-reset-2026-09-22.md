# CI/CD reset — 2026-09-22

The workflow directory was cleared and rebuilt from zero. Four Actions files remain because each has one distinct boundary:

1. `ci.yml` is the only PR, merge queue, main, and weekly entry point. It computes one conservative selection plan, runs repository and security checks, calls selected app suites, and exposes the five existing protected check names. Documentation checks and Pages deployment live here.
2. `app.yml` runs the selected Player, Subtitles, or Dashboard suite. Lint, race/coverage, Go vulnerability checks, production-container checks, a Chromium browser smoke, installer checks, and Player Swift compilation run as relevant. Weekly and manual deep runs execute the complete browser suite across Chromium, Firefox, and WebKit. Its jobs read the plan directly, without a serial selection runner. The final app check validates the plan and fails closed on missing, cancelled, failed, or unexpected skips.
3. `publish.yml` accepts only the same main run's five successful required results. It builds affected apps on AMD64 and ARM64, tests and scans by digest, signs, attests, and promotes without rebuilding. Per-app promotion is serialized and rejects stale main commits.
4. `release.yml` handles all three independent version tags. It requires successful CI for the exact main commit, then builds and signs versioned images and, for Player/Subtitles, installer archives. It writes only the exact version tag, so older releases cannot roll back continuous `latest` or a mutable minor tag.

The selector treats unknown, workflow, shared, renamed, and deleted inputs conservatively. Documentation source changes select the docs build without unrelated server suites. Required checks are always emitted, including for intentionally skipped apps. The workflow has no path filter that could strand branch protection.

The required browser gate runs one populated Chromium journey for Player, one for Subtitles, and Owner setup plus a board journey for Dashboard. The full suites run every Monday and on manual deep dispatch. This consciously delays detection of Firefox, WebKit, and non-smoke browser regressions until a deep run; code, race, security, and production-container checks remain required before publication. A regression that escapes the smoke gate should become a focused smoke assertion when it covers a common deployment path.

The local commit hook checks the staged source-file cap and renews the current worktree lease; pre-push checks the pushed revision's cap and renews that lease. `make worktree-audit` still reports inactive checkouts, but unrelated expired leases do not block a different task's commit or local verification. The revised pre-commit path took 0.84 seconds in this worktree; hosted checks remain separate.

Installer signature verification follows the new reusable `publish.yml` identity and unified `release.yml` identity. [Fulcio's GitHub identity documentation](https://docs.sigstore.dev/certificate_authority/oidc-in-fulcio/) derives the certificate URI from `job_workflow_ref`; [GitHub documents](https://docs.github.com/en/actions/how-tos/secure-your-work/security-harden-deployments/oidc-with-reusable-workflows) that this claim names the called workflow.

Swift CodeQL runs weekly because the last hosted scan took about 65 minutes before cancellation. Player native changes still require an Apple build before merge. This is an explicit delay in Swift security findings; it is not pre-merge Swift scanning. Expensive quality metrics also run weekly.

The latest observed main run spent 61 minutes in the previous publication preflight polling for other workflows, then failed ([run 35660487392](https://github.com/Kinosail/kinosail/actions/runs/35660487392)). The new publication path consumes the five results from its own run, eliminating that poll. It still builds and scans the final two-architecture image after main CI, because that exact digest must pass before promotion. The reusable app suites no longer wait on a separate selection runner.

No end-to-end runtime improvement is claimed from local checks. Measure queue, execution, and publication time across at least 20 successful runs per scenario before setting or claiming a speed target. Preserve separate evidence for source tests, browser checks, image publication, deployed revision, health, TLS, and physical devices.
