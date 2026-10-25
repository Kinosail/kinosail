---
title: Privacy policy
description: How the Kinosail Player apps handle data on your device and your household Server.
section: Security
---

# Kinosail Player privacy policy

Kinosail Player connects to media servers that you or your household choose and operate. These can be Kinosail, Jellyfin, Emby, or Plex servers. The iPhone, iPad, Apple TV, and Apple Watch apps use those servers to show your library and play your media. Kinosail does not operate a central account, media relay, or analytics service for these apps.

## Data the apps use

The apps send your session credential and requests needed for browsing, playback, downloads, and saved position directly to your selected Server. The Server stores Viewer Profiles, account and session records, library metadata, and viewing activity in its private application data. The Server Owner controls that installation, its backups, access, and retention. Original media stays in the Owner's media storage unless you choose to download a copy to an iPhone or iPad.

On your device, the apps keep the session credential in Keychain. They may keep preferences, viewing progress awaiting sync, cached catalog data and artwork, and, on iPhone and iPad, media you explicitly download. Apple TV can show a credential-free snapshot of titles and artwork in Top Shelf; this can be turned off in Settings, and the snapshot expires after 24 hours. The apps do not include advertising or analytics SDKs and do not use this data for tracking.

## Jellyfin, Emby, and Plex servers

When you add a Jellyfin or Emby server, the app sends your user name and password, or a Quick Connect request, directly to that server. When you add Plex, the app shows a code that you approve on plex.tv. The app then receives a Plex access token from plex.tv, asks plex.tv for the servers your Plex account can use, and can load artwork from Plex's metadata service. Those servers and Plex handle your data under their own privacy practices. The app keeps their credentials in Keychain on your device.

## Apple Watch heart rate

The Apple Watch app can make an optional heart rate graph for a movie. It is off by default. When you start a graph, the Watch asks for read-only access to heart rate in Apple Health. The app reads heart rate samples for the time you watched, matches them to the movie position, and keeps the graph on the Watch. The app does not write to Apple Health. It does not send heart rate data to the iPhone, Apple TV, any server, or Kinosail, and it does not use the data for advertising. You can turn off access at any time in Health settings. Starting a new graph replaces the saved graph, and deleting the Watch app removes it.

## Optional services

The Server Owner may enable integrations, such as metadata providers or an identity provider. Those services receive only the requests the Owner's configuration sends to them and have their own privacy practices. The apps do not enable those services themselves. See [Architecture and privacy]({{ '/reference/architecture-and-privacy/' | relative_url }}) for the data flow and optional integrations.

## Retention and deletion

Sign out in the app to remove its saved credential and cached catalog and artwork for that Viewer Profile. Offline downloads remain on the device until you remove them in Downloads, reset device storage, or uninstall the app. To remove Server-held Profile, session, and viewing data, contact the Owner of your Server; an Owner can manage Profiles, sessions, and stored application data. Server backups follow that Owner's retention settings. Kinosail cannot access or delete data on a Server it does not operate.

You can stop local-network discovery in the device's privacy settings. You can also stop using a Server by signing out and revoke a session from the Server. For questions about this policy, use the [Kinosail project support channels](https://github.com/Kinosail/kinosail/issues/new/choose); do not post credentials, private Server addresses, or media details in a public issue.
