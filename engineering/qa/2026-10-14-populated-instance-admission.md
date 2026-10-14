# Populated-instance admission

The full browser archive inventory leaves 225 populated-instance cases without a passing execution in those archives.
The existing `test-instance-check` command selects 22 spec files covering 71 of those cases.
It needs a separate hosted execution because local runtime remains held.

The first hosted baseline uses the existing generated catalog, Owner setup, inventory verification, and connection-settings journey.
It runs one existing journey without retries before expanding the selected suite.
This is admission evidence, not acceptance of all 71 cases.

## Failure cases and retained evidence

The runner must reject non-hosted execution before creating a fixture.
Setup, inventory verification, and browser failures must remain failures.
A skipped or absent journey cannot establish acceptance.
Cleanup must run once for the owned Compose project and leave no containers or volumes.
Private fixture data must be removed after both successful and failed runs.
Lifecycle receipts must exclude the Owner credential and authenticator secret.
They record a bounded operation stage and an `info` or `error` level for success or failure.

Isolated shell contracts inject these failures before implementing the runner.
Real browser journeys cannot reliably force setup or teardown failures while preserving the hosted runner for inspection.
The stand-ins establish lifecycle behavior only; they do not establish container, TLS, catalog, or browser acceptance.

The hosted run retains its exact revision, command, environment, generated-data inputs, browser reports, screenshots, and checksums.
Its production and derived test images must identify the checked-out revision.
The existing browser configuration keeps certificate validation enabled.
The baseline does not add a certificate workaround or change application behavior.
Any setup failure must be inspected before repair.

## Source validation

The lifecycle contracts fail before the runner exists, then pass with its implementation.
The workflow-selection contract fails before adding the explicit `INSTANCE` dispatch choice, then passes.
The complete CI contract suite runs 196 tests with five hosted scanner integration skips.
Its log SHA-256 is `ba7dbf8bd62bbe952e61351ae9b627eab089d3c40994d013893e44ee161ac568`.
Moving the new workflow contract into the fixture-specific test file keeps the source-file limit intact.
All 23 affected contracts pass afterward.
Workflow validation, pinned Actionlint, ShellCheck, shell syntax, source limits, and whitespace checks pass.
These checks do not establish hosted admission.
The failure-stage assertions also fail before adding that diagnostic context, then all four fixture contracts pass.

## First hosted execution

PR #605 contains the admission runner and its closed workflow selection.
Independent source review through `977752b2d90a9853145fb63b1527fcc53118e9fd` finds no blockers.
Manual run `38607117013` selects only the populated admission job at that revision.
It retains actual setup, catalog, browser, and cleanup evidence.

Earlier required CI run `38605993157` passes all 196 CI contracts, including the new lifecycle contracts.
Its separate platform-kit check times out while running `docker compose version` with its existing ten-second limit.
The failing source and platform Compose files are unchanged from the passing parent revision.
The private job log has SHA-256 `f6f9ffb70b41de67a5f0e4c1d3864a35153f953b6884885837176850329f3d1f`.
The cause remains unconfirmed. No timeout, assertion, or gate is weakened.
The next required run includes the committed failure-stage diagnostics.
At `9ebf293e`, the unchanged platform-kit suite passes all five tests in 1.562 seconds.
The complete tooling job passes; its private log SHA-256 is `aca48ef41ab335d443ece8790b9f2263fb9cf6c4e388c336d826fded4a80fc4c`.
That successful execution does not explain the earlier timeout.

The first admission reaches the browser after setup, image identity, and catalog verification pass.
Chromium rejects its first navigation with `ERR_CERT_AUTHORITY_INVALID`, without a retry.
Artifact `11915569090` has SHA-256 `a519f36a912f628278d51dabce26316766e98a81caee3ee135b74e29f9122620`.
Its clean revision, ten source pins, 31 child hashes, and 32 checksum entries verify.
Cleanup succeeds with zero owned containers or volumes and no private fixture directory.
The screenshot is blank; the connection-settings assertions never run.

The repair exports only the fixture's public CA and reuses the existing certificate validator.
[Chromium's Linux certificate documentation](https://chromium.googlesource.com/chromium/src/+/master/docs/linux/cert_management.md) specifies its NSS store and server-CA trust flag.
Use that store with a unique fixture nickname, then remove and verify only that entry during cleanup.
The existing legacy store takes precedence when present.
Keep certificate validation enabled and supply the same public CA to Node requests.

Failure controls cover rejected CA exports, import failure, nickname collision, and failed trust removal.
Rejected or untrusted setup must not launch the browser.
An existing nickname must not be deleted or replaced.
Cleanup failure must remain a failed run, even after the browser passes.
These lifecycle injections cover failures that a real browser journey cannot force reliably.
The new controls fail before the repair, then all five fixture contracts pass.
The failing log SHA-256 is `3977a2056bfa35ecbe36d5f0cd7c80ba540626f12ea1d5ba02bca6dcabf1c516`.
The full source suite passes 197 tests with five hosted scanner integration skips.
Its log SHA-256 is `fe9a32bff35639cf3d182b6ae7ed5ce960b821b39d99ea135c36803ec1ec30a5`.
Workflow validation, Actionlint, ShellCheck, shell syntax, source limits, and whitespace checks pass.
The repaired hosted browser journey remains pending.
