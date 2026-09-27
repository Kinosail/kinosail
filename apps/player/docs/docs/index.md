---
title: Your media. In a browser or on mobile.
description: Install the free Kinosail Server with Docker. Watch in a browser or a compatible Jellyfin mobile app.
hide_contribute: true
---
# Your media. In a browser or on mobile.

Kinosail Server is free to run on your own hardware. The web Player is included. You can also connect compatible Jellyfin mobile apps after you set up trusted HTTPS and enable Jellyfin support.

Bring movies, shows, music, books, and photos together in one library. Watch them in a browser or a compatible Jellyfin mobile app.

<a class="start-link" href="{{ '/quickstart/' | relative_url }}">Install with Docker</a>

On a NAS or Proxmox VE, [get the importable Player and Subtitles files]({{ '/getting-started/platforms/' | relative_url }}) for Synology, TrueNAS, QNAP, CasaOS, Portainer, and Unraid.

## From Server to first play

<div class="app-directory">
<a href="{{ '/quickstart/' | relative_url }}"><strong>Install Player</strong><span>Start the prebuilt Docker container. The installer checks its signed image. Your media stays on your hardware.</span></a>
<a href="{{ '/getting-started/first-setup/' | relative_url }}"><strong>Make it yours</strong><span>Create the Owner account and secure it. Then choose settings for your household.</span></a>
<a href="{{ '/getting-started/add-media/' | relative_url }}"><strong>Add your library</strong><span>Choose your media folder. Let Player scan it. Then find your first film or album.</span></a>
<a href="{{ '/user-guide/playback/' | relative_url }}"><strong>Press play</strong><span>Watch and listen in the web Player or a compatible Jellyfin mobile app. Pick an audio track, turn on subtitles, or resume where you left off.</span></a>
</div>

<figure class="product-shot">
<picture><source media="(max-width: 760px)" srcset="{{ '/assets/images/player-home-mobile.webp' | relative_url }}"><img src="{{ '/assets/images/player-library.webp' | relative_url }}" srcset="{{ '/assets/images/player-library-688.webp' | relative_url }} 688w, {{ '/assets/images/player-library-1024.webp' | relative_url }} 1024w, {{ '/assets/images/player-library.webp' | relative_url }} 1280w, {{ '/assets/images/player-library-2880.webp' | relative_url }} 2880w" sizes="(max-width: 760px) calc(100vw - 32px), (max-width: 1280px) calc(100vw - 360px), 900px" width="1280" height="800" loading="lazy" alt="Kinosail Player Home on desktop or phone, with a fictional film library and original posters"></picture>
<figcaption>Player running with a fictional demo library. <a href="{{ '/assets/images/player-library-2880.webp' | relative_url }}">View desktop image</a> · <a href="{{ '/assets/images/player-home-mobile.webp' | relative_url }}">View phone image</a></figcaption>
</figure>

## Already watching?

Read the [Player feature guide]({{ "/features/" | relative_url }}) for details about watching, reading, offline use, Server management, the API, and MCP.

- **Use another browser.** [Connect it to your home network]({{ '/getting-started/connect-devices/' | relative_url }}).
- **Use a Jellyfin mobile app.** [Turn on compatibility and connect]({{ '/getting-started/connect-devices/' | relative_url }}). Trusted HTTPS is required.
- **Bring your viewing history.** [Keep your media folders and preview a viewing-history import]({{ '/owner-guide/migration/' | relative_url }}).
- **Share your Server with the household.** [Set up Viewer Profiles]({{ '/user-guide/profiles/' | relative_url }}).
- **Look after your library.** [Plan backups and updates]({{ '/owner-guide/backups-and-updates/' | relative_url }}).
- **Something isn't working?** [Start with the symptom]({{ '/troubleshooting/' | relative_url }}).

## Built around your privacy

Local use does not need a Kinosail account. Your Server sends media directly to the web Player or a compatible Jellyfin mobile app. It mounts your original media as read-only. Owners control each Viewer Profile's library and playback access.

Read [Architecture and privacy]({{ '/reference/architecture-and-privacy/' | relative_url }}) to learn where the Server stores data and what optional services can access. Kinosail is source-available under the [PolyForm Perimeter license](https://github.com/Kinosail/kinosail/blob/main/LICENSING.md).

Read the [Security overview]({{ '/security/' | relative_url }}) for account, API, network, and data protections. It also explains what you must protect on your host and how to [report a vulnerability privately](https://github.com/Kinosail/kinosail/security/advisories/new).

## Building on Player?

The [Developer guide]({{ '/developer-guide/' | relative_url }}) covers the versioned API and MCP. [Build from source]({{ '/source-install/' | relative_url }}) to contribute or try code that is not released yet.
