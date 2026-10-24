# Connect the Apple clients to other media servers

Status: accepted for implementation, 2026-10-24.

## Decision

The iOS, iPadOS and tvOS clients browse and play from Plex, Jellyfin and Emby servers, as well as from Kinosail Server. The baseline is the source list of Infuse. The goal is the Kinosail experience on every source. Kinosail Server stays the most complete experience because it owns its metadata, markers and conversion.

The product is opinionated: one way to add each kind of server, one combined library, and automatic playback choices.

- v1 sources: Kinosail, Jellyfin, Emby and Plex.
- v2 sources: SMB, WebDAV and NFS shares, then cloud drives.

## Architecture: an on-device Kinosail API

The clients already depend on one strict contract: `/api/v1`, `/art`, `/media` and `/hls` on one origin. Every screen, cache, resume row and validator uses it.

A device library runs inside the app. It serves the same contract on `127.0.0.1` and translates each request to the connected sources. The app connects to it like any Kinosail Server, so existing screens and response validation stay unchanged.

- The listener binds only to `127.0.0.1` and requires a per-install bearer token. It serves only the bounded routes it implements.
- A route that the sources cannot support returns the error a Kinosail Server returns for a missing feature. The device library never returns invented state or empty success.
- Source credentials live in the Keychain. They are never logged. They never appear in URLs that the app's screens or player receive.
- Media and HLS requests are proxied with the source's own authorization, as `MediaTransport` already proxies Kinosail media.
- Item IDs are stable 16-character hex values derived from the source and its item ID. A local index maps them back.

## Combined library

All connected sources appear in one library. A movie, show or episode that exists on more than one source shows once. Matches use exact provider IDs (TMDB, IMDb, TVDB) only. Title matching alone never merges items.

Playback picks the best version: a version that plays directly before one that needs conversion, then the highest resolution, then a source on the local network. Watch progress is written back to every source that holds the title.

## Onboarding

One Add a library screen finds servers without typing:

- Kinosail: Bonjour, as before.
- Plex: Sign in with Plex shows a code and QR code for plex.tv/link. All servers on the account appear, local and remote.
- Jellyfin and Emby: a bounded probe of the local /24 network on port 8096 reads `/System/Info/Public`. That response names the product, so each server shows as Jellyfin or Emby with its own name.
- Jellyfin signs in with Quick Connect first, then username and password. Emby signs in with username and password.
- A typed address is always available.

UDP discovery (Jellyfin and Emby port 7359, Plex GDM) needs Apple's multicast networking entitlement. It can be added when Apple grants that entitlement. The probe works without it.

## Playback

The order for each file:

1. Apple's player plays the original when it can. It keeps Picture in Picture, AirPlay, HDR and Dolby Vision output.
2. The on-device engine plays the original when Apple's player cannot. It covers MKV, HEVC, AV1, VC-1, MPEG-2, DTS, TrueHD, FLAC, and PGS, ASS and VobSub subtitles.
3. A server conversion is used only when the device cannot decode the file.

Buffering alone never selects conversion.

## Limits

- Downloads, casting, bookmarks, device approval, supporter state and reader positions stay Kinosail Server features in v1.
- Plex managed-user switching, Plex Watchlist and Emby Connect are out of scope for v1.
- Intro and credits markers are used when the source provides them.
