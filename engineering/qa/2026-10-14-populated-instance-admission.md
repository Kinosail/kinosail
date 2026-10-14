# Populated-instance admission

The retained full browser archives leave populated-instance journeys without a passing execution.
The existing `test-instance-check` command selects 22 spec files covering 81 previously unpassed cases.
Exact identity reconciliation classifies 76 as populated journeys and five as isolated production-template cases.
It needs a separate hosted execution because local runtime remains held.

The first hosted baseline uses the existing generated catalog, Owner setup, inventory verification, and connection-settings journey.
It runs one existing journey without retries before expanding the selected suite.
This establishes fixture admission; acceptance of the broader selection requires its own execution.

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
The repaired hosted browser journey passes at `18f91de79793fa876af1c31faf403d5f884b71a7` in run `38608420461`.
Artifact `11917515109` has SHA-256 `c6f69ce1df656284985300bedacbfb2d0f34f21ac8102c7dab0925418a6d562b`.
Its clean source, ten source pins, eleven child hashes, and twelve checksum entries verify.
The derived fixture image identifies the same revision.
The connection-settings journey executes once without retries or global errors.
It passes accessibility assertions and captures 320, 390, and 1440-pixel layouts.
All three screenshots were inspected; full-page captures alone do not establish viewport occlusion behavior.
Cleanup verifies zero owned containers and volumes, removed private data, and removed browser trust.
PR #605 is ready; required CI and protected delivery remain pending.
Its complete layout workflow `38608355808` passes in Chromium, Firefox, and WebKit.
Chromium artifact `11917483431` has SHA-256 `d299341769a1f327d69bb31e36b36e33c9bf35d0b6a62bfa37dabff4a40ef156`.
It verifies 97 Player measurements across 29 flows and 111 Subtitles measurements across 33 flows.
All nine source pins, seven manifest cases, and four login readiness controls verify.
The controls pass without retries; the two isolated Firefox-only cases remain skipped in Chromium.
The required Subtitles browser artifact passes all 38 cases without retries or global errors.
Its SHA-256 is `a2096784607fbcae25f50ad677a26ec0c0c5f835bb4b07854fc579026e466b37`.
The tested synthetic merge `779f6e02efb2d4a0476c381380ad30488d27320d` has the same tree as reviewed head `18f91de7`.

## Broader selected suite

The next explicit `selected` mode reuses the existing 22-spec browser command.
It generates the existing production-template fixtures and retains browser output outside the temporary fixture root.
The inventory maps 71 previously unpassed populated cases into that selection.
Exact identity reconciliation finds five more populated cases under alternate skip labels, plus five production-template fixture cases.
The broader selection therefore targets all 81 previously unpassed cases in those 22 spec files.
The command must run with one worker, no retries, and stop after its first failure.
Each emitted case must pass exactly once; skipped, empty, retried, or failed reports cannot establish acceptance.
The report must contain at least those 81 cases, and later artifact inspection will reconcile their exact identities.
Unknown modes must reject before fixture side effects.
Success and failure must retain the existing resource and certificate cleanup checks.

Isolated lifecycle controls cover unknown selection, omitted execution, skipped reports, and cleanup after browser failure.
They cannot establish actual broader-suite acceptance.
The original admission mode remains the default and keeps its existing time limits.
Independent review identifies that unrestricted forwarding could override the existing single-worker option.
Six negative shell controls reproduce unsupported arguments reaching fixture generation before rejection.
The helper now accepts only the three exact hosted options and rejects everything else before generating fixtures.
Their failing log SHA-256 is `620f710c189b8800a0febd7663c827deba0af800e814098c9d7ee16b9bdc1e67`.
An 80-case negative control also fails before raising the corrected minimum to 81.
Its log SHA-256 is `2d7deddf1a904e95215460adacfde359f92502636bbefb8228536d5e824cd3ff`.
After these corrections, all 201 source contracts pass with five hosted scanner integration skips.
The final log SHA-256 is `ccc082802166d64482e563fc63f135c7869e619808b20e6cbb3a5cefea48108e`.
Independent review confirms the worker guard and corrected count check, with no further source blockers.
Actual selected-suite execution and exact identity reconciliation remain pending.
PR #606 contains the broader selection at `a2f600151c2014433d3d3635f8c3ee1ada665f2c`.
Manual run `38610622218` executes that revision with the explicit `selected` choice.
The private expected-case inventory pins all 81 identities and the unchanged contents of all 22 selected specs.
