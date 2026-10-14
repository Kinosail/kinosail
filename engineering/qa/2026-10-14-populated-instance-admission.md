# Populated-instance admission

The retained full browser archives leave populated-instance journeys without a passing execution.
The existing `test-instance-check` command selects 22 spec files covering 82 previously unpassed cases.
Contextual identity reconciliation classifies 77 as populated journeys and five as isolated production-template cases.
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
PR #605 passes required CI `38608355820` and merges as `0d09639cd4799a4cb2470d6cb2e3d24c90f32279`.
Fetched ancestry confirms the reviewed commit is included in protected main.
Exact-main workflow `38613543319` and all nine publication jobs pass.
Registry inspection verifies both apps, both architectures, revision labels, tested candidates, and matching production tags.
The Player digest is `sha256:2d9ac6f79c868c4aee2d74ec43ba728e0b16d9dbaccc2a2f0371bd37837d289c`.
The Subtitles digest is `sha256:c03f8055fd2a8942fb20aef23e5c2516c590896aae602d7b86a6e7061fce44cd`.
Fetched ancestry still identifies that merge as protected main.
Its Player smoke artifact verifies 536 passing executions, 63 skipped executions, and zero retries, flakes, or global errors.
Archive SHA-256: `e8106ead1584eee70057192ef9068d580551867ae506a07bba4a0d6c0ebc09bd`.
Its Subtitles smoke artifact verifies all 38 passes without skips, retries, or global errors.
Archive SHA-256: `dd6dca998d43570c6d9cfcbaaa1dff2eb811db3c596e2a3a614173b093553c13`.
Both browser manifests verify exact clean source, ten input pins, and every evidence checksum.
This establishes hosted delivery and publication; device and deployment acceptance remain separate.
Its complete layout workflow `38608355808` passes in Chromium, Firefox, and WebKit.
Chromium artifact `11917483431` has SHA-256 `d299341769a1f327d69bb31e36b36e33c9bf35d0b6a62bfa37dabff4a40ef156`.
It verifies 97 Player measurements across 29 flows and 111 Subtitles measurements across 33 flows.
All nine source pins, seven manifest cases, and four login readiness controls verify.
The controls pass without retries; the two isolated Firefox-only cases remain skipped in Chromium.
The required Subtitles browser artifact passes all 38 cases without retries or global errors.
Its SHA-256 is `a2096784607fbcae25f50ad677a26ec0c0c5f835bb4b07854fc579026e466b37`.
The tested synthetic merge `779f6e02efb2d4a0476c381380ad30488d27320d` has the same tree as reviewed head `18f91de7`.
Its Player browser artifact verifies 536 passing executions across 466 distinct tests, without retries, flaky results, or global errors.
It also records 63 skipped executions across 22 skipped-only identities.
Ten source pins, 434 child hashes, and 435 checksum entries verify.
Artifact `11918268092` has SHA-256 `6058a7f0d50be5bab98b023f29d34bbcf556c78ddbf33083b6b6743bcea049e1`.

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
PR #606 contains the broader selection at `a2f600151c2014433d3d3635f8c3ee1ada665f2c`.
Manual run `38610622218` executes that revision with the explicit `selected` choice and fails.
The private expected-case inventory pins all 81 identities and the unchanged contents of all 22 selected specs.
The job log reports 74 passing executions, one failure, and twelve cases that did not run.
The title-letter toggle expects zero scroll after clearing the letter; it observes 22 pixels at `ui-happy-paths.spec.ts:34`.
The root cause remains unconfirmed, and the assertion remains unchanged.

