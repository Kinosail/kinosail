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
Actual hosted admission remains pending.
The failure-stage assertions also fail before adding that diagnostic context, then all four fixture contracts pass.

## First hosted execution

PR #605 contains the admission runner and its closed workflow selection.
Independent source review through `977752b2d90a9853145fb63b1527fcc53118e9fd` finds no blockers.
Manual run `38607117013` selects only the populated admission job at that revision.
It retains actual setup, catalog, browser, and cleanup evidence when the job finishes.

Earlier required CI run `38605993157` passes all 196 CI contracts, including the new lifecycle contracts.
Its separate platform-kit check times out while running `docker compose version` with its existing ten-second limit.
The failing source and platform Compose files are unchanged from the passing parent revision.
The private job log has SHA-256 `f6f9ffb70b41de67a5f0e4c1d3864a35153f953b6884885837176850329f3d1f`.
The cause remains unconfirmed. No timeout, assertion, or gate is weakened.
The next required run includes the committed failure-stage diagnostics.
