# Remove Player features from Subtitles

Status: Stage 1 complete (inventory and plan). Stage 2 waits for Owner approval.
Revision inspected: `3f292de8` (2026-10-14).

## Problem

Subtitles is defined in [CONTEXT.md](../CONTEXT.md) as subtitle sidecar maintenance with a writable media mount. Its server is still a copy of the Player server with a `SubtitleApp` flag.

- `apps/subtitles/internal/server` has 659 entries. `apps/player/internal/server` has 673.
- 166 entries are byte-identical. 293 have the same name but different content. 200 exist only in Subtitles.
- `server.go` registers every Player feature, then changes only `/`, onboarding, and the settings page when `config.SubtitleApp` is true.
- The static route scan finds 452 route patterns. Only 42 are subtitle routes. About 200 belong to Player.

The result is a large attack surface on the only Kinosail container that can write to media. It includes the Jellyfin API, DLNA, watch rooms, public media shares, HLS transcoding, and downloads. The two copies also drift apart, so a fix in one app does not reach the other.

The [user docs](../docs/user-guide/sharing.md) already say that sharing, Watch Rooms, and public viewing belong to Player. The code does not match the docs.

## How the inventory was made

1. Route patterns: `routeinventory.Discover("apps/subtitles/internal/server", "packages")` from a temporary test in `packages/routeinventory`. The test was not committed. The output is in [the route inventory](player-feature-removal-routes.md).
2. Web UI use: links, forms, and `hx-*` targets in Subtitles templates (`subtitle_*.go`, `subtitle_dashboard*.html`, `subtitle_settings.go`, `subtitle_navigation.go`), and `fetch` targets in `static/subtitle-*.js` and `static/supporter.js`.
3. E2E use: path literals in the 13 specs that `scripts/test-container.sh` runs in CI, and in their local imports.
4. CI use: `.github/workflows/app.yml` and `scripts/test-container.sh`.
5. File comparison: `filecmp.cmp(shallow=False)` on every entry in both server directories.

## What Subtitles actually uses

### Web UI

The Subtitles shell (`subtitleAppHeader`) links only to `/?view=summary|wanted|library|history`, `/settings`, `/supporter`, and `/account`.

The dashboard, inspector, and settings use:

- `/subtitles/manage/*`, `/subtitles/providers/test`, `/subtitles/inspect/{id}`
- `/api/v1/subtitle-library/*`, `/api/v1/subtitle-operations/*`, `/api/v1/subtitles/cleanup*`
- `/settings/subtitles*`, `/settings/libraries*`, `/settings/scans`, `/settings/configuration*`, `/settings/trusted-https*`, `/settings/backups`, `/settings/onboarding`
- `/onboarding/subtitles/*`, `/onboarding/finish`, `/setup`, login, passkey, and session routes
- `/api/v1/supporter*`, `/supporter`
- `/art/{id}` for dashboard posters
- `/media/{id}` for the inspector's `<video>` preview

The inspector also links to `/watch/{id}` as "open the compatible player" when the browser cannot play the file. Two legacy handlers redirect to `/watch/{id}`: `subtitle_provider.go` (`POST /subtitles/{id}/fetch`) and `metadata_provider.go`.

### E2E journeys in CI

`scripts/test-container.sh` runs 13 specs. They use subtitle, settings, login, passkey, profile, supporter, and static routes. Two specs depend on Player features:

- `player-unplayed-progress.spec.ts` tests `/watch`, `/progress`, `/media`, and HLS playlists. It is a Player journey.
- `subtitle-dashboard-language-tests.ts` (in `subtitle-dashboard.spec.ts`) proves cleanup by opening `/watch/{id}` and counting the player's subtitle menu options.

The other 33 specs in `apps/subtitles/e2e` are not run by CI. At least 16 are Player copies, for example `player-*.spec.ts`, `playback-*.spec.ts`, `instant-*.spec.ts`, `jellyfin-setup.spec.ts`, `home-assistant.spec.ts`, and `watch-backdrop-contrast.spec.ts`.

### CI steps that test Player behavior on Subtitles

`.github/workflows/app.yml` runs these only for `inputs.app == 'subtitles'`:

