# Nox web Player QA — 2026-09-29

## Run record

- Scope: the actual Nox production web Player, plus an isolated Podman production binary for account, configuration, curation, and offline mutations.
- Initial production revision: `1bf953f4795c5e80c34c978c34730a4ba79a6fef`. Nox reported running, healthy, and zero restarts. Its configured public HTTPS health endpoint returned 200 with certificate verification enabled.
- Fix commit: `71538432`. Reconciled Podman runtime: `6b630812f9826aa16d5db5c890cc13dfcc01ad9a`, including the Safari startup changes from PR #354. Later commits change test fixtures and this record, not the production fix.
- Environment: macOS 27.0 ARM64, the user's signed-in Chrome session, and Playwright 1.63.0 Chromium/Firefox/WebKit. Live production checks used 1728px desktop and 390px phone viewports. A viewport override is not a physical phone.
- Production data: existing populated libraries and original Kinosail sample media. Private media titles, URLs, keys, cookies, and account details are excluded from this report.
- Isolated data: the repository's generated Example movie/show/audio/PDF/EPUB/comic/photo fixtures, local generated TMDB responses, a fixture Owner with MFA, and disposable QA collections, playlists, and Viewers. No production configuration or accounts were changed.
- Evidence is retained locally under `~/Documents/Kinosail QA/2026-09-29-nox-player-web/`. It includes a run manifest, sanitized command logs, generated-media screenshots, and keyboard audit records. Do not publish browser traces containing fixture sessions.

## Reproduce the isolated checks

Run from the task revision in the repository. Use a task-specific project name and test root.

```sh
cd apps/player
KINOSAIL_TEST_PROJECT=nox-player-web-qa ./scripts/test-instance.sh up
KINOSAIL_TEST_PROJECT=nox-player-web-qa ./scripts/test-instance.sh verify
```

Another task already owned UDP port 51821 during this run. Its container was preserved. The fixture used a temporary Compose override instead:

```yaml
services:
  kinosail:
    ports: !override
      - "127.0.0.1::38127"
```

The generated fixture Owner was enrolled and confirmed through the public setup/MFA API. The test secret stayed in the ignored fixture root with mode 0600. Supply `KINOSAIL_TEST_INSTANCE=1`, `KINOSAIL_TEST_TOTP_SECRET`, `KINOSAIL_UI_FIXTURE_DIR`, and `KINOSAIL_E2E_URL` to the populated Playwright commands. Never write the secret to an evidence file.

## Journey coverage

| Journey | Actual Nox production | Isolated production binary / browser evidence | Boundary |
| --- | --- | --- | --- |
| Browse | All 12 main library destinations, navigation editor, collections and media detail pages; desktop and phone; settled artwork and no page overflow | Populated library and reader journeys; 45 discovered pages at each of two widths; 2,315 keyboard controls with no focus failures | No physical remote or phone |
| Search and large libraries | Sample query, absent query, recovery, title sorting, Year sorting, infinite scroll from 100 to 200 cards, reversible title-letter filtering | Search, artwork ratios, episode stills and small-screen detail layouts | Not a controlled performance benchmark |
| Video | Sample MP4 and a real 1920×800 HLS audio-transcode title decoded and advanced; pause, restart, theater and receiver picker | Seeking, adaptive quality, skip marker edits, resume and complete fresh-install journey | Short live playback, not a full-film soak |
| Audio, books and photos | Music and audiobook timelines advanced; EPUB chapter rendered; photo loaded | Music/audiobook playback, PDF/EPUB/comic readers, every media section and generated metadata | Live metadata provider response not independently forced |
| Curation | Existing default collection reproduced the singular count defect | Create, search, add, remove and delete owned playlists/collections; zero/one/many counts; red and green regression | Mutation testing remained isolated |
| Settings and administration | Basic category switching, search result navigation, absent-setting state; configuration, management, remote readiness, shares, agent connections, system and backups at both widths | Home Assistant enable/disable and pairing flow, MFA instructions, navigation customization and session timeout accessibility | No production access grants, backup restore, account deletion or secret changes |
| Offline and Watch Together | Utility pages opened | Real file preparation, verified storage, network disconnect, offline playback and seek; two browser clients synchronized and reconnected; quota, corruption, storage initialization, Web Locks and cancellation boundaries | Large transfers use controlled bytes; no multi-GB production transfer |
| Supporter | Monthly/yearly/one-time chooser, ten levels per choice at both widths; activation disclosure and privacy copy; all three real Polar checkout forms opened | Current three-cadence checkout test and accessibility passed; header badge clearing and malformed-level checks passed | No purchase or real signed activation; five older Supporter presentation tests remain failing below |

