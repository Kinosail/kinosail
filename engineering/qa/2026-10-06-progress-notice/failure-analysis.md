# Progress notice mobile visibility

Before production edits, the frozen Mobile Safari cohort repeatedly showed a blank
Retry saving position row. That observation does not establish a failed save.

The real Go template gives the notice `primary-player-actions` and `hidden`. At
700px and below, a later `.primary-player-actions { display:grid!important }`
rule has the same specificity as `[hidden] { display:none!important }`. The
mobile layout therefore displays an idle or successfully cleared notice.

Failure modes to verify before and after the scoped CSS repair:

- Idle and empty status: hidden notice, no box, no accessible Retry button.
- A real rejected progress request: useful status and appropriate recovery.
- Retryable failure: visible status and a usable Retry button of at least 44px.
- Pending retry: busy notice, real pending copy and disabled Retry.
- Successful acknowledgement: notice and Retry disappear with no reserved space.
- Responsive boundaries: 360, 390, 700, 701, 1440 and 1920px without overflow.
- Existing keyboard focus, live-region semantics and failed-save accessibility.

Extend the existing populated progress E2E before production code. Its real
Server rejection and persisted-state readback remain required. The recoverable
503 is injected at the browser transport boundary; this is controlled failure
coverage within a populated journey, not proof of a naturally failing Server.
Bare DOM progress fixtures use a different class and omit the actual CSS, so
they cannot protect this concrete cascade failure. No unit test is added.

Use the existing serial native Go progress runner with GOMAXPROCS=2, build -p1
and one browser worker. Preserve the failing baseline receipt and candidate
screenshots/computed styles. Keep this separate from physical iPhone/Apple TV,
live Nox deployment, decoded-media repair and exact progress identity proof.

Current main already includes the contextual catalog-return repair in Q14
(2c8ed4ec4). Do not duplicate that implementation based on the older f532 run.
