# Contributing to Kinosail Subtitles

Development, Git workflow, verification, and documentation rules are maintained in the [monorepo contribution guide](../../CONTRIBUTING.md). In a release bundle without the monorepo, read the [online contribution guide](https://github.com/Kinosail/kinosail/blob/main/CONTRIBUTING.md).

## Contributor agreement

Read the [Individual Contributor License Agreement](CLA.md) and accept it in the pull-request description. Obtain required employer authorization or arrange the [Corporate Contributor License Agreement](CCLA.md) when another entity owns rights in the work. Contributors retain ownership; see [LICENSING.md](LICENSING.md).

## App development

Clone the full monorepo and work in `apps/subtitles/`. The shared `packages/` module and repository-root container context are required. The [app README](README.md) covers local setup; [engineering](engineering/README.md) contains architecture and release guidance.

When quality gates are enabled, run the focused regressions and `make verify-changed` for this app, plus applicable release checks. While the root `.gates-disabled` marker exists, suites remain disabled and skipped checks are not passes. GitHub Actions is disabled; do not infer CI results from workflow files.

## Synthetic media

Do not add downloaded or third-party creative media to fixtures. Use non-creative mathematical inputs or wholly original work you may contribute. Copyrightable fixture contributions under `testdata/` or the media generator are dedicated under [CC0 1.0 Universal](testdata/CC0-1.0.txt), including their generated media output. Generator code retains the repository software license. See [testdata](testdata/README.md).

Report non-sensitive problems through [Support](../../SUPPORT.md); report vulnerabilities privately using [SECURITY.md](SECURITY.md).
