# R10: bounded caption loading and direct retry

Baseline: `529eb9cd5440b3f04ed33699c65a607fa84f90d8` (R04 product is frozen).

The public interface is the selected caption track, caption status and recovery
control. Caption transport must not become a prerequisite for video playback.
The current loader fetches same-origin VTT into a bounded Blob, then attaches
that local source to a track. Existing caption tests finish every request or
body; they do not cover a real HTTP peer which leaves either boundary pending.

Failure modes identified before product changes:

- A peer delays response headers forever: fetch never settles and selector
  reselection cannot clear a still-loading entry.
- A peer sends valid headers and a VTT prefix but never ends the body: the
  reader never settles, despite the existing 16 MiB maximum.
- Deadline only covers headers, or restarts on every chunk: total caption
  loading can remain unbounded. Use one 20-second attempt deadline through
  completion, including the browser track load event.
- A timeout reports failure but leaves the reader/request alive. Abort and
  cancel transport, clear timers, detach stale listeners and revoke failed
  object URLs without blocking the failure UI on cleanup.
- Repeated Retry clicks create overlapping requests, or old responses/events
  overwrite the current attempt. Track attempt identity and reject stale
  completion, including after pagehide.
- Retry changes the selected track, its mode, playback state, time or source.
  Reload only the chosen failed track. Keep Off and language preferences.
- Off or a new selected language exposes the previous track's failure/retry.
  Tie status and retry to the current showing track.
- Cross-origin, redirects, wrong type, empty or oversized bytes become caption
  sources. Preserve same-origin, redirect, MIME and byte bounds before src.
- Raw URLs, tokens or arbitrary response errors leak into notices/diagnostics.
  Use fixed failure classes and only bounded validated request correlation.
- Recovery remains hidden in playback settings with no direct accessible
  control, or status is not announced. Expose a clear keyboard Retry next to
  the caption failure and preserve existing settings/controls behavior.

Initial reproduction uses an isolated browser and a real disposable local HTTP
stand-in serving the unmodified caption loader. It leaves headers/body pending
and advances the browser clock beyond the proposed public 20-second deadline.
The shell fixture is intentionally isolated; this verifies running loader and
network behavior, not real Go rendering, media decode or production frequency.
A populated real Go Server journey will follow in the integration owner's
serial build slot, using synthetic local captions and a copied fictional
12-second video from the completed R03 proof. The original remains unchanged.
The video SHA-256 is
`9dbd85e7863d921e209978c8349992e5f8505b0d7ffa468c37b8caf97af3f0a4`. No production
seam, global timer setting or test-only loader parameter will be introduced.

The regression observes failure status, keyboard Retry, recovered real cues,
selected track/mode, unchanged media state, request cancellation and safe
failure diagnostics. Public playback/track invariants derive from the fixture,
not from the implementation's private maps. Existing finite caption and
security cases remain retained for their distinct boundaries.

No production, user-data deletion, devices, media encoders, deployments or
PiP/miniplayer work is included.
