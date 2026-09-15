---
title: Architecture and privacy
description: Understand the one-container design, local data boundary, direct media path, and optional service calls.
section: Reference
last_reviewed: 2026-08-28
---

# Architecture and privacy

Kinosail is an API-driven monolith. One Kinosail Server container provides the HTTP API, web app, background work, embedded SQLite state, media scanning, and FFmpeg playback work.

## One deployable boundary

The supported self-hosted deployment starts one Kinosail Server container. It does not require a database sidecar or a Kinosail-managed media proxy. The usual persistent mounts are:

| Mount | Purpose | Content rule |
| --- | --- | --- |
| `/media` | Library Content | mounted read-only |
| `/config` | configuration and application state | private and persistent |
| `/cache` | reproducible playback and analysis cache | disposable |
| `/backups` | encrypted backup files | private; use another disk or NAS when possible |

The container includes or invokes FFmpeg, FFprobe, and `fpcalc` through configured paths. It stores metadata, Profiles, credentials, sessions, viewing state, and the activity journal in the application data boundary. It does not copy Library Content into the application database.

## Media data flow

The Server scans the read-only Library Content mount and stores normalized catalog state. A Viewer request is authorized by the same application operation whether it comes from the web app, `/api/v1`, a supported Jellyfin flow, or an optional MCP connection.

For direct playback, the Viewer connects to the owner-hosted Server and receives the source through the direct media route. For a compatible client, the Server creates a bounded HLS or remux representation in `/cache` and serves it from that Server. No Kinosail-operated relay, tunnel, or media cache receives the media.

## Connection choices

- Secure LAN HTTPS keeps normal use on the private network. Its private certificate is the default.
- Trusted HTTPS through DuckDNS or deSEC gives the LAN address public certificate trust. It remains optional and does not open a router port.
- Optional Jellyfin compatibility adds tested client routes without changing the HTTPS or network boundary.
- WireGuard provides a verified direct path for Owner administration and paired managed devices.
- Public HTTPS exposes a separate, restricted Viewer boundary when the Owner enables it.

See [Connect phones, TVs, and Jellyfin apps]({{ '/getting-started/connect-devices/' | relative_url }}) for the device setup choices.

Remote access is off by default. Public HTTPS requires public TCP 443 reachability and a matching TLS name. Carrier-grade NAT, router policy, DNS, firewall rules, and physical network security remain Owner responsibilities. Kinosail cannot remove a network reachability limit without a relay.

The public boundary excludes Owner administration, API keys, setup, Profile management, backups, diagnostics, MCP/OAuth administration, and other administrative routes. Remote password login is disabled. A remote Viewer uses a passkey or a short-lived Quick Connect request approved from a strongly authenticated local path.

## Optional integrations

An integration is disabled until the Owner configures it. Kinosail sends only the requests needed by that integration:

- TMDB can enrich movie and Show metadata and artwork. Follow TMDB attribution and licensing terms.
- OpenID Connect (OIDC) can provide identities that an existing local Profile explicitly links.
- System for Cross-domain Identity Management (SCIM) 2.0 can provision passwordless Viewer Profiles at `/scim/v2`.
- DLNA can publish a configured local DLNA service.
- Webhooks can receive configured event notifications.
- MCP can provide an authenticated tool connection through the configured OAuth resource and authorization server.
- DuckDNS or deSEC can update the configured trusted LAN DNS record. DuckDNS also supports the separate remote-access record.

Outbound integration URLs are validated. Kinosail does not follow redirects by default and rejects unsafe network destinations. Integration providers can still receive the data present in the request. Review each provider's terms before enabling it.

## Privacy and security boundary

Library Content, Profile policy, credentials, and Viewing Activity stay on the owner-hosted Server unless the Owner explicitly exports or sends related data through an enabled integration. Passwords are stored as Argon2id hashes. API keys are scoped, hashed, tracked, and normally expire after 30 days. Automatic backups are encrypted and fail closed without a key.

The local activity journal links records with an integrity chain. Security events omit passwords, tokens, secret values, request bodies, and URL queries. The release container uses a non-root, read-only runtime with no capabilities and resource limits.

Privacy does not remove host duties. Protect the host, disks, backups and backup key, router, DNS account, TLS trust, firewall, and physical access. Remove private paths and titles before sharing diagnostics.

## Source of truth

This explanation follows the application boundary in `internal/server`, configuration in `internal/configuration`, the release README, and `SECURITY.md`. See [Configuration]({{ '/reference/configuration/' | relative_url }}), [Remote access]({{ '/owner-guide/remote-access/' | relative_url }}), and [Security]({{ '/owner-guide/security/' | relative_url }}) for operating procedures.
