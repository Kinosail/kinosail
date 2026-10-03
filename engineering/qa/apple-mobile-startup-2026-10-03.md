# Apple mobile playback failure analysis

Public surface: `/watch/{id}`, its Play action, native media events, and existing
`/api/v1/items/{id}/playback-events`. Direct First and subtitle filtering remain.
No player selector or deployment belongs to this change.

Failure modes recorded before isolated test or production changes:

- Metadata preload may stop before a two-second playable buffer. Hiding Play
  while waiting for that buffer prevents the gesture Safari needs to fetch it.
- Muted inline preparation can advance, pause, seek, and save false progress.
- Native fullscreen can reject before metadata. A readiness callback cannot
  replace a new user gesture.
- Calling container fullscreen exposes custom controls on Apple touch devices.
- Dismissing Apple fullscreen can leave a movie playing inline or hide reentry.
- A rejected play or fullscreen request can leave the launcher disabled or busy.
- Pause, seek, selected captions, and saved progress can regress during handoff.
- Desktop and non-Apple touch controls must keep their existing behavior.
- Apple native subtitle menus can offer in-band languages excluded by the web
  preference. The web must continue disabling excluded tracks on track changes.
- Synthetic API calls cannot prove physical Safari fullscreen, Wi-Fi throughput,
  decode performance, or the reported1917 session.

Verification layers: retain existing startup/fullscreen regressions; add isolated
gesture/readiness/failure tests first; run a real local Server with synthetic
media and record build/media hashes, timings, command, revision and artifacts.
Physical iPhone Safari is a separate boundary, never inferred from those runs.

Follow-up browser evidence:

- Independent review found missing theater controls, rejected Play leaving native
  fullscreen open, and saved seeks hiding the launcher. Assertions failed first,
  then passed after the fixes.
- iPad does not share iPhone's automatic fullscreen presentation. Cold iPad Play
  now loads metadata and asks for a fresh Play gesture. Keyboard requests outside
  Apple presentation stay paused without emitting a playing intent. Both
  failure paths reproduced before their production changes.
- The disposable native Go Server journey passed real HTTP media decoding, seek,
  captions, recovery and repeated loads at revision `8e1564e`; startup measured
  1034 ms and 1800 ms. The Apple fullscreen API was simulated. Final source
  changes require a new artifact; this run does not prove physical Safari.
- Further QA uses supported loopback HTTP with TLS disabled and strict browser
  error settings. There are no certificate bypass flags in the Apple runner.

CI and retained-fixture failures, recorded before harness repairs:

- CI Arrival was generated as Matroska stdout. The captured full file has no
  duration, including when ffprobe reads it from a seekable file. Chrome parsed
  the new master and decoded five frames, then reported media error 4 at 848 ms
  while receiving a one-segment unfinished EVENT playlist. No HTTP error or
  mismatched codec was found. This does not identify Chromium's exact rejection.
- Write the same 12-second 1280×720 FFV1 fixture to a finalized regular file
  before copying it out. Keep the existing happy-path assertion and media codec.
- The retained touch-native test installs its simulated fullscreen API after
  script startup, while production capabilities are captured during startup.
  Install that API before the script; retain the same gesture and pause checks.
