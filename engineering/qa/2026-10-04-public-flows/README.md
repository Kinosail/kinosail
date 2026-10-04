# Public flow inventory and evidence

`inventory.json` lists 917 declared operations, registered routes, and native
screen or activity owners at base `d27d55cca01d08ea6b14b24e93d2330c3fca606b`.
Dashboard and Director have no active source here. Their archived repositories
were not changed. API operations come from each app's `api_openapi.json`.
Registered routes supplement those operations. Native screens, activities,
watch views, and App Intents identify additional client entry points.

Each row records its status and boundary. Existing test references are
discovery candidates, not passing coverage. One operation can have many input,
role, state, and format variants. A passing assertion covers only its subset.
Dynamic route variants and every native control require further review.

Current counts: 31 partial contracts passed on an earlier working revision;
848 entries were not run; 38 native entries are blocked by device/build
capacity. The final committed real-process suite awaits hosted verification.
These counts do not support an all-flows-tested claim.

| Public journey | Executable coverage | Current evidence |
| --- | --- | --- |
| Owner setup, MFA, session | `scripts/e2e/tests/owner.setup.e2e.ts` | Both real apps passed |
| Paging, search, rejected queries | `public-flows.e2e.ts` | Both passed; invalid input leaves catalog unchanged |
| Progress persistence | `public-flows.e2e.ts` | Both passed, including negative and oversized writes |
| Viewer create/update/delete | `public-flows.e2e.ts` | Both passed; unknown rating red/green confirmed |
| Session timeout and auth boundaries | `public-flows.e2e.ts` | Both passed; rejected writes leave settings unchanged |
| Phone/desktop library and settings | `public-flows.e2e.ts` | Populated renders passed at 390/1440px |
| Playlist import/reorder/delete | `media-flows.e2e.ts` | Both passed; rejected reorder leaves membership unchanged |
| Real Direct First playback and byte ranges | `media-flows.e2e.ts` | Player passed; corrected Subtitles expectation awaits rerun |
| Subtitle preview/save/stale-save/restore | `media-flows.e2e.ts` | Preview, rejection and stale-save passed; corrected restore status awaits rerun |
| Direct-only/HLS/offline recovery | Existing `player-experience` and `player-direct-fallback` Playwright suites | Final focused run passed 57 checks with passkeys and worker security |
| Other web/API roles, states and operations | Inventory candidate test paths | Not run; hosted existing suites remain required |
| Apple iOS/tvOS/watchOS and App Intents | Native `Tests/`, `make -C apps/player client-check` | Not run; source tests do not prove devices |
| Android/mobile/TV/Wear OS | Existing Gradle unit and instrumented tests | Not run; emulator/device and build capacity required |

Two production defects were confirmed before fixes. Both apps accepted an
unknown Viewer rating and persisted the profile. Shared policy validation now
rejects that value before persistence. The browser also treated an active HLS
source as Direct Play and retained a stale pending start position. Two focused
source changes preserve HLS recovery and the active playback position.
Existing regression assertions were kept unchanged.

The first browser run had six failures: three disk errors and three HLS
assertions. A healthy focused rerun reproduced all three HLS assertions.
After both fixes, all 57 selected existing Chromium checks passed.
The earlier real-process run had 19 passes, two test-expectation failures,
and one unsupported Player subtitle-editing skip. Corrected expectations
were not rerun: ENOSPC prevented the app from starting. No failure was
converted into a product skip. The complete runner remains a hosted gate.

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
