# Nox live QA addendum — 2026-09-27

## Run record

- Mode: read-only production follow-up to [the full audit](report.md) and [the fixes](followup.md).
- Time: 2026-09-27, about 18:23 MDT. Host: macOS Chrome and Codex in-app browser. Desktop viewport: 1920 × 842; phone viewport: 390 × 844. Browser control used the live Nox pages, not a synthetic fixture.
- Running Player and Subtitles containers: `d49461422c410fc3c4693622b59523796756923a`, both running and healthy by `docker inspect` on Nox. This merge contains the shared passkey fix from PR #325. Both login pages load `/static/passkeys.js?v=15`.
- `origin/main` was `2155a7a994ca8b1ce93e959302cdb66fe66798f5` at inspection. The commits after `d4946142` changed README, docs pages, and the docs installer, not the Player or Subtitles runtime. The local deployment marker named the newer main commit; the actual container revision label is recorded above.
- Both configured HTTPS `/healthz` endpoints returned HTTP 200 with successful certificate verification (`curl --max-time 12 -fsS -o /dev/null -w '%{http_code} %{ssl_verify_result}'`). The Subtitles check used its own configured hostname. An earlier mismatch came from checking the Player hostname on the Subtitles port.

## Browser journeys

| Journey | Observed result | Boundary |
| --- | --- | --- |
| Player populated desktop Shows | Loaded 132 shows, title jump, sort, navigation, poster cards, and playback links. No visible load error. | Navigation and render only; playback was not started on production. |
| Player phone Shows, Home, and detail | At 390 px, the two-column poster grid, alphabet rail, bottom navigation, featured title, continue-watching rows, and movie detail fit the viewport. The document width was 390 px. | Read-only navigation; no progress, list, or download change. |
| Player empty search | A query absent from the populated library produced “0 results” and “No matching titles. Try another title, person, or genre.” The original Shows URL was restored afterward. | One search query; no search performance or large-library claim. |
| Player browser console | Zero warnings or errors recorded during these journeys. | This is one Chrome session, not a long-duration log review. |
| Subtitles phone sign-in | At 390 px, the password, code, passkey, and language controls fit. A clean in-app browser session showed an empty passkey status. | The existing authenticated Subtitles settings tab was owned by another browser session, so this follow-up did not inspect its populated settings. The isolated Subtitles audit did. |
| Returning passkey browser | Chrome showed “Waiting for your passkey…” on both login pages. The shared script opens a chooser when a browser has a previous passkey-use hint; existing E2E tests assert that behavior. This observation is consistent with that path. | No product defect established from this status alone. A clean browser did not show it. |

No production media, settings, accounts, or payments were changed. This follow-up did not run physical Apple or Android devices, a Cast receiver, external subtitle provider fetching, paid checkout, or long-duration playback. The [full audit](report.md) and [follow-up](followup.md) contain the isolated browser, emulator, simulator, and regression-test results. Those boundaries still prevent a 20/20 claim for every app.
