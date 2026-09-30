# All-app polish — web batch, 2026-09-30

This batch fixes three visible web defects. It does not certify every app as perfect. Player native clients remain in scope for subsequent work. Dashboard is retired in the current repository.

## Findings and changes

1. **Administration inherited the browsing backdrop.** The signed-in shell adds browsing classes to settings and account pages. Player and Subtitles therefore painted CinemaSail behind forms. Both apps now exclude settings and authentication forms from that decorative rule. Browsing keeps the artwork and its accessibility fallbacks. See [Player before](evidence/player-settings-before.png), [Player after](evidence/player-settings-after.png), [Subtitles before](evidence/subtitles-settings-before.png), and [Subtitles after](evidence/subtitles-settings-after.png).
2. **Player landscape Search extended beyond the viewport.** A later mobile Supporter rule overrode the compact search width. At 720 × 450, Search started at −64px. The landscape override now retains the compact control and expands within the viewport when focused. See [before](evidence/player-search-before.png) and [keyboard focus after](evidence/player-search-focus-after.png).
3. **Player landscape Support links overlapped Search and Actions.** The duplicated desktop link is hidden in this layout. The mobile link sits beside the brand with space reserved for the fixed controls. It yields while Search expands. The regression checks link/control intersection as well as viewport geometry. See [390px landscape after](evidence/player-phone-landscape-after.png).

The CSS cache versions advance to Player `electric-38` and Subtitles `cinema-11`. Both isolated-instance runners include the regressions. The Subtitles production-container browser runner also includes them; Player already discovers every `@smoke` test.

## Repeatable run record

The initial source was `55c0b765c16eb68a403b1a8f4c76f8a71340bdad`. The implementation checkpoint is `9d10883b4`; reconciliation with current main produced `94962a5ce`. [run-context.json](run-context.json) records source hashes, environment, and fixture data. Each new E2E test also attaches its Git revision, source state, platform, browser version, fixture description, command, and outcome.

Container startup failed because the shared Podman VM had no free storage. The browser runs used the production Go entry points with isolated data, cache, backups, and generated CC0 media. Player listened on loopback HTTPS port 39227 and Subtitles on 39228. The fixture contained four movies, a show with two episodes, music, an audiobook, a book, and a photo. The temporary Owner used TOTP. Credentials and traces remain outside Git.

For a fresh supported container run, execute the relevant app's `./scripts/test-instance.sh up`, `verify`, and `browser`, with a unique `KINOSAIL_TEST_PROJECT` and `KINOSAIL_TEST_ROOT`. The scripts generate media and account fixtures. Set `KINOSAIL_BROWSER_MATRIX=full` for Chromium, Firefox, and WebKit. Clean only that project's resources with `down --volumes`.

The focused command is `pnpm --dir e2e exec playwright test polish-shell.spec.ts --workers=1 --reporter=line`, with `KINOSAIL_TEST_INSTANCE=1`, the isolated origin in `KINOSAIL_E2E_URL`, the fixture secret in `KINOSAIL_TEST_TOTP_SECRET`, and a task-owned `KINOSAIL_E2E_OUTPUT_DIR`.

## Verification

| Check | Result and boundary |
| --- | --- |
| New regressions before the fixes | Player background and search assertions failed; Subtitles background assertion failed. A later bounding-box assertion reproduced the remaining Support overlap before its fix. |
| New Player regressions | [6/6 passed](evidence/player-browser-matrix.log), Chromium/Firefox/WebKit; desktop and phone administration, 720px and 390px short layouts, and keyboard focus. |
| New Subtitles regression | [3/3 passed](evidence/subtitles-browser-matrix.log), Chromium/Firefox/WebKit; desktop and phone administration. |
| Player adjacent production fixtures | 15/15 Chromium skeleton, Home stack, and shared-button layout tests passed again with fresh fixtures after reconciliation. |
| Subtitles inspector fixtures | 14/14 Chromium checks passed again with fresh reconciled fixtures across seven widths, dark/light, and pending/loaded/empty/failed states. The selected 1440px and 390px dark matrix passed 6/6 across all three browsers. Axe checks are included in those selected fixture tests. |
| Subtitles populated adjacent journeys | 4/4 Chromium navigation, responsive accessibility, keyboard/forced-color, Library search, and Overview search tests passed. |
| Player server package | `go test ./internal/server` passed in 310.755s before the final landscape spacing adjustment. Reconciled changed-path checks are recorded below. |
| Subtitles server package | `TMPDIR=/tmp/kp-subs-01a0f2ab go test -count=1 ./internal/server` passed in 111.782s. Earlier attempts reported PASS but failed to write their test log when storage filled; another retry used a path too long for its Unix socket. Those failed commands are not counted as passes. |
| Reconciled changed-path gates | [Player passed](evidence/player-verify-changed.log), including focused Go, CSS detector, shellcheck, and both browser regressions. [Subtitles passed those stages](evidence/subtitles-verify-changed.log), then failed its container stage because Podman storage was full. |
| Repository checks | `make max-loc`, `git diff --check`, and shellcheck on all three changed runners passed. |
| Container instance checks | Both `make test-instance-check` commands failed before startup with Podman `no space left on device`. No shared images or unrelated resources were pruned. |

The new background test compares rendered canvas pixels with a temporarily plain canvas, then restores the browser style. In this local WebKit build, the painted body image was visible while `getComputedStyle(body).backgroundImage` returned `none`. Pixel comparison retains a visible-behavior assertion across browsers. The apps' content security policies remain active.

## Remaining work and evidence boundaries

The broader Player layout baseline passed 2 tests and failed 11. The search overflow was confirmed and fixed here. Other failures include obsolete sidebar/markup expectations, a narrow form-width contract, a small heading-position mismatch, and intermittent navigation errors. They remain audit observations until individually reproduced and triaged. Their assertions were not weakened in this batch.

Apple iPhone/iPad, Apple TV, Apple Watch, Android phone/tablet, Android TV, and Wear OS have no fresh rendered or device evidence from this batch. Host storage fluctuated below 1GiB during verification, and the shared container VM remained full. Host storage later recovered, allowing native verification to continue after this web checkpoint. No Android device was attached. Existing simulators belonged to other work and were left alone. Native builds, authenticated native journeys, physical remotes, casting, long playback, real provider operations, deployment, and public TLS remain separate verification boundaries.

The primary checkout's unrelated dirty work and inactive unique worktrees were preserved. Hosted CI, protected-main integration, and container publication are separate from the local source and browser results.
