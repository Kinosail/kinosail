# R06 action recovery checkpoint

Status: implementing. Public Save/Restore controls passed, and all eight isolated browser deadline cases failed as intended. The additive Go receipt implementation is prepared but unverified. Browser recovery and actual native lost-response proof remain pending.

This branch starts at reviewed R07 `95b41857a4e337c2e3e8d335abf3fa4d9a8f6e1b`. R07 production sources are unchanged. R16 preparation is excluded and preserved.

- Two public Go control journeys prepare separate Save and Restore browser fixtures. They check installed cue timing, public fingerprints, recovery bytes, history, and stale Save rejection without additional writes.
- Eight isolated Chromium regressions cover completed Save/Restore with a lost response at 390 and 1440 pixels, stalled dashboard Restore at both widths, and unknown Save/Restore outcomes at 390 pixels.
- A native fixture template and one parameterized native browser journey are prepared. The template allows one selected mutation only, invokes the actual public Go handler, records completion/current/recovery hashes, then withholds the successful response. The journey accepts a fresh Save or Restore fixture at 390 or 1440 pixels. Node-only discovery passed; this fixture has not been built or run.
- Node-only Playwright discovery passed: eight tests, one file. This does not execute Chromium or prove a product failure.
- No production change or new public API is included. The native fixture, linter, container, provider, and deployment have not run for R06.

Discovery command, from `apps/subtitles/e2e`:

```sh
node node_modules/@playwright/test/cli.js test subtitle-action-recovery.spec.ts --list --project=chromium
```

At source `a046ec8553d7eeca985cd8b1204fd6bebce20c73`, both public Go controls passed with zero skips or failures. Save returned `200`; replay with its stale fingerprint returned `409`, preserving current/recovery bytes and the single manual-save history event. Restore returned `204`, exchanged the exact current/recovery bytes, and exposed the two factual history events in newest-first order. The Go command used the R16 compilation overlay, `GOMAXPROCS=2`, and `-p 1`; package time was 6.429 seconds and the bounded command took 12.262 seconds.

The first eight-case isolated Chromium run used the installed `chrome` channel, one worker, no retries, and video off. Its 30-second wall bound expired, producing exit `124`, not a complete suite result. Two terminal failures were reported: Save and Restore at 390 pixels retained a disabled language selector after 45 simulated seconds. The third dashboard case captured its intended `aria-busy="true"` failure, trace, request attachment, and screenshot, but the runner was terminated during teardown. The remaining five cases were not completed. No prerequisite assertion failure was observed in the three captured cases. Each recorded exactly one mutation and zero reconciliation History reads.

All three failure PNGs were inspected. Browser captures preserve the existing scroll position, fixed header, and bottom navigation. The boundary remains isolated: the scripts and HTML are real Go outputs, but completed mutation reads are captured responses and media is aborted. Uncaptured auxiliary shell assets were not part of this proof. No actual native lost-response mutation has executed yet.

The initial aggregate is excluded from full matrix proof. The completed retry retained every assertion and used cached bundled Chromium with an empty `PLAYWRIGHT_CHANNEL`, one worker, zero retries, video off, and a 90-second wall bound. Cached Chromium and its headless shell were both verified at `153.0.8010.12`; every case records that runtime version. The changed environment and larger bound addressed process startup/teardown overhead without changing the simulated deadline or expected behavior.

The retry completed with exit `1` in 18.923 seconds, with eight failures, zero skips, zero flaky cases, and zero prerequisite failures. Each reached its intended first deadline assertion after 45 simulated seconds:

| Case | Width | Observed failure |
| --- | --- | --- |
| Lost completed Save response | 390, 1440 | Subtitle language remains disabled. |
| Lost completed Restore response | 390, 1440 | Subtitle language remains disabled. |
| Dashboard Restore response | 390, 1440 | Action form remains `aria-busy="true"`. |
| Still-pending Save | 390 | Safe editing remains disabled. |
| Still-pending Restore | 390 | Safe editing remains disabled. |

Each case recorded exactly one mutation, zero History reads, and only the initial inspection where applicable. Later reconciliation/navigation assertions were not reached. All eight failure PNGs were inspected. This is complete isolated UI failure reproduction, with the public data-integrity controls reported separately. It is not native actual-write/transport evidence.

The baseline manifest records source and curated artifact hashes, including 30 public fixture files, the full per-case classification, complete receipt, traces, and PNGs. Raw logs, HTML reports, and the earlier partial attempt remain preserved locally. All owned verification processes exited before each slot was released. Headroom was 8.7 GiB before the completed cached run. R16 files were never physically changed by the Go overlay and retained the hashes below.

The concrete proposed Go prepared-receipt contract is in `operation-contract-proposal.md`. It requires read-only independent review of restart, expiry, Owner access, single activation, and audio concurrency before any production/API implementation.

Independent review cleared the frozen baseline evidence at `ebc74cee`: all 14 executed-source hashes, all 105 curated artifact sizes/hashes, the per-case classifications, and all 11 PNGs matched. Semantic review cleared the pinned proposal at `10e191d2ee9ce1a1f9369aa4d4f73e84a06f3a95`. It requires durable digest/running state before launch, admission held until actual application return and process `Wait`, counted concurrent legacy operations with exclusive new receipt jobs, existing draft job correlation, and every prior-process prepared receipt discarded on restart. The receipt-only external-time seam is accepted for controlled expiry/cancellation tests; those remain separate from actual lifecycle/process restart proof.

Thirteen named public regression tests cover strict preparation inputs, capacity, activation/replay/body conflict, Owner/CSRF, a legacy audio process prerequisite control, exclusive/legacy audio admission, prepared/completed lifecycle restart, and failed durable activation followed by restart. The proposed receipt API does not exist yet. Clock-controlled expiry, cancellation-delaying child/pipe settlement, and interrupted-running restart controls remain preparation. Production is unchanged.

