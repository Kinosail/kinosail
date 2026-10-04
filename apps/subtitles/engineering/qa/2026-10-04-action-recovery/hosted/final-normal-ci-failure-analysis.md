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
