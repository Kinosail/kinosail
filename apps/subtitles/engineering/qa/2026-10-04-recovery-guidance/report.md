# R19 and R20 implementation evidence

Baseline: `2e9ede47a`.

R19 is confirmed and repaired. A synthetic failed track check renders the unavailable-file Library notice. Its Settings link previously named `#library`, which did not resolve to Media Libraries. The repaired link names `#libraries` and resolves against the actual Settings HTML.

R20 is confirmed and repaired. The provider guide contradicted both source and release Compose. The corrected guide and README distinguish those bases, all additive overlays, minimal platform/catalog manifests, and the inline installation example. The configuration files and their exact values are unchanged. [Compose inventory](evidence/compose-inventory.json) records each checked file and SHA-256.

## Verification

- Red: `GOMAXPROCS=2 go test -p 1 ./internal/server -run '^TestSubtitleUnavailableRecoveryReachesMediaLibraries$' -count=1`, from `apps/subtitles/`. [Result](evidence/r19-red.log): the rendered `/settings#library` does not resolve to Media Libraries.
- Green: `GOMAXPROCS=2 go test -p 1 ./internal/server -run '^(TestSubtitleUnavailableRecoveryReachesMediaLibraries|TestSubtitleDashboardTracksFailuresAndRecoversAfterSourceChange|TestSubtitleAppUsesKinosailSisterSetupAndFocusedSettings)$' -count=1`. [Result](evidence/r19-green.log): all three checks passed.
- `make max-loc` and `git diff --check` passed.
- After the implementation commit, `GOMAXPROCS=2 GOFLAGS=-p=1 BASE=2e9ede47a make verify-changed` passed max-LOC, diff-check, server compilation, and the focused HTTP regression. [Result](evidence/verify-changed.log).
- The mechanical design detector inspected the changed template. Its stylesheet resolver could not load the embedded `/static/app.css` asset from source, so reported default browser colors/type; these are not a populated-render result. The link repair changes no geometry or styling. [Detector output](evidence/r19-design-detector.json).

Environment: macOS arm64, Go 1.27.1, bounded `GOMAXPROCS=2` and `-p 1`, temporary synthetic media/settings/cache, no provider calls or user data. These are rendered HTTP-adapter checks with a local failing probe stand-in, not populated-server browser E2E.

Populated `make test-instance-check` was not run: Podman is unavailable, the parent owns the serial build schedule, and no image or deployment changes are authorized here. Required hosted checks and independent review remain with the integration owner. No physical device, production, Nox, or external provider validation is claimed.
