# Journey owners and current verification

This table groups user journeys. The 920 inventory rows are entry points,
not 920 journeys. Existing tests listed here own behavior and remain required.
Their source presence alone does not establish current passing coverage.

Hosted commands for the final PR head:

- Each app's existing Go job runs `scripts/ci/test-go.sh <app>` in quick mode.
  That runs the complete app Go packages; deep mode adds race and coverage.
  Shared fixture contracts below execute through their app test owners.
- The existing populated Chromium job runs `scripts/test-container.sh` with
  the required smoke and dedicated populated journeys. Weekly/manual deep
  runs retain the complete Firefox/WebKit suites. This task does not replace them.
- The added Player step runs all tests in `passkeys.spec.ts`,
  `worker-message-security.spec.ts`, `player-experience.spec.ts`, and
  `player-direct-fallback.spec.ts`, with no smoke filter: 57 local assertions.
- Each app's added real-process step runs `scripts/e2e/run.sh <app>`.
  It now discovers 38 setup/test executions across both apps (32 passed,
  six app-specific skips on repaired latest-main source). Player's
  subtitle-editing skip remains a scope boundary.
- Swift compilation/contract tests use `make -C apps/player client-check`.
  Android compilation/unit checks use the existing Android job. Neither job
  establishes physical playback, focus, casting, or background behavior.

At reviewed head e18a2cc, both populated Chromium jobs passed, including the new
real-process runner. Complete deep and layout runs failed with specific Go,
scanner, cross-browser fixture, and security findings. Their repairs remain
pending a new exact-head run. Focused local repairs passed the public startup race,
Subtitles compact status/scroll journeys, Firefox real pagination, and six
Chromium/Firefox/WebKit download recovery cases. Already-DCL Linux navigation
failures remain unreproduced; new bounded diagnostics record transport state.
The final report records run and artifact URLs.

