# R06 Save browser failure analysis and test-first candidate

## Snapshot and scope

Read-only base: main a57dfc29de798d15f1c6094b5039f8e20f361062,
tree 48ed234a7236e7d7630f53700fe261ee2014696b. The merged R06
backend is c3f0cb0d47a44cb952ea27107a8e5fb3307c9a3f. Q14 locale/PWA,
all native progress/teardown/media sources, and incoming owner work are preserved.

This is a NEW disposable Save-only fixture/spec. There is no production change.
All source below is unexecuted. App/public authentication, compilation, collection,
controls, actual browser RED, and protected gates are not yet accepted. Existing
R06 protocol 33 top-level +45 subtest evidence/selector/80s/90s bounds, source
scope, overlay, driver and historical receipts remain an independent scope.

## Source-supported hypotheses, not runtime conclusions

Current subtitle-inspector.js awaits fetch and response.json without a finite
Save deadline. Its Save callback lacks the existing inspect/preview revision
ticket; it clears text/file/draft and renders after the response arrives.
A post-header body stall matters separately from a pre-header response stall.
A late callback can overwrite a newer correction after another path unlocks.
Neither browser cancellation nor server receipt failure proves a rollback.

Legacy POST /api/v1/subtitle-library/{id}/apply returns200 review JSON.
Prepared POST /api/v1/subtitle-operations returns201; the old apply route with
the matching X-Kinosail-Operation returns202 admission receipt. The detached
worker can still be running at202. Its exact known-ID GET and actual persisted
SRT/History must establish completion. Apply can fail500 after Save/History if
library refresh fails. That effect must be observed rather than erased.

## Authenticated standalone fixture

Exported server.New(Config) suffices within the Subtitles module; no private
manager/profile access or new product export is used. Use a real loopback TLS
listener with exact Config.AuthURL, RequireAuth=true, SubtitleApp=true and an
owned lifecycle. Self-signed certificate tolerance is limited to this fixture.
Real POST /setup(totp=true, updateMode=manual), public MFA enrollment/confirmation,
the scoped Secure/HttpOnly/Strict cookie, current protected-page CSRF and normal
Owner middleware remain active. Runtime credentials, secret, cookies, CSRF and
actual response bodies stay private. The source contains only fictional cues.

The indexed .mp4 contains a fixture label, not playable media. Video/media/watch
routes are denied and media tool paths are unavailable. Load-preview-text turns
automaticSync off; Preview and Save use text only. No decoded-video, seek, audio,
encoder, provider or ready-draft proof is inferred.

## Four public controls and completion witness

Four Go top-level names, no subtest omission:
- TestSaveHeadersLegacyActualCompletion
- TestSaveBodyLegacyActualCompletion
- TestSaveHeadersReceiptActualCompletion
- TestSaveBodyReceiptActualCompletion

Each starts an empty retained disposable directory and authenticates over actual
TLS. The normal Go runner uses a fresh retained os.MkdirTemp directory when the
opt-in private root is absent; no skip or real user path is introduced. It performs real public inspection, preview, optional201 preparation and
one actual apply. Before withholding response data, the witness reads actual
installed SRT and recovery bytes plus Owner public export/inspect/History:
current export equals the persisted bytes; public fingerprint equals their SHA256;
the two fictional reviewed cues match; recovery retains the original bytes;
exactly one matching en/manual/updated History event exists. Bound apply also
requires the actual known-ID completed/success/status200 receipt. Failed receipt
effects remain factual flags, not a no-effects claim.

The real handler status/headers/body are captured privately within64KiB. Header
mode withholds all headers/body only after witness. Body mode delivers original
application status/headers and flushes, then withholds the original entire body.
A documented transport Content-Length may be added for exact body length.
Controls require a200ms real pending interval and exact released body plus all
application-owned header values. Extra Date/Content-Length/Transfer-Encoding/
Connection are allowed only as transport additions. Header values/digests never
enter artifacts. Request/body readers are cancelled and joined on failure.

