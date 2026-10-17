# App QA report — Player web, iOS, tvOS — 2026-10-16

## Run record
- Mode: change-agnostic smoke audit of main.
- Revision: bb385aa9565b86b9f3824ebb32469ffdc2310569 (origin/main, PR #643 merge). Binary built from this tree with `go build ./cmd/kinosail`; /healthz returned {"status":"ok"} and the log shows "Kinosail ready" (tls true).
- Environment: Linux cloud container, Chromium 141 (headless shell, Playwright 1.63.0), isolated data in /tmp/claude-0/qa-root, synthetic media from scripts/generate-test-media.sh (local ffmpeg).
- Deviation from README: Docker Hub returned 429 on the image build, so the compose test instance was not used. The server ran as a native process with the same env as compose.test.yaml; the TMDB fixture was served by python http.server on :8090. The owner was set up through the same /api/v1/setup, MFA and onboarding calls as test-instance.sh.
- Browser fixes (environment only): shimmed chromium_headless_shell-1243 to the installed 1194 build, trusted the instance CA in the NSS db, and set NODE_EXTRA_CA_CERTS. No repo files were changed.
- Command: pnpm --dir apps/player/e2e test <10 specs from test-instance.sh browser> --workers=1 --project=chromium --retries=0
- Log: /tmp/claude-0/qa-root/pw4.log; traces/videos/screenshots: /tmp/claude-0/qa-root/playwright-results

## Web result (Chromium only): 39 passed, 4 failed
Failed:
1. focus-audit (390px and 1440px) — video readyState stayed 0.
2. test-instance-production "saved video Compatibility ... music and audiobook" — audio has no supported source.
3. test-instance-production "real offline download plays and seeks" — video readyState stayed 0.

Cause: this Chromium build reports canPlayType('video/mp4; codecs="avc1.42E01E, mp4a.40.2"') = "" and AAC = "" (MP3 = probably). It has no H.264/AAC decoder, and the fixtures are H.264/AAC. These are environment limits, **not confirmed product defects**. They need re-running on Chrome or in GitHub Actions, which is the authority.

## Not run (web)
Firefox/WebKit, race/coverage, container image and publication gates, Viewer-role deep checks beyond the specs above, specs outside the 10 listed (player-*, playback-*, layout-audit-player, supporter-checkout, jellyfin-setup, session-timeouts, and others), manual exploratory and screen-reader testing, and real-GPU transcoding. No product bugs were found, but this is not proof the app is fully tested.

## Native iOS / tvOS
Cloud has no Xcode. The work was handed to the Mac Remote Control session (session_01LHYVKHBwovfpPPYoEDeoPX) with the commit, commands and report format. **No results yet. Nothing about iOS or tvOS is verified.** PRs #642/#644 (Apple TV speed pass) remain unverified on Swift tests and device timing.
