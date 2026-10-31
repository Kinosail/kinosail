# Subtitles route inventory

This file lists every route pattern that the Subtitles server registers at revision `3f292de8`. It supports [the Player feature removal plan](player-feature-removal-plan.md).

Each group shows its proposed outcome:

- **keep**: Subtitles needs the route.
- **remove**: Player owns the feature. Stage 2 deletes the route and proves that it returns 404.
- **decide**: The Owner must choose before Stage 2 changes the route.

The list comes from a static scan of `apps/subtitles/internal/server` and the shared route packages that `packages/routeinventory` scans. A static scan does not see a route whose pattern is built at run time. Stage 2 adds a run-time check.

### Subtitle domain (keep, 42)

```text
POST /api/v1/items/{id}/subtitles
GET /api/v1/subtitle-library
POST /api/v1/subtitle-library/fetch-wanted
POST /api/v1/subtitle-library/maintain
POST /api/v1/subtitle-library/{id}/apply
POST /api/v1/subtitle-library/{id}/audio
GET /api/v1/subtitle-library/{id}/draft
POST /api/v1/subtitle-library/{id}/draft
GET /api/v1/subtitle-library/{id}/export
POST /api/v1/subtitle-library/{id}/fetch
GET /api/v1/subtitle-library/{id}/inspect
POST /api/v1/subtitle-library/{id}/preview
POST /api/v1/subtitle-library/{id}/replacement
POST /api/v1/subtitle-library/{id}/restore
POST /api/v1/subtitle-operations
GET /api/v1/subtitle-operations/{operation}
GET /api/v1/subtitle-operations/{operation}/result
POST /api/v1/subtitle-providers/test
POST /api/v1/subtitles/cleanup
POST /api/v1/subtitles/cleanup/preview
POST /onboarding/subtitles/language
POST /onboarding/subtitles/libraries
POST /onboarding/subtitles/libraries/remove
POST /onboarding/subtitles/scans
POST /settings/subtitles
GET /settings/subtitles/cleanup
POST /settings/subtitles/cleanup
POST /settings/subtitles/picker
POST /settings/subtitles/subsource
POST /settings/subtitles/subsource/reset
GET /static/subtitle-inspector.css
GET /static/subtitle-inspector.js
GET /static/subtitle-status.js
GET /subtitles/inspect/{id}
POST /subtitles/manage/fetch-wanted
POST /subtitles/manage/maintain
POST /subtitles/manage/{id}/fetch
POST /subtitles/manage/{id}/replacement
POST /subtitles/manage/{id}/restore
POST /subtitles/providers/test
POST /subtitles/{id}/fetch
GET /subtitles/{id}/{language}
```

### Owner platform (keep, 140)

