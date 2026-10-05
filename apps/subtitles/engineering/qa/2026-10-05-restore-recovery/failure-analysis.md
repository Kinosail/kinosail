# R06 Restore: failure analysis and first public proof

Status: source-only proposal. No Go, browser, formatter or fixture has run for this slice.

Source: main `bd1ea2e148787ae8d4a0a46640bc2a965e10fe6a`, tree `6070e86cc8a3522810218d311cc2d5d6246f02d0`.
Save PR483 is merged. Its controller, tests, fixtures and evidence remain unchanged.
The accompanying test-contract.json pins the inspected sources and proposed case identities.

## Current public behavior

- The inspector's Restore button uses legacy POST `/api/v1/subtitle-library/{id}/restore` with `{language}`.
- The Owner-wrapped route preserves existing request validation, CSRF, visibility and mutation admission.
- A successful legacy Restore returns 204 with no response body. It is not a JSON-body response.
- Optional receipt use already exists: prepare action=restore (201), activate with the known ID (202), then GET that ID.
- Preparation starts no file work. A 202 response proves admission, not completed Restore.
- Receipt reads are private and Owner-bound. There is no current-operation discovery API.
- Completed success carries the application's factual 204 status. Running, unknown, failed or unavailable is not success.
- Prepared IDs expire after five minutes and are unavailable after restart. Running becomes unknown on restart.
- Completed/unknown receipts are retained for thirty minutes; the registry has a sixty-four-record cap.

## Why a public reproduction is needed

`subtitle-inspector.js:20–26` has no fetch/body deadline. The Restore handler at line208 locks inputs,
awaits the legacy response, then awaits `load()` before its finally releases busy controls.
`load()` at line112 also awaits the inspection JSON with no deadline.
Source predicts an indefinite lock under a held response; it does not prove a runtime defect.
The existing Back to library link is outside this lock. Navigation must be exercised, not declared blocked.
Restore currently does not clear the text/file inputs. These incumbent retention behaviors must stay protected.

Restore swaps the current Subtitle File with `.kinosail.bak`; it does not delete the recovery copy.
A second legacy Restore can therefore reverse the first. Timeout, reload, Back or status checks must not replay it.
`restoreLanguage` can return 500 after Restore/history when index refresh fails.
Provider error paths attempt rollback, but a failed response alone never proves that files remained unchanged.

## Disposable Go fixture feasibility

Use only `server.New(server.Config)` and public HTTP routes; no private manager calls or product exports.
Follow the merged Save fixture's independent owned TLS target before reading a controller-owned root.
Use a fresh empty 0700 root with fictional indexed marker data, literal SRT A and independently expected offset cues B.
Configure media tools to an absent owned path, deny media/watch/audio/draft routes and never play the video.
This is actual Go/HTTP/file proof; it makes no decoded-media, encoder, provider or device claim.
Keep setup/auth networking independent from rooted filesystem state, with checked body and listener ownership.
Obtain the real secure cookie, pending TOTP, account-page CSRF, MFA303 and current CSRF through public flows.
Do not reuse setup-page CSRF or fabricate authenticated responses. All secrets exist only at runtime.
Register the exact single item from the actual bounded catalog response, not from a title or guessed ID.
Register a receipt only from an actual matching 201 action/item/prepared response.
Allow only fixed routes and those canonical registered item/receipt routes on the actual loopback TLS authority.

Setup reads A through public inspect, then uses one real public legacy Save with an explicit 500ms offset
and automaticSync=false to create B/current, A/backup and one updated/manual History event.
Assert exact SRT/public export/inspection/fingerprint and History before enabling any Restore fault.
Setup Save is a prerequisite, counted separately from Restore; it does not rerun the Save browser suite.

## First thin slice: four fixed browser cases

Run each fault as its own two-case desktop/phone suite. Case names and proposed bounds are in test-contract.json.

1. Hold Restore headers: run the real Restore handler to completion, capture its actual 204 headers/body privately,
   and witness current=A, backup=B plus exactly one new restored/restore History event before admitting the hold.
   Withhold headers until explicit release or actual client/lifecycle cancellation.
2. Hold inspection body: deliver the real Restore204, then capture the first browser post-Restore inspect200.
   Send its actual headers and withhold its actual JSON body. Private witness reads bypass only this opt-in fault;
   they still call the real authenticated public Server and preserve their actual request context.

Do not call a held204 body a stall. An eventual prepared client may stall on its actual202 JSON body,
but that is a distinct later test and must witness real completion rather than equating admission with success.
First prove four Go controls: legacy swap, receipt activation/replay, held204 headers and held inspect200 body.
The transport controls observe a real read remaining pending for 200ms before release; no scheduler-default shortcut.
Compare all application-owned response headers and actual body privately after release, then join their sole owner.

## Browser assertions and truthful accounting

