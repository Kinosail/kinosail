# Apple library recovery failure analysis

Base revision: `2e9ede47a` (contains PR #461). Scope: R05, R11 and R17.
Physical playback, signing, device delivery and PiP are outside this work.

## R05 — cached photo authorization

The public boundary is the native Photo screen loading `/api/v1/items/{id}`
and the authenticated photo resource. A saved photo is useful when the Server
is unreachable. The existing `ClientError.discardsCachedContent` contract
requires removal after explicit 401, 403 or 404 responses.

Failure modes to cover before changing production code:

- A cached photo remains visible after explicit access denial or removal.
- Connection failures wrongly remove a usable saved photo.
- A cancellation from leaving a screen wrongly becomes an error.
- A late response from an old profile replaces the new profile's content.
- Photo titles or credentials appear in diagnostic output.
- Retry reuses a fresh metadata response and cannot revalidate authorization.

The retained regression hosts the production SwiftUI screen against a real
loopback HTTP stand-in with fictional media. It first verifies that the cached
photo is rendered while refresh is pending. It then checks rendered content
after a controlled HTTP response. Existing populated browser E2E tests cannot
exercise the native UIImage view or its saved-content catch path. This is
isolated native rendering evidence, not a production Server or physical-device
claim. Screenshots and result bundles are repeatable artifacts.

## R11 — complete-library refresh

The public boundary is the native Library screen with cached complete or
paginated results followed by a failed refresh. All loaded failures must offer
a reachable Try again action. Retry must repeat the failed offset, force a
remote refresh, keep current titles visible, and avoid duplicate pages.
Authorization denial must continue to clear saved titles. Pending, loaded,
empty and failed screens need separate checks. Siri Remote operation requires
tvOS evidence; iOS rendering is not a substitute.

## R17 — download recovery

The public boundary is the native Downloads screen and its download manager.
A transient authorization or network failure must offer Retry/Reconnect first.
Automatic downloads must retain their pending completion record for retry.
Snapshot refresh failure must preserve verified local media. Device-wide reset
must remain available only after a diagnosed storage failure, with the current
explicit warning and destructive confirmation. Retry must not prepare duplicate
jobs, remove downloaded media, or resume a different Viewer Profile's jobs.
Tests use disposable storage and never remove actual user downloads.

## Verification status

Source paths are confirmed at the base revision. R05 has a confirmed native
baseline below. R11 and R17 runtime reproduction and every green result remain
unverified. No device or Server availability is inferred from source review.

The coordinated iOS run used a fresh task-owned iPhone 18 Pro simulator on
iOS 27.0 (`1E2B50BE-788B-4B79-A43C-C930F5ABEF20`). The initial build exposed a
test-only actor-isolation error. The test now awaits the client identity before
using it in synchronous assertions. This harness error is not a product red.

The next run compiled that test source, then failed to reach test execution
within the 360-second bound. Its private build activity log contains the exact
`Build succeeded` marker. The runner interrupted its own process group.
No test-app process or controlled HTTP request was observed. The result bundle
is incomplete and cannot establish a completed test run or a failed product test.
R11 and R17 were not run after the test-startup timeout. No `.xctestrun` was
generated; a coordinated scheme-based `test-without-building` remains an option.
Production source is unchanged.

Recorded invocation (two sequential baseline iterations):

```text
python3 engineering/qa/2026-10-04-apple-library-recovery/run-native-test.py \
  PhotoAuthorizationJourneys red-runtime \
  1E2B50BE-788B-4B79-A43C-C930F5ABEF20 --iterations 2
```

Private, ignored evidence is retained in
`.verification/apple-library-recovery/ios-PhotoAuthorizationJourneys-red-runtime.{json,log,xcresult}`.
The JSON records the exact command, revision, source checksums and timestamps
(`2026-10-04T07:42:33Z` to `07:48:34Z`). The log SHA-256 is
`2a91106d49036e61ba80dcd593cee91a24af16158ce9b3f0a9c704613c4303c9`.

Read-only follow-up found the task app and test bundle on disk. The task app
passes `codesign --verify --deep --strict`; this does not prove installation or
launch. A later task-simulator lookup reported Shutdown. No physical-device
delivery, signing change, shared cleanup or user-download removal occurred.
The safe result-marker projection and activity-log hashes are retained at
`.verification/apple-library-recovery/build-log-projection.json`.

## Harness review corrections

Independent review found no decoder-contract mismatch. It found that the first
runner did not require matching executed tests, or bind reused binaries to their
pre-build inputs. The earlier attempts remain startup/build evidence only.

The current runner rejects the iOS-only Downloads suite on tvOS. It requires
matching, nonzero executed cases and per-method run results. Zero-test,
skip-only and incomplete results are rejected. It captures native source,
configuration and resource hashes before compilation, verifies they remain
unchanged, and retains signed app/test product hashes after a successful
`build-for-testing`. A replay without building must match that preserved
manifest, its original input file and the toolchain. Old products without a
pre-build manifest are ineligible.

The build and selected test execute sequentially under one overall deadline.
An invocation with `--timeout-seconds 120` therefore bounds both operations.
Use a fresh phase name for another run; receipts are never overwritten.

Photo assertions now wait for the production catalog producer to finish, then
wait for two consecutive observations of the expected rendered state. The
producer observation is a synchronization seam, not the behavior assertion.
Library and Downloads checks wait for accessible content before inspecting
recovery controls. Failure screenshots precede missing-control assertions.

## Confirmed R05 baseline

The next coordinated run booted the existing task simulator. Its fresh
`build-for-testing` succeeded, verified the signed app/test bundle, and retained
the pre-build inputs and compiled-product manifest. The selected test action
completed with exit 65 before the 120-second deadline, from
`2026-10-04T09:09:03Z` to `09:10:35Z`. No native source or product changed
during the run. All eight expected fresh screenshots were retained.

```text
python3 engineering/qa/2026-10-04-apple-library-recovery/run-native-test.py \
  PhotoAuthorizationJourneys red-provenance-120 \
  1E2B50BE-788B-4B79-A43C-C930F5ABEF20 --iterations 2 --timeout-seconds 120
```

The original receipt remains rejected and immutable. It expected `Test Case
Run` nodes. The actual installed Xcode 27 result schema uses `Repetition` nodes
beneath each Swift Testing argument. The preserved tree records failed
repetitions 1 and 2 for each of 401, 403 and 404 at the settled rendered-state
assertion. It records passed repetitions 1 and 2 for the 503 preservation case.
The 403 settled screenshot was visually inspected; the fictional magenta photo
remains visible after the completed access-denial response.

The corrected offline parser requires the exact expected methods, denial
arguments and repetition indices. It rejects missing, skipped, duplicated and
unknown results. Negative probes rejected zero selected tests, a missing
repetition, a skipped repetition, a missing denial argument and a duplicate
index. Independent review cleared the schema correction and supplemental
eligibility. The separate `-schema-validation.json` accepts the original run
and pins every original evidence file plus the corrected parser. It is offline
validation, without a fresh runtime run. Its SHA-256 is
`00ef62a6e5ec993146d2d1b00812e85923178d193e426a70e423688cd884d3b7`.
The original rejected receipt remains byte-exact. R05 is confirmed. Its narrow
production fix now clears the displayed cached image and title on explicit
denial, and rejects cancelled-task error writes. Its completed green evidence
and remaining delivery gates are recorded in [R05-report.md](R05-report.md).

Private evidence prefix:
`.verification/apple-library-recovery/ios-PhotoAuthorizationJourneys-red-provenance-120`.

| Evidence | SHA-256 |
| --- | --- |
| Original rejected receipt | `d5926d8cd71fdaebaed8ef92c8273ea5f42032205889fc5575a4fb97e4f834a9` |
| Pre-build inputs | `d85d472bf48e7ca47334e51661e9a97592d8696f5a8ec0bacbb383c4a90f5b51` |
| Signed compiled-product manifest | `4f16f48ac36e8d8ed6a11f32100a240a642bc63d3bf2229b62c3a438b6e033e4` |
| Selected result tree | `5dbe91728f8b4e00d48549b0274438bc300d8bd05603363884877d713ffafa28` |
| Test summary | `7e755b9b2d2213712c0d03954d605d5204b52c210d456b5bec9d6a32dea1d092` |
| Private process log | `77dc3bcd2d3632352f3bb4b6712323161348f494aa374afa3836e13f8373e109` |

The screenshots record the final iteration's states; the result tree retains
both iterations. This is isolated native rendering against fictional loopback
HTTP, without production Server, physical device, tvOS or deployment evidence.

Before another build, the original signed app, embedded tests and watch products
were copied byte-for-byte to the private `-compiled-products` directory. All 93
file hashes match the original manifest. The separate preservation receipt is
`-product-preservation.json`, SHA-256
`08e6b9dc45b2672fab72af041889c0890c2fe1f3b2aedad6f4cf1733faf88528`.
This preserves the original compiled evidence as later build products change.

A bounded read-only OSLog query on the task simulator found eight correlated
`items` GET failures, two each for 401, 403, 404 and 503, during the original R05
interval. The safe projection records only validated request UUID, operation,
method, status and OSLog message type. The runtime reports message type `Error`
for every record; boundary source uses `Logger.warning` for 4xx and `error` for
503. No raw URL, credential, response body or title is in those projected
messages. Private log and safe projection are separately retained. Projection
SHA-256: `91a3a1860931e9693956e79ef73594fe57a212262f999ac94965525eb0bfae40`.

## R11 and R17 prerequisite baselines

Both selected suites ran twice from the matching original signed manifest,
without rebuilding. R11 completed in 36 seconds, and R17 in 36 seconds. Each
test action exited 65 without timing out. Inputs and products stayed unchanged.
The runner correctly rejected their receipts because accessible-state readiness
failed before the expected recovery screenshots. These are harness failures;
R11 and R17 remain unconfirmed. No Reset confirmation or deletion action was
reached, and no downloaded media was removed.

| Evidence prefix under `.verification/apple-library-recovery` | Rejected receipt SHA-256 |
| --- | --- |
| `ios-LibraryRefreshRecoveryJourneys-red-baseline-matched-60` | `d8a465d2353a093984af18b0f86e57c3cd7371733de67eb132d89d95ba22bf9b` |
| `ios-DownloadManagerRecoveryJourneys-red-baseline-matched-60` | `57995e4a7d60d54c42ae72b5406fb789eb9e3e4cf6062ea4db71b11a9ebe36b9` |

Existing native gallery hosts establish a key window and an active scene. The
two recovery fixtures now follow that same UIKit pattern, restore the previous
key window, lay out before accessibility inspection, and capture separately
named diagnostic screenshots before a specific readiness assertion fails.
Independent source review cleared this alignment. Behavior assertions remain
unchanged. Changed Swift inputs require a fresh build and manifest; the old
compiled manifest must fail closed. Their corrected runtime remains pending.