```text
GET /account
POST /account/mfa/disable
POST /account/mfa/enable
POST /account/mfa/setup
POST /account/passkeys/remove
GET /api/v1
GET /api/v1/activity
GET /api/v1/activity/export
GET /api/v1/agent-connections
GET /api/v1/agent-connections/certificate
DELETE /api/v1/agent-connections/{id}
GET /api/v1/api-keys
POST /api/v1/api-keys
DELETE /api/v1/api-keys/{id}
GET /api/v1/backup
GET /api/v1/backups
POST /api/v1/backups
POST /api/v1/backups/verify
GET /api/v1/configuration
DELETE /api/v1/configuration/{key}
PUT /api/v1/configuration/{key}
GET /api/v1/devices
DELETE /api/v1/devices/{id}
GET /api/v1/diagnostics
GET /api/v1/events
DELETE /api/v1/libraries
POST /api/v1/libraries
GET /api/v1/maintenance
DELETE /api/v1/management-access
GET /api/v1/management-access
POST /api/v1/management-access
DELETE /api/v1/management-access/devices
POST /api/v1/management-access/devices
GET /api/v1/me
PUT /api/v1/me/language
DELETE /api/v1/me/mfa
PUT /api/v1/me/mfa
POST /api/v1/me/mfa/setup
GET /api/v1/metrics
GET /api/v1/openapi.json
GET /api/v1/passkeys
POST /api/v1/passkeys/login/begin
POST /api/v1/passkeys/login/finish
POST /api/v1/passkeys/register/begin
POST /api/v1/passkeys/register/finish
DELETE /api/v1/passkeys/{id}
DELETE /api/v1/session
POST /api/v1/session
DELETE /api/v1/sessions
GET /api/v1/settings
PUT /api/v1/settings/mfa
PUT /api/v1/settings/onboarding
PUT /api/v1/settings/scans
PUT /api/v1/settings/server
PUT /api/v1/settings/session-timeouts
PUT /api/v1/settings/subtitles
DELETE /api/v1/settings/trusted-https
PUT /api/v1/settings/trusted-https
PUT /api/v1/settings/updates
POST /api/v1/setup
GET /api/v1/supporter
POST /api/v1/supporter/activate
GET /api/v1/supporter/certificates/{family}
POST /api/v1/tasks/{task}
GET /api/v1/updates
POST /api/v1/updates
POST /api/v1/updates/check
POST /auth/passkeys/login/begin
POST /auth/passkeys/login/finish
POST /auth/passkeys/register/begin
POST /auth/passkeys/register/finish
GET /favicon.ico
GET /healthz
GET /language
POST /language
GET /login
POST /login
GET /login/mfa
POST /login/mfa
POST /logout
GET /manifest.webmanifest
GET /onboarding/finish
POST /scan
GET /service-worker.js
GET /settings
GET /settings/agent-connections
POST /settings/agent-connections/revoke
POST /settings/api-keys
POST /settings/api-keys/revoke
GET /settings/backup
GET /settings/backups
POST /settings/backups/verify
GET /settings/configuration
POST /settings/configuration
POST /settings/configuration/reset
GET /settings/diagnostics.json
POST /settings/encrypted-backup
POST /settings/libraries
POST /settings/libraries/remove
GET /settings/management
POST /settings/management/devices
POST /settings/management/devices/revoke
POST /settings/management/disable
POST /settings/management/enable
GET /settings/metrics
POST /settings/mfa
POST /settings/onboarding
POST /settings/scans
POST /settings/server
POST /settings/session-timeouts
POST /settings/sessions/device
POST /settings/sessions/revoke
GET /settings/system
POST /settings/tasks/maintain
POST /settings/tasks/scan
POST /settings/trusted-https
POST /settings/trusted-https/disable
POST /settings/updates
POST /settings/updates/check
GET /setup
POST /setup
GET /static/app.css
GET /static/apple-touch-icon.png
GET /static/cinema-backdrop.jpg
GET /static/htmx.min.js
GET /static/icon-192.png
GET /static/icon-512.png
GET /static/icon-maskable-512.png
GET /static/icon.svg
GET /static/main.kinosail.bundle.js
GET /static/manrope.woff2
GET /static/passkeys.js
GET /static/supporter.css
GET /static/supporter.js
GET /static/supporter/badges/{file}
GET /static/theme.js
GET /static/tmdb-logo.svg
GET /supporter
POST /supporter/activate
GET /{$}
```

### Jellyfin compatibility (remove, 60)

```text
GET /Audio/{id}/{stream}
GET /Auth/Keys
POST /Auth/Keys
GET /Branding/Configuration
GET /Items
GET /Items/Latest
GET /Items/{id}
GET /Items/{id}/Download
GET /Items/{id}/File
GET /Items/{id}/Images/{type}
GET /Items/{id}/Images/{type}/{index}
GET /Items/{id}/PlaybackInfo
POST /Items/{id}/PlaybackInfo
GET /Library/MediaFolders
GET /MediaSegments/{id}
GET /Persons
GET /Playback/BitrateTest
POST /QuickConnect/Authorize
GET /QuickConnect/Connect
GET /QuickConnect/Enabled
GET /QuickConnect/Initiate
POST /QuickConnect/Initiate
POST /Sessions/Capabilities
POST /Sessions/Capabilities/Full
POST /Sessions/Logout
POST /Sessions/Playing
POST /Sessions/Playing/Progress
POST /Sessions/Playing/Stopped
GET /Shows/NextUp
GET /Shows/{id}/Episodes
GET /Shows/{id}/Seasons
GET /System/Info
GET /System/Info/Public
DELETE /UserFavoriteItems/{id}
POST /UserFavoriteItems/{id}
GET /UserItems/Resume
GET /UserItems/{id}/UserData
POST /UserItems/{id}/UserData
DELETE /UserPlayedItems/{id}
POST /UserPlayedItems/{id}
GET /UserViews
GET /Users
POST /Users/AuthenticateByName
POST /Users/AuthenticateWithQuickConnect
GET /Users/Me
GET /Users/Public
GET /Users/{id}
GET /Users/{user}/Items
GET /Users/{user}/Items/Latest
GET /Users/{user}/Items/{id}
DELETE /Users/{user}/PlayedItems/{id}
POST /Users/{user}/PlayedItems/{id}
GET /Users/{user}/Views
GET /Videos/{id}/{source}/Subtitles/{index}/{stream}
GET /Videos/{id}/{stream...}
GET /Videos/{id}/{stream}
PUT /api/v1/settings/jellyfin
POST /onboarding/jellyfin
POST /settings/jellyfin
GET /system/info/public
```

