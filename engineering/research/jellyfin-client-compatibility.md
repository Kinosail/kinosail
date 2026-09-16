# Jellyfin client compatibility for Kinosail

Research snapshot: 2026-08-22.

## Decision

It is feasible to let current Jellyfin clients connect to Kinosail without running Jellyfin itself. The narrowest useful target is a **Jellyfin-compatible HTTP facade** for server discovery, password login, movie/TV browsing, direct playback, artwork, and playback-progress reporting.

Build and test that facade against **Swiftfin 1.6** first. Its source and pinned SDK expose the wire contract.

Do not describe this as full Jellyfin compatibility until the broader API is implemented. The initial claim should be “Jellyfin-client compatibility for login, browse, direct play, and progress.”

## Sources and compatibility target

- Current Swiftfin release 1.6 is commit [`24b4fb3b`](https://github.com/jellyfin/Swiftfin/releases/tag/1.6). It pins `jellyfin-sdk-swift` 3.0.0 at commit [`37a2f502`](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Swiftfin.xcodeproj/project.xcworkspace/xcshareddata/swiftpm/Package.resolved), whose declared server API version is [`12.0.0`](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/Extensions/Info.swift#L16-L18).
- Swiftfin 1.6's SDK targets Jellyfin API `12.0.0`. The version check is only a warning: it considers a server compatible when the advertised major/minor is at least the SDK target. [Swiftfin version check](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/SwiftfinStore/SwiftfinStore%2BServerState.swift#L120-L128)
- Jellyfin `10.11.11` is the latest stable server reference used here for routes, authorization, and streaming behavior; its release source is commit [`1fbd8739`](https://github.com/jellyfin/jellyfin/tree/1fbd8739292cce610231be93daf43368733edf63).

For Swiftfin 1.6, advertise `12.0.0` to avoid the warning. A `10.11.x` value still lets the user continue, but is reported as incompatible. `ProductName` can remain `Kinosail`; the compatibility `Version` is a negotiated API level and must not imply that Kinosail is a Jellyfin distribution.

## Wire conventions

### Authorization

Swiftfin sends this on every API request, including the unauthenticated login request:

```http
Authorization: MediaBrowser DeviceId=<stable device id>, Device=<device name>, Client=Swiftfin <platform>, Version=<app version>[, Token=<access token>]
```

The SDK constructs those fields and adds the header centrally. After a successful login it adds `Token`. [SDK header construction](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/PassthroughAPIClientDelegate.swift#L17-L64), [SDK token persistence](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/JellyfinClient.swift#L196-L220)

The compatibility middleware should accept both `Authorization` and `X-Emby-Authorization`, parse the `MediaBrowser` scheme case-insensitively, accept quoted or unquoted values, and bind a token to the Kinosail profile/session and stable device identifier. Jellyfin 10.11 also accepts `X-Emby-Token`, `X-MediaBrowser-Token`, `ApiKey`, and (when legacy authorization is enabled) `api_key`; supporting `api_key` is useful for media and WebSocket URLs, but query-string secrets must be kept out of logs. [Jellyfin authorization source](https://github.com/jellyfin/jellyfin/blob/1fbd8739292cce610231be93daf43368733edf63/Jellyfin.Server.Implementations/Security/AuthorizationContext.cs#L73-L111), [header parsing](https://github.com/jellyfin/jellyfin/blob/1fbd8739292cce610231be93daf43368733edf63/Jellyfin.Server.Implementations/Security/AuthorizationContext.cs#L229-L264), [Jellyfin reverse-proxy warning](https://jellyfin.org/docs/general/post-install/networking/reverse-proxy/#logging)

Return `401` for missing/invalid credentials, `403` for an authenticated profile lacking permission, and normal JSON errors or an empty body as expected by the endpoint. Do not redirect API clients to the HTML login page.

### JSON and identifiers

Jellyfin JSON uses PascalCase keys (`AccessToken`, `ServerId`, `Items`, `PlaySessionId`, `MediaSources`). Unknown fields are harmless to Swiftfin, and most generated model properties are optional, but the fields called out below are dereferenced or guarded by the app.

Use stable opaque identifiers. A true Jellyfin `BaseItemDto.Id` is a .NET `Guid`, so a deterministic 128-bit UUID/32-hex representation is the safest external format. [Jellyfin `BaseItemDto.Id`](https://github.com/jellyfin/jellyfin/blob/1fbd8739292cce610231be93daf43368733edf63/MediaBrowser.Model/Dto/BaseItemDto.cs#L37-L43) Swiftfin decodes IDs as strings.

Preserve an entered base URL path. Clients can connect through a reverse-proxy subpath, so resolving `/Items` must append it to the configured server base rather than replacing it.

## Exact Swiftfin 1.6 core flow

The shortest source-confirmed HTTP sequence is:

1. `GET /System/Info/Public` to validate and save the server.
2. Concurrently fetch `GET /QuickConnect/Enabled`, `GET /Users/Public`, and `GET /Branding/Configuration` to render sign-in.
3. `POST /Users/AuthenticateByName`; retain `AccessToken`, user ID, and user name.
4. On the authenticated session, probe `GET /System/Info/Public` again and fetch `GET /Users/Me`.
5. Fetch `GET /UserViews`, then home/list data through `/UserItems/Resume`, `/Shows/NextUp`, `/Items`, and `/Items/Latest`. Some home requests are concurrent and empty successful results are valid.
6. For playback, refresh `GET /Items/{id}`, post `POST /Items/{id}/PlaybackInfo`, then open `/Videos/{id}/stream` with byte-range support.
7. During playback, post `/Sessions/Playing`, `/Sessions/Playing/Progress`, and `/Sessions/Playing/Stopped`.

Swiftfin may also post `/Sessions/Capabilities` and open `/socket`, but those are not on the critical login/browse/direct-play path.

### 1. Find and validate the server

Manual connection:

```http
GET /System/Info/Public
```

Minimum successful response:

```json
{
  "Id": "<stable server id>",
  "ServerName": "Kinosail",
  "Version": "12.0.0",
  "ProductName": "Kinosail",
  "StartupWizardCompleted": true,
  "LocalAddress": "https://example.test"
}
```

`Id` and `ServerName` are hard requirements in Swiftfin's connection view model. `Version` avoids an incompatible-version warning; Swiftfin 1.6 compares the response's major/minor to its SDK target, 12.0. [Swiftfin server connection](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/ViewModels/ConnectToServerViewModel.swift#L75-L100), [version check](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/SwiftfinStore/SwiftfinStore%2BServerState.swift#L120-L128)

Optional LAN discovery uses UDP broadcast port `7359`. The client sends the UTF-8 payload `who is JellyfinServer?` and decodes a JSON datagram containing required `Id`, `Name`, and `Address` values. Manual URL entry works without discovery. [Swift SDK discovery implementation](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/ServerDiscovery.swift#L24-L53)

### 2. Render the login screen

Swiftfin requests these concurrently:

| Request | Minimum response | Required for password login? |
|---|---|---|
| `GET /QuickConnect/Enabled` | JSON boolean, initially `false` | Yes, because Swiftfin treats a failed public-data batch as an error; `false` avoids implementing Quick Connect. |
| `GET /Users/Public` | JSON array, initially `[]` | Yes; empty is valid and keeps usernames private. |
| `GET /Branding/Configuration` | `{}` or `{"LoginDisclaimer":"..."}` | Yes; disclaimer is optional. |

The call sites and decoding are in [`UserSignInViewModel`](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/ViewModels/UserSignInViewModel.swift#L101-L112) and its [public-data helpers](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/ViewModels/UserSignInViewModel.swift#L272-L294).

### 3. Authenticate and restore the user

```http
POST /Users/AuthenticateByName
Content-Type: application/json

{"Username":"mike","Pw":"secret"}
```

The exact route/body comes from the pinned SDK. [Authentication path](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/Paths/AuthenticateUserByNameAPI.swift#L12-L17), [request model](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/Entities/AuthenticateUserByName.swift#L11-L39)

Minimum successful response:

```json
{
  "AccessToken": "<opaque session token>",
  "ServerId": "<same stable server id>",
  "User": {
    "Id": "<stable user id>",
    "Name": "Mike",
    "ServerId": "<same stable server id>",
    "Configuration": {},
    "Policy": {
      "AuthenticationProviderId": "kinosail",
      "PasswordResetProviderId": "kinosail",
      "IsDisabled": false,
      "EnableMediaPlayback": true,
      "EnableRemoteAccess": true
    }
  }
}
```

Swiftfin explicitly guards `AccessToken`, `User.Id`, and `User.Name`. Its generated SDK decoder also requires both provider-ID strings when `Policy` is present, while Swiftfin hides the play button unless `EnableMediaPlayback` is `true`. [Swiftfin password sign-in](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/ViewModels/UserSignInViewModel.swift#L117-L151), [SDK `UserPolicy` decoder](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/Entities/UserPolicy.swift#L158-L203), [Swiftfin play-button check](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/Extensions/JellyfinAPI/BaseItemDto/BaseItemDto.swift#L501-L512)

After selecting the saved session, Swiftfin probes `GET /System/Info/Public` again and refreshes the account with:

```http
GET /Users/Me
```

Return the same user DTO. `Configuration.LatestItemsExcludes` and `Configuration.MyMediaExcludes` can be absent or empty. [Swiftfin current-user refresh](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/SwiftfinStore/SwiftinStore%2BUserState.swift#L118-L136)

### 4. Render home and browse libraries

The first home request is:

```http
GET /UserViews?userId=<user id>
```

Return a `BaseItemDtoQueryResult`:

```json
{
  "Items": [
    {"Id":"<movies view id>","Name":"Movies","Type":"CollectionFolder","CollectionType":"movies","IsFolder":true},
    {"Id":"<shows view id>","Name":"Shows","Type":"CollectionFolder","CollectionType":"tvshows","IsFolder":true}
  ],
  "StartIndex": 0,
  "TotalRecordCount": 2
}
```

`Id`, `Name`, `Type`, and `CollectionType` are the practical minimum. Swiftfin filters home views to supported collection types and later uses the view ID as `parentId`. [Home view loading/filtering](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/ViewModels/ContentGroupViewModel/DefaultContentGroupProvider.swift#L20-L39), [user-view path](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/Paths/GetUserViewsAPI.swift#L12-L48)

Swiftfin then requests home rails. Implement these even if some initially return an empty valid wrapper:

| Request | Response shape / purpose |
|---|---|
| `GET /UserItems/Resume?...` | `{Items,StartIndex,TotalRecordCount}` for Continue Watching. |
| `GET /Shows/NextUp?...` | Same wrapper for Next Up; empty is valid. |
| `GET /Items?...` | Same wrapper for recently added, recently played, library browse, filters, and search. Honor at least `userId`, `parentId`, `includeItemTypes`, `recursive`, `searchTerm`, `sortBy`, `sortOrder`, `startIndex`, and `limit`; tolerate all other query keys. |
| `GET /Items/Latest?parentId=<view>&...` | JSON array (not a query-result wrapper) for “Latest in …”. |

Swiftfin asks list endpoints for `Fields=MediaSources,ParentId` and commonly `EnableUserData=true`. [minimum requested fields](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/Extensions/JellyfinAPI/ItemFields.swift#L12-L24), [library query construction](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/Objects/Libraries/ItemLibrary.swift#L101-L184), [resume request](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/Objects/Libraries/ResumeItemsLibrary.swift#L26-L44), [latest request](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/Objects/Libraries/LatestInLibrary.swift#L25-L42)

Minimum useful movie/episode item:

```json
{
  "Id": "<stable item guid>",
  "Name": "Title",
  "SortName": "Title",
  "Type": "Movie",
  "MediaType": "Video",
  "IsFolder": false,
  "ParentId": "<view or season id>",
  "RunTimeTicks": 72000000000,
  "ImageTags": {"Primary":"<art revision>"},
  "UserData": {
    "ItemId": "<same item id>",
    "PlaybackPositionTicks": 0,
    "Played": false,
    "IsFavorite": false
  },
  "MediaSources": [{"Id":"<media source id>"}]
}
```

For episodes add `SeriesId`, `SeriesName`, `SeasonId`, `ParentIndexNumber`, and `IndexNumber`. For a series use `Type:"Series"`, `IsFolder:true`, and a stable ID. Show navigation additionally needs:

```http
GET /Shows/{seriesId}/Seasons?userId=...&fields=...&enableUserData=true
GET /Shows/{seriesId}/Episodes?userId=...&seasonId=...&fields=...&enableUserData=true
```

Both return query-result wrappers. [Swiftfin season call](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/Objects/Libraries/SeasonLibrary.swift#L39-L60), [episode call](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/Objects/Libraries/EpisodeLibrary.swift#L23-L48), [SDK routes](https://github.com/jellyfin/jellyfin-sdk-swift/tree/37a2f5028bd24689b772effb559f72ec5388f021/Sources/Paths)

Artwork is fetched with:

```http
GET /Items/{itemId}/Images/{imageType}?maxWidth=...&tag=...
GET /Items/{itemId}/Images/{imageType}/{index}?maxWidth=...&tag=...
```

Support `Primary` first and the indexed variant for backdrop/other image types; return a correct image MIME type and let unknown sizing/format parameters be advisory. [SDK image route](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/Paths/GetItemImageAPI.swift#L12-L81)

### 5. Negotiate and play an item

Tapping an item first refreshes its full DTO:

```http
GET /Items/{itemId}?userId=<user id>
```

Swiftfin then posts its complete device profile:

```http
POST /Items/{itemId}/PlaybackInfo
Content-Type: application/json

{
  "UserId": "<user id>",
  "MediaSourceId": "<source id>",
  "MaxStreamingBitrate": 120000000,
  "DeviceProfile": {"...":"client capability profile"},
  "AudioStreamIndex": null,
  "SubtitleStreamIndex": null,
  "AutoOpenLiveStream": true
}
```

Accept the full body even if the first implementation only chooses direct play. The exact route is in the [pinned SDK](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/Paths/GetPostedPlaybackInfoAPI.swift#L12-L92), and Swiftfin's builder populates the fields [before sending it](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/Objects/MediaPlayerManager/MediaPlayerItem/MediaPlayerItem%2BBuild.swift#L65-L92).

Minimum direct-play response:

```json
{
  "PlaySessionId": "<opaque per-play session id>",
  "MediaSources": [{
    "Id": "<same source id>",
    "ETag": "<stable media revision>",
    "Name": "Original",
    "Protocol": "File",
    "Type": "Default",
    "Container": "mp4",
    "Size": 123456789,
    "RunTimeTicks": 72000000000,
    "SupportsDirectPlay": true,
    "SupportsDirectStream": true,
    "SupportsTranscoding": false,
    "MediaStreams": [
      {"Index":0,"Type":"Video","Codec":"h264","IsDefault":true},
      {"Index":1,"Type":"Audio","Codec":"aac","IsDefault":true}
    ]
  }]
}
```

`PlaySessionId` and at least one `MediaSources` entry are guarded requirements. Give the full-item source and playback response source the same `Id` and/or `ETag`, because Swiftfin matches by ETag, then open token, then ID. Include `MediaStreams` with stable indexes so audio/subtitle selection can be built. [Swiftfin source selection and guarded fields](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/Objects/MediaPlayerManager/MediaPlayerItem/MediaPlayerItem%2BBuild.swift#L94-L132)

With `MediaType:"Video"` and no `TranscodingUrl`, Swiftfin constructs:

```http
GET /Videos/{itemId}/stream?static=true&tag=<etag>&playSessionId=<id>&mediaSourceId=<source id>
```

It can also request `HEAD` and the extension form `/Videos/{itemId}/stream.{container}`. Jellyfin exposes both forms. [Swiftfin URL construction](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/Objects/MediaPlayerManager/MediaPlayerItem/MediaPlayerItem%2BBuild.swift#L186-L236), [Jellyfin video routes](https://github.com/jellyfin/jellyfin/blob/1fbd8739292cce610231be93daf43368733edf63/Jellyfin.Api/Controllers/VideosController.cs#L312-L376)

Swiftfin builds this URL with the SDK's default `queryAPIKey:false` and hands the bare URL to the native player; the media request therefore cannot be assumed to carry the normal `MediaBrowser` authorization header or an `api_key`. Authorize the stream using a short-lived `PlaySessionId` bound to the authenticated user, item, and media source (or another equally scoped stream grant), and reject expired/mismatched sessions. [SDK URL builder](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/JellyfinClient.swift#L253-L273), [Swiftfin AVPlayer construction](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/Objects/MediaPlayerManager/MediaPlayerProxy/MediaPlayerProxy%2BAVPlayer.swift#L154-L168)

Do not expose an owner filesystem path as `MediaSource.Path`. For video direct play Swiftfin does not need it. If audio is added, implement the Jellyfin `/Audio/{itemId}/stream` route rather than returning a server-local path.

### 6. Byte-range behavior

Direct media must support seeking and probing:

- `HEAD` returns the same representation headers as `GET` without a body.
- A full `GET` returns `200`, correct `Content-Type`, and full `Content-Length`.
- Advertise `Accept-Ranges: bytes`.
- Honor single byte ranges, including open-ended (`bytes=N-`) and suffix (`bytes=-N`) forms, with `206`, exact `Content-Range: bytes start-end/total`, partial `Content-Length`, and only the requested bytes.
- Return `416` with `Content-Range: bytes */total` for an unsatisfiable range.
- Preserve range headers/status when the source is proxied rather than local.

Jellyfin's static local response explicitly enables range processing, and its remote-stream helper forwards `Range`, propagates `206`, `Accept-Ranges`, `Content-Range`, and `Content-Length`. [Jellyfin file-stream helper](https://github.com/jellyfin/jellyfin/blob/1fbd8739292cce610231be93daf43368733edf63/Jellyfin.Api/Helpers/FileStreamResponseHelpers.cs#L28-L110) These details match the normative HTTP range semantics in [RFC 9110, sections 14.1–14.4](https://www.rfc-editor.org/rfc/rfc9110.html#section-14).

### 7. Persist playback progress

Swiftfin reports playback with authenticated JSON posts:

```http
POST /Sessions/Playing
POST /Sessions/Playing/Progress
POST /Sessions/Playing/Stopped
```

The body includes `ItemId`, `MediaSourceId`, `PlaySessionId`, `SessionId`, `PositionTicks`, and selected audio/subtitle indexes; progress adds `IsPaused`. Return any 2xx with no required response body. [Swiftfin progress reporter](https://github.com/jellyfin/Swiftfin/blob/24b4fb3ba7abc6552021a7741ef4850f4acc44a8/Shared/Objects/MediaPlayerManager/MediaProgressObserver.swift#L115-L181), [SDK paths](https://github.com/jellyfin/jellyfin-sdk-swift/tree/37a2f5028bd24689b772effb559f72ec5388f021/Sources/Paths)

Jellyfin ticks are 100-nanosecond units: `seconds * 10,000,000`. Return updated positions in each item's `UserData.PlaybackPositionTicks`, plus `Played` and preferably `PlayedPercentage`, so Continue Watching and resume behavior stay coherent.

Also implement these small state endpoints soon after the core path because both clients advertise watched/favorite synchronization:

```http
POST   /UserPlayedItems/{itemId}?userId=...
DELETE /UserPlayedItems/{itemId}?userId=...
POST   /UserFavoriteItems/{itemId}?userId=...
DELETE /UserFavoriteItems/{itemId}?userId=...
```

They return a `UserItemDataDto`. [SDK watched path](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/Paths/MarkPlayedItemAPI.swift#L12-L32), [favorite path](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/Paths/MarkFavoriteItemAPI.swift#L12-L29)

## Optional, not required for the first successful login/browse/play

- UDP discovery on port 7359. Manual URL entry is sufficient.
- Quick Connect. Return `false` from `/QuickConnect/Enabled` until the full authorization flow exists.
- WebSocket `/socket?api_key=...&deviceId=...`. Swiftfin starts it for realtime events, but core HTTP browsing and playback do not depend on a successful socket. The pinned SDK's URL construction is documented in [source](https://github.com/jellyfin/jellyfin-sdk-swift/blob/37a2f5028bd24689b772effb559f72ec5388f021/Sources/JellyfinClient.swift#L313-L336).
- Transcoding/HLS, Live TV, music, photos, downloads, admin dashboard, remote control, collections, playlists, trickplay, external subtitle delivery, and server-side search/filter parity.

Return syntactically valid empty responses for home rails and optional lists when that lets the client continue; do not fabricate successful mutation behavior.

## Acceptance gate

Do not call the feature complete from HTTP unit tests alone. Verify this exact matrix against release apps over a fresh Kinosail profile:

| Client/mode | Required proof |
|---|---|
| Swiftfin 1.6 iOS | Add manual server URL; see Kinosail server name; password login; see Movies and Shows; open a movie; seek forward/back; stop; reopen at saved position. |
| Swiftfin 1.6 tvOS | Same path, because playback/player behavior can differ from iOS. |
| Remote HTTPS URL/subpath | Repeat login and playback without stripping the configured base path; confirm media bytes remain direct to the owner-hosted Kinosail endpoint. |

During client acceptance, record only method, normalized route template, status, response size, and `Client` value. Never log `Authorization`, `X-Emby-Authorization`, `api_key`, passwords, or raw query strings. Any observed extra route should get a focused fixture/test before implementation.

## Recommended implementation order

1. Compatibility auth middleware plus public system/login endpoints.
2. User views, item/query wrappers, movie browse, full item, and artwork.
3. PlaybackInfo, direct video stream aliases, HEAD/range behavior, and playback progress.
4. Series/seasons/episodes and Continue Watching.
5. Swiftfin release-app acceptance.
6. Watched/favorite mutations, UDP discovery, then optional WebSocket support.

This order preserves Kinosail's existing direct-media architecture: the compatibility API supplies control metadata, while the actual media stream remains between the client and the owner-hosted Kinosail server.
