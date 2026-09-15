---
title: Add movie and episode folders
description: Choose writable folders inside the configured media mount.
section: Start here
last_reviewed: 2026-09-15
---

# Add movie and episode folders

The host directory in `KINOSAIL_MEDIA_PATH` appears at `/media` inside the container. Select relative subfolders in setup or library settings. For example, host `/srv/media/Movies` appears as `/media/Movies`, and the folder entry is `Movies`.

```text
media/
  Movies/
    Example Film (2025)/
      Example Film (2025).mkv
      Example Film (2025).en.srt
  Shows/
    Example Show/
      Season 01/
        Example Show S01E01.mkv
        Example Show S01E01.en.srt
```

Start with a small folder you control. The whole mount is selected when the folder list is `.`; adding a specific folder narrows the scope. Scan and confirm that videos appear before requesting subtitles. Keep season/episode identifiers and useful release information so providers can distinguish versions.

An exact untagged sidecar such as `Film.srt` counts as a default subtitle. A tagged sidecar such as `Film.en.srt` covers that language. Provider downloads must pass validation before an atomic write beside the scanned video.

Subtitles needs write access. Player can read the same media tree through its own read-only mount, but the apps must not share their configuration databases. See [library management]({{ '/owner-guide/libraries/' | relative_url }}).
