---
title: Architecture and privacy
description: Understand local state, provider searches, and filesystem writes.
section: Reference
last_reviewed: 2026-09-15
---

# Architecture and privacy

One Subtitles Server process contains the web UI, versioned API, scanner, embedded state, and provider adapters. Web and API requests use shared operations and validation. It does not require a Kinosail-hosted account or relay your media.

Media, sidecars, credentials, and application state stay on the owner-hosted machine. Enabled providers receive search metadata such as title, filename/release information, language, media type, episode identity, external IDs, and an OpenSubtitles file hash where applicable. That hash is computed locally; the media file is not uploaded.

Downloaded candidates are untrusted. The Server bounds responses, redirects, hosts, archives, sizes, timing, and subtitle content before writing atomically beside a scanned video. Unknown sidecars receive additional replacement protection and recovery copies. SubSource downloads are validated but stored unchanged.

Backups of application state do not include the external media tree. Protect original videos, sidecars, deployment files, and keys separately. See [recovery]({{ '/owner-guide/backups-and-updates/' | relative_url }}).
