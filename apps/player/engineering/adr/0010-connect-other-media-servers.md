# Connect the Apple clients to other media servers

Status: accepted, 2026-10-24. Jellyfin, Emby and Plex are implemented. The on-device engine and Kinosail Server in the combined library are not implemented yet.

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
- Plex: Sign in with Plex uses a plex.tv code. On iPhone and iPad, Continue with Plex opens the Plex sign-in page in a secure browser sheet. Apple TV shows the code for plex.tv/link and a QR code that opens the same Plex page on a phone. All servers on the account appear, local and remote. One server is added at once. Plex Media Server on port 32400 also shows in the network probe, and a typed address finds Plex too.
- Plex connections are ranked local, then remote, then relay. Each one is checked with `/identity` before use. When the chosen connection stops answering, the next working one is used.
- Jellyfin and Emby: a bounded probe of the local /24 network on port 8096 reads `/System/Info/Public`. That response names the product, so each server shows as Jellyfin or Emby with its own name.
- Jellyfin signs in with Quick Connect first, then username and password. Emby signs in with username and password.
- When a server lists its users, setup shows a Who's watching picker. A server with one user goes straight to its password.
- A typed address is always available.
- After sign-in, a ready screen shows posters and counts from the new library. It starts the device library and loads the combined library in the background, so Start watching opens a populated Home.

## Sign-in on other devices

A viewer signs in to each server once. Other devices then add it with one tap.

- A signed-in account is stored as a synchronizable Keychain item. iCloud Keychain copies it to the viewer's other Apple devices. Removing a server removes the shared copy. Signing out of one device keeps it, so other devices are not affected.
- Setup shows these accounts under From your Apple devices. On Jellyfin, the new device starts Quick Connect and approves it with the shared token, so each device gets its own token and appears as its own device on the server. When Quick Connect is off, and on Emby, the device checks the shared token with the server and uses it.
- The Apple TV and iPad Quick Connect card shows a QR code with the link `kinosail://quickconnect?server=<server ID>&code=<code>`. Scanning it with an iPhone camera opens Kinosail Player, which asks the viewer to approve the code with the matching signed-in Jellyfin account. The TV then signs in without typing.
- Passwords are never stored or shared. Only server tokens are stored.

Plex sign-ins sync the same way. A new device checks the shared token against the server and uses it.

UDP discovery (Jellyfin and Emby port 7359, Plex GDM) needs Apple's multicast networking entitlement. It can be added when Apple grants that entitlement. The probe works without it.

## Playback

The order for each file:

1. Apple's player plays the original when it can. It keeps Picture in Picture, AirPlay, HDR and Dolby Vision output.
2. The on-device engine plays the original when Apple's player cannot. It covers MKV, HEVC, AV1, VC-1, MPEG-2, DTS, TrueHD, FLAC, and PGS, ASS and VobSub subtitles.
3. A server conversion is used only when the device cannot decode the file.

Buffering alone never selects conversion.

Until the on-device engine exists, step 2 is a server remux. Jellyfin and Emby copy compatible video into fragmented MP4 HLS. Plex copies H.264 and HEVC video into MPEG-TS HLS and converts only the streams the device cannot play. Plex sidecar SRT and VTT subtitles are converted to WebVTT on the device. Plex cannot send subtitles that are embedded in a file without converting the video, so those subtitles wait for the on-device engine.

## Limits

- Downloads, casting, bookmarks, device approval, supporter state and reader positions stay Kinosail Server features in v1.
- Plex managed-user switching, Plex Watchlist and Emby Connect are out of scope for v1. My List is not available for titles that only Plex holds; the app says this source does not support the feature.
- Intro and credits markers are used when the source provides them.