- "Verify Subtitles copied-video clocks": 8 runs of `apps/player/scripts/test-hls-resume-browser.py --app subtitles`.
- Two more steps: about 20 runs of `apps/player/scripts/test-subtitles-copied-prefix.py`.
- `scripts/test-container.sh` lines 109–110 check FFmpeg frame scaling, which serves trickplay.

These steps keep HLS as a required Subtitles gate. Stage 2 must remove them in the same change that removes HLS.

### Container

The `Containerfile` installs `jellyfin-ffmpeg` and VA-API and Vulkan drivers (`mesa-va-drivers`, `mesa-vulkan-drivers`, `i965-va-driver`, `intel-media-va-driver`). `compose.gpu.yaml` and `compose.rkmpp.yaml` pass GPU devices into the container.

Subtitles still needs `ffmpeg` and `ffprobe` for embedded-track extraction, audio for alignment and the inspector, and OCR input. It does not need hardware decoding. The Debian `ffmpeg` package can replace `jellyfin-ffmpeg` after a check that alignment, OCR, and probe results do not change.

## Feature groups

Route counts come from [the route inventory](player-feature-removal-routes.md).

| Group | Routes | Outcome | Evidence |
| --- | ---: | --- | --- |
| Subtitle domain | 42 | keep | Used by dashboard, inspector, settings, MCP, CI journeys |
| Owner platform (setup, login, passkeys, MFA, sessions, settings, backups, configuration, updates, trusted HTTPS, management access, API keys, agent connections, supporter, static shell) | 140 | keep | Used by UI or CI journeys; required by `/api/v1` policy |
| Jellyfin compatibility | 60 | remove | No UI link; docs say not needed |
| DLNA | 7 | remove | No UI link |
| HLS, transcoding, playback | 23 | remove | Only the inspector fallback link and Player-copy journeys |
| Watch rooms | 6 | remove | Docs assign to Player |
| Media shares | 13 | remove | Docs assign to Player |
| Downloads and offline | 14 | remove | No Subtitles UI |
| Playlists, collections, lists | 26 | remove | No Subtitles UI |
| Books, music, browse pages | 16 | remove | No Subtitles UI |
| Markers | 7 | remove | No Subtitles UI |
| Viewing imports and syncs | 13 | remove | No Subtitles UI |
| Quick Connect | 8 | remove | TV and Jellyfin client sign-in |
| Player home and onboarding pages | 7 | remove | Subtitles uses its own onboarding |
| Home Assistant | 18 | decide | See question 1 |
| Metadata editing | 8 | decide | See question 2 |
| Viewer profiles and federation | 30 | decide | See question 3 |
| Remote access | 6 | decide | See question 4 |
| Media files for the inspector | 6 | decide | See question 5 |
| Library catalog API | 2 | decide | Used by one CI journey and MCP |

## Questions for the Owner

1. **Home Assistant.** The integration exposes players, playback commands, and media. Remove it from Subtitles? Recommended: yes. A subtitle-coverage sensor can come back later as a read-only `/api/v1/subtitle-library` consumer.
2. **Metadata.** Matching uses `ProviderIDs` and `ShowProviderIDs`. These come from NFO files and from background TMDB or TVmaze resolution. Recommended: keep background resolution and remove the editing routes (`/metadata/*`, `/api/v1/metadata/bulk`, `PUT /api/v1/items/{id}/metadata`).
3. **Viewer profiles, OIDC, SAML, SCIM, public session timeouts, navigation.** `CONTEXT.md` names only the Owner. Recommended: keep Owner login, passkeys, MFA, and sessions. Remove viewer profiles, SCIM, and public session timeouts. Keep OIDC and SAML only if an Owner signs in with them today.
4. **Remote access kill switch and readiness.** These protect public viewing. Recommended: remove them, and remove `compose.remote-https.yaml` from Subtitles. Note that `apps/subtitles/AGENTS.md` still allows a public gateway "when public viewing is enabled". That line must change too.
5. **Inspector video preview.** Keep `/media/{id}` (Owner-only) for the preview. Remove the `/watch/{id}` fallback link, so unsupported formats show only the audio waveform and text. Keep `/art/{id}`. Remove `/backdrop`, `/download`, and `/person`. Keep `/subtitle/{id}*` only if the inspector loads tracks from it; the scan found no use.
6. **"Playback subtitle choices" setting** (`/settings/subtitles/picker`). It limits the subtitle menu in Subtitles' own player. Without that player it does nothing. Recommended: remove it from Subtitles and leave it in Player. Cleanup stays.