### DLNA (remove, 7)

```text
PUT /api/v1/settings/dlna
POST /dlna/{token}/content
GET /dlna/{token}/content.xml
GET /dlna/{token}/device.xml
GET /dlna/{token}/media/{id}
HEAD /dlna/{token}/media/{id}
POST /settings/dlna
```

### HLS, transcoding, playback (remove, 25)

```text
GET /api/v1/audio/{id}/queue
GET /api/v1/hardware
DELETE /api/v1/items/{id}/continue-watching
GET /api/v1/items/{id}/playback
POST /api/v1/items/{id}/playback-events
PUT /api/v1/items/{id}/progress
GET /api/v1/items/{id}/watch-progress
PUT /api/v1/items/{id}/watched
PUT /api/v1/settings/playback
PUT /api/v1/settings/transcoder
PUT /api/v1/shows/{id}/watched
POST /api/v1/transcoder/test
POST /continue-watching/{id}/remove
GET /hls/{id}/audio/{track}/{file...}
GET /hls/{id}/{file...}
POST /progress/{id}
POST /settings/cache/clear
POST /settings/playback
POST /settings/transcoder
POST /settings/transcoder/test
GET /static/hls.min.js
GET /static/player.js
GET /trickplay/{id}/{second}
GET /watch/{id}
POST /watched/{id}
```

### Watch rooms (remove, 6)

```text
POST /api/v1/watch-rooms
GET /api/v1/watch-rooms/{id}
GET /api/v1/watch-rooms/{id}/events
GET /room/{id}
POST /watch-together
GET /watch-together/{id}
```

### Media shares (remove, 13)

```text
GET /api/v1/media-shares
POST /api/v1/media-shares
POST /api/v1/media-shares/claim
DELETE /api/v1/media-shares/{id}
GET /settings/media-shares
POST /settings/media-shares
POST /settings/media-shares/{id}/revoke
GET /share
GET /share/items
GET /share/media/{id}
HEAD /share/media/{id}
GET /static/media-share.js
GET /static/public-login.js
```

### Downloads and offline (remove, 14)

```text
GET /api/v1/downloads
GET /api/v1/downloads/identity
DELETE /api/v1/downloads/{id}
GET /api/v1/downloads/{id}
GET /api/v1/downloads/{id}/file
GET /api/v1/downloads/{id}/manifest
GET /api/v1/items/{id}/download-tracks
POST /api/v1/items/{id}/downloads
GET /download/{id}
GET /offline
GET /offline-downloads
POST /offline-downloads/{id}/remove
POST /offline/{id}
GET /static/downloads.js
```

### Playlists, collections, lists (remove, 28)

```text
GET /api/v1/collections
POST /api/v1/collections
DELETE /api/v1/collections/{name}
GET /api/v1/collections/{name}
PUT /api/v1/collections/{name}/items/{id}
PUT /api/v1/items/{id}/list
GET /api/v1/playlists
POST /api/v1/playlists
DELETE /api/v1/playlists/{name}
GET /api/v1/playlists/{name}
PUT /api/v1/playlists/{name}/items/{id}
PUT /api/v1/playlists/{name}/order
POST /api/v1/smart-playlists
GET /api/v1/shows/{id}/list
PUT /api/v1/shows/{id}/list
GET /collection/{name}
POST /collection/{name}/delete
POST /collection/{name}/items/{id}
POST /collection/{name}/{id}
POST /collections
POST /list/{id}
GET /playlist/{name}
POST /playlist/{name}/delete
POST /playlist/{name}/items/{id}
POST /playlist/{name}/order
POST /playlist/{name}/{id}
POST /playlists
POST /smart-playlists
```

### Books, music, browse pages (remove, 16)

```text
GET /album/{id}
GET /api/v1/albums
GET /api/v1/albums/{id}
GET /api/v1/books/{id}/reader
GET /api/v1/books/{id}/reader/progress
PUT /api/v1/books/{id}/reader/progress
GET /api/v1/history
GET /api/v1/shows
GET /api/v1/shows/{id}
GET /book/{id}
GET /person/{id}/{person}
GET /read/{id}
GET /read/{id}/asset/{asset...}
GET /read/{id}/file
POST /read/{id}/progress
GET /show/{id}
```

