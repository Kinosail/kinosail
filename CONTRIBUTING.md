# Contributing to Kinosail

Start with a reproducible bug, a concrete user need, or a documentation correction. Read [AGENTS.md](AGENTS.md) and the affected app's guidance before changing code. Keep each contribution focused and preserve unrelated work.

## Contributor agreement

Read the [Individual Contributor License Agreement](apps/player/CLA.md) and accept it in the pull-request description. If another entity owns rights in your work, obtain its permission or arrange the [Corporate Contributor License Agreement](apps/player/CCLA.md). These Kinosail agreements cover contributions to this monorepo; app copies accompany independently distributed source. Contributors retain ownership. See [licensing](LICENSING.md).

## Set up the workspace

Clone the complete monorepo, including `packages/`; copying an app directory alone does not provide its shared Go dependencies or container build context.

```sh
git clone https://github.com/MikeO7/kinosail.git
cd kinosail
make hooks
```

- Install Go 1.27 or newer, Git, and Make for Go development. [go.work](go.work) defines the workspace.
- For app/container work, install Docker Compose or Podman Compose.
- Player/Subtitles runtime work outside containers also needs FFmpeg/ffprobe; consult the app's [Brewfile](apps/player/Brewfile) and configuration templates for the other tools used by specific features.
- On macOS, `make -C apps/player bootstrap` or `make -C apps/subtitles bootstrap` installs the app's declared Homebrew tools and shared Git hooks. Review the Brewfile first.
- Browser tooling uses Node and pnpm, with versions/dependencies declared in each app's `e2e/package.json` and lockfile.
- Native Apple development uses [Xcode and the Swift build scripts](apps/player/apps/native/README.md). Shared JavaScript quality tools live in `scripts/quality`; browser tests keep their app-scoped packages.

Compile without starting a service:

```sh
cd apps/player
go build ./cmd/kinosail
# Subtitles: cd ../subtitles && go build ./cmd/kinosail
# Dashboard: cd ../dashboard && go build ./cmd/kinosail-dashboard
```

Use the app README for a disposable local instance. Never use a real library or production database as a test fixture.

## Git workflow

Use a topic branch. Concurrent tasks use one dedicated leased worktree each:

```sh
git fetch origin main
git worktree add -b docs/example ../kinosail-example origin/main
cd ../kinosail-example
make worktree-lease TASK=docs-example
```

Commit only task-owned files, with a message describing the behavior or documentation change. Review `git diff` and explicitly stage the intended paths. Do not stash, discard, stage, or rewrite another task's work.

Before integration, fetch current `origin/main`, reconcile the topic branch, and repeat checks affected by reconciliation when gates are enabled. A maintainer completes local integration with `make agent-finish TASK=docs-example` from the task worktree. It requires clean task and main worktrees, locks integration, fast-forwards local `main`, and removes the merged task checkout and branch. If unrelated changes prevent this, preserve them and report the blocker. Do not force-push `main`.

External contributors submit a pull request against `main`. Agents follow the authorized delivery policy in [AGENTS.md](AGENTS.md). Local integration, remote publication, deployment, and release acceptance are separate steps.

## Implementation expectations

- Make the smallest complete production change and reuse existing shared application operations. Web and `/api/v1` adapters must enforce the same rules.
- Validate untrusted input before side effects. Add negative tests for missing, malformed, unknown, oversized, out-of-range, and conflicting values where applicable; prove rejection makes no changes.
- Test observable behavior and important regressions. Test code is outside production line-count limits.
- Follow the affected app's `DESIGN.md` for visible changes. Exercise populated flows, keyboard use, narrow layouts, and recovery when gates permit.
- Update installation, configuration, API, and recovery instructions in the same change as the behavior they describe.
- Keep secrets, private addresses, personal media, and real account data out of source, screenshots, logs, and issues.

## Verification

**The root `.gates-disabled` marker overrides the commands below.** While it exists, do not run disabled suites manually or remove it without explicit maintainer authorization. Record checks as **not run — gates disabled**, not passed. GitHub Actions is disabled; retained workflow files do not prove that CI ran.

When gates are enabled, run the affected app checks and focused regressions. From the repository root:

```sh
make -C apps/player verify-changed
make -C apps/player check
make max-loc
```

Substitute `subtitles` or `dashboard` as appropriate. Shared-package changes require affected consumers as well as `make packages-check`. Root checks run serially because tooling uses a shared lock. Relevant container, browser, race, performance, and release requirements are described by each app's Makefile and release checklist; do not run unrelated suites solely to add checkmarks.

Report exact commands and results. A source check does not prove a browser flow, a healthy container does not prove playback, and a simulator build does not prove a physical-device installation. See [engineering](engineering/README.md) for the release boundary.

## Documentation contributions

Use one page for one reader task. Include prerequisites, the working directory for commands, the expected outcome, common failure recovery, and links to the next step. Verify ports, paths, flags, defaults, and API routes against current source. Use reserved example names such as `server.example.test`.

The root README is the app chooser and first-start path. App READMEs own product-specific setup. Published user docs live under `apps/<app>/docs/`; architecture notes and agent guidance live under `engineering/`. See the [documentation guide](apps/player/docs/contributing/documentation.md).

## Synthetic fixtures

Player and Subtitles fixtures must be generated from non-creative mathematical inputs or wholly original work that the contributor can dedicate under CC0. Do not download third-party creative media for fixtures. See [Player testdata](apps/player/testdata/README.md) and [Subtitles testdata](apps/subtitles/testdata/README.md) for the dedication and generation instructions.

## Review and reporting

Explain the problem, resulting behavior, verification performed, and remaining limits in the [PR template](.github/pull_request_template.md). Use [Support](SUPPORT.md) for non-sensitive bugs and feature requests. Use [Security](SECURITY.md) for private vulnerability reports.
