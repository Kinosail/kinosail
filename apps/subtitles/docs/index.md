---
title: "Kinosail Subtitles: self-hosted subtitle automation"
description: Find missing movie and episode subtitles, validate matches and timing, and save sidecar files on your own server.
section: Start here
last_reviewed: 2026-09-27
---

# Kinosail Subtitles

Kinosail Subtitles is a free, self-hosted app that scans local movies and episodes, finds missing subtitles, and writes validated SRT or WebVTT sidecar files beside the videos. It runs independently from Kinosail Player. Optional Supporter badges do not unlock core features.

<figure class="product-screenshot">
  <picture>
    <source media="(max-width: 600px)" srcset="{{ '/assets/images/subtitles-dashboard-mobile.webp' | relative_url }}" width="780" height="2480">
    <img src="{{ '/assets/images/subtitles-dashboard-desktop.webp' | relative_url }}" width="2880" height="3044" alt="Kinosail Subtitles dashboard with five wanted fictional files, one ready file, and the next subtitle action" fetchpriority="high">
  </picture>
  <figcaption>Actual Subtitles app running in Podman with fictional movies and episodes. <a href="{{ '/assets/images/subtitles-dashboard-desktop.webp' | relative_url }}">View desktop image</a> · <a href="{{ '/assets/images/subtitles-dashboard-mobile.webp' | relative_url }}">View phone image</a></figcaption>
</figure>

- [Install the published container]({{ '/getting-started/install/' | relative_url }})
- [Complete first setup]({{ '/getting-started/first-setup/' | relative_url }})
- [Connect a subtitle provider]({{ '/owner-guide/integrations/' | relative_url }})
- [Find and manage subtitles]({{ '/user-guide/' | relative_url }})
- [Back up and update]({{ '/owner-guide/backups-and-updates/' | relative_url }})
- [Use the HTTP API]({{ '/reference/api/' | relative_url }})
- [Support Kinosail Subtitles and activate a badge]({{ '/owner-guide/supporter/' | relative_url }})
- [Fix a problem]({{ '/troubleshooting/' | relative_url }})

## How does Kinosail Subtitles find a match?

The Server scans the movie and episode folders you choose. It checks existing sidecars and embedded text tracks first. For missing coverage, it searches your configured SubDL, OpenSubtitles.com, or SubSource account. It ranks results by release identity, episode, language, and other match signals, then validates subtitle content and timing before a save.

You can fetch one wanted file or let bounded maintenance fill gaps. The dashboard shows what is ready, wanted, pending, or unavailable. [See matching and timing]({{ '/user-guide/playback/' | relative_url }}) and [configure providers]({{ '/owner-guide/integrations/' | relative_url }}).

## What files can it change?

For a movie such as `Film.mkv`, an English sidecar is named `Film.en.srt`. The Server writes beside the video, so its media mount must be writable. It can upgrade its own managed files after a meaningful improvement. An unknown sidecar needs an exact OpenSubtitles hash match before replacement, and the original is kept as a `.kinosail.bak` recovery file. SubSource downloads remain unchanged under that provider's terms.

Kinosail Player has a different role: it reads media and does not change source files. The apps have separate containers and app state. You can run Subtitles without Player. [Read the library guide]({{ '/owner-guide/libraries/' | relative_url }}).

## Does it upload my videos?

No. Media bytes and saved subtitles stay on your Server. Providers receive bounded search metadata, which can include title, episode, language, release identifiers, and a locally calculated file hash. [Read the privacy details]({{ '/reference/architecture-and-privacy/' | relative_url }}).
