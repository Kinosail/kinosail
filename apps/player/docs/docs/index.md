---
title: Your media. At home in your browser.
description: Install the free Kinosail Server with Docker and start watching in your browser.
hide_contribute: true
---
# Your media. At home in your browser.

Kinosail Server is free to run on your own hardware. The web Player is included. You can also connect compatible Jellyfin mobile apps after you set up trusted HTTPS and enable Jellyfin support.

Bring movies, shows, music, books, and photos together in one library. Watch them in a browser or a compatible Jellyfin mobile app.

<a class="start-link" href="{{ '/quickstart/' | relative_url }}">Install with Docker</a>

## From Server to first play

<div class="app-directory">
<a href="{{ '/quickstart/' | relative_url }}"><strong>Install Player</strong><span>Start the prebuilt Docker container. The installer checks its signed image. Your media stays on your hardware.</span></a>
<a href="{{ '/getting-started/first-setup/' | relative_url }}"><strong>Make it yours</strong><span>Create the Owner account and secure it. Then choose settings for your household.</span></a>
<a href="{{ '/getting-started/add-media/' | relative_url }}"><strong>Add your library</strong><span>Choose your media folder. Let Player scan it. Then find your first film or album.</span></a>
<a href="{{ '/user-guide/playback/' | relative_url }}"><strong>Press play</strong><span>Watch and listen in the web Player or a compatible Jellyfin mobile app. Pick an audio track, turn on subtitles, or resume where you left off.</span></a>
</div>

## Already watching?

Read the [Player feature guide]({{ "/features/" | relative_url }}) for details about watching, reading, offline use, Server management, the API, and MCP.

- **Use another browser.** [Connect it to your home network]({{ '/getting-started/connect-devices/' | relative_url }}).
- **Use a Jellyfin mobile app.** [Turn on compatibility and connect]({{ '/getting-started/connect-devices/' | relative_url }}). Trusted HTTPS is required.
- **Share your Server with the household.** [Set up Viewer Profiles]({{ '/user-guide/profiles/' | relative_url }}).
- **Look after your library.** [Plan backups and updates]({{ '/owner-guide/backups-and-updates/' | relative_url }}).
- **Something isn't working?** [Start with the symptom]({{ '/troubleshooting/' | relative_url }}).

## Built around your privacy

Local use does not need a Kinosail account. Your Server sends media straight to your browser. It mounts your original media as read-only. Owners control each Viewer Profile's library and playback access.

Read [Architecture and privacy]({{ '/reference/architecture-and-privacy/' | relative_url }}) to learn where the Server stores data and what optional services can access. Kinosail is source-available under the [PolyForm Perimeter license](https://github.com/Kinosail/kinosail/blob/main/LICENSING.md).

## Building on Player?

The [Developer guide]({{ '/developer-guide/' | relative_url }}) covers the versioned API and MCP. [Build from source]({{ '/source-install/' | relative_url }}) to contribute or try code that is not released yet.
