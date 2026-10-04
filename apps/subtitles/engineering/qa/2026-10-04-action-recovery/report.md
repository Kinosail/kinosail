# R06 preparation checkpoint

Status: tests prepared; public runtime reproduction and implementation are pending.

This branch starts at reviewed R07 `95b41857a4e337c2e3e8d335abf3fa4d9a8f6e1b`. R07 production sources are unchanged. R16 preparation is excluded and preserved.

- Two public Go control journeys prepare separate Save and Restore browser fixtures. They check installed cue timing, public fingerprints, recovery bytes, history, and stale Save rejection without additional writes.
- Eight isolated Chromium regressions cover completed Save/Restore with a lost response at 390 and 1440 pixels, stalled dashboard Restore at both widths, and unknown Save/Restore outcomes at 390 pixels.
- A native fixture template and one parameterized native browser journey are prepared. The template allows one selected mutation only, invokes the actual public Go handler, records completion/current/recovery hashes, then withholds the successful response. The journey accepts a fresh Save or Restore fixture at 390 or 1440 pixels. Node-only discovery passed; this fixture has not been built or run.
- Node-only Playwright discovery passed: eight tests, one file. This does not execute Chromium or prove a product failure.
- No production change or new public API is included. No Go test, native fixture, browser runtime, linter, container, provider, or deployment ran for R06 at this checkpoint.

Discovery command, from `apps/subtitles/e2e`:

```sh
node node_modules/@playwright/test/cli.js test subtitle-action-recovery.spec.ts --list --project=chromium
```

The intended first browser failure is a disabled language selector or persistent dashboard `aria-busy` after 45 seconds of simulated elapsed time. Prerequisite failures will be reported separately. Captured completed-read data is isolated transport evidence; a later native fixture must prove actual write completion before withholding a response.

Preserved R16 SHA-256 checks:

```text
subtitle_maintenance_test.go: f26e00b3f914130793b72c5187b546244ed4d47222380743899f02aa9f7993fb
subtitle_dashboard_revision_test.go: c0111e937ade97838c3422b89de5a56089efd3e289828598147860615ddd7de6
```