| User journey | Existing executable owner | New real-process coverage / current gap |
| --- | --- | --- |
| Owner setup, sign-in, MFA and recovery | Both apps `internal/server/mfa*_test.go`; `packages/servertest/api_parity_scenarios.go`; `e2e/session-resume.spec.ts` | Owner setup/MFA passed earlier; recovery variants owned by legacy suite, hosted pending |
| Passkeys and session persistence/timeouts | Both apps `e2e/passkeys.spec.ts`, `session-timeouts.spec.ts`, `test-instance-session-persistence.spec.ts`; Player `session_security_boundary_test.go` | Passkeys in local 57 pass; negative timeout persistence passed; full durability hosted pending |
| Viewer policy and credential lifecycle | `packages/identitycore/*profile*_test.go`; `packages/servertest/api_identity_lifecycle.go`; both apps `identity_permissions_test.go` | Unknown rating red/green, create/update/delete and revoked login passed |
| API-key scopes and revocation | Player `api_key_test.go`; `packages/servertest/api_key_scope.go`, `api_identity_lifecycle.go` | Real-process scoped issuance/use/revocation and invalid-scope no-mutation checks passed on both apps |
| Search, sort, filter, paging and catalog views | Player `library_api_*_test.go`; `e2e/library-pagination*.spec.ts`, `test-instance-library.spec.ts` | Populated paging/search and negative query persistence passed; other variants owned by legacy suite |
| My List, playlists and collections | Both apps `playlist*_test.go`, `collection*_test.go`; `packages/servertest/api_parity_scenarios.go` | Real-process playlist, My List/collection lifecycle, reload persistence and rejection/no-mutation checks passed |
| Direct First, compatibility and original playback | Player `playback_*_test.go`, `hls_*_test.go`; both apps `e2e/player-direct-fallback.spec.ts` | Real Player decoded frames/ranges passed; Subtitles source/decode expectation passed; full format matrix remains a gap |
| Offline failure, retry, source transitions | Player `e2e/player-experience.spec.ts`; `e2e/download-resilience.spec.ts` | Complete affected recovery files in 57 pass; real offline decode/network loss remains separate |
| Download preparation, tracks, pause, resume, transfer and ownership | Player `offline_download_test.go`, `downloads_boundary_test.go`, `download_concurrency_test.go`; `e2e/download-pause*.spec.ts`, `download-transfer-recovery.spec.ts` | Six local native browser pause/offline/resume and exact integrity cases passed; real Server/hosted full matrices remain required |
| Resume progress, history, chapters, bookmarks and skip markers | Player `reader_progress_test.go`, `media_bookmarks_test.go`, `playback_timeline_test.go`; shared API parity scenarios | Real video progress/rejection passed; other media variants await legacy hosted evidence |
| Music/audiobooks, audio tracks and queues | Both apps `album_test.go`, Player `audiobook_test.go`, `audio_tracks_test.go`, `audio_formats_test.go` | Existing handler/media owners; Player real AAC album/queue and chaptered M4B playback/resume passed |
| Books, comics, PDFs, photos and archive boundaries | Player `books_test.go`, `reader_boundary_test.go`, `reader_archive_trust_http_test.go`; native reader/photo tests | Existing source coverage; Player EPUB/comic/photo passed; PDF document/ranges passed with rendering partial |
| Casting, receivers, watch rooms and remote players | Player `remote_players_test.go`, `live_events_test.go`; shared API parity collaboration scenarios | Fixture coverage only; real receiver/casting/device integrations blocked |
| Server/library/scanning/playback/subtitle configuration | Both apps `settings_*_test.go`, `scan_test.go`; Player `e2e/settings-discovery.spec.ts`, `layout-audit-configuration.spec.ts` | Account/configuration reachable at phone/desktop; other save/control journeys owned by existing tests |
| Metadata providers, editing, refresh and maintenance | Both apps `metadata*_test.go`, `maintenance_test.go`; shared API parity contracts | Existing fake-service tests; real provider credentials remain blocked |
| Backups, diagnostics, metrics, tasks and updates | Player `api_backup_test.go`; both apps `operations_test.go`, `update_test.go`; shared API parity scenarios | Existing executable owners; full process restore/update remains uncovered in new runner |
| Quick Connect, SSO, trusted HTTPS and public access | Both apps `quick_connect_test.go`, `trusted_https_test.go`; Player `oidc*_test.go`, `remote_access_security_test.go` | Source/service fixtures owned; public DNS/TLS and real IdP remain blocked |
| Jellyfin/Home Assistant integration and Supporter account | Player `jellyfin_*_test.go`, both apps `supporter*_test.go`; `e2e/jellyfin-setup.spec.ts`, `home-assistant.spec.ts` | Existing fixture coverage; live external integration remains blocked |
| Subtitle dashboard, wanted, history, language/provider configuration | Subtitles `subtitle_provider_test.go`, `subtitle_onboarding_test.go`, `subtitle_*_test.go`; `e2e/subtitle-dashboard.spec.ts`, `subtitle-history.spec.ts` | Existing fake provider/populated journeys; new runner has local English sidecar only |
| Subtitle inspect, preview, manual/automatic sync, save, undo and replacement | Subtitles `subtitle_edit*_test.go`, `subtitle_*input*_test.go`; `e2e/subtitle-inspector*.spec.ts` | Preview/rejection/stale save passed; restore passed; automatic/provider variants owned by legacy tests |
| Prepared subtitle operation, uncertain response and factual recovery | Subtitles `subtitle_operation_*_test.go`; `e2e/subtitle-action-recovery*.spec.ts` | Added on reconciled main; legacy prepared-operation checks own protocol; current-head hosted pending |
| Apple browse/setup/details/music/readers/photos/downloads/settings | `apps/player/apps/native/Tests/`; screen rows identify feature owners | All 25 feature screen entries blocked locally; source tests are executable, not device proof |
| Apple TV focus/Top Shelf, Watch remote/heart and App Intents | Native `TVOSFocusTests.swift`, `TopShelfTests.swift`, `WatchRemoteTests.swift`, `LibraryIntentTests.swift`; Watch sources | Runtime/device checks blocked; no macOS substitute claimed |
| Android phone/TV browse/play/read/settings and Wear remote | Android `src/test/` and `src/androidTest/`, including `NativeParityJourneyTest.kt` and `LivePlaybackOverlayTest.kt` | Emulator/device checks blocked; existing instrumented tests remain executable owners |

The new process journeys completed here include scoped key issuance and
revocation, My List/collection persistence, sidecar save/undo, and decoded
media/range delivery. The runner includes applicable negative inputs and
no-mutation assertions. Real Player audio/EPUB/comic/photo and both-app process-restart journeys
passed. M4B audiobook playback/resume passed; PDF native rendering and external/device
variants remain unfinished. Storage and hosted results must resolve before certifying
those or claiming complete active-app journey verification.