The public protocol RED completed at exact `42f3c2a930e9f609c5450ef4cd41ce3335101044`, using offline cached dependencies, `GOMAXPROCS=2`, `-p 1`, `-parallel 1`, the R16 overlay, and a 60-second external bound. It exited `1` in 12.315 seconds without timeout, skips, or prerequisite failure. Of the 13 named controls, one passed and 12 failed at the intended additive-contract boundary:

| Boundary | Result and limit |
| --- | --- |
| Legacy public audio control | Passed in 1.55 seconds. One real local stand-in process supplied fictional one-minute silence; the actual application returned the bounded public audio shape after its process `Wait`. Sidecar and History remained unchanged. This is process/API proof, not playable-media proof. |
| Preparation endpoint | Eleven named controls failed with `404` instead of `201`. Active Owner setup, MFA confirmation, disposable profile creation, and library prerequisites reached the expected preparation assertion. The later receipt authorization, busy, replay, result, and restart assertions were not reached. |
| Header rejection | The remaining named parent failed its five leaves. Unknown, empty, uppercase, oversized, and duplicate operation headers were ignored by the existing Restore route, which returned `204`. Each safe observation recorded changed current/recovery bytes and two History events after one Save prerequisite. This proves the additive protocol is absent; it is not a claim that the legacy authorization contract is broken. |

No receipt worker started. Actual receipt restart/expiry, cancellation settlement, audio concurrency, no-effects rejection, and new-route authorization acceptance remain pending. The protocol RED manifest includes 19 source inventory hashes and five artifact hashes; JavaScript and native templates in that inventory were not executed by this Go run. All hashes verified. The owned Go runner and fictional audio process exited before the slot was released; R16 hashes stayed exact. The `.go.txt` clock controls are preparation only and are excluded from compilation and execution.

Preserved R16 SHA-256 checks:

```text
subtitle_maintenance_test.go: f26e00b3f914130793b72c5187b546244ed4d47222380743899f02aa9f7993fb
subtitle_dashboard_revision_test.go: c0111e937ade97838c3422b89de5a56089efd3e289828598147860615ddd7de6
```

## Complete test-first source checkpoint

Before receipt implementation, the prepared public Go coverage now contains 26 named tests. The additional source covers clock-controlled expiry, cancellation-delaying real child/pipe settlement, interrupted-running lifecycle restart, corrupt or unavailable storage, Owner-bound bounded audio results, safe diagnostics, and admission across every public subtitle write adapter. The legacy general API and web fetch adapters are included. Reviewed cleanup is tested with a fresh public preview digest: conflicting admission must preserve all sidecar bytes, avoid hidden-file renames, and preserve public settings.

These additions have not run. They reference the approved receipt-only `Config.SubtitleOperationTime` boundary, which does not yet exist in production. This is deliberately incomplete test-first source, not another runtime RED or an accepted implementation. The prior exact `42f3c2a9` completed RED and its manifest remain unchanged and independently verifiable through Git.

The child process controls use fictional audio supplied by a real local shell executable. External clock cancellation and actual lifecycle restart are separate tests. Both require running status and exclusive admission to stay held until the application returns after child output and process `Wait`; a failed completion write must expose uncertainty without replay. The delayed child fixture is released and allowed to settle on fixture exit.

Automatic maintenance also needs the shared application admission boundary. Its existing 15-minute throttle remains intact. A short focused run cannot establish the full automatic scheduling interval; that remains an explicit runtime limit alongside source review and the existing lifecycle controls. Draft start/cancel retain their existing explicit job correlation and two-hour bound; no coarse receipt is added.

## Go implementation candidate, not runtime acceptance

The candidate adds three Owner-only public receipt endpoints and opt-in activation on the seven reviewed existing routes. Typed adapters validate the existing request shape before activation. Preparation and the exact body digest/running transition are synced before issuing a receipt or launching work. All current-process prepared validation/busy conflicts remain reusable. Every prior-process prepared ID is discarded; interrupted running work becomes unknown and is never resumed. Completion status records the actual existing application outcome; failed completion persistence exposes uncertainty.

One shared admission boundary counts concurrent legacy work and excludes receipt jobs until it settles. The public manager adapters, older general provider fetch adapters, automatic-sync preview, audio consumers, embedded extraction, provider fetch/upgrade, automatic maintenance, and reviewed cleanup participate. Cleanup's existing opt-in preview, digest, confirmation, renames, and settings behavior are retained. An admission conflict performs none of that work. Draft keeps its existing explicit start/cancel/status workflow and concurrency behavior.

The registry uses strict version-1 JSON, at most 64 receipts and 96 KiB, an installation-root file boundary, `0600` temporary metadata, file sync, rename, and directory sync. Invalid/unreadable registry data fails closed for receipt requests while legacy Owner workflows stay available. Receipt metadata contains no submitted text, raw path, provider output, or audio. At most two bounded transient audio results are retained. Worker admission is released only after application return, including the existing real audio process `Wait`. Browser request cancellation does not cancel a dispatched write; the job follows Server lifecycle and its reviewed operation deadline.

The candidate updates the private Owner/MCP route classifications and OpenAPI document. A lightweight source inventory calculation matched the unchanged 468-route baseline digest before adding the three classified routes; it produced 471 routes and digest `f4ae5a85309bd96129c09c3859bce01d8ab3dad262da39164ae93465f79a6610`. This is source preparation, not execution of the route authorization suite. `make max-loc` and whitespace checks passed. No Go, linter, Chromium, container, provider, or deployment verification has run against this candidate. The 26 public tests were all committed first at `b4d86db0`; no unit test was written after implementation. R16 preparation remains excluded and byte-exact.
