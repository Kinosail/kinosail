---
title: Installation overview
description: Start with Docker, then set up your web Player and media library.
section: Start here
---
# Installation overview

**[Install with Docker]({{ '/quickstart/' | relative_url }})** is the recommended path for running the web Player. Use the prebuilt `ghcr.io/kinosail/kinosail-player:latest` container with the current deployment files. Git downloads those files; Go and a local image build are not required.

The Docker guide installs the continuous container channel. You do not need a numbered GitHub release or an installer archive. Source builds remain a separate path for contributors and development.

## Your first successful setup

1. [Install with Docker]({{ '/quickstart/' | relative_url }}).
2. [Create and secure your Owner]({{ '/getting-started/first-setup/' | relative_url }}).
3. [Add your media]({{ '/getting-started/add-media/' | relative_url }}).
4. [Play an item in your browser]({{ '/user-guide/playback/' | relative_url }}).
5. [Connect browsers and Jellyfin apps]({{ '/getting-started/connect-devices/' | relative_url }}) on your home network.

## Keep media and state separate

| Location | Purpose | Backup treatment |
| --- | --- | --- |
| Media path | Original movies, shows, music, books, and photos | Back up separately; Player mounts it read-only |
| `/config` | Profiles, settings, credentials, sessions, and viewing state | Included in state backups |
| `/cache` | Reproducible playback conversion data | Not included |
| Backup directory | Encrypted recovery archives | Protect with the backup key |

Follow [backups and updates]({{ '/owner-guide/backups-and-updates/' | relative_url }}) before relying on your Server for important state.

## Development and evaluation

Use [build from source]({{ '/source-install/' | relative_url }}) if you are contributing or need to evaluate unpublished code. This is separate from installing the prebuilt container.
