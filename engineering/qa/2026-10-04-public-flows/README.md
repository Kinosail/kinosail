# Public flow inventory and evidence

`inventory.json` lists 920 declared operations, registered routes, and native
screen or activity owners at base `d27d55cca01d08ea6b14b24e93d2330c3fca606b`.
Dashboard and Director have no active source here. Their archived repositories
were not changed. API operations come from each app's `api_openapi.json`.
Registered routes supplement those operations. Native screens, activities,
watch views, and App Intents identify additional client entry points.

Each row records its status and boundary. Existing test references are
discovery candidates, not passing coverage. One operation can have many input,
role, state, and format variants. A passing assertion covers only its subset.
Dynamic route variants and every native control require further review.

The inventory statuses describe the initial audit, not current runner totals.
The latest local tester-army/e2e run passed 36 executions with six app-specific
skips. It used real binaries from `0f4b97c8`, composed with main `d4bb501e`,
and the checksummed download/caption test additions. The earlier 32-pass runs
on A57 and `29cd65f7` remain historical, independently reviewed evidence.
Current-head hosted and independent verification remain required.
These results do not support an all-flows-tested claim.

| Public journey | Executable coverage | Current evidence |
| --- | --- | --- |
| Owner setup, MFA, session | `scripts/e2e/tests/owner.setup.e2e.ts` | Both real apps passed |
| Paging, search, rejected queries | `public-flows.e2e.ts` | Both passed; invalid input leaves catalog unchanged |
| Progress persistence | `public-flows.e2e.ts` | Both passed, including negative and oversized writes |
| Viewer create/update/delete | `public-flows.e2e.ts` | Both passed; unknown rating red/green confirmed |
| Session timeout and auth boundaries | `public-flows.e2e.ts` | Both passed; rejected writes leave settings unchanged |
| Phone/desktop library and settings | `public-flows.e2e.ts` | Populated renders passed at 390/1440px |
| Playlist import/reorder/delete | `media-flows.e2e.ts` | Both passed; rejected reorder leaves membership unchanged |
| Real Direct First playback and byte ranges | `media-flows.e2e.ts` | Both real apps passed |
| Prepared original download and integrity | `download-captions.e2e.ts` | Both real apps passed full/source SHA, sealed manifest, bounded ranges, rejection/no-mutation and exact owned-job cleanup |
| Source captions and Off/re-enable | `download-captions.e2e.ts` | Both real apps decoded two sidecar cues and the active English cue; Off/on preserved video source |
| Subtitle preview/save/stale-save/restore | `media-flows.e2e.ts` | Subtitles preview/save/rejection/stale-save/restore passed |
| Direct-only/HLS/offline recovery | Existing `player-experience` and `player-direct-fallback` Playwright suites | Final focused run passed 57 checks with passkeys and worker security |
| Album and natural queue advance | `audio-readers.e2e.ts` | Player decoded both real AAC tracks; negative progress leaves queue unchanged |
| Chaptered audiobook | audio-formats.e2e.ts | Real AAC decode, chapter seek, speed/timer controls and resume passed; timer expiry remains untested |
| PDF reader | audio-formats.e2e.ts | Document/ranges passed; native PDF rendering remains partial |
| EPUB, comic and photo | `audio-readers.e2e.ts` | Player decoded content and persisted chapter position; archive negatives passed |
| Real process restart | `process-restart.e2e.ts` | Both apps retained session, list and progress after replacing the Go child |
| Other web/API roles, states and operations | Inventory candidate test paths | Not run; hosted existing suites remain required |
| Apple iOS/tvOS/watchOS and App Intents | Native `Tests/`, `make -C apps/player client-check` | Not run; source tests do not prove devices |
| Android/mobile/TV/Wear OS | Existing Gradle unit and instrumented tests | Not run; emulator/device and build capacity required |

Production defects were confirmed before fixes. Both apps accepted an
unknown Viewer rating and persisted the profile. Shared policy validation now
rejects that value before persistence. The browser also treated an active HLS
source as Direct Play and retained a stale pending start position. Two focused
source changes preserve HLS recovery and the active playback position.
Existing regression assertions were kept unchanged. A later real-process
negative matrix confirmed that missing or null progress seconds reset saved
state in both apps. The shared adapter now requires seconds and retains an
explicit zero reset. Eighteen rejected input cases preserve the complete
progress state, including its mutation timestamp.

Historical local evidence includes the 57-check focused Chromium run and
Player's reconciled 90.3% Go coverage. Those results apply to earlier source.
Hosted deep run `37263160515` at `9e940771` passed both complete Go race/atomic
coverage suites: Player 89.6% total and Subtitles 91.6% total. It also passed
Player Chromium/Firefox and Subtitles Chromium/WebKit. It failed Player
WebKit cross-tab Resume and rejected two Subtitles Firefox flaky login retries.
The layout job separately failed when its handler fetched a browser Blob URL.

The peer Resume defect now has a real lock-order regression and three stale
state rejection controls. The smallest shared control fix waits for the actual
job lock to release and rechecks the current state. Independent native WebKit
checks passed 11 cases; the real Go-backed native WebKit suite passed 16 cases.
The layout Blob repair passed 12 native WebKit routing/skeleton checks and
API-authenticated real Watch measurements at 390/1440px on macOS HTTP.
That Watch probe does not establish UI sign-in or hosted Linux HTTPS behavior.
The test-instance login helper passed a real held-image regression after
waiting for document and form readiness. The dashboard helper already uses
that milestone; its hosted Firefox stall remains unexplained.

`local-evidence.json` records the earlier hashes and results. Current private
receipts, runner JSON/JUnit, source and binary manifests live under
`/Users/mikeo/Documents/e2e-rollout-2026-10-04/kinosail-final-runtime/evidence/`.
The latest runner report is in
`scripts/e2e/.e2e/runner-final-0f4-with-downloads/`. Hosted artifacts retain
exact-head commands and context. Failed attempts remain separate from passes.

Remaining boundaries include current-head hosted suites, publication,
deployed revision, real providers/receivers and physical devices. Prepared
Server downloads do not establish browser-local storage ownership; the native
browser suites own pause, interruption and byte retention. Native PDF rendering,
timer expiry, complete restore/update flows and every role/format variant remain
untested in the new runner. Existing source, native compilation, browser and
physical-device evidence are separate facts.

Reconciliation with main added three subtitle-operation API entries. Their
prepared/running/completed/unknown protocol is owned by the existing
`subtitle_operation_*_test.go` suites and R06 browser recovery journeys.
See `journeys.md` for journey-level ownership and hosted verification commands.
The new automation-key and My List/collection process journeys passed on
reconciled RAM source. Audiobook playback passed; PDF document delivery passed with native rendering
still partial. Other existing owners and external/device boundaries remain explicit.