## Shared code for `packages/`

Most Player-only logic already lives in shared packages (`jellyfincompat`, `dlna`, `watchrooms`, `mediashares`, `downloads`, `playback`, `hlsmanifest`, `transcodehardware`, `transcodepolicy`, `quickconnect`, `homeassistant`, `markers`, `viewing`). The app copies are thin adapters of 10–90 lines. Stage 2 deletes the Subtitles adapters. The packages stay for Player.

After Stage 2, these identical or near-identical adapters remain in both apps. Move each one only where both apps need the same behavior:

| Code in both apps | Target | Reason |
| --- | --- | --- |
| `probe*.go` (5 identical), `embedded_subtitle_*.go` (3 identical) | `packages/mediaprobe` | Both apps read embedded text tracks. Drift changes coverage results. |
| `profiles_credentials.go`, `profiles_storage.go`, `passkey_account.go`, `csrf_adapter.go`, `tripwire.go`, `access.go`, `audit.go`, `notification.go` | `packages/identitycore`, `packages/auditjournal` | Owner authentication must behave the same in both apps. |
| `settings_library*.go`, `settings_scans.go` | `packages/settings`, `packages/library` | Library add, remove, and scan rules must match. |
| `updates_http.go`, `server_health.go`, `server_startup.go`, `application_requests.go`, `live_events.go` | `packages/updatecontrol`, `packages/servertransport`, `packages/liveevents` | Lifecycle and transport rules. |
| Sidecar naming (Default Subtitle, Language Subtitle, `.hidden`, `.kinosail.bak`) | `packages/library` | Subtitles writes these files and Player reads them. Both sides must agree. |

Do not move the auth, settings, or subtitle files before Stage 2. Their Subtitles copies still carry Player branches, and moving them first would move dead code into `packages/`.

## Staged plan

Each stage is one pull request. Each stage must:

1. Keep the 13 CI subtitle journeys green, after the two Player-dependent tests are replaced.
2. Add an E2E check that every removed route returns 404 for an Owner session and for no session.
3. Prove that a request to a removed route changes nothing: same `GET /api/v1/subtitle-library` and `GET /api/v1/settings` responses, and the same media tree checksum, before and after the requests.
4. Update every route list that names a removed route: `api_openapi.json`, `mcp_route_policy.go`, the API-key scope list in `api_keys.go`, locales, docs, and the [Code Atlas](architecture-explorer/README.md).
5. Pass `make max-loc` and `make -C apps/subtitles verify-changed`.

### Stage 2a: proof harness and Player journeys

- Add `e2e/removed-routes.spec.ts`. It sends each route from a removed group, checks for 404, and records the before and after checksums. It starts with an empty list and grows in each stage.
- Add a Go run-time route test. It must fail when a removed pattern is registered again, including patterns that the static scan cannot see.
- Remove `player-unplayed-progress.spec.ts` from the CI list and delete it.
- Change the cleanup journey to prove its result through `GET /api/v1/subtitle-library/{id}/inspect` and the media tree, not `/watch`.
- Delete the 16 or more Player-copy specs that CI does not run.

### Stage 2b: public and compatibility surfaces

Remove Jellyfin compatibility, Quick Connect, DLNA, watch rooms, media shares, and public session timeouts. This stage removes the largest security risk first.

Files: `jellyfin*.go`, `quick_connect*.go`, `dlna.go`, `settings_dlna*.go`, `settings_jellyfin_id.go`, `media_share*.go`, the `withDLNA` wrapper in `server.go`, the matching settings and onboarding routes, and the `/Sessions/Logout` check in `test-container.sh`.

Stages 2a and 2b ship together. Result:

- 94 routes are removed. They are listed in `internal/server/testdata/removed-routes.txt`.
- `TestRemovedPlayerRoutesAreNotRegistered` proves that each listed route returns 404 from the running handler.
- `TestRemovedPlayerPackagesAreNotImported` fails if production code imports `dlna`, `jellyfincompat`, `mediashares`, `quickconnect`, or `watchrooms` again.
- `e2e/removed-routes.spec.ts` (`@smoke`) sends each route with an Owner session and with no session against a populated instance. It records the library, settings, and media checksums before and after.
- The registered-route inventory in Go tests leaves out the listed routes. Its count changed from 473 to 379.
- Stored settings keep the `jellyfinCompatibility`, `jellyfinId`, and `dlnaToken` fields, because strict decoding rejects unknown fields. `jellyfinId` is now the Server ID that Home Assistant uses.
- The configuration keys `dlna.url`, `dlna.enabled`, and `integrations.jellyfin.enabled` stay accepted, but Subtitles ignores them. Removing them would make existing `kinosail.yaml` files fail to load.
- Shared `servertest` contracts got opt-out fields (`NoJellyfin`, `NoQuickConnectOrShares`, `InitialCopy`). Stages 2c and 2d renamed or added others. The defaults keep Player behavior unchanged.
- `identitycore.ValidSingleByteRange` replaces the copy in `mediashares` for request logging.

Moved to Stage 2e, because they depend on the remote-access decision:

- Public session timeouts.
- The public password-login message and remote-access setup text that mention Quick Connect.
- Proof of remote playback through the public listener. Without Quick Connect, a public session needs a passkey.

Moved to Stage 2c: the cleanup journey's `/watch` check, and `player.js` calls to removed routes.

### Stages 2c and 2d: playback and library browsing

These stages shipped together. They removed HLS, `/watch`, trickplay, progress, markers, transcoder settings, `player.js`, `hls.min.js`, downloads, offline pages, playlists, collections, lists, books, the reader, albums, shows, people, viewing imports and syncs, continue watching, playback history, and the Player home page.

Result:

- 73 more routes are removed. `internal/server/testdata/removed-routes.txt` now lists 167 routes. The registered-route inventory changed from 379 to 249.
- `/` serves only the subtitle dashboard. `/media/{id}` and `/art/{id}` stay for the inspector.
- `api_openapi.json`, `mcp_route_policy.go`, and the API-key scopes match the remaining routes. API keys offer only the `admin`, `library`, and `stream` scopes.
- The MCP gateway hides `recommendation_context` and `create_playlist` for Subtitles through `NoPlaylists`.
- The service worker no longer stores offline media. It caches only the app shell and deletes the old offline database on activation.
- Locale catalogs no longer carry 150 messages that no Subtitles page renders.
- The Subtitles media proof shards and `test-subtitles-copied-prefix.py` are removed. Media proofs now run only for Player.
- The hls.js license notice and the unused `ProbeHardware`, `HardwareDevices`, `DLNAURL`, `WatchRoomTTL`, and `TranscodeCacheLimit` server options are removed.
- Shared `servertest` contracts use `NoPlayback`, `NoPlayerSettings`, and `NoOnboarding` for Subtitles. The Player defaults are unchanged.

Still open:

- Stored settings keep the playback, transcoder, Home Assistant, and navigation fields, because strict decoding rejects unknown fields. Removing them needs a stored-settings migration.
- The container still ships `jellyfin-ffmpeg`, the GPU driver packages, `compose.gpu.yaml`, and `compose.rkmpp.yaml`, and `test-container.sh` still checks the encoders. Change to Debian `ffmpeg` only after alignment, OCR, and probe journeys pass on the new image.

### Stage 2e: decided groups

Apply the answers to the six questions above.

### Stage 3: shared code

Recompute the identical-file list. Move the adapters in [Shared code for packages](#shared-code-for-packages) when both apps still need them, with both apps' CI journeys green.

## Verification limits

- The route list comes from a static scan. The Stage 2a run-time test now checks every removed route against the running handler.
- Stage 1 changed only documentation. No tests were run for it.
- The Home Assistant repository and any native client may call Subtitles routes. This repository cannot show those callers. Check them before Stage 2b and Stage 2e.
