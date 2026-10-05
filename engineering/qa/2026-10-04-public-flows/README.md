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

The original inventory statuses describe the initial audit, not the current
runner totals. The repaired real-process suite passed 32 executions, with six
app-specific skips, on RAM source composed with main a57dfc29. Hosted exact-head verification
remains required. These results do not support an all-flows-tested claim.

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
| Subtitle preview/save/stale-save/restore | `media-flows.e2e.ts` | Subtitles preview/save/rejection/stale-save/restore passed |
| Direct-only/HLS/offline recovery | Existing `player-experience` and `player-direct-fallback` Playwright suites | Final focused run passed 57 checks with passkeys and worker security |
| Album and natural queue advance | `audio-readers.e2e.ts` | Player decoded both real AAC tracks; negative progress leaves queue unchanged |
| Chaptered audiobook | audio-formats.e2e.ts | Real AAC decode, chapter seek, speed/timer and resume passed |
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

The first browser run had six failures: three disk errors and three HLS
assertions. A healthy focused rerun reproduced all three HLS assertions.
After both fixes, all 57 selected existing Chromium checks passed.
The earlier incomplete real-process run was superseded by the repaired
32-pass/six-skip run. Player's full reconciled local Go suite passed with
90.3% total coverage. Subtitles' full local race/atomic coverage suite passed
after fixture repairs; that frozen run predates the final progress-presence
fix. Both apps' focused progress/API race suites and the shared affected
race suites passed after that production fix. Linux hosted gates remain
the authority for the committed revision.

`local-evidence.json` records hashes, data, environment, and run results.
Private reports remain under
`/Users/mikeo/Documents/e2e-rollout-2026-10-04/reports/kinosail-runner-evidence/`
and the sibling `kinosail-*` browser logs and summaries. Earlier trace ZIPs
and task-built binaries were removed after ENOSPC; JSON, Markdown, and failure
screenshots remain. Hosted runs retain full final-revision context artifacts.

Remaining boundaries include the complete populated browser suite, Firefox,
WebKit, race/coverage, containers, release publication, deployed revision,
external provider/casting integrations, and physical devices. Pending, loaded,
empty, and failed native layouts were not inspected in this run. The recovery
change has source-browser failed/pending/loaded evidence, not a new physical
device or complete responsive geometry verdict.

Reconciliation with main added three subtitle-operation API entries. Their
prepared/running/completed/unknown protocol is owned by the existing
`subtitle_operation_*_test.go` suites and R06 browser recovery journeys.
See `journeys.md` for journey-level ownership and hosted verification commands.
The new automation-key and My List/collection process journeys passed on
reconciled RAM source. Audiobook playback passed; PDF document delivery passed with native rendering
still partial. Other existing owners and external/device boundaries remain explicit.