### Markers (remove, 7)

```text
PUT /api/v1/items/{id}/markers
DELETE /api/v1/items/{id}/markers/{type}
GET /api/v1/marker-analysis
POST /api/v1/marker-analysis
POST /markers/{id}
POST /markers/{id}/remove
POST /settings/marker-analysis
```

### Viewing imports and syncs (remove, 13)

```text
POST /api/v1/viewing-imports/preview
POST /api/v1/viewing-imports/{id}/apply
GET /api/v1/viewing-syncs
POST /api/v1/viewing-syncs
DELETE /api/v1/viewing-syncs/{id}
POST /api/v1/viewing-syncs/{id}/run
POST /onboarding/viewing-imports/apply
POST /onboarding/viewing-imports/preview
POST /settings/viewing-imports/apply
POST /settings/viewing-imports/preview
POST /settings/viewing-syncs
POST /settings/viewing-syncs/remove
POST /settings/viewing-syncs/run
```

### Quick Connect (remove, 8)

```text
POST /api/v1/quick-connect
POST /api/v1/quick-connect/token
POST /api/v1/quick-connect/{code}
POST /auth/quick-connect
POST /auth/quick-connect/cancel
POST /auth/quick-connect/token
GET /quick-connect
POST /quick-connect
```

### Player home and shell pages (remove, 7)

```text
GET /onboarding
GET /onboarding/connection
GET /onboarding/migrate
POST /onboarding/trusted-https
POST /onboarding/updates
POST /onboarding/updates/check
GET /static/pwa.js
```

### Home Assistant (decide, 18)

```text
GET /api/v1/home-assistant
GET /api/v1/home-assistant/library
POST /api/v1/home-assistant/pair
POST /api/v1/home-assistant/pairings
POST /api/v1/home-assistant/playback/{id}
GET /api/v1/home-assistant/players
POST /api/v1/home-assistant/players/claims
PUT /api/v1/home-assistant/players/{id}
POST /api/v1/home-assistant/players/{id}/commands
POST /api/v1/home-assistant/players/{id}/release
POST /api/v1/home-assistant/token
PUT /api/v1/settings/home-assistant
GET /home-assistant/authorize
POST /home-assistant/authorize
GET /home-assistant/media/{id}
POST /onboarding/home-assistant
POST /settings/home-assistant
POST /settings/home-assistant/pair
```

### Metadata editing (decide, 8)

```text
PUT /api/v1/items/{id}/metadata
POST /api/v1/items/{id}/metadata/refresh
POST /api/v1/metadata/bulk
GET /metadata/bulk
POST /metadata/bulk
POST /metadata/{id}
POST /metadata/{id}/refresh
POST /settings/tasks/metadata
```

### Viewer profiles and federation (decide, 30)

```text
POST /account/oidc/unlink
POST /account/saml/unlink
DELETE /api/v1/me/oidc
GET /api/v1/me/oidc/link
DELETE /api/v1/me/saml
GET /api/v1/me/saml/link
GET /api/v1/profiles
POST /api/v1/profiles
DELETE /api/v1/profiles/{id}
PUT /api/v1/profiles/{id}
PUT /api/v1/profiles/{id}/password
GET /api/v1/session/oidc
GET /api/v1/session/saml
PUT /api/v1/settings/navigation
DELETE /api/v1/settings/public-session-timeouts
PUT /api/v1/settings/public-session-timeouts
GET /login/oidc
GET /login/oidc/callback
GET /login/saml
POST /login/saml/acs
GET /login/saml/metadata
GET /onboarding/household
POST /onboarding/household
POST /settings/navigation
POST /settings/profiles
POST /settings/profiles/password
POST /settings/profiles/permissions
POST /settings/profiles/remove
POST /settings/public-session-timeouts
POST /settings/public-session-timeouts/reset
```

### Remote access (decide, 6)

```text
GET /api/v1/remote-access
DELETE /api/v1/remote-access/kill
POST /api/v1/remote-access/kill
GET /settings/remote-readiness
POST /settings/remote/enable
POST /settings/remote/kill
```

### Media files for inspector (decide, 6)

```text
GET /art/{id}
GET /backdrop/{id}
GET /media/{id}
GET /subtitle/{id}
GET /subtitle/{id}/embedded/{stream}
GET /subtitle/{id}/{track}
```

### Library catalog API (decide, 2)

```text
GET /api/v1/items/{id}
GET /api/v1/library
```
