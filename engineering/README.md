# Kinosail engineering

Start with [CONTRIBUTING.md](../CONTRIBUTING.md) for development and Git workflow, and [AGENTS.md](../AGENTS.md) for repository policy. This directory contains engineering evidence and decisions; user instructions belong in app READMEs and app-local `docs/`.

## Find the relevant material

| Area | Entry point |
| --- | --- |
| Player architecture, release checklist, and research | [Player engineering](../apps/player/engineering/README.md) |
| Subtitle automation and provider handling | [Subtitles engineering](../apps/subtitles/engineering/README.md) |
| Dashboard operations and architecture | [Dashboard engineering](../apps/dashboard/engineering/README.md) |
| Apple implementation and evidence | [Native client](../apps/player/apps/native/README.md) |
| Shared domain and application operations | [Packages](../packages/README.md) |
| Cross-app design | [Current design direction](design/cinema-direction.md) |
| Cross-app investigation | [Research](research/) |
| Agent issue workflow | [Issue tracker](agents/issue-tracker.md) |

## Release and deployment

Each app owns its binary, container, version, and release acceptance. The local deployment watcher can build and deploy development images from remote `main`; that does not create a signed customer release.

GitHub Actions is disabled. The Player [release checklist](../apps/player/engineering/release-checklist.md) records the signed-image publishing dependency. Subtitles has its own [checklist](../apps/subtitles/engineering/release-checklist.md). Dashboard checks are defined in its [Makefile](../apps/dashboard/Makefile) and deployment scripts; it does not currently have a separate release-checklist document.

While `.gates-disabled` exists, do not run disabled suites or count skipped checks as passes. Record source, rendered/browser behavior, remote ancestry, deployed image revision, container health, TLS, and physical-device evidence separately.

## Documentation maintenance

The root README routes readers to a complete first-start path. App READMEs own app-specific requirements and lifecycle commands. Keep ports, working directories, configuration examples, permissions, and release availability consistent with source. Historical research and design studies are dated evidence, not promises of current behavior. See [documentation preparation notes](research/documentation-production-readiness-2026-09-15.md).
