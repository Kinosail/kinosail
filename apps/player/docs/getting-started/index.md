---
title: Installation overview
description: Start with Docker, then set up your web Player and media library.
section: Start here
---
# Installation overview

**[Install with Docker]({{ '/quickstart/' | relative_url }})** is the recommended path for running the web Player. Use the prebuilt image with the matching signed release bundle. You do not need Git, Go, or a local image build.

Check [Releases](https://github.com/Kinosail/kinosail/releases) for a published Player bundle before starting. No releases were published when checked on September 20, 2026. Source builds remain a separate path for contributors and early evaluation.

## Your first successful setup

1. [Install with Docker]({{ '/quickstart/' | relative_url }}).
2. [Create and secure your Owner]({{ '/getting-started/first-setup/' | relative_url }}).
3. [Add your media]({{ '/getting-started/add-media/' | relative_url }}).
4. [Play an item in your browser]({{ '/user-guide/playback/' | relative_url }}).
5. [Connect another browser]({{ '/getting-started/connect-devices/' | relative_url }}) on your home network.

## Keep media and state separate

| Location | Purpose | Backup treatment |
| --- | --- | --- |
| Media path | Original movies, shows, music, books, and photos | Back up separately; Player mounts it read-only |
| `/config` | Profiles, settings, credentials, sessions, and viewing state | Included in state backups |
| `/cache` | Reproducible playback conversion data | Not included |
| Backup directory | Encrypted recovery archives | Protect with the backup key |

Follow [backups and updates]({{ '/owner-guide/backups-and-updates/' | relative_url }}) before relying on your Server for important state.

## Development and evaluation

Use [build from source]({{ '/source-install/' | relative_url }}) if you are contributing or need to evaluate a checkout before a signed release exists. This is separate from the normal Docker release installation.
