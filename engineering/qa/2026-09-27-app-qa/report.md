# Player app QA — 2026-09-27

## Run record

- Mode: full audit, focused on Player web. The repository also has Subtitles and Dashboard web apps and Swift iOS/tvOS clients.
- Source before fix: `147afde826aa2c65fd96eb11b392ea23fdc7bfba`. The isolated container was built from this worktree. The post-fix image was rebuilt from the same worktree plus the changes in this report.
- Reconciliation: this change was rebased after the local browser runs. The newer base changed Player and shared Go dependencies, so the local browser image does not prove the final PR build. Fresh local Go and container checks are reported below; hosted CI is the authority for the final PR revision.
- Environment: macOS, local Podman, isolated `kinosail-app-qa-20260927` Compose project and volumes, synthetic media, Playwright 1.63.0 and axe 4.13.0. Browser results are labeled below. No production account or data was used.
- Start: from `apps/player`, set a unique `KINOSAIL_TEST_PROJECT` and an absolute `KINOSAIL_TEST_ROOT`, then run `./scripts/test-instance.sh up` and `./scripts/test-instance.sh verify`. Use `./scripts/test-instance.sh url` for the loopback base URL. Reset with `./scripts/test-instance.sh down --volumes` using the same project and root.
- Evidence: [offline repro trace](evidence/offline-no-locks-red-trace.zip), [offline repro screenshot](evidence/offline-no-locks-red.png), [offline repro video](evidence/offline-no-locks-red.webm), [Viewer MFA repro](evidence/viewer-mfa-owner-copy.png) and [fixed state](evidence/viewer-mfa-fixed.png), [passkey prompt test failure](evidence/passkey-offer-test-race.png), [phone empty search](evidence/search-empty-phone.png), [phone menu](evidence/more-phone.png), and [exploration observations](evidence/exploration.json). The published trace is from a synthetic page without credentials. The iOS launch screenshot stays local because nearby discovery exposed a private Server address.

## Journey coverage

| Journey | Evidence | Result or boundary |
| --- | --- | --- |
| Owner sign-in and Home | Live desktop and phone navigation; default populated Chromium suite | Passed. |
| Library, detail, search and empty results | Live Movies and detail pages, phone search for an absent title, axe scans, screenshots | Result text and recovery copy appeared; no horizontal overflow in the eight captured states. |
| Navigation and keyboard | Phone More menu, Escape, Search focus; existing keyboard tests | Escape closed the menu. Selected default suite checks passed. |
| Playback and interruption | Default populated suite; direct and compatibility startup, bandwidth, recovery, and offline playback checks | Passed in Chromium. This audit did not measure physical playback or long running sessions. |
| Offline storage failure | Existing negative test, new full app test, related resilience suite, browser matrix | Confirmed and fixed QA-001. |
| Viewer permissions and Watch Together | Default profile management checks; Viewer MFA login and copy regression; repaired two-client Watch Together test | The Viewer MFA copy was wrong and is fixed. Watch Together passed after the test created a Viewer session allowed by the test instance's MFA setting. |
| Settings and accessible UI | Live Settings and axe scans on eight captured states; selected populated tests | No axe violations in checked `main` regions. An optional settings search test is blocked by its read-only fixture assumption. |
| Native iOS/tvOS | iOS and tvOS simulator builds; iOS simulator install and launch; 17 focused iOS tests | Builds and tests passed. Authenticated simulator journeys, tvOS focus interactions, physical devices, and Siri Remote input were not checked. |
| Subtitles and Dashboard | Repository inspection only | No runtime QA in this run. Subtitles has the same Owner-specific MFA template in source; its Viewer path is unverified. Configure this skill for each app before claiming coverage. |

## Prioritized findings

### QA-001 — Offline progress opens storage without Web Locks (P2, confirmed, fixed)

**Expected:** When Web Locks are unavailable, the Downloads page should show offline storage as unavailable and avoid opening IndexedDB or starting a transfer.

**Actual before fix:** Startup opened `kinosail-offline-v1` once. No persistence request or file transfer occurred. The page's progress synchronization path checked IndexedDB and connectivity but did not check Web Locks before opening the database. A later progress write would therefore reach a mode the UI had declared unavailable; that write risk is an inference from code, not an observed write.

**Reproduction:**

