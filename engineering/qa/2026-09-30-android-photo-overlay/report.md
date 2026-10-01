# Android photo overlay verification

Source commit: `2726828a615e7e3d7e2e2a490eb2bd4412adbd31`. Baseline: `1c0ddabe68724556390971409ccb9bbd50ae75e6`.

A loaded light photo hid its white title. The existing title and control overlay now has an opaque black backdrop. The photo remains contained in the same viewport. Zoom, focus, retry and dismissal keep their existing behavior.

The production change adds one modifier to `PhotoScreen`. It adds no inputs, dependencies, flags or test-only production seams.

## Evidence

Tests were written before the production change. On the original owner, six loaded configurations failed at **1:1** title contrast. Recovery and permission checks passed. The repaired owner passes all eight tests at **21:1** contrast. The title and controls fit the tested viewports.

- Phone: 390×844 dp, and 844×390 dp.
- Tablet: 834×1210 dp, and 1210×834 dp.
- TV: 960×540 dp, initial Zoom focus and DPAD center zoom/fit.
- Phone at 1.3× font scale with a long title.
- Delayed HTTP, HTTP503 failure, polite error, retry pending and recovery.
- Missing stream permission: no image request, no retry, disabled Zoom.
- Loaded states: Zoom/Fit and Done work. The lower half of all six photos remains pixel-identical to the baseline.

The public SessionStore saves an encrypted synthetic session. CatalogModel loads the catalog through real loopback HTTP. ArtworkClient fetches and decodes the generated PNG. PhotoScreen receives that image through its normal flow. A test provider supplies the AndroidKeyStore key lookup missing on the JVM. It does not replace CatalogModel or image state.

The first repaired run passed contrast but failed TV zoom because the test used a touch click. The final test sends DPAD key events and checks image focus. The diagnostic failure is retained. No production interaction changed.

## Verification

- Focused PhotoScreen tests: 8 passed.
- Full Android suite: 120 app, 6 watchcore and 5 Wear tests passed. Zero skipped, failures or errors.
- App and Wear debug APK builds passed. The app instrumentation APK build passed.
- `make max-loc` and `git diff --check` passed.
- Post-commit Player `verify-changed` passed its source-file cap and diff checks.
- Gitleaks found no secrets in the QA artifacts. Hosted CI remains pending before publication.

[Open the render gallery](gallery.html). The images and semantics dumps are under `evidence/`. Commands, source hashes, environment and results are in `run-context.json`. The baseline owner and test source allow the contrast failure to be repeated.

## Limits

These are Robolectric native graphics renders of the real client flow against a synthetic HTTP fixture. They are not emulator screenshots, live Server integration, physical-device, TalkBack-service, gesture, frame-rate or signed-store evidence. No Android device was attached and the host had about 1 GB free. The unchanged empty library route is outside this photo-screen test. Missing photo permission verifies the screen's unavailable-content state.

The black backdrop occupies the existing overlay area. It improves legibility while covering that portion of the photo. The underlying photo layout and zoom logic are unchanged.

## Reconciled main

The branch incorporates main `f1e7139ca5b38b7db57280adc2c02059b9d3ef34`. Android source and test hashes remain unchanged, so the 131-test and build evidence still applies. API-key sorting and shared test coverage match the already-merged upstream change. Both current changed-app gates, shared package tests, the latest catalog tests and root tooling pass. Fresh hosted CI is required on the reconciled review head.
