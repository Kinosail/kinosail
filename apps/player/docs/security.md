---
title: Security
description: Learn how Kinosail protects accounts, requests, media, and remote access, and how to report a vulnerability privately.
section: Security
---

# Security

Kinosail runs on your hardware. You control its network access, data, and updates. Kinosail includes account, request, and network controls. These controls reduce risk. They cannot protect a Server or its host from every attack.

This page describes Kinosail Player. It explains the main protections, their technical limits, and the settings you must manage.

## Report a vulnerability

[Report a vulnerability privately on GitHub](https://github.com/Kinosail/kinosail/security/advisories/new). Do not post exploit details in a public issue, discussion, or support request.

Include these details in your report:

- The app and version, or the source commit.
- The host OS, architecture, and container engine.
- The affected feature or endpoint.
- The fewest steps that reproduce the issue.
- The expected and observed behavior.
- The impact and access needed, such as Owner, Viewer, local network, or public listener.

Do not include passwords, tokens, backup files, private URLs, media paths, media titles, or viewing activity. Redact logs and screenshots.

If private reporting is unavailable to you, follow the fallback steps in the [repository security policy](https://github.com/Kinosail/kinosail/blob/main/SECURITY.md). There is no published response-time promise. The maintainer coordinates validation and disclosure privately.

## What Kinosail protects

Kinosail checks identity, role, Profile policy, and access scope before it returns protected data or changes state. It keeps Owner administration on the private management path. It mounts original media as read-only in the supported installation.

The supported release container runs as a non-root user, uses a read-only runtime filesystem, drops Linux capabilities, and has resource limits. When you enable public HTTPS, the supported setup adds a separate restricted gateway. The gateway has no media or application-state mounts.

These protections do not replace host security. Anyone with control of the host, its storage, or the container runtime may be able to access Server data.

## Accounts and credentials

### Roles and Profile rules

- An **Owner** can manage the Server. A **Viewer Profile** can use only the Libraries and features allowed by its settings.
- Viewer rules can limit ratings, Libraries, viewing times, remote access, transcoding, and downloads.
- Kinosail requires recent strong sign-in for sensitive Owner actions.
- Extra sign-in protection is required for Viewer Profiles by default. An Owner can change this policy.

### Sign-in methods and sessions

Kinosail supports passwords, passkeys, and time-based one-time passwords (TOTP). Passwords use Argon2id hashes. Kinosail does not store the original password.

Sessions expire according to Owner settings. Owners can revoke a session or sign out other devices. Recovery codes work once; store them as secrets.

Passkey sign-in uses a browser ceremony bound to the Server address. Passkey ceremony cookies use short lifetimes, `HttpOnly`, and `SameSite=Strict`. Passkeys need HTTPS that the device trusts, except for the browser's supported local development cases.

### API keys and integrations

An API key receives only the scopes selected by its Owner. Kinosail stores a hash of the key, shows the secret once, and normally sets a 30-day expiry. Revoke keys that are lost or no longer needed.

Optional MCP connections use explicit grants. An Owner can review and revoke HTTPS grants. Host STDIO MCP uses the host's Docker, Podman, or SSH access and has Owner access; treat that host access as privileged.

OpenID Connect, SAML, SCIM, webhooks, and metadata providers are optional. Enable only the services you use. Their providers may receive data included in each request.

See [Secure accounts]({{ '/owner-guide/security/' | relative_url }}) and [Connect an MCP client]({{ '/developer-guide/mcp/' | relative_url }}).

## Requests and authorization

Player uses the same application operations for its web interface and API. Each route must have an explicit access rule. Unknown routes, unsupported methods, and unclassified API routes fail closed.

For protected requests, Kinosail checks the session or API key, its role, Viewer policy, remote-access policy, and required API-key scope. An `admin` API-key scope does not grant library access by itself. API keys cannot use session-only identity and passkey operations.

Mutations also require valid input, an allowed request origin, and the expected content type. Request bodies have size limits. Invalid, expired, revoked, or insufficient credentials are rejected before the operation changes state.

Public access has a separate route policy. It does not expose Owner administration, API-key management, setup, backups, Profile management, diagnostics, or MCP administration. Public sign-in is limited to approved Viewer methods. Some media routes use short-lived or purpose-bound capabilities; a capability for one resource does not authorize an arbitrary URL.

The exact route contract is in the repository's [API authorization matrix](https://github.com/Kinosail/kinosail/blob/main/SECURITY.md#api-authorization-matrix), with route tests for [Player](https://github.com/Kinosail/kinosail/blob/main/apps/player/internal/server/auth_all_routes_test.go) and shared [API-key scopes](https://github.com/Kinosail/kinosail/blob/main/packages/identitycore/access.go).

## Network and remote access

Remote access is off by default. Set up the first Owner over a private local connection before you expose the Server to another network.

The local HTTPS certificate is private to your Server. Devices must trust it. Trusted HTTPS can provide a publicly trusted certificate for LAN use without opening a router port. Public HTTPS is a separate feature that needs Owner setup, a public hostname, a certificate, and router access.

Public HTTPS allows only eligible remote Viewer access. Owner administration stays private. The supported public HTTPS setup uses a separate gateway container with no media, configuration, account, backup, or DNS-token mounts. The application still checks each request against the public Viewer policy.

The gateway is an extra boundary, not a guarantee against a full compromise of the application or host. A Server or host vulnerability could expose more than the gateway's intended routes.

Read [Configure remote access]({{ '/owner-guide/remote-access/' | relative_url }}) before opening a public listener. It explains setup, route limits, recovery, and the public-access kill switch.

## Data, media, and secrets

The Server stores its database, Profiles, credentials, sessions, viewing activity, and configuration in its private application-data directory. It does not copy original Library Content into its database. The supported installation mounts media read-only.

Subtitles are a separate app and can write subtitle sidecar files beside media. Protect those files and keep independent recovery copies.

Configuration rejects unknown keys and invalid combinations. Secret values can come from mounted files. The configuration API does not return secret values. Do not put secrets in a committed configuration file.

Automatic backups are encrypted and fail closed when no backup key is configured. Store that key separately from the backup files. Backups can contain sensitive Server data, so restrict access and keep an off-host copy in a protected location.

Security events omit passwords, tokens, secret values, request bodies, and URL query values. Diagnostics can still contain private details such as local file paths or media titles. Review and redact diagnostics before sharing them.

See [Architecture and privacy]({{ '/reference/architecture-and-privacy/' | relative_url }}) and [Back up and update]({{ '/owner-guide/backups-and-updates/' | relative_url }}).

## Software and release checks

The official public website is [kinosail.com](https://kinosail.com/), and the source repository is [Kinosail/kinosail on GitHub](https://github.com/Kinosail/kinosail). The published Player image is `ghcr.io/kinosail/kinosail-player:latest`.

The recommended installer verifies the container image's keyless signature and pins its digest. A checksum can detect a changed file, but it does not prove who published it. Use the signed check described in [Install with Docker]({{ '/quickstart/' | relative_url }}).

Security maintenance targets current `main` and the latest published app release, when one exists. Check the [repository security policy](https://github.com/Kinosail/kinosail/blob/main/SECURITY.md) for the current supported revisions and disclosure process. Older releases do not have a long-term-support promise.

## What you must protect

Kinosail cannot set your host firewall, disk encryption, physical access, DNS account security, certificate trust, router rules, or backup custody. Keep the operating system and container engine updated. Limit who can access the host and container socket. Use a strong unique Owner password, keep recovery codes and backup keys private, and update Kinosail when a fix is available.

If you do not need remote access, leave it off. If you enable it, follow the [remote access guide]({{ '/owner-guide/remote-access/' | relative_url }}) and test the public address from outside your home network.

## Technical references

- [Security policy and route authorization matrix](https://github.com/Kinosail/kinosail/blob/main/SECURITY.md)
- [Player route authorization tests](https://github.com/Kinosail/kinosail/blob/main/apps/player/internal/server/auth_all_routes_test.go)
- [API scope policy](https://github.com/Kinosail/kinosail/blob/main/packages/identitycore/access.go)
- [Configuration reference]({{ '/reference/configuration/' | relative_url }})
- [Architecture and privacy]({{ '/reference/architecture-and-privacy/' | relative_url }})
