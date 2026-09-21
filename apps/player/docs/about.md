---
title: About these docs
description: How Kinosail documentation is organized, maintained, and published.
section: Build & contribute
---
# About these docs

These docs cover the Kinosail web Player: installing its Server with Docker, watching in your browser, and keeping household access and state secure. Start with [Docker installation]({{ '/quickstart/' | relative_url }}) or select the app you already run.

## Find the right kind of guide

- **Getting started** takes you through a first successful setup.
- **User guides** explain everyday tasks.
- **Owner guides** cover configuration, security, backups, and updates.
- **Reference** provides exact settings, formats, and API details.
- **Troubleshooting** starts with the symptom you can see.

Search covers the Player guides and reference together. On wide screens, use **On this page** to jump within a long guide; on smaller screens, use **Menu** to open navigation and search. Press **Ctrl K** or **Command K** to focus search.

## Maintained with the source

Guides live in `apps/player/docs`, beside the Player implementation. The reading interface lives alongside the guides; publishing tools live in `engineering/documentation`. Source installation is a secondary path for contributors and early evaluation. Other Kinosail applications and native clients are outside this site’s current scope.

GitHub Pages hosts static files. Search runs in your browser; this site does not ask for Server credentials or connect to your installation. No analytics or external font service is required.

## Design references

The organization follows the clear separation of installation, administration, and clients in [Jellyfin's documentation](https://jellyfin.org/docs/) and the first-success setup path in [Immich's documentation](https://docs.immich.app/). Kinosail keeps its own typography, colors, and product language.

## Report a correction

Use **Edit this page** or **Open an issue** below. For application failures, include your app and revision and follow the [support guide](https://github.com/Kinosail/kinosail/blob/main/SUPPORT.md). Never paste credentials or unredacted private logs into a public issue.
