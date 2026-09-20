---
title: Secure Subtitles
description: Protect account access, provider credentials, and media writes.
section: Own the Server
last_reviewed: 2026-09-15
---

# Secure Subtitles

Create and secure the first Owner privately before exposing the web port. Owners require a passkey or TOTP authenticator. Trust the exact HTTPS origin you intend to use; passkeys belong to that origin.

Keep administration local or behind the configured private management access. Subtitle provider requests are outbound and do not require publishing the admin interface to the internet. Grant the container only the needed media-write permissions.

Store provider credentials in protected Owner settings or mounted secret files. Deployment-managed values remain read-only in the UI. Back up external configuration and keys separately from application state and media.

Report vulnerabilities through the [repository security policy](https://github.com/Kinosail/kinosail/blob/main/SECURITY.md). Never include credentials, private media, or backup archives in public reports. See [privacy]({{ '/reference/architecture-and-privacy/' | relative_url }}) and [recovery]({{ '/owner-guide/backups-and-updates/' | relative_url }}).
