---
title: Your media. At home in your browser.
description: Run the free Kinosail Server with Docker, connect a Jellyfin mobile app, and start watching in your browser.
hide_contribute: true
---
# Your media. At home in your browser.

Kinosail Server is free to run on your own hardware, and the web Player is included. Kinosail mobile apps may have a separate purchase price. You can also connect compatible Jellyfin mobile apps after enabling the optional integration over trusted HTTPS.

Bring your movies, shows, music, books, and photos together, then enjoy them in a browser or a compatible Jellyfin mobile app.

<a class="start-link" href="{{ '/quickstart/' | relative_url }}">Install with Docker</a>

## From Server to first play

<div class="app-directory">
<a href="{{ '/quickstart/' | relative_url }}"><strong>Install Player</strong><span>Start the prebuilt Docker container using the installer that verifies its signed image. Keep your media on your own hardware.</span></a>
<a href="{{ '/getting-started/first-setup/' | relative_url }}"><strong>Make it yours</strong><span>Create your Owner, secure access, and choose the settings for your household.</span></a>
<a href="{{ '/getting-started/add-media/' | relative_url }}"><strong>Add your library</strong><span>Point Player at your media, let it scan, and find your first film or album.</span></a>
<a href="{{ '/user-guide/playback/' | relative_url }}"><strong>Press play</strong><span>Watch and listen in the web Player, or connect a compatible Jellyfin mobile app. Pick audio tracks, turn on subtitles, and continue where you left off.</span></a>
</div>

## Already watching?

Explore the [complete Player feature guide]({{ "/features/" | relative_url }}) for viewing, reading, offline use, Server management, API, and MCP workflows.

- **Use another browser.** [Connect on your home network]({{ '/getting-started/connect-devices/' | relative_url }}).
- **Use a Jellyfin mobile app.** [Enable compatibility and connect]({{ '/getting-started/connect-devices/' | relative_url }}); trusted HTTPS is required.
- **Share your Server with the household.** [Set up Viewer Profiles]({{ '/user-guide/profiles/' | relative_url }}).
- **Look after your library.** [Plan backups and updates]({{ '/owner-guide/backups-and-updates/' | relative_url }}).
- **Something isn't working?** [Start with the symptom]({{ '/troubleshooting/' | relative_url }}).

## Built around your privacy

Local use needs no Kinosail-hosted account. Your Server sends media directly to your browser and mounts the original media read-only. Owners control each Viewer Profile's library access and playback permissions.

Read [architecture and privacy]({{ '/reference/architecture-and-privacy/' | relative_url }}) for storage and optional-service boundaries. Kinosail is source-available under the [PolyForm Perimeter license](https://github.com/Kinosail/kinosail/blob/main/LICENSING.md).

## Building on Player?

The [developer guide]({{ '/developer-guide/' | relative_url }}) covers the versioned API and MCP. [Build from source]({{ '/source-install/' | relative_url }}) when contributing or evaluating an unreleased revision.