1. Start the isolated populated Player instance from the pre-fix revision.
2. In a clean Chromium context, make `navigator.locks` unavailable and provide an active local offline worker.
3. Sign in to the test Owner and open `/offline-downloads`.
4. Count calls to `indexedDB.open` for `kinosail-offline-v1`. Expected: zero. Actual: one. The new full app Playwright assertion failed on the old image. The existing fixture test failed twice with `{ databaseOpens: 1, persistCalls: 0 }` against expected `{ databaseOpens: 0, persistCalls: 0 }`.

**Fix and regression:** `syncOfflineProgress()` now returns when Web Locks are unavailable. The existing negative Playwright test passed after the change. A new populated-page Playwright test failed on the old image and passed on the rebuilt image without changing its assertion. The related 18-test offline/download run passed. The negative test passed in Chromium, Firefox, and WebKit.

### QA-002 — Viewer MFA page names the wrong account (P2, confirmed, fixed)

**Expected:** A Viewer required to enroll a second sign-in method should see instructions that apply to their account.

**Actual before fix:** The Viewer reached `/account?mfa=required`, but the page said “One is enough for the Owner account.” This was visible during a two-client Watch Together attempt and in a clean Viewer login. The same template served both roles.

**Reproduction:**

1. In the isolated instance, keep extra sign-in protection required and create a Viewer without a passkey or authenticator.
2. Sign in as that Viewer in a fresh browser context.
3. At `/account?mfa=required`, read the enrollment instruction. Expected: role-neutral guidance. Actual: “Owner account.” [Screenshot](evidence/viewer-mfa-owner-copy.png).

**Fix and regression:** The shared page now says “Add a passkey or authenticator app to continue.” A new browser test asserts the Viewer redirect and exact instruction. It failed on the old image and passed on the rebuilt image without changing the intended assertion. The [after screenshot](evidence/viewer-mfa-fixed.png) shows the corrected page.

### QA-003 — Watch Together test blocked its own Viewer (P2 test reliability, confirmed, fixed)

**Expected:** The two-client test should sign in its temporary Viewer and reach the Watch Together room. **Actual:** The isolated Server required extra sign-in protection, so the new Viewer reached `/account?mfa=required` before joining. This was correct product behavior, not a Watch Together defect. To reproduce, start the isolated instance with Require MFA on and run the Watch Together test with a new Viewer; [the Viewer screenshot](evidence/viewer-mfa-owner-copy.png) shows the blocking page. The test now records the requirement, disables it only for its isolated Viewer run, removes the Viewer, and restores the original requirement. The full five-test offline spec passed after this repair.

### QA-004 — Optional passkey prompt races a browser test helper (P2 test reliability, observed once, repaired)

**Expected:** After Owner sign-in, the helper should dismiss an optional passkey offer and reach Home. **Actual:** In one full-suite run, its immediate “Not now” visibility check ran before the offer appeared; the test then waited for Home while still on `/account?passkey=offer&next=%2F`. To reproduce the observed run, use the isolated Owner without a passkey and run `./scripts/test-instance.sh browser`; the [failure screenshot](evidence/passkey-offer-test-race.png) shows the prompt left open. Its exact recurrence rate is unverified. The trace remains local because login fields may be recorded. The helper now waits for either Home or the offer redirect, dismisses the offer only after that redirect, and keeps its Home assertion. The affected spec passed twice after this repair.

### QA-005 — Optional expanded suite has stale assumptions (P2 coverage gap, open)

The exploratory 22-test Chromium run had 9 passes and 13 failures before the fixes. QA-001 and QA-003 account for two failures and now pass individually. The other 11 were not rerun as a group. The observed failures include an old search accessible name, an exact `0s` motion string where the browser reports `1e-05s` with no animation, a Docker-managed Server name that the test tries to edit, links expected to go directly to `/watch/` where the UI now opens item detail first, and layout geometry measured across separate grid rows. These are test-maintenance leads. No product defect is claimed from them without a fresh behavior check. Their traces were kept local because they may contain sign-in inputs.

### QA-006 — Local certificate and canonical-origin diagnostics (P3 environment boundary)