Nominal auth, preview, activation, current state or transport failure is a fixture
prerequisite failure, never a product RED. Startup admission409 is not hidden by
an automatic write retry. No mutation or response is mocked. Guarded extra
mutations cannot silently become accepted replay.

## Unconditional browser collection and ordering

Exact spec file: apps/subtitles/e2e/subtitle-save-recovery.journey.ts.
- save-headers-phone: R06 Save headers held after completed write - phone (390)
- save-headers-desktop: R06 Save headers held after completed write - desktop (1440)
- save-body-phone: R06 Save body held after completed write - phone (390)
- save-body-desktop: R06 Save body held after completed write - desktop (1440)

The new file uses .journey.ts so the ordinary *.spec.ts suite remains unchanged.
Fixture helpers are all *_test.go, outside the production coverage denominator.
Collection is unconditional, not environment-driven. Root selects exactly two
with fixed save-headers or save-body routing. One worker, no retries/skips,
trace/video/screenshots off. Save-click time and witness admission time are
recorded separately. A capture-phase real click observer and DOM attribute
observer record browser monotonic click/unlock times without mocked clocks,
forced events or app-state writes. Product editing/navigation release remains <=45s from Save
click;10s observation grace does not permit a55s release. Case bound70s, two-case
runtime150s/external155s; collection15s/20s and compile90s/95s are separate root
driver phases. Original protocol80s/90s is unchanged.

After actual witness, sample real editable text/file/language controls and finite
truthful status. Permit either explicitly uncertain completion or a genuinely
reconciled saved message: only the actual browser known-receipt GET plus saved
inspection, not fixture witness alone, supports that UI claim. Preserve the old
edit while uncertain; ordinary clearing after authoritative success is allowed.

Once controls genuinely unlock, enter a newer text correction in the same
document. Release the held original response there, check newer correction and
status survive, then actually click Back to library and use browser Back. Verify
no activation replay and current server state. Do not force-fill disabled fields.
An unreachable newer-edit/stale check is attempted=false/passed=null. Navigation
still executes through deferred fixed observations. The real Save request observer is installed before the click. After release,
requestfinished plus a privately completed response body and the fixture full-body
write/zero-hold witness prove delivered response coverage. requestfailed plus the
fixture request-context cancellation/zero-hold witness proves cancellation safety
separately; no delivered-response race is claimed for that branch. Neither proven
terminal makes the case incomplete and stale assertions unattempted/null. Two
actual browser animation frames after the observed terminal let pending body
callbacks/rendering settle; no fixed750ms sleep is used. Browser Back waits for
actual rendered reviewed cues within the original case bound, not a250ms sample. Fresh-document unsaved text persistence,
selected import retention and ready-draft provenance require later distinct cases.

## Safe projection and admission

Reporter schema r06-save-browser-v1. Collection and runtime are distinct kinds.
Runtime allowlists15 fixed assertion IDs,8 fixed stages, four fixed IDs/titles/
filename, protocol enum, bounded typed counters/booleans/timings and one served
inspector script SHA256. Unattempted assertions have null pass, never fake failure
coverage or acceptance. Reporter emits no error message/stack/body/URL/auth/
configuration/header/storage/trace. Root validates private safe-results.json as
regular nonsymlink, size/schema/source/ownership bounded, then includes it in
four exact exported receipt/results/source-manifest/artifact-manifest JSONs.

The revised v1 source pin adds exactly six runtime fields: saveRequestObserved,
saveResponseObserved, saveTerminal(unreached|pending|finished|request-failed),
releaseAttempted, responseBodyDelivered, fixtureClientCancelled. The Go snapshot
adds responseBodyWritten/clientCancelled factual booleans. A successful server
Write is never alone treated as browser delivery. Source compilation/type-checks
and runtime compatibility remain unexecuted at this draft checkpoint.

