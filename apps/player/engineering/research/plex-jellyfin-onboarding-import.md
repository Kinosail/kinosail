# Plex and Jellyfin onboarding import

Research snapshot: **2026-08-23 (America/Denver)**.

This note asks what Kinosail can responsibly import through current first-party Plex and Jellyfin interfaces so that a household can continue where it left off. It covers source discovery/authentication, people, viewing state, personal lists, playlists, collections, matching, and the limits that the onboarding UI must make visible. It does not propose reading either product's private database files.

## Bottom line

A high-confidence first release can import:

- a selected source server and its accessible libraries;
- mapped users/profiles, without copying credentials;
- watched/unwatched state, resume position, play count or last-played data when supplied, Plex server playback-history events, favorites/likes, and ratings;
- static playlist membership and order; and
- manual collection membership.

The important asymmetries are:

- Plex exposes rich per-user media state from a selected Plex Media Server, but its public PMS API does not document Plex Home enumeration or user switching. Managed users cannot sign in directly, so a production importer should not make private Plex Home endpoints a hard dependency ([Plex Home](https://support.plex.tv/articles/203815766-what-is-plex-home/); [public PMS API](https://developer.plex.tv/pms/)).
- Plex's Universal Watchlist is hosted account state, can contain titles not present on the selected server, and is not part of the documented PMS endpoint surface. Plex does document a user-generated RSS feed for Plex Pass subscribers, which is a supportable optional import path ([Universal Watchlist](https://support.plex.tv/articles/universal-watchlist/)).
- Jellyfin exposes local users and per-user item data directly, including playback position, played state, play count, favorite, like, last-played date, and rating ([user API](https://typescript-sdk.jellyfin.org/classes/generated-client.UserApi.html); [`UserItemDataDto`](https://typescript-sdk.jellyfin.org/interfaces/generated-client.UserItemDataDto.html)). Its current first-party SDK has favorite operations but no documented first-class watchlist operation, so favorites must remain favorites rather than being silently reinterpreted as a watchlist ([user-data API](https://typescript-sdk.jellyfin.org/functions/generated-client.UserDataApiFactory.html)).
- Neither source provides a portable identity for every item. Exact external IDs make many movie, show, and episode matches safe, but unmatched and ambiguous records are unavoidable and need an explicit review/report step ([Plex metadata identifiers](https://developer.plex.tv/pms/); [Jellyfin `ProviderIds`](https://typescript-sdk.jellyfin.org/interfaces/generated-client.BaseItemDto.html)).

The honest product promise is therefore **resume the matched local library for each mapped person**, not clone an entire Plex or Jellyfin installation.

## Recommended wizard

### 1. Choose and connect to a source

For Plex, use the documented Plex Auth App/PIN flow, then query `clients.plex.tv/api/v2/resources` and let the user choose a server. The response includes each PMS's machine identifier, per-server access token, and connection URLs; Plex says to prefer local connections and use Relay only as a last resort. New integrations should use the documented short-lived JWT flow when practical ([Plex authentication and server resources](https://developer.plex.tv/pms/#section/API-Info/Authenticating-with-Plex)). Kinosail should not silently select a Relay URL.

For Jellyfin, listen for LAN discovery on UDP 7359 and always retain manual URL entry because discovery does not cross subnets. Validate the chosen URL with public system information, then offer Quick Connect or username/password authentication. Quick Connect lets the user authorize Kinosail from an already authenticated Jellyfin client without typing the source password into Kinosail ([Jellyfin networking](https://jellyfin.org/docs/general/post-install/networking/); [Quick Connect](https://jellyfin.org/docs/general/server/quick-connect/); [first-party SDK authentication guide](https://kotlin-sdk.jellyfin.org/guide/authentication.html)).

Treat source access tokens, Plex Watchlist RSS URLs, passwords, and Quick Connect secrets as credentials: never place them in URLs or logs, keep them only for the import run by default, and revoke/delete the local copy when the run finishes. A separately authorized recurring sync could be added later; it should not be implied by onboarding.

### 2. Inventory before changing Kinosail

Show the selected server, source users Kinosail can actually read, library names/types, and counts for each import category. Keep every source call read-only.

Plex state is token-relative: the PMS metadata schema returns `userRating`, `viewOffset` in milliseconds, `viewCount`, and `lastViewedAt` for the calling user, and the playlist API describes its result as playlists “for a user” ([PMS metadata](https://developer.plex.tv/pms/#tag/Content/operation/libraryMetadataGetSlash); [playlist API](https://developer.plex.tv/pms/#tag/Playlist/operation/playlistGetSlash)). Inventory each independently authorized Plex identity rather than assuming the home administrator's state represents the household.

Jellyfin administrators can enumerate local users, while the public-user endpoint deliberately returns only visible login choices. A full authenticated inventory can also read user policy and configuration, including administrative status, library access, parental restrictions, language preferences, and playback permissions ([Jellyfin user controller](https://github.com/jellyfin/jellyfin/blob/af8e19b16929188a4621a17c4a2f4d2008f886ec/Jellyfin.Api/Controllers/UserController.cs#L84-L140); [`UserPolicy`](https://typescript-sdk.jellyfin.org/interfaces/generated-client.UserPolicy.html); [`UserConfiguration`](https://typescript-sdk.jellyfin.org/interfaces/generated-client.UserConfiguration.html)).

### 3. Configure and scan Kinosail libraries

Imported personal state can only attach to media Kinosail has indexed. Reuse source library names and types as suggestions, but do not blindly reuse filesystem paths: Plex and Jellyfin paths describe the source server's filesystem, while Kinosail's single container may see different mounts. Require each proposed root to exist and be readable inside the Kinosail Server container before scanning.

### 4. Map people deliberately

For each readable source identity, offer:

- map to an existing Kinosail profile;
- create a Viewer with the source display name; or
- skip.

Do not copy passwords, PINs, access tokens, device sessions, or external authentication-provider bindings. The person must establish fresh Kinosail credentials. Plex explicitly describes a Home PIN as convenience rather than account security, and managed users cannot directly authenticate to apps ([Plex Home](https://support.plex.tv/articles/203815766-what-is-plex-home/)). Jellyfin separates user enumeration from password-change operations and supports pluggable authentication providers, so its source credential is not portable identity material ([Jellyfin user API](https://typescript-sdk.jellyfin.org/classes/generated-client.UserApi.html); [Jellyfin user management](https://jellyfin.org/docs/general/server/users/adding-managing-users/)).

Default newly created profiles to Viewer. Do not infer a Kinosail Owner merely from Plex Home administration or Jellyfin `IsAdministrator`; show an explicit promotion choice to the already authenticated Kinosail Owner. Library restrictions can be translated only where the target library mapping is unambiguous; present the proposed policy before applying it.

### 5. Match source items to Kinosail items

Use a deterministic confidence ladder:

1. exact external provider ID plus media type — IMDb/TMDb/TVDb and any other provider Kinosail understands;
2. for episodes, series identity plus season and episode number, or original air date where numbering is absent;
3. for movies, normalized title plus release year;
4. for music, stable recording/release identifiers when both sides have them; and
5. otherwise leave the item unmatched.

Plex metadata can carry a Plex `guid` plus a `Guid` array containing identities such as `imdb://`, `tmdb://`, and `tvdb://`; its own matching documentation treats a GUID as less ambiguous than title/year hints ([Plex metadata and matching](https://developer.plex.tv/pms/)). Jellyfin `BaseItemDto` can carry `ProviderIds`, path, production year, and episode parent/index fields ([Jellyfin item DTO](https://typescript-sdk.jellyfin.org/interfaces/generated-client.BaseItemDto.html)). Source-local item IDs and paths are useful provenance but are not cross-server identities.

Never auto-apply a title-only ambiguous match. Show matched, ambiguous, unsupported, and missing totals before commit, and make the unmatched report available after completion.

### 6. Preview categories and commit once

The confirmation screen should show changes per Kinosail profile and category. On a new installation, applying the source snapshot is unsurprising. If Kinosail already contains state, default to preserving it and require an explicit per-category overwrite choice: neither Plex `viewOffset` nor Jellyfin `PlaybackPositionTicks` carries a reliable per-field modification timestamp suitable for a universal “newest wins” merge ([Plex metadata schema](https://developer.plex.tv/pms/); [Jellyfin user-data DTO](https://typescript-sdk.jellyfin.org/interfaces/generated-client.UserItemDataDto.html)).

Record import provenance and make retries idempotent. Re-running the same source snapshot must not duplicate history events, playlists, playlist entries, collections, or list membership. A failed run should resume from the last committed category or safely recompute the preview.

Jellyfin's own migration documentation supports this API-first shape: it says internal databases cannot be copied or adjusted easily and points to API-based scripts for copying users and watched status from Plex, Emby, or another Jellyfin instance ([Jellyfin migration guidance](https://jellyfin.org/docs/general/administration/migrate/)).

## Capability matrix

| Data | Plex | Jellyfin | Kinosail import behavior |
| --- | --- | --- | --- |
| Server discovery/auth | Strong: hosted PIN/JWT authentication followed by `/api/v2/resources`; resource connections include local/remote/Relay choices ([PMS authentication](https://developer.plex.tv/pms/#section/API-Info/Authenticating-with-Plex)) | Strong: LAN discovery on UDP 7359, manual URL, password auth, and Quick Connect ([networking](https://jellyfin.org/docs/general/post-install/networking/); [authentication](https://kotlin-sdk.jellyfin.org/guide/authentication.html)) | Test every advertised address, prefer direct local HTTPS, show the selected server identity, and never silently fall back to Relay. |
| Profiles/users | Partial through the supported surface: Plex Home has full accounts, managed users, Guest, PINs, restrictions, and one Home admin, but managed users cannot sign in directly and user switching requires Plex connectivity ([Plex Home](https://support.plex.tv/articles/203815766-what-is-plex-home/)) | Strong for an authenticated administrator: enumerate local users and read configuration/policy ([user API](https://typescript-sdk.jellyfin.org/classes/generated-client.UserApi.html)) | Map readable identities to existing profiles or new Viewers. Require fresh Kinosail credentials and explicit Owner promotion. |
| Watched/unwatched | Strong per PMS/user token through `viewCount`/played filters; Plex separately offers optional account sync for movie/episode watched state ([PMS metadata](https://developer.plex.tv/pms/#tag/Content/operation/libraryMetadataGetSlash); [watch-state sync](https://support.plex.tv/articles/sync-watch-state-and-ratings/)) | Strong through `Played`, `PlayCount`, `LastPlayedDate`, played filters, and mark played/unplayed operations ([DTO](https://typescript-sdk.jellyfin.org/interfaces/generated-client.UserItemDataDto.html); [item query](https://github.com/jellyfin/jellyfin/blob/af8e19b16929188a4621a17c4a2f4d2008f886ec/Jellyfin.Api/Controllers/ItemsController.cs#L79-L174)) | Import leaf item state. Do not manufacture a full viewing-event history from aggregate state. |
| Resume position | Strong for the selected PMS: `viewOffset` is milliseconds. Plex explicitly says in-progress position is not included in hosted watch-state sync ([PMS metadata](https://developer.plex.tv/pms/#tag/Content/operation/libraryMetadataGetSlash); [sync limitation](https://support.plex.tv/articles/sync-watch-state-and-ratings/)) | Strong: `PlaybackPositionTicks` and `PlayedPercentage` are first-class per-user fields ([DTO](https://typescript-sdk.jellyfin.org/interfaces/generated-client.UserItemDataDto.html)) | Convert units exactly, clamp to target runtime, reject negative/out-of-range values, and clear resume for completed items. This is the core “continue where I left off” promise. |
| Plex Watchlist / My List | Strong product feature but partial export: Universal Watchlist is account-hosted, movie/show only, may contain non-library titles, and has an optional RSS feed for Plex Pass users ([Universal Watchlist](https://support.plex.tv/articles/universal-watchlist/)) | No documented first-class watchlist in the current SDK surface; favorite is distinct ([SDK API modules](https://typescript-sdk.jellyfin.org/modules/generated-client.html)) | Offer an optional Plex RSS import and add only matched local movies/shows to My List. Because Kinosail defines My List as private favorite Library Content, map matched Jellyfin favorites to My List while labeling that this is a favorite mapping, not a Jellyfin watchlist. |
| Favorites / likes | No general PMS favorite field parallel to Jellyfin's documented `IsFavorite`; Watchlist and user rating are separate Plex concepts ([PMS API](https://developer.plex.tv/pms/); [Universal Watchlist](https://support.plex.tv/articles/universal-watchlist/)) | Strong: `IsFavorite` and nullable `Likes`, with mark/unmark favorite operations ([DTO](https://typescript-sdk.jellyfin.org/interfaces/generated-client.UserItemDataDto.html); [user-data API](https://typescript-sdk.jellyfin.org/functions/generated-client.UserDataApiFactory.html)) | Import Jellyfin favorite to favorite. Preserve like/dislike only if Kinosail has an exact semantic field; otherwise report it as unsupported. |
| Ratings | Strong per selected server through `userRating`; watched state and ratings can also be optionally synchronized by Plex account GUID ([PMS metadata](https://developer.plex.tv/pms/#tag/Content/operation/libraryMetadataGetSlash); [sync details](https://support.plex.tv/articles/sync-watch-state-and-ratings/)) | Strong at the API level through nullable numeric `Rating` and update/delete rating operations ([DTO](https://typescript-sdk.jellyfin.org/interfaces/generated-client.UserItemDataDto.html); [user-data API](https://typescript-sdk.jellyfin.org/functions/generated-client.UserDataApiFactory.html)) | Normalize only after confirming both scales. Keep “no rating” distinct from zero and do not convert favorites/likes into numeric ratings. |
| Play history | Strong but server-scoped: `/status/sessions/history/all` is paginated, users can read their own events, and an administrator can read all users' events; each record carries item, account, device, and `viewedAt` identifiers. Plex Profile Watch History is separate hosted activity and may not equal current watch state ([PMS playback history](https://developer.plex.tv/pms/#tag/Status/operation/statusGetHistoryAll); [Plex Profile](https://support.plex.tv/articles/profile/)) | Partial: user data exposes play count and last-played date, not a complete core event log ([DTO](https://typescript-sdk.jellyfin.org/interfaces/generated-client.UserItemDataDto.html)) | Import matched Plex history events with a stable source-history key when Kinosail supports event history. For Jellyfin, import only aggregate count/date and do not manufacture events. |
| Playlists | Strong: documented per-user list/item APIs cover static and smart playlists and preserve playlist item identity/order ([PMS playlist API](https://developer.plex.tv/pms/#tag/Playlist/operation/playlistGetSlash)) | Strong: create/read/update playlists, enumerate ordered items, and current APIs include user-sharing permissions ([playlist API](https://typescript-sdk.jellyfin.org/classes/generated-client.PlaylistApi.html)) | Recreate static playlists from matched items in source order. Snapshot smart/dynamic results unless every source rule has an exact Kinosail equivalent. Report skipped entries. |
| Collections | Strong server-library API; collection management is generally administrative ([PMS collection API](https://developer.plex.tv/pms/#tag/Library/operation/librarySectionGetCollections)) | Strong server-level collection operations protected by collection-management permission ([collection controller](https://github.com/jellyfin/jellyfin/blob/af8e19b16929188a4621a17c4a2f4d2008f886ec/Jellyfin.Api/Controllers/CollectionController.cs#L20-L108)) | Import manual membership as an Owner-approved server collection. Snapshot smart membership unless rule translation is exact. Do not present collections as private per-profile lists. |
| Preferences and restrictions | Plex Home documents rating profiles, library sharing, download/Live TV choices, and PIN switching, but these do not map one-for-one to Kinosail policy ([Plex Home](https://support.plex.tv/articles/203815766-what-is-plex-home/); [library restrictions](https://support.plex.tv/articles/204232573-restricting-the-shares/)) | User policy/configuration expose library, remote, download, playback/transcode, parental, audio, and subtitle preferences ([policy](https://typescript-sdk.jellyfin.org/interfaces/generated-client.UserPolicy.html); [configuration](https://typescript-sdk.jellyfin.org/interfaces/generated-client.UserConfiguration.html)) | Offer a reviewed mapping for exact equivalents. Never broaden access because a source field has no target equivalent. Keep unsupported settings in the report. |

## Unavoidable limitations and explicit non-goals

1. **No credential migration.** Passwords, Plex account credentials, Home PINs, passkeys, external IdP bindings, API tokens, and active sessions are not profile data. Every imported person must establish Kinosail credentials.
2. **No private Plex API dependency in the critical path.** The public PMS API is now documented, but Plex Home enumeration/switching and hosted Watchlist mutation are not documented there. If a future first-party contract appears, it can deepen the adapter without changing the import model.
3. **No complete Plex Home clone.** Managed-user state may require an authorized switched-user PMS token that the public contract does not currently explain. The wizard must say which people were readable rather than silently copying the administrator's state to everyone.
4. **No hosted-catalog cloning.** Plex Watchlist entries for streaming services or unreleased/unowned titles have no local Kinosail item. Keep them in the unmatched report unless Kinosail later adds an intentional catalog-placeholder model.
5. **No false cross-product history claim.** Plex PMS has a server-scoped playback-history endpoint, but Plex Profile history is separate hosted activity, while Jellyfin core user data provides aggregates rather than a complete event ledger ([PMS playback history](https://developer.plex.tv/pms/#tag/Status/operation/statusGetHistoryAll); [Plex Profile](https://support.plex.tv/articles/profile/); [Jellyfin DTO](https://typescript-sdk.jellyfin.org/interfaces/generated-client.UserItemDataDto.html)).
6. **No perfect match rate.** Old metadata agents, custom libraries, changed episode ordering, editions, duplicate movies, music releases, and unmatched local files can defeat identifier matching. Ambiguity is a review state, not permission to guess.
7. **No automatic source-path trust.** Source paths may be remote, platform-specific, or outside the Kinosail container. Library roots need an explicit container readability check.
8. **No silent smart-rule approximation.** A static snapshot preserves useful membership; pretending a translated smart playlist or collection remains dynamically equivalent is worse than labeling the snapshot accurately.
9. **No default bidirectional sync.** A one-time, read-only source import keeps ownership clear and lets credentials expire. Re-import can be an explicit, idempotent action; continuous or two-way sync is a separate product capability with conflict and deletion semantics.

## Acceptance evidence for the eventual implementation

The importer should not be called seamless until fixtures and live-source tests demonstrate:

- Plex PIN/JWT authorization, resource selection, direct-connection preference, token expiry, and cancellation;
- Jellyfin LAN discovery, manual URLs with reverse-proxy subpaths, password login, Quick Connect, and disabled Quick Connect;
- multiple mapped people with intentionally different watched, resume, favorite/list, rating, playlist, and restriction states;
- exact external-ID matches, safe episode-number fallback, duplicate/ambiguous matches, missing media, and changed paths;
- unit conversion and boundary behavior for completed, zero, negative, and beyond-runtime resume positions;
- stable playlist order, duplicate playlist names, smart-playlist snapshots, collection membership, and unmatched entries;
- retry after partial failure without duplicate output;
- an existing Kinosail profile conflict that is previewed and preserved unless overwrite is explicitly selected; and
- proof that logs, audit records, browser history, and persisted import summaries contain no source passwords, tokens, Quick Connect secrets, or private Plex RSS URLs.

Physical/live verification remains necessary because public API schemas cannot prove how every supported Plex/Jellyfin server version, metadata agent, household configuration, reverse proxy, or real library behaves.
