# Firefox Library navigation timeout

The iOS-backup baseline run also fails before one queued-playback journey can start.
Revision `4fb2d4a5bfa4de9edcedd38c7f5034ba19d6d208`, run `38522246479`, job `115714452714` retains the failure.
The saved-35-second stop case times out in its initial Library `page.goto`, before playback assertions.
The attempt lasts 60.519 seconds, with no retry. Thirty-five preceding cases pass.
The top-level browser suite never starts, so this artifact does not establish full Firefox coverage.

Artifact `11874154902` has SHA-256 `38bda91744ee86eee8e3de92814cd34ae0058bf6c9040faa11aed8d9d62e8055`.
The exact clean revision, ten input pins, 48 uploaded hashes, and 49 checksum entries verify independently.
The trace records a document 200 response in 2.624 milliseconds and successful dependent requests.
The failure screenshot shows the populated Library. The pending navigation has no completion or recorded lifecycle event.
A rendered page does not establish that navigation bookkeeping or DOM readiness completed.

[Playwright issue 42183](https://github.com/microsoft/playwright/issues/42183) reports a similar Firefox symptom.
That report includes live ready-state and lifecycle observations missing from this trace.
Its closed status does not identify a verified upstream repair for this case.
A shared root cause remains unproven. No navigation wait, retry, assertion, or dependency hold changes on this evidence alone.
The current deep run supplies a separate repeat. The original failure remains retained even if that repeat passes.

A recurrence needs bounded live document-readiness, lifecycle, and navigation-identity evidence before any workaround.
Raw URLs, session data, request bodies, and unbounded console output must stay out of diagnostic projections.
No browser or application was run locally during this investigation.