Exploration recorded certificate-related script errors and a `421` passkey begin response on the loopback test origin. The app's canonical passkey origin differs from that loopback address, and the certificate is local. Password and TOTP sign-in succeeded. These observations are not classified as product bugs; repeat them with trusted HTTPS before making a production claim.

## Commands and results

- `./scripts/test-instance.sh verify`: passed before and after the fix, covering media inventory, playback, API inventory, and local metadata.
- `./scripts/test-instance.sh browser`: 57/57 passed on the pre-fix image; 58/58 passed after the offline storage fix, before the MFA copy fix. With both product fixes, the first run passed 58/59; the one failure was QA-004. Its test helper was fixed and the affected spec was rerun below.
- `pnpm --dir e2e test test-instance.spec.ts --grep 'unavailable Web Locks' --workers=1`: failed on the old image (`["kinosail-offline-v1"]` versus `[]`), then passed on the rebuilt image (1/1).
- `pnpm --dir e2e test test-instance.spec.ts --grep 'unavailable Web Locks|Viewer MFA enrollment' --workers=1`: both new regressions passed on the final rebuilt image (2/2). Two intermediate runs failed because the test used a device-status element absent from an empty Downloads page, then a non-exact heading locator. The assertions about storage and Viewer copy were retained.
- `go test ./internal/server` from `apps/player`: passed after the MFA copy fix.
- `pnpm --dir e2e test test-instance-offline.spec.ts --grep 'offline writes fail closed' --workers=1`: failed twice before the fix (one database open), then passed (1/1).
- `pnpm --dir e2e test download-resilience.spec.ts test-instance-offline.spec.ts --grep-invert 'Watch Together' --workers=1`: 18/18 passed after the fix.
- `KINOSAIL_BROWSER_MATRIX=full ... pnpm --dir e2e test test-instance-offline.spec.ts --grep 'offline writes fail closed' --workers=1`: 3/3 passed (Chromium, Firefox, WebKit).
- `pnpm --dir e2e test test-instance-offline.spec.ts --workers=1`: 5/5 passed after the test fixture repair.
- `pnpm --dir e2e test ui-happy-paths.spec.ts --workers=1`: 8/8 passed after the login wait repair, including the previously failing share-claim journey.
- `./scripts/build-apple.sh ios` and `./scripts/build-apple.sh tvos`: passed. Focused iOS `xcodebuild test` run: 17 tests in `ServerReachabilityTests`, `LibraryEmptyStateTests`, and `PlayerStateTests` passed.
- Skill validation: `quick_validate.py` passed with PyYAML 6.0.2 installed outside the repository.
- `make max-loc` and `git diff --check`: passed. `make worktree-audit` reported 59 unrelated inactive checkouts; no other checkout was changed.

### Checks after rebasing onto current main

- `make -C apps/player verify-changed` and `make -C apps/subtitles verify-changed`: both reached `shared-packages` and stopped at full local `golangci-lint` findings in untouched Go packages. Player had already passed its file cap, diff, compile, and focused server tests. Local lint is 2.13.2; GitHub CI pins 2.13.1 and checks new findings against the PR base. `golangci-lint run --new-from-rev=origin/main` passed in both `packages` and `apps/player` locally.
- `make -C packages test`: first failed when the isolated QA container held management UDP port 51821. After stopping that container, the full package test command passed.
- `make -C apps/player test`: one failure in metadata test temporary-directory cleanup (`metadata: directory not empty`) while background work was active; the affected test passed twice in isolation. The full command is not counted as a pass.
- `make -C apps/subtitles test`: one analogous metadata temporary-directory cleanup failure; the affected test passed twice in isolation. The full command is not counted as a pass.
- `make -C apps/player container-test`: passed on the rebased source, including image build and runtime container checks.
- `./scripts/quality/check-script-lint.sh`: passed after installing the repository-pinned `scripts/quality` dependencies with `pnpm --frozen-lockfile`. Its first attempt could not start because ESLint was not installed in this worktree.

## Remaining verification boundaries

The live browser exploration covered selected Player journeys, not every feature. Automated axe checked eight captured `main` regions and cannot prove complete accessibility; only the targeted keyboard actions above were checked. The full Firefox/WebKit journey matrix, authenticated native media use, tvOS remote focus, physical devices, Subtitles, Dashboard, production TLS, published containers, and deployment health were not checked. No production purchase, email, deletion, or record change was attempted.