Per-case reporter admission additionally exports retry, totalErrorCount,
knownAssertionErrorIDs, unknownErrorCount and assertionErrorsExact. Only existing
soft-expect errors at exact journey lines/columns21:74,23:71,24:53,26:65 and their
fixed private first-message identities are known. The error multiset must equal
all false/null observations, including both attempted/value errors for each
unreachable newer-edit assertion. Nonzero retry, missing/extra/unclassified error
or interrupted/timedout outcome is incomplete. At most32 fixed known IDs can be
projected; raw message/stack/snippet/location never leaves the private runner.
Exact call-site compatibility is source-assessed against Playwright1.63 and not
runtime-proven.

Intended baseline RED requires both selected cases settled, exact collection,
source/runtime/auth/Preview/fault/effects admission and the actually attempted
45s mandatory recovery failure. Later GREEN requires every intended assertion
attempted and passed plus universal root-owned process-group settlement.
A fixture process exit alone is not universal group proof. Raw logs stay private.
No actual execution is accepted from source formatting or source review.

## Compatibility and later boundaries

No mandatory localStorage/Web Locks or fresh-tab Owner operation discovery.
A later client can use one validated history.state.kinosailSubtitleSave namespace
holding only known receiptID/action=apply/item/language/dispatched, preserving
unrelated state; payload stays in memory. Same-entry reload may GET that ID only,
never automatically activate/replay. Prepared after dispatch is still uncertain.
Unknown, expired, foreign, malformed or unauthorized receipt must not trigger a
write. Receipt201 preparation does not own admission; delayed Restore freshness
requires separate public negative/compatibility proof.

Proposed later Save phases are15s preparation +30s activation + one absolute15s
reconciliation budget. Foreground editing/navigation release occurs at activation
deadline (<=45s worst-case from click), independently of later reads; do not claim
all reconciliation finished within45s after a worst-case15s preparation. Do not
reset15s separately for status/inspect/History or blanket-limit long preview/audio/
draft work. Pending uncertain writes need truthful status and GET-only checking.
Audio, Restore, Generate, maintenance factual summary and the rest of R06 remain
open. No new operation discovery/freshness API or product implementation is
authorized by this fixture candidate.

## Root integration interface

Compile the fixture package once from apps/subtitles:
go test -c -o PRIVATE_BINARY ./engineering/qa/2026-10-05-save-browser/fixture

Public controls use the four-name exact selector and -test.parallel=1, separately
projected from browser assertions. The same compiled test binary serves a case:
PRIVATE_BINARY -r06-serve -r06-root EMPTY_PRIVATE_DIR -r06-fault headers|body

Node helper receives absolute R06_SAVE_FIXTURE_BINARY/R06_SAVE_PRIVATE_ROOT;
reporter receives absolute R06_SAVE_SAFE_RESULTS, R06_SAVE_REPORT_MODE=collection|
runtime, and private Playwright output directory. Root owns bounded compile/
controls/collection/runtime, exact binary/source/tree provenance, final group
settlement, metadata, workflows, publication and merge. Agent staged only the
eleven agreed unreferenced test-first blobs, including the approved test-only
subtitle-save-recovery-network.ts request observer and fixture/owner_test.go
unchanged enrollment extraction. Hosted gofmt is source-only for the five new *_test.go inputs, preserves
old blobs, and cannot be called runtime acceptance.

## Preserved source-only repair checkpoint

Original unreferenced blobs remain preserved. TypeScript boundary validation uses
explicit recursive JSON values/objects and a typed Window intersection, without
prohibited AnyKeyword/UnknownKeyword types. Startup termination awaits owned
child close after both SIGTERM and bounded SIGKILL escalation; inability to settle
remains incomplete. No local formatter, compiler, test, app, browser or fixture
has run. Hosted canonical gofmt may expand dense Go statements; the first root-owned hosted format-only run37256453928 at613da5cd preserved outputs
main158/transport247/witness137/fixture318 lines (artifact11322827629). The318-line
fixture exceeded the300 cap, so parent approved exact enrollOwner extraction to
owner_test.go. Auth logic remains byte-exact. Root must run the separately reviewed
five-file formatter again after this revision; no current unformatted candidate
is claimed to satisfy canonical gofmt or the formatted line cap.
