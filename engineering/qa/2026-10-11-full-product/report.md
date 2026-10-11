# Full product QA — October 11, 2026

This ongoing audit covers Player, Subtitles, shared packages, installation,
delivery, and Apple and Android clients. It continues the
[October 10 audit](../2026-10-10-complete-audit/report.md).
Supporter and Home Assistant implementations belong to their own repositories.
Their Kinosail integration boundaries remain in scope.

## Run record

- Initial revision: `655b7d6db39028db19cad3a1e8dd84dbd77fb936`.
- Worktree: `codex/full-product-qa-20261011`, leased to this task.
- Private evidence: `/Users/mikeo/Documents/Codex/full-product-qa-20261011/`.
- Data: generated CC0 media, disposable Server state, and synthetic accounts.
- Local web environment: macOS ARM64, pinned Playwright 1.63.0,
  Chromium, Firefox, and WebKit. Browser versions appear in runner attachments.
- Hosted baseline: [deep run 38151255345](https://github.com/Kinosail/kinosail/actions/runs/38151255345).
  Its result is pending at this checkpoint.

The primary checkout's two unrelated Apple edits remain untouched. Eight
unrelated inactive worktrees contain dirty or unique work and remain preserved.
Another task owns the active artwork worktree.

## Journey inventory

For each domain, check outcomes and persistence; failure, empty, permission,
and recovery states; then input, accessibility, layout, console, and network.
Passing executions are evidence for named contracts, not a completeness score.

| Domain | Required scope | Current status |
| --- | --- | --- |
| Installation and identity | First install, Owner protection, MFA/passkeys, sign-in/out, recovery, sessions | Fresh Chromium, Firefox, and WebKit journeys passed; broader identity review remains |
| Viewer access | Profiles, restrictions, public access, Quick Connect, expired/rejected grants | Hosted baseline pending; additional review remains |
| Library | Every media type, search, filters, paging, details, My List, playlists, collections | Hosted baseline pending; responsive shell checks below passed in two engines |
| Video | Direct First, preparation, seeking, progress, tracks, captions, retry, session cleanup | Caption transport/lifecycle checks below passed; complete decoded-media and device review remains |
| Audio | Queue, next/previous, sleep timer, persisted progress, interruptions | Hosted baseline pending; platform review remains |
| Reading and photos | Supported formats, paging, bookmarks, preferences, zoom, invalid resources | Hosted baseline pending; platform review remains |
| Downloads | Preparation, integrity, pause/resume, ownership, offline use, removal, interrupted state | All 27 ownership executions passed; large-file and hardware boundaries remain |
| Subtitles | Search, inspect, pair, save, cleanup, history, restore, provider failures | Hosted baseline pending; expanded local review remains |
| Administration | Configuration, integrations, security, backups, updates, diagnostics | Responsive administration and session settings passed in two engines; remaining operations pending |
| Apple | iPhone/iPad touch, TV focus/remote, Watch relay, lifecycle, accessibility | Native inventory refreshed; current interaction review pending |
| Android | Phone/tablet/TV/Wear, lifecycle, input, accessibility, supported media | Hosted compilation/unit/lint proof pending; emulator and hardware review remain |
| Quality and security | Go tests/races/coverage, lint, metrics, supply chain, workflow contracts | Tooling passed; Halstead loading repaired; package lint remains failed |
| Containers and release | Both apps and architectures, scan, installer, publication, deployment, TLS | Hosted baseline pending; publication and deployment are separate boundaries |

## Iteration 1 results

- `make tooling-check` passed. The CI contract suite reports 169 cases,
  including five environment-dependent skips.
- `download-pause-ownership.spec.ts` passed 27 executions across Chromium,
  Firefox, and WebKit, without retries or skips. The two previously failing
  cases also passed their first focused Chromium replay.
- Caption lifecycle/recovery passed 21 executions across three engines.
  Three decoded-media cases skipped because this runner deliberately exercises
  isolated HTTP transport. These are not populated-server media proof.
- First-install journeys passed separately on fresh real Servers in Chromium
  and Firefox. Chromium used a virtual passkey; Firefox used real TOTP validation.
  Each journey reached decoded media, sign-out, and another sign-in.
- The exact WebKit first-install journey also passed on Linux ARM64 over
  HTTPS. The disposable container alone trusts the generated Server CA.
  Browser certificate validation stays enabled. The input snapshot's Git tree
  and commit match the recorded source; the container was removed afterward.
- Responsive shell, TMDB instructions, session settings, and six show-action
  widths passed 22 executions across Chromium and Firefox. Show-action cases
  use exported real templates with rendered fixtures. Other cases use the
  real Server. These forms of proof remain distinct.
- `make max-loc` and `git diff --check` passed before the first checkpoint.
- `make packages-check` still fails at existing package lint findings. Its
  subsequent aggregate checks did not run. No rule or threshold changed.

## QA-001 — Halstead cannot load three source configurations (P2; repaired)

The complete baseline command reproduces three errors: the fallback source
is excluded on Windows, the `q47proof` fixture has no selected source without
its build tag, and the standalone HLS command cannot resolve its workspace.
The same failures occur through a new command-level regression before repair.

The authoring gate established the following before production edits:

- Contract: the public quality command must measure every selected production
  source under a valid platform, build tag, and module/workspace context.
- Regression: selecting an excluded platform or disabling the command's
  workspace makes the gate fail without measuring that source.
- Coverage gap: existing metric unit checks do not exercise the Python gate's
  selection. Product E2E cannot reach this source-tool boundary.
- Seam: none. The regression runs the actual command and compiled metric tool
  against a disposable Git workspace with independent Go modules.

The repair selects FreeBSD for `_other.go`, enables the fixture's existing
`q47proof` tag, and loads standalone commands as named Go files with their
workspace. Ordinary module packages retain their isolated module loading.
The difficulty threshold and measured-source inventory remain unchanged.

The new regression passes after repair. Existing Go metric tests and the
complete `scripts/quality/check-halstead.sh` also pass. The regression is wired
into `make tooling-check`.

## QA-002 — WebKit HTTP fixture loses secure authentication (resolved environment)

Two fresh-state WebKit runs return to sign-in after Owner creation. The
bounded network projection shows no retained session cookie. The Server uses
a Secure, HttpOnly session cookie. Chromium and Firefox permit this localhost
development case; this WebKit HTTP environment does not establish it.

The remaining WebKit HTTP shell cases were interrupted after this prerequisite
was identified. The combined batch contains 22 passes, one failed case, and
ten skipped or interrupted cases. They are not reported as a passing batch.
The exact first-install journey passes over trusted HTTPS in the disposable
Linux container. Cookie protections remain intact. The remaining WebKit shell
journeys are being replayed there. Local HTTP proof makes no TLS trust claim.

## Reconciliation and hosted findings

Main advanced to `d69cb7a7` with the artwork repair. Reconciliation commit
`6d97305b` includes that work. The full Halstead command passes again with the
new source inventory.

Hosted tooling then identified stale generated architecture snapshots for
the newly merged production source. Both snapshots were regenerated with the
existing generator. The architecture contract suite passes after regeneration.
No generator, template, or validation rule changed.

The baseline deep run records a native snapshot failure in
`MediaCardRatingTests`: PNG encoding returns no data on hosted Xcode 27.1 beta.
Its Swift Testing contracts pass, but the complete native job is failed.
The cause and product impact remain unverified. Retain that first result.

The focused iPad orientation run reproduces all three phone-only contract
failures. One applicable detached-window case passes. A first touch-suite
attempt lacked its required fixture and failed before product interaction.
That setup failure is excluded from product defects. The documented touch
fixture is running for the corrected replay.

## Evidence and remaining work

Private command receipts record revision, command, duration, result, log path,
and SHA-256. Local Server receipts also record the exact binary checksum and
generated media checksums. Browser JSON, traces, videos, screenshots, and
synthetic Server logs remain private. No credentials or raw logs are committed.

Remaining work includes the previous unproved browser cases, full-tree lint
and complexity findings, native phone-only test applicability, physical
devices, external receivers and services, long playback, codec/HDR diversity,
assistive technologies, and release/store acceptance. Source, browser,
simulator, physical-device, CI, publication, deployment, health, and TLS
evidence must remain separate. This checkpoint does not certify full QA.