Artifact upload also fails because `polish-shell.spec.ts` puts a query string's question mark in a screenshot filename.
The retained job log has SHA-256 `a3c3a7833dfd2b8c2eab468d171e12801b73646daf04144ffcf758f06dca39b7`.
No uploaded manifest, trace, or cleanup receipt is available from that attempt.
The logged pass count therefore does not establish checksummed acceptance of the selected identities.
Encode each route when constructing those screenshot filenames, preserving distinct routes and their query strings.
This follows [GitHub's artifact path validation](https://github.com/actions/toolkit/blob/main/packages/artifact/src/internal/upload/path-and-artifact-name-validation.ts), which rejects question marks in uploaded paths.
Commit `1252849d` contains only that screenshot-name correction and its evidence note.
TypeScript syntax, source limits, and whitespace checks pass; local `verify-changed` remains plan-only.
The next hosted execution must retain the artifact before diagnosis can use its trace and cleanup evidence.
Manual run `38613332317` executes the corrected source with the same browser selection, assertions, and limits.

## Retained title-letter failure and inventory correction

The second selected run fails at the same title-letter assertion and successfully uploads artifact `11920280325`.
Its archive SHA-256 is `3b3aaaa5046614bde8598a83cf6220980aaac731e00ac30db970c8edc03e3703`.
The clean revision, ten source inputs, 22 selected specs, 575 child hashes, and 576 checksum entries verify.
The derived image identifies `1252849d32dfd59e5469ee0c0d40bea43aed9c30`.
All 87 reported cases reconcile by filename, nested describe titles, and exact leaf title.
There are 74 passes, one failure, and twelve unrun cases; the stop-after-first-failure error remains recorded.
Of those passes, 64 are target populated journeys, five are target isolated-template cases, and five were previously covered.
Cleanup confirms zero owned containers and volumes, removed private data, and removed browser trust.
This verifies the screenshot-name repair and retained failure evidence, not full-suite acceptance.

The earlier 81-case inventory collapsed two direct-retry cases with the same leaf title under different viewport descriptions.
The corrected target count is 82: 77 populated journeys and five isolated-template cases.
The inspector now requires unique contextual identities and report IDs instead of ignoring that collision.
An 81-case negative control fails before raising the runner's minimum to 82.
Its log SHA-256 is `d36a73821691f99cd256e4a6e57827a43567160de16a8b8378941780037c7385`.
The minimum remains a coarse runner guard; acceptance requires exact artifact identity reconciliation.

The retained trace records scrolling through 6, 17, 21, and 22 pixels after clearing the title letter.
The selected-letter state records zero; thirteen later scroll evaluations record 22.
The failure screenshot was inspected. The trace SHA-256 is `8d68232bff663c90669615413965074300a339d92ee35278f8193fe173f94823`.
The current HTMX option aligns the replaced main region; its exact scroll extent is not measured in that trace.
The next regression records bounded geometry and settled scrolling before changing production behavior.
It covers desktop, compact phone dialog, tall-phone index, and TV-sized web viewports, with content, focus, and accessibility assertions.
These added viewport assertions remain unexecuted until the next hosted run.

All nine affected lifecycle contracts pass after the count correction.
Their log SHA-256 is `3bc748bed397dfc15a06fb94a35ce5ca9f1b7d56f1b65538227701021008b6ec`.
Native Node TypeScript syntax checks, shell syntax, ShellCheck, source limits, and whitespace checks pass.
No local application or browser runtime starts.

## Earlier loading-review fixture failure

The instrumented baseline at `a285858d23d1b981f89d64e185257c26aa04adde` stops before the title-letter case.
Run `38616953081` records seventeen passes and fails `loading-review.spec.ts:53`.
Artifact `11920935810` has SHA-256 `9bfff4af820d404462e4f840999f79c151e496071f00ac3d8815d4d5e803cb90`.
Its exact clean source, ten input pins, 22 spec pins, supporting helper, 418 child hashes, and 419 checksum entries verify.
All 87 contextual cases remain present. Cleanup removes the owned containers, volumes, private fixture, and browser trust.

The trace records a 12-second movie resuming at 10.228067 seconds; the inspected screenshot shows the same position.
The presentation test then replaces its buffered range with `[0, 7.2]` and sends synthetic loading and readiness events.
That range ends behind the current position, so the production readiness guard correctly keeps loading visible.
The previous `currentTime > 0.25` assertion can pass immediately at the inherited position without proving fresh advancement.
The trace SHA-256 is `4da6cc3b8bfa085b219570c336c31a0dc61024810266e3698942f9dfb886c40f`.

Normalize the real media position before injecting presentation states.
Wait for initialized media, pause, seek to zero, and require the seek to settle with current-frame data.
Then resume and require fresh advancement before freezing the synthetic buffered range.
Check that the frozen media has future data, no error or active seek, and at least two seconds inside that range.
A bounded numeric receipt retains inherited, reset, and frozen states without URLs or media identifiers.
The loading, buffering, settings, and geometry assertions remain intact.
Independent source review confirms the fixture inconsistency and the correction's preconditions.
This remains a populated-page presentation journey with injected media state, not natural bandwidth-recovery proof.
The next hosted execution must verify the correction and reach the unchanged title-letter regression.

## Complete selected execution and remaining scroll diagnosis

Run `38618533354` passes all 87 selected cases at `656c30821262c2b3a5bfc035b23630f584de2a94`.
Artifact `11920844630` has SHA-256 `e51d76735b91fc18945596bf9cf606a3899e547ec054dad819cc89deaa8d6897`.
The exact clean revision, ten inputs, 22 spec pins, supporting helper, 590 child hashes, and 591 checksum entries verify.
All 82 target identities pass: 77 populated journeys and five isolated production-template cases.
The five previously covered cases also pass. There are no skips, retries, flaky results, or global errors.
Cleanup verifies zero owned containers and volumes, removed fixture data, and removed browser trust.

The loading receipt records inherited playback near zero, a settled reset to zero, and fresh advancement to 0.401489 seconds.
Its synthetic endpoint is 7.2 seconds, with future data, no active seek, and no media error.
This execution validates the repaired setup's postconditions but does not exercise an inherited late-resume position.
The next version therefore establishes a real settled seek to 85% before normalization on every run.
It preserves the original presentation assertions and records both inherited and deliberately resumed positions.

All twelve paced title geometry receipts record zero settled scroll across four viewport sizes.
At 1440 by 900, the main region begins at 80 pixels and the document has 22 pixels of scroll extent.
The two phone layouts also have positive scroll extent while retaining zero scroll.
Four retained screenshots were inspected across desktop, compact phone, tall phone, and TV-sized web views.
These results do not close the original rapid-toggle failure.
The new measurements inserted 800 milliseconds and a screenshot between selecting and clearing the letter.
That changed the interaction timing before the previously failing action.

Add a rapid desktop pair before those paced checks, preserving the original select, focus, zero-poll, and immediate clear sequence.
Measure the settled position only after clearing, and retain the exact zero assertion.
Require the same document time origin after filtering so a full navigation cannot provide a false pass.
Independent source review finds no blockers in these controls.
The production title control remains unchanged while the next baseline tests this timing hypothesis.

## Rapid baseline and stable scroll target

Run `38622019193` reproduces the rapid title-toggle failure at `9b2ab330cbf63f5b77017b439cf8db4ec6c4450d`.
Artifact `11923355629` has SHA-256 `c7dcc9d9e434add74b38fc6f44b1cb9d1680dd451700a08e41ab954f8363c02d`.
All 87 identities, ten input pins, 22 spec pins, the helper, 588 child hashes, and 589 checksum entries verify.
There are 74 passes, one failure, and twelve unrun cases. Resource and trust cleanup passes.
The same-document assertion passes. After rapid clearing, all sixteen settled samples record 22 pixels.
The document extent is also 22 pixels; the main region begins at 58 pixels in the scrolled viewport.
All four paced viewport pairs in that same case record zero settled scroll.
This isolates a rapid-interaction failure; slower passing measurements cannot close it.

The loading control passes after a real seek to 10.2 of 12 seconds.
It resets to zero, advances to 0.383317 seconds, and freezes with future data and a 7.2-second buffer endpoint.
The recorded media has no error or active seek. The injected-state presentation assertions pass.

The title-link candidate changes one existing swap attribute to `outerHTML show:top showTarget:body`.
It requests the top of the stable page body instead of the swapped main region.
The installed HTMX source and [HTMX 4 documentation](https://four.htmx.org/reference/attributes/hx-swap/#show) support that selector.
The public interface remains title filtering through the existing browse links.
The retained rapid regression runs unchanged so only the production scroll target changes in this comparison.
The exact timing mechanism remains unconfirmed; this candidate is not accepted until the original failure passes.
The adjacent real HTMX Back journey must also preserve its query, content extent, focus, and saved position.
Current hosted layout coverage checks pending, loaded, empty, failed, and recovery states separately.
Local runtime remains held. Source formatting, source limits, and whitespace checks do not establish browser acceptance.

The first adjacent HTMX Back execution passes at candidate `a9059357` in run `38624433421`.
Artifact `11923616484` has SHA-256 `f08d5a6e8125e3ed2da3e2e9080c84818051347931cdf1ccc4ba20ae27ace40c`.
The exact tree, changed template, 160 source pins, three child hashes, and stopped process groups verify.
The real Server observes the history request; all 34 titles and title-letter focus return in the same document.
Its saved and restored scroll positions are both zero, so it cannot detect an unwanted reset to the page top.
Strengthen this existing adjacent journey by scrolling to 120 pixels before departure and requiring a nonzero recorded position.
Keep its current history, query, content, focus, and coordinate-restoration assertions.
This is an additional regression guard for the scroll-target change; it adds no production seam or duplicate case.
The rapid populated regression and production candidate remain unchanged.
