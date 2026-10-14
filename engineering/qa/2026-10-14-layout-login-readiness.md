# Layout login destination readiness

## Contract and failure cases

The layout login helper can follow the optional **Not now** link after signing in.
It must finish loading that destination before the caller starts its next navigation.
An indefinitely pending destination must reject the helper without requesting Library.

The populated-instance helper already has isolated HTTP controls for this contract.
The layout helper has a separate implementation and is not exercised by those controls.
Its click currently has no explicit destination-load barrier.
That source difference is a candidate ordering defect, not a reproduced failure.

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
Hosted baseline execution is pending. Local browser execution remains held.
The existing browser retries and deadlines have not changed.

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