No console warning or error was recorded in the sampled live production journeys. Canceled artwork requests during navigation were not treated as defects.

## Confirmed fix

### QA-001 — Singular curation count uses plural grammar (low)

- Expected: a collection or playlist with one title reads `1 item`.
- Actual: detail pages read `1 items`; their list cards already used the singular form.
- Reproduce: open an existing one-title collection, or create a disposable collection/playlist and add one title.
- Repeated evidence: live Nox collection plus both isolated playlist and collection regressions. The unfixed production binary failed the new assertion with received `1 items`.
- Fix: use the existing item count to select singular or plural in the two shared detail templates. Zero and multiple titles retain `items`. No input or persistence behavior changes.
- Regression: `curation-count.spec.ts` checks the complete zero → one → two → one → zero lifecycle and cleans up only its own data. Both tests passed against the rebuilt production binary. Screenshots include the single-title and empty states.

## Verification and test repairs

- Complete Player Go tests passed before and after the fix, and again after reconciling PR #354. Shared-package Go tests passed. `make max-loc` and lint for the changed lines passed.
- Fresh-container security checks rejected unauthenticated, cross-origin, wrong-host and oversized setup requests. Fresh-install smoke runs passed 7/7 in Chromium, 7/7 in Firefox, and 7/7 in WebKit.
- The first populated Chromium audit completed 50 tests: 35 passed and 15 failed. An earlier 441-test exploratory run was interrupted; it is not a completed suite.
- Ten initial failures came from stale or nondeterministic test setup. Repaired checks preserve observable behavior: hold artwork until its pending geometry is measured; keep focus auditing off the unloaded QR camera; pause short fixture media during focus checks; select current settings/navigation/subtitle controls; prevent service workers from bypassing deliberately intercepted scripts; use real worker-owned OPFS for the quota test; provide the synthetic progress endpoint; and exercise current cross-tab cancellation with the stale source identity kept in its original browser page.
- The repaired tests passed in focused reruns. The current checkout test and real offline playback test cover the current interfaces separately from injected boundaries. The local manifest records each command, runtime revision, test data, result and evidence directory.
- `make -C apps/player verify-changed` passed its file cap, diff, Go compile and focused server checks, then failed at the full shared-package lint stage. Full lint reported 110 issues in unchanged files. Changed-line lint reported zero. The required hosted gates use changed-line lint; no gate was disabled or bypassed.

## Remaining evidence boundaries

Five tests in `test-instance-supporter.spec.ts` did not pass: pending legacy signature, old chooser copy, old two-family badge visibility, collected-badge count, and old Home recognition. Current live UI and the current three-cadence checkout test passed. These older assertions must not be counted as green, and collected legacy certificates are not certified by this audit.

Physical iPhone/Safari, Android, AirPlay, Google Cast, DLNA receivers, remote internet access outside the local network, actual supporter purchase/activation, disaster recovery, multi-GB downloads, and long playback soak were not verified. Required CI, image publication, the deployed revision, image scan, TLS and health are separate delivery facts recorded after merge in the local delivery manifest and the task's final report. This audit does not establish universal 100% reliability.
