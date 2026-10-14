# Layout login destination readiness

## Contract and failure cases

The layout login helper can follow the optional **Not now** link after signing in.
It must finish loading that destination before the caller starts its next navigation.
An indefinitely pending destination must reject the helper without requesting Library.

The populated-instance helper already has isolated HTTP controls for this contract.
The layout helper has a separate implementation and is not exercised by those controls.
The original click had no explicit destination-load barrier.
The hosted baseline reproduced early completion and premature Library navigation.

The existing fixture now runs both helpers through the same real form and redirect chain.
A held Home script separates navigation commitment from destination load.
The delayed control requires no early return or Library request, then exactly one Library navigation after release.
The blocked control requires the existing three-second navigation timeout and no Library request.
Both controls still require exactly one login submission and one Home request.

The important failures are an early successful return, premature Library navigation, and an ignored destination timeout.
The existing populated Server journeys cannot hold this particular post-login resource deterministically.
These tests are isolated HTTP fixtures, not populated-server E2E acceptance.
No application response or production login code changes in this baseline.

## Evidence

The test retains bounded request counts and helper completion observations before fixture cleanup, including on failure.
The observations omit credentials, URLs, and raw errors.
The browser report and source manifest bind them to the hosted revision and command.
Local browser execution remains held.
The existing browser retries and deadlines have not changed.

### Failing hosted baseline

Run `38596823703` tested baseline `8bd2898b752cbe26441f3886f80821923617c517` through merge revision `58020d27b69c44d351e299723adf4445d0484f07`.
Their Git trees match.
Artifact `11912449326` has SHA-256 `edcf6f631f1bca8503895878c6530332748402fe8df7022dae6f5dd0cfe76de9`.
Its clean revision receipt, ten source pins, 386 child hashes, and 387 checksum entries verify.

Chromium `153.0.8010.12` passed both populated-instance helper controls without retries.
Both layout helper controls failed on their original attempt and the configured retry.
All four failed attempts recorded one login submission, one Home request, and one premature Library request.
The layout helper returned with Home still `interactive` and its held script uninitialized.
These observations were captured before releasing the fixture during cleanup.
The aggregate report has 454 passing executions, two failed cases, 63 skipped executions, and no global errors.
This failed run does not establish populated settings acceptance; that later suite was not reached.

### Repair and pending verification

The layout helper now waits for the **Not now** destination's `load` event alongside the click.
It uses the destination URL from that link, matching the existing populated-instance helper.
The blocked destination still uses the unchanged navigation timeout and must prevent Library navigation.
No application code, retries, deadlines, or assertions change in this repair.
Hosted verification of the repaired controls and populated settings journeys is pending.

## Hosted coverage routing

The existing layout workflow now selects the login readiness controls in Chromium, Firefox, and WebKit.
Changes to either the layout login helper or its readiness tests trigger that workflow.
An extended CI contract fails before this routing change, then passes after it.
Browser execution alone cannot prove that an omitted test file or path filter will run.
The original failure log has SHA-256 `abfd1abfda087f2ed2a34355c2b98bd092ee747d2f9feb803457f4605f8b11e7`.

The local CI contract suite runs 192 tests with five skips for hosted scanner integration.
Workflow validation, pinned Actionlint 1.17.0, source limits, and whitespace checks pass.
The suite log has SHA-256 `9f24f50ec24b403add47dc400a31932d5ef0241d640d11bff9c94ae4722372ea`.
This source validation does not establish browser acceptance.
