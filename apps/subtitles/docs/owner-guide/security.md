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

## Automatic sign-out

Open **Settings → Security → Automatic sign-out** as an Owner on a private connection.
Choose inactivity and total sign-in limits separately for **Private access** and **Public access**.
Private access covers local network and WireGuard browsers. Public access covers public-internet Viewer sessions.

Inactivity choices range from 15 minutes to one year. Total sign-in choices range from four hours to one year.
The inactivity limit must not exceed the total limit. There is no unlimited option.
Longer limits keep signed-in devices authorized for longer unless you revoke their sessions.

Longer limits apply to new sign-ins. Existing sessions retain their original ceilings.
Shorter limits also apply to existing sessions from their original sign-in and last activity times.
A session already beyond either limit signs out and cannot be restored by raising the limits.

Until you save public timeouts, public browsers keep the existing private inactivity limit and an absolute limit of at most eight hours.
Public non-browser sessions keep their existing eight-hour expiry. Saving public timeouts makes that policy independent.
Owner administration, strong authentication, profile restrictions, and session revocation still apply.

The Owner API uses `PUT /api/v1/settings/session-timeouts` for private access and `PUT /api/v1/settings/public-session-timeouts` for public access.
Both accept `inactiveHours` and `absoluteHours`. Read effective limits and `publicSessionTimeoutsConfigured` from `GET /api/v1/settings`.

Inactivity limits apply to browser sessions. Other public clients use the absolute limit.

Before using a Server version released before public timeout settings, select **Use original public limits** in Settings.
This removes only the public timeout settings and shortens existing public sessions to the original limits.
It preserves private settings, other preferences, current data, and session revocations.
The API equivalent is `DELETE /api/v1/settings/public-session-timeouts` with an empty body and current Owner authentication.
A successful reset sets `publicSessionTimeoutsConfigured` to `false`.
Confirm this result. Select **Sign out other devices**, then sign out of your current Owner session.
The API equivalents are `DELETE /api/v1/sessions`, then `DELETE /api/v1/session`.
Stop the Server, change its version, and sign in again. Do not restore an old authentication backup.
Older versions do not enforce stored browser inactivity ceilings, so signing everyone out is required for this rollback.