Bind the actual Restore Request before click; record click time separately from effects-witness time.
Observe disabled state only after the real handler locks inputs, never from a capture-listener microtask.
Editor release must occur by 45000ms from click. A 10000ms observation grace cannot extend that deadline.
Require a finite truthful Restore outcome: authoritative completion or explicit completion uncertainty.
Never require uncertainty when the UI has actually read its known successful receipt and current subtitle.
Never treat fixture-only success, 202, stale initial text or HTTP500 as UI-authoritative completion/rollback.

Before Restore, select an in-memory fictional import and enter an unsaved fictional correction through real controls.
Keep both until user choice; never require arbitrary unsaved text to survive a fresh document.
Observe actual delivered-response completion or requestfailed plus fixture cancellation and joined request ownership.
Restore204 and the held inspection200 each have independent request/body terminal records.
A completed Restore request cannot settle the held inspection body; cancellation does not prove body delivery.
Cancellation safety is separate from delivered-old-response coverage. If no terminal is observed, the case is incomplete.
A separate follow-up must refresh the actual current review read-only, create a real no-audio newer Preview, and
release the old response in the same document before asserting newer correction/preview/status survive.
Do not force Preview using stale B's fingerprint after the Server restored A; that would create a legitimate409.
Unavailable unlock/current-refresh makes that follow-up unattempted/null, never a synthetic edit or PASS.

Exercise Back to library and actual browser Back. Await real rendered current cues, not a fixed sleep.
Read the actual Restore count, current/recovery and History again; require exactly one Restore and no replay.
Sample status and textarea atomically in one synchronous browser evaluation before any awaited private witness.
Missing/wrong DOM nodes fail the prerequisite; no default value may manufacture retention or success.
Each assertion records attempted/completed separately; passed stays null until a completed observation.
Require the exact known expect.soft error multiset by fixed assertion ID and canonical owned source location.
Unknown, excess, missing or interrupted errors block admission; never export raw error text or stack.
Any teardown error, retry, skipped case, source drift or unsettled owned work blocks admission.

## Budgets and artifacts (proposal, not an executed command)

The new pair gets its own explicit 230s runtime/235s external bound and one worker, with 110s per case.
Each case uses a fresh fixture with a 120s lifetime; that is not a lifetime shared across the pair.
The effects witness has at most 10s inside the same click-to55s observation window, never afterward.
Setup20 + observation55 + release5 + terminal5 + Back10 + cleanup5 = 100s, leaving 10s case headroom.
Release/body settlement remain distinct phases. Never shrink a release request to a nominal 1ms.
Keep every incumbent Save/protocol command, timeout, selector and fixture exact.
Root may choose a tighter declared bound before test implementation; no original assertion can be relaxed.

Export only allowlisted booleans, counts, fixed case/stage/assertion IDs, bounded timings and source/tool hashes.
Keep responses, cookies, CSRF, TOTP, URLs, bodies and raw diagnostics private; no raw Playwright error export.
Bind exact source tree, served inspector composition, fixture/test binary, private-root ownership and all process settlement.
All twelve first-slice assertions must be attempted, completed and passed for GREEN; state follow-ups remain separate.
Intended deadline RED requires real auth/Restore/effects/fault prerequisites plus complete case settlement.

## Exact next preparation boundary

Only NEW files under this QA directory are owned. Proposed files are:

- failure-analysis.md and test-contract.json (this proposal)
- fixture/*_test.go, using the reviewed constructor/auth/target/body/witness separations
- e2e/subtitle-restore-recovery.journey.ts and dedicated helpers/config/reporter

The precise fixture/helper inventory and root driver/router integration need review before source implementation.
Use an opt-in `.journey.ts`; do not enter the ordinary default `*.spec.ts` collection.
Fixture helpers stay test-only and under 300 physical lines; ordinary Go tests need a fresh retained disposable root.
No Save module, native, shared package, workflow, existing test or Q47 path is changed by this proposal.

## Remaining compatibility and scope limits

No production Restore fix is authorized by source inspection or by this document alone.
Do not add a new API, mandatory storage, Web Locks, credentials, discovery, automatic write fallback or automatic retry.
Known-ID read-only receipt/current/History reconciliation is available; legacy clients keep their existing204 behavior.
After an uncertain dispatch, any new deliberate Restore must be distinguished from replay: it can swap the files again.
Prepared-but-unactivated is not running; preparation failure must not silently become a legacy write.
Do not infer a fresh-tab/reload operation identity or adopt mandatory history.state without a separate established test.
Audio, Generate, generated drafts, full reload edit persistence, dashboard form Restore and cross-browser acceptance
remain separate surfaces. Newer-preview, failed-after-effects and stale-language/lifecycle callbacks need subsequent public cases.
Existing protocol33top-level/45subtest proof remains a separate unchanged gate, not borrowed evidence.

Next: root/independent source review, then authorized fixture/tests and hosted controls before browser RED.
No local write, process, compiler, formatter, device, ref or dispatch was performed.
