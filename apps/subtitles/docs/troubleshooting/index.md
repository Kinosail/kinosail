---
title: Troubleshoot Subtitles
description: Start with the visible symptom and preserve your data.
section: Fix a problem
last_reviewed: 2026-09-15
---

# Troubleshoot Subtitles

| Symptom | First check |
| --- | --- |
| Server will not start | Read container logs; check the media path, port, configuration, and mounted secret-file paths. |
| No videos appear | Check the container's `/media` mapping, selected relative folders, permissions, and scan status. |
| No provider results | Confirm credentials, language, episode/release identity, and provider quota. A configured provider may have no match. |
| Subtitle cannot be saved | Check write permission for the container account and whether an existing sidecar is protected. |
| Subtitle language/timing is wrong | Confirm the selected language and release; inspect the result in a player before accepting a replacement. |
| A setting is read-only | Change its managing environment or YAML value, then restart if required. |

From `apps/subtitles/`, inspect `docker compose ps` and `docker compose logs --tail 100 kinosail` (or Podman equivalents). Keep credentials, private titles/paths, and backup files out of reports.

Server logs use `debug`, `info`, `warn`, and `error`; the default is `info`. An Owner can temporarily set `logging.level` to `debug` in **Settings → Configuration** without restarting. Repeat the problem, then restore `info`. Match a browser response's `X-Request-ID` to the Server log's `request_id`, status, and route pattern. Remove secrets and private details before sharing a log excerpt.

- [Installation and startup]({{ '/troubleshooting/install-and-startup/' | relative_url }})
- [Scanning and matching]({{ '/troubleshooting/scanning-and-metadata/' | relative_url }})
- [Subtitle timing and saves]({{ '/troubleshooting/playback/' | relative_url }})
- [Sign-in and access]({{ '/troubleshooting/sign-in-and-access/' | relative_url }})

Do not delete the state volume to fix sign-in or startup. Restore deliberately from a known backup; see [recovery]({{ '/owner-guide/backups-and-updates/' | relative_url }}).
