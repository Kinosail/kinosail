# Player layout audit refresh — September 30, 2026

This batch fixes a narrow WebKit settings defect and updates older browser checks to the current Player design. At 320px, the selected video-format text extended beyond its dropdown and made the page 3px wider than the viewport. The existing zero-overflow check reproduced the defect twice.

Settings dropdowns now clip long selected text inside the control, with ellipsis styling where the browser supports it. The full option labels remain available, and the regression checks focus and selection. The stylesheet cache key advances to `electric-40`. The earlier Search contrast and TMDB instruction fixes remain in [PR #385](https://github.com/Kinosail/kinosail/pull/385).

The initial audit passed 3/13 checks. Several assertions described the previous navigation and settings layout. Source inspection, the current `DESIGN.md`, and populated renders established the current behavior before those assertions changed.

| Contract | Current check |
| --- | --- |
| Navigation | Full-width utility header, 15rem desktop sidebar above 1100px, horizontal navigation at intermediate widths, and fixed bottom navigation at phone widths. |
| Browse | Artwork and results appear in the useful viewport. The actual mobile dock leaves the final control reachable. |
| Landscape Search | Canonical accessible name, usable collapsed and focused actions, safe-area clearance, keyboard focus, forced colors, and no horizontal overflow. |
| Quick Connect | All six code inputs fit between the fixed header and bottom dock. Primary actions remain valid hit targets after scrolling. |
| Settings | Update content clears desktop task navigation. Search preserves the active category until a result is selected. Media Share content can be selected and cleared without submitting the form. |
| Hardware report | A nonempty automatic hardware choice and working technical disclosure on the server being tested. Processor support remains visible; platform-specific hardware is allowed. |

Quick Connect's former scroll-offset assertion failed at 17px, and its form-border assertion failed by 1px. Neither established covered code inputs. The replacement checks the six inputs directly and preserves the existing action hit tests. Media Shares at 320px has about 261px of usable form width; the former 280px minimum could not fit that viewport's gutters. The current test checks readable headings, working selection, control dimensions, clipping, and overflow.

The broad settings-category audit performs 54 accessibility scans and saves 54 captures. Its original 60-second budget expired while saving a screenshot, after preceding assertions passed. The final test uses independent viewport cases, preserving every category, unsaved-value, accessibility, navigation-history, and malformed-hash check. The multi-viewport shell and hardware audits have 120-second budgets.

The first refreshed matrix passed 32/39 checks. Five failures exhausted aggregate audit budgets. One Firefox sign-in stopped while waiting for document load, before layout assertions; its recorded local resources returned HTTP 200 quickly. The seventh failure was the confirmed WebKit overflow. That run remains failed in the evidence.

The first attempted dropdown minimum-width rule did not remove the overflow. A CSSOM diagnostic compared control geometry without changing the page's security policy. It showed that grid-track and maximum-width changes also left the page at 323px. Clipping selected text brought it to 320px. Only that successful rule remains. An earlier inline stylesheet preview was rejected by the existing Content Security Policy; the policy was preserved. Diagnostic scripts are retained as evidence outside the production test target.

See the [responsive gallery](gallery.html) and [run context](run-context.json). All library titles and configuration data are synthetic. Credentials and raw authenticated traces remain outside Git.

## Verification

| Check | Result |
| --- | --- |
| Final populated layout matrix | 54/54 passed across Chromium, Firefox, and WebKit in 14.2 minutes. |
| Widths and accessibility | Configuration and shell checks span 320–1440px; browse includes 1640px. Keyboard focus, full option labels, selection, forced colors, and Axe checks passed where asserted. |
| Final Player Go suite | All packages passed; server 443.793s and command package 390.176s. Several other packages were cached. |
| `make -C apps/player verify-changed` | Passed file cap, diff check, Go compilation and focused tests, CSS lint, and E2E discovery. |
| `make max-loc` | Passed after the final production changes. |
| `make tooling-check` | Passed. Subsequent changes did not alter tooling behavior. |
| Evidence secret scan | Passed with Gitleaks. Synthetic CSRF values in the failed Go cache log were redacted. |

The committed production and test files at `94c972af866a5c7639f5f3b5d9d8f895fccfef5e` match the final run's recorded hashes. The full final matrix remained stable during execution. Intermediate failed attempts are retained separately and are not counted as final passes.

## Reproduction and boundaries

Start the repository's isolated populated Player test instance. Export its URL, `KINOSAIL_TEST_INSTANCE=1`, and its TOTP secret from the local fixture. From `apps/player/e2e`, run:

```sh
KINOSAIL_BROWSER_MATRIX=full KINOSAIL_BROWSER_WORKERS=1 \
  pnpm exec playwright test layout-audit-configuration.spec.ts \
  layout-audit-library.spec.ts layout-audit-shell.spec.ts
```

This host used the production Go HTTPS entry point because shared Podman storage remains full. The baseline server was built at `7583b933e86b2380082c2804212e718acddf0cee`; its Player server and shared package sources matched base main `05da53145bc05235aa510fecc6392ec5928de061`. The server was rebuilt after the fix, and its served stylesheet was checked for the final rule. Exact production and test hashes are in the run context. A trusted loopback `/healthz` request returned `{"status":"ok"}`. No asynchronous production surface changed.

These layout checks do not certify the full browser suite, physical devices, actual Cast receivers, wearable pairing, long playback, external provider operations, public TLS, or Nox deployment. The [native checkpoint](../2026-09-30-native-polish/report.md) retains those separate boundaries. Unrelated dirty primary-checkout work prevents automatic local-main cleanup and was preserved.
