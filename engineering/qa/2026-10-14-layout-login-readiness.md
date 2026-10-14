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
