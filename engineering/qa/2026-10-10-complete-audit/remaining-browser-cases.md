# Remaining browser cases

This index lists collected cases without a passing execution in the hosted deep run or additional local replays.
Inherited #542 evidence is not subtracted. Other opt-in cases omitted from collection remain environment boundaries.
A skipped or interrupted case is not a pass. Engine-specific gaps remain even when another engine passes.

25 collected cases remain without a pass in these artifacts. Exact suite contexts and per-engine statuses are private.

## browse-return-bfcache.spec.ts

- native BFCache preserves loaded Movie DOM without repeated continuation — Failed prerequisite or assertion; see the audit report.

## player-scrub-process.spec.ts

- generated video previews decode, reuse frames and seek with a visible timeline — requires the isolated synthetic scrub runner

## player-subtitles-lifecycle.spec.ts

- persisted caption restore cancels the old attempt and reloads the selected language — isolated native PageTransitionEvent control for persisted caption lifecycle
- persisted caption restore keeps Off without starting another request — isolated native PageTransitionEvent control for persisted caption lifecycle

## player-subtitles-recovery.spec.ts

- isolated transport: stalled caption body reaches a deadline and keyboard Retry at 390px — requires the opt-in disposable Server runner or explicit isolated transport control
- isolated transport: stalled caption headers reaches a deadline and keyboard Retry at 390px — requires the opt-in disposable Server runner or explicit isolated transport control

## polish-shell.spec.ts

- TMDB connection instructions remain readable without horizontal scrolling — requires an isolated populated Player instance
- administration uses a plain canvas while dark media browsing retains CinemaSail — requires an isolated populated Player instance
- landscape search stays reachable before and after focus on signed-in pages — requires an isolated populated Player instance
- long Server names preserve landscape header controls — requires an isolated populated Player instance

## session-timeouts.spec.ts

- Owner can understand and customize automatic sign-out — requires the populated instance authentication fixture

## show-action-layout.spec.ts

- show actions stay compact with long episode titles at 1024px — requires TestWriteShowActionFixtures exports
- show actions stay compact with long episode titles at 1440px — requires TestWriteShowActionFixtures exports
- show actions stay compact with long episode titles at 1920px — requires TestWriteShowActionFixtures exports
- show actions stay compact with long episode titles at 320px — requires TestWriteShowActionFixtures exports
- show actions stay compact with long episode titles at 390px — requires TestWriteShowActionFixtures exports
- show actions stay compact with long episode titles at 720px — requires TestWriteShowActionFixtures exports

## test-instance-apple-launch.spec.ts

- real Server media supports Apple launch, seek, pause, captions and repeat loads — Requires the disposable local runner

## test-instance-first-install.spec.ts

- fresh install protects the Owner and reaches playback and a new sign-in — requires a dedicated empty Server

## test-instance-large-offline-a.spec.ts

- offline download stores and verifies every transfer chunk — failed and repeated; see QA-013

## test-instance-launch.spec.ts

- real large transfer survives browser restart and plays with the Server disconnected — requires the isolated large-file launch fixture

## test-instance-positive-reentry.spec.ts

- positive Matroska reentry decodes the saved scene through native HLS — requires the populated public test instance

## test-instance-positive-selector.spec.ts

- positive saved selector cold saved6 and separately labeled inline decoder control — requires the populated public test instance
- positive saved selector explicit zero during actual negotiation and separately labeled inline decoder control — requires the populated public test instance

## test-instance-startup.spec.ts

- bounded startup preparation preserves the exact stream and playback priority — Requires disposable synthetic local runner
