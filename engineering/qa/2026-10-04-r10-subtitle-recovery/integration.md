# R10 protected CI integration

The separate integration branch retains the independently reviewed R10 source
and historical evidence. The caption journey now runs before general smoke for
each selected browser in the Player container check. It reuses the existing
fictional `Direct Retry Control.mp4`; it does not add an encoder invocation.
Pagination, the two settings journeys, both progress journeys, and general smoke
remain mandatory. The container script contains 299 lines.

The launcher contract failed before implementation because the helper and
container call were absent. Both contract checks pass after implementation.
The existing three populated-result verifier checks and shellcheck also pass.
Architecture metadata was generated for both consumers; only Player output
changed. No historical source or artifact checksum was rewritten.

At committed integration `2628e40e`, the same helper passed all six actual Go
Server caption cases at 390, 1440 and 1920 pixels. There were zero skips,
unexpected results, flaky results or global browser errors. Every case verified
the delivered immutable asset checksum and the cancelled transport. Keyboard
Retry, active recovered cues, Off and continuing video playback were exercised.
The fixture uses a real HTTP stall and an accelerated deadline clock. It does
not route browser responses. The preserved 12-second fictional media was reused
read-only without encoding.

The [safe native projection](integration/native-proof.json) pins all 12 relevant
source/test/launcher/metadata inputs, six cases and 24 responsive captures.
Its input bytes remain identical after reconciliation with protected R05 main.
The [integration manifest](integration-artifact-sha256.json) pins 38 artifacts.
Original private browser results remain preserved and checksummed. Logs have
readable copies and deterministic gzip copies of their exact original bytes.

Committed Player verification passed source caps, diff checks, shellcheck,
affected Go compilation/tests and browser discovery. It failed only at the
container-native step because the existing Podman socket was unavailable.
Subtitles changed-path verification and the shared player template test passed.
The separate required test-instance attempt failed at the same Podman boundary;
the machine was not restarted. See [check receipts](integration/checks.json).
Hosted required checks remain the authority for full selected suites.

Synthetic persisted page events do not establish actual BFCache admission.
Other browsers, physical devices and deployment remain unverified here. The
integration test-runner binary hash was not captured; committed source and all
six delivered immutable asset hashes were verified. A separate R08 correction
tracks the known responsive hidden-action CSS issue; this batch does not change
that CSS.

## Protected lint repair

The required Player lint job on PR #469 reported cognitive complexity 18 in the disposable caption fixture. The test-only repair extracts the existing fixture setup into a synchronous helper. Mode validation, locking, state resets, generation checks, transport behavior and all assertions remain unchanged. Independent review verified byte-level equivalence before the repair was committed.

At `cc3beadfda7659e34f22dba03856d3019f0265f2`, changed lint passed and all six actual Go caption journeys passed again. Each served asset matched the prior bundle and each stalled transport closed exactly once. The 24 fresh screenshots, source hashes, safe result projection and exact compressed logs are in [the repair evidence](integration/lint-repair/native-proof.json). Committed affected checks passed before their container stage; local Podman remains unavailable. Required hosted checks remain the authority for the complete suites.

Supplementary layout evidence is classified separately. No authentication, security rule, test predicate or timeout was relaxed.
