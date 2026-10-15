# R19 and R20 implementation evidence

Baseline: `2e9ede47a`.

R19 is confirmed and repaired. A synthetic failed track check renders the unavailable-file Library notice. Its Settings link previously named `#library`, which did not resolve to Media Libraries. The repaired link names `#libraries` and resolves against the actual Settings HTML.

R20 is confirmed and repaired. The provider guide contradicted both source and release Compose. The corrected guide and README distinguish those bases, all additive overlays, minimal platform/catalog manifests, and the inline installation example. The configuration files and their exact values are unchanged. [Compose inventory](evidence/compose-inventory.json) records each checked file and SHA-256.

Independent review identified one additional distributed variant: `apps/player/packaging/platform-compose-both.yaml`, published by `engineering/documentation/build.py` as `install-assets/both.yaml`. Its Subtitles service maps only application directories and has no provider credential mappings. The guide and inventory now explicitly cover this combined installation file. No Player files were changed.

Protected CI identified excess cognitive complexity in the new R19 regression. DOM parsing and element lookup are now separate helpers; the test still creates an unavailable state, follows its rendered link, and requires the target heading to be Media Libraries. No assertion or linter rule was removed or relaxed.

The corrected `2afa1cfa` regression passed `GOMAXPROCS=2 go test -p 1 ./internal/server -run '^TestSubtitleUnavailableRecoveryReachesMediaLibraries$' -count=1`. `GOMAXPROCS=2 GOFLAGS=-p=1 golangci-lint run --new-from-rev=origin/main ./internal/server` reported zero issues. Uncommitted R07 test preparation was temporarily kept outside the package during lint and restored unchanged. [Test](evidence/r19-refactor-green.log) and [lint](evidence/r19-refactor-lint.log) logs record these checks.

## Verification

- Red: `GOMAXPROCS=2 go test -p 1 ./internal/server -run '^TestSubtitleUnavailableRecoveryReachesMediaLibraries$' -count=1`, from `apps/subtitles/`. [Result](evidence/r19-red.log): the rendered `/settings#library` does not resolve to Media Libraries.
- Green: `GOMAXPROCS=2 go test -p 1 ./internal/server -run '^(TestSubtitleUnavailableRecoveryReachesMediaLibraries|TestSubtitleDashboardTracksFailuresAndRecoversAfterSourceChange|TestSubtitleAppUsesKinosailSisterSetupAndFocusedSettings)$' -count=1`. [Result](evidence/r19-green.log): all three checks passed.
- `make max-loc` and `git diff --check` passed.
- After the implementation commit, `GOMAXPROCS=2 GOFLAGS=-p=1 BASE=2e9ede47a make verify-changed` passed max-LOC, diff-check, server compilation, and the focused HTTP regression. [Result](evidence/verify-changed.log).
- The mechanical design detector inspected the changed template. Its stylesheet resolver could not load the embedded `/static/app.css` asset from source, so reported default browser colors/type; these are not a populated-render result. The link repair changes no geometry or styling. [Detector output](evidence/r19-design-detector.json).

Environment: macOS arm64, Go 1.27.1, bounded `GOMAXPROCS=2` and `-p 1`, temporary synthetic media/settings/cache, no provider calls or user data. These are rendered HTTP-adapter checks with a local failing probe stand-in, not populated-server browser E2E.

Populated `make test-instance-check` was not run: Podman is unavailable, the parent owns the serial build schedule, and no image or deployment changes are authorized here. Required hosted checks and independent review remain with the integration owner. No physical device, production, Nox, or external provider validation is claimed.

## Removed evidence

These referenced files were removed from the working tree. Each remains in Git history at commit `3f292de8c5d13b14c67ce8d7cdd5c1a12db2c022`. See [REMOVED-EVIDENCE.md](../REMOVED-EVIDENCE.md).

| Path | Bytes | SHA-256 |
| --- | ---: | --- |
| `2026-10-04-recovery-guidance/evidence/r19-green.log` | 65 | `b9efc839e539d5adbd198c21be1f56e18ba27929559137d4eae1b55a360aa70f` |
| `2026-10-04-recovery-guidance/evidence/r19-red.log` | 265 | `94c5d6ffc928754eadad99645f37a5751d84930ad6e630bc1ca91c9311d603ad` |
| `2026-10-04-recovery-guidance/evidence/r19-refactor-green.log` | 65 | `55d935e17b2c3d2c2a1b6eab8b5682870fb4dc48e1cb750f65cbf23733f78d78` |
| `2026-10-04-recovery-guidance/evidence/r19-refactor-lint.log` | 10 | `e92606b0bf483111dff0a120c315ea165821348f31365020e2468a0059095c47` |
| `2026-10-04-recovery-guidance/evidence/verify-changed.log` | 181 | `a0fb3213e5367af640abfa8d367da675c62682e9109db18820a33d50fab80822` |
