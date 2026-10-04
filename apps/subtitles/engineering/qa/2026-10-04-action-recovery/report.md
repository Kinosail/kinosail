# R06 preparation checkpoint

Status: public Save/Restore controls passed; all eight isolated browser deadline cases failed as intended. The production implementation and actual native lost-response proof remain pending.

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

Independent review cleared the frozen baseline evidence at `ebc74cee`: all 14 executed-source hashes, all 105 curated artifact sizes/hashes, the per-case classifications, and all 11 PNGs matched. Semantic review identified proposal gaps, which the updated proposal now resolves explicitly: durable digest/running state before launch; admission held until actual application return and process `Wait`; counted concurrent legacy operations with exclusive new receipt jobs; and existing draft job correlation preserved. That updated proposal still awaits final semantic clearance.

Seven named public regression tests are prepared for the proposed additive contract, including strict preparation inputs, capacity, activation/replay/body conflict, Owner, and CSRF. They have not run and are not product RED proof. Their proposed API does not exist yet. Restart, expiry, persistence-failure, and cancellation-delaying concurrency controls remain to be prepared after contract review. Production is unchanged.

Preserved R16 SHA-256 checks:

```text
subtitle_maintenance_test.go: f26e00b3f914130793b72c5187b546244ed4d47222380743899f02aa9f7993fb
subtitle_dashboard_revision_test.go: c0111e937ade97838c3422b89de5a56089efd3e289828598147860615ddd7de6
```
