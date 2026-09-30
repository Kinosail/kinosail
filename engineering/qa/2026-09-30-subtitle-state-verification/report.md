# Subtitles state verification — September 30, 2026

The populated Subtitles pass found a WebKit test-fixture defect. An active service worker let the inspector request reach the real Server despite the test's intended hold. The pending-state assertion failed. A controlled comparison reproduced that failure with service workers allowed and passed with them blocked. This follows [Playwright's request-routing guidance](https://playwright.dev/docs/api/class-page#page-route).

The existing real-API review journey now blocks service workers within its own test group. It also verifies that the initial request reaches the hold. Every existing state, accessibility, timing-choice, preview and file-preservation assertion remains. Captures reset scroll and focus and verify that the skip link is offscreen. Other dashboard journeys retain their default service-worker setting. Production code is unchanged.

See the [responsive gallery](gallery.html) and [run context](run-context.json). Credentials and raw authenticated traces remain outside Git.

## Verification

| Batch | Result |
| --- | --- |
| Fresh production-template history and status | 36/36 passed across Chromium, Firefox and WebKit. History compares loaded, pending, failed and empty states; failed navigation retains loaded rows and offers recovery. |
| Populated dashboard and responsive settings | 27/27 passed across the three engines. Checks include 320px layouts, landscape controls, keyboard access, forced colors, 200% reflow and accessibility. |
| Cleanup previews and language labels | 6/6 passed across the three engines. Dark, light and forced-color checks passed. No cleanup confirmation was submitted. |
| Repaired real-API inspector journey | 3/3 passed across the three engines. Pending, loaded, failed, preview and empty states were checked at 390px and 1440px. The original subtitle's hash stayed unchanged, and the temporary sidecar was removed. |
| Changed-path gate | Passed file cap and diff checks for the single changed test helper. This gate selected no compilation or browser execution. |
| Root file cap | `make max-loc` passed. |
| Evidence secret scan | Gitleaks passed. All 216 initial findings were verified source digests; no scanner exceptions were added. |

The final review source hashes match commit `c1c78d999ca67c3311341be2b87b47d52f655bae`. The previous inspector matrix remains recorded as 2 passed and 1 failed; the diagnostic remains 1 passed and 1 failed. Those runs are not counted among the 72 final passes.

The actual Go HTTPS Server used isolated data, generated CC0 media and a temporary Owner. Its served application CSS and inspector JavaScript/CSS match fresh current-source fixtures. The running binary's SHA256 is recorded. Its `go run` build information has no VCS revision, so this is asset and browser proof, not a container revision attestation. History and status tests route production templates and synthetic responses separately from the actual Server journeys.

## Reproduction and boundaries

Regenerate the production fixtures with `TestWriteUIStateFixtures` and `TestWriteUIStateFixturesSubtitleInspector`. Use a populated local test instance, its TOTP secret and generated-media root. From `apps/subtitles/e2e`, run the recorded commands with `KINOSAIL_BROWSER_MATRIX=full` and one worker. Each batch's context records its command, data, environment, source hashes and result.

The previous [Player layout and runtime fix](../2026-09-30-player-layout-audit/report.md) merged through [PR #387](https://github.com/Kinosail/kinosail/pull/387). All four required PR checks passed, and all four fresh runtime builds installed the fixed OpenSSL package. The exact main run [36774578971](https://github.com/Kinosail/kinosail/actions/runs/36774578971) also passed and promoted both production images. Published manifest digests are retained in the checkpoint.

A local full Go run was not repeated for this test-only change. Hosted required suites remain the publication authority. Full browser-suite certification, Nox deployment, public TLS, physical devices, actual Cast receivers, wearable pairing, long playback and external-provider operations remain separate boundaries. Shared Podman storage is full; unrelated resources and dirty primary-checkout work were preserved.
