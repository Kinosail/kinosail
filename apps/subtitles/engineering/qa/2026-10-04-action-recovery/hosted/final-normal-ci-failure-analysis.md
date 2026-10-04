# R06 final normal-CI boundary repair

The post-main focused run `37233940041` at `1e8ba4fe` passed all 33 top-level
and 45 subtest contracts. Its compiled executable digest starts `830c6f79`.
Metadata proof also passed. These results do not cover subsequent source edits.

Normal lint job `111529263618` reported three remaining diagnostics. The
private log SHA256 is
`a6d6cdd24367fc40fc6dc5a39e3b4a36ce80a75b01b61fa0c338a7d075d9703c`.
Raw lint output remains private.

## Structural lint repair

The hosted workflows pin golangci-lint `v2.13.1`. Its cached module metadata
pins fatcontext `v0.10.0` and gosec `v2.28.0`. Reading those dependency sources
did not execute Go or a linter.

- Worker line 47 records the existing context in `operations.active`.
  Fatcontext's function-declaration scan treats this non-declaration assignment
  of a context-valued map element as a nested context. Moving the goroutine
  body would leave that assignment. A one-entry `maps.Copy` records the same
  key and context under the same held mutex. Go 1.27.1's `src/maps/maps.go`
  defines Copy as ordinary destination assignment for each source pair.
  Context creation, request values, lifecycle linkage, outer deadline, cloned
  request, logging, goroutine and cancellation/settlement order stay unchanged.
- History lines 138/139 index expected action and reason slices after a
  combined negative bounds guard. Gosec's dynamic-length recognition accepts
  strict less-than comparisons. Separate positive bounds branches read each
  expected string. A failed bound retains the same fatal message and return.
  Successful values retain the same comparison and failure text. No assertion,
  test declaration, suppression or threshold was removed or relaxed.

The incumbent tests existed before this structural repair. Source inspection
and whitespace validation are local evidence only. Fresh hosted lint, the
33/45 focused proof, metadata and ordinary required gates remain mandatory.

## CodeQL action logging control

An exact `refs/pull/474/merge` query found open medium `go/log-injection`
alerts 99 and 100 in `subtitle_operation.go` lines 121/126. The alert commit
is `24f922efa36072ce0765aabf28213440b90b54bf`. A default-main query had missed
these PR findings. No scanner, policy or alert dismissal is changed.

Prepare a public unsupported-action request containing a fictional secret and
newline inside the existing diagnostics test before changing production
logging. Require rejection, unchanged subtitle bytes, absent recovery data,
empty History, zero analysis starts and private diagnostics. Keep all 33
top-level and 45 subtest names unchanged, with no new subtest.

The existing decoder rejects unsupported actions and passes literal `prepare`
to the rejection logger. This control may pass at baseline; do not manufacture
a runtime failure. CodeQL's dynamic enum arguments remain an actual analyzer
finding, while a public privacy defect is unconfirmed. Keep production logging
unchanged until the parent records the hosted baseline. A later fixed
enum-to-literal normalization must preserve valid labels and correlation.

## Frozen test-first checkpoint

Structural lint source is committed at `5d98760b`. The public privacy control
is committed at `977e01c4b49c6c59f06346f80ffd1815d85078a0`, before a logging
production change. The production logger SHA256 is still
`021c251baeda0cd37275e74ee41f0a0b271b333dc5fcdcd9a7483d5e413a984a`.

The new immutable `protocol-scope-977e01c4.json` selects the same 53 inputs,
33 top-level tests, 45 known subtests and byte-preserving R16 overlay. Exactly
three source pins change: the worker, History helper and diagnostics test.
Its SHA256 is
`63c0bfe58e7e870c3b6b943c9d26cae7ca66d217f1223310e8695eadc2d479f5`.
Both older scopes stay unchanged. The provenance helper changes only its
active scope filename and digest; driver, projection and execution bounds
stay unchanged. `final-normal-ci-source-inspection-977e01c4.json` pins this
source-only boundary. Fresh hosted baseline and lint acceptance are pending.

## Hosted privacy baseline and fixed literal fields

The earlier pending baseline is now historical. Hosted run `37235575973`
at `3c93eac3` passed all 33 top-level and 45 subtest contracts with the logger
unchanged. The new unsupported-action privacy/no-effects control passed;
it did not reproduce a privacy leak. Root admitted the safe proof: compilation
66.635 seconds, public runtime 21.625 seconds, settled groups, unchanged
71,402,410-byte executable digest starting `7d31d692`. Canonical evidence is
root checkpoint `0fc600fc` and remains independent of later source changes.

After that baseline, only the two CodeQL-flagged action fields use fixed
literal normalization. Seven operation labels and three request labels retain
their existing exact values. Every other string produces `unknown`; raw input
is never returned. The small separate switches keep each helper within the
existing cyclop limit of 10; its pinned `v1.2.3` counts default case clauses.
Log levels, messages, request and operation IDs, outcomes, status, response
body and all operation/state/admission behavior stay unchanged. No test,
selector, scanner rule, suppression or policy changes in this repair.

This is source-only evidence. Fresh hosted protocol, lint, metadata, exact
PR-ref CodeQL and normal required gates must verify the final changed bytes.

The literal-field source checkpoint is
`3f465e222a37df4ba2624fd3061c367ae5fd90f7`. Its new immutable
`protocol-scope-3f465e22.json` SHA256 is
`759e8218384a99eeed887d4451463dd4c5deb6269c05e3a94ce9ee6e86e00987`.
Only `subtitle_operation.go` changes among the same 53 selected inputs.
The new logger SHA256 is
`ed9eb9dc894f38e4b989ed4b7df41c645fd13ad101d70d279c245504ed09998b`.
All tests, selector, baseline overlay, older scopes, driver and projection
are unchanged. The provenance helper updates only its active filename/digest.
`fixed-action-source-inspection-3f465e22.json` records exact literal labels,
the accepted pre-change baseline and the unrun final verification boundary.
