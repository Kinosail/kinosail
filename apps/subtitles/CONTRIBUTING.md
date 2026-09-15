# Contributing

## Contributor agreement

Before submitting a contribution, read the [Kinosail Individual Contributor
License Agreement](CLA.md). Check its acceptance box in every pull-request
description. If an employer or another entity owns rights in your work, obtain
its permission or arrange a signed [Corporate Contributor License
Agreement](CCLA.md) before contributing. Contributions cannot be accepted
without the applicable agreement.

Contributors retain ownership of their work. The agreements give the Kinosail
project owner the rights needed to distribute Kinosail under source-available
and commercial terms and to operate paid services.

## Public test media

Do not add downloaded or third-party creative media to the public test instance. Test films, episodes, music, audiobooks, books, photos, artwork, voices, likenesses, and provider metadata must be generated from non-creative mathematical inputs or be wholly original work that you have authority to contribute. By contributing copyrightable fixture material under `testdata/` or `scripts/generate-test-media.sh`, you apply the CC0 1.0 Universal dedication described in `testdata/CC0-1.0.txt` to that fixture material and its generated media output.

## One-time setup

Install Homebrew first, then run:

```sh
make bootstrap
```

This installs the declared development tools from `Brewfile` and registers both
pre-commit and pre-push hooks.

## Required checks

Every commit and push runs `make check`. It enforces:

- at most 300 physical lines in each hand-written Go file;
- formatting with `gofumpt` and `goimports`;
- cyclomatic complexity at most 10, cognitive complexity below 15, and
  functions no longer than 60 lines or 40 statements;
- static analysis, including unused/dead code, error handling, SQL/resource
  cleanup, logging, and security checks;
- a clean `go mod tidy -diff` result;
- unit tests, coverage baselines, and race-detector tests;
- reachable-vulnerability analysis with `govulncheck`; and
- commit-diff secret scanning with `gitleaks` (CI scans repository history).
- Shell scripts with `shellcheck` and GitHub workflows with `actionlint`.

Pull requests also run the production image through restart-persistence and
Chromium happy-path tests, including real playback and automated accessibility checks.

Dependabot checks Go modules and GitHub Actions weekly, while `.editorconfig`
keeps basic whitespace behavior consistent across GoLand and other editors.

Generated Go files carrying the standard `Code generated ... DO NOT EDIT.`
header are exempt from line and formatting limits. Do not bypass hooks; CI runs
the same policy. During development, `make check-fast` omits only the slower race
and vulnerability scans, and `make format` applies supported formatting fixes.

Go-aware checks intentionally skip until the repository has a `go.mod` or
`go.work`. Once the module is initialized, all checks become active without any
configuration changes.
