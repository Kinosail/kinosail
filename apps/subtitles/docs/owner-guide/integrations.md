---
title: Connect integrations
description: Configure metadata, subtitles, compatible clients, Home Assistant, identity, webhooks, DLNA, and MCP.
section: Own the Server
---

# Connect integrations

Enable only the integrations you need. Open **Settings → Access** and use **Settings → Deployment configuration** for values that need a restart or an external secret.

## Configure metadata and subtitles

TMDB enrichment is optional. Set its access token and addresses in deployment configuration, then select **Refresh missing metadata** from **Settings → Library discovery**. Kinosail uses local NFO files, embedded tags, and folder artwork without TMDB. TMDB attribution and licensing terms still apply.

Subtitle search is optional. Configure SubDL, OpenSubtitles.com, or SubSource. One provider is enough. Kinosail searches every configured provider automatically. Then choose a lowercase ISO **Preferred language** such as `en` or `pt-br` in **Settings → Subtitles**. Kinosail checks sidecars and embedded text first. Weak release matches must align with local speech before Kinosail writes them. Providers receive only the bounded search data required for the selected action.

SubSource is limited to personal household use. Accept its terms when you save its API key. Kinosail validates SubSource files but preserves their stored bytes. It does not rewrite, synchronize, shift, or convert the stored sidecar.

## Enable compatible clients

Jellyfin support is off by default. Open the setup wizard and go to **Devices**. Configure trusted HTTPS through DuckDNS or deSEC before you select **Allow compatible Jellyfin apps to connect**.

This requirement prevents certificate failures in clients that reject private certificates or cannot install a local certificate. Kinosail does not offer HTTP or an ignore-certificate fallback.

Save trusted HTTPS, enable Jellyfin apps, then restart Kinosail once. Settings shows the trusted HTTPS Server URL to enter in a compatible client. This enables tested flows for Android, Android TV, iOS, and Swiftfin. It does not certify every physical device.

Prefer Quick Connect. Approval requires a recently verified local or WireGuard session. Every Jellyfin sign-in session is media-only, including Owner approval.

Local password fallback is blocked for Profiles with TOTP and when the Server requires MFA. Public password login is always blocked.

Some clients omit credentials on artwork requests. Kinosail permits only local anonymous item artwork. Anonymous browsing and media access remain blocked.

Follow [Connect phones, TVs, and Jellyfin apps]({{ '/getting-started/connect-devices/' | relative_url }}) for the complete wizard and connection procedure.

## Configure Home Assistant

Home Assistant support is off by default. Enable it in the setup wizard or Owner settings. Create a ten-minute pairing code, then add the [Kinosail Home Assistant integration](https://github.com/MikeO7/kinosail-home-assistant). The integration can browse Library Content and control active Kinosail browser players. Streams travel directly from this Server.

Turning the setting off hides every Home Assistant route and revokes paired connections. Deployment-managed installations can set `KINOSAIL_HOME_ASSISTANT_ENABLED`.

## Configure identity and provisioning

OpenID Connect and SAML settings live in **Settings → Deployment configuration**. Restart Kinosail after a change.

For OpenID Connect, register the displayed **Return address** exactly. Set the issuer, client ID, client secret, and stable **Identity claim**. Keep `sub` unless the provider uses another stable claim, such as Microsoft Entra `oid`.

For SAML, copy the displayed service-provider metadata and Assertion Consumer Service URLs. Supply one provider metadata source: a URL or downloaded XML. Keep the **Identity attribute** as `NameID`, or use one signed assertion attribute.

SCIM provisioning uses the `/scim/v2` base URL. In deployment configuration, save the **SCIM provisioning token** and **Expiration (UTC)** together. Provision the same `externalId` value as the OIDC identity claim or SAML identity attribute. Provision users, not groups. SCIM creates passwordless least-privilege Viewers and can update, disable, and restore them. A SCIM-managed Profile cannot use a local password.

## Configure webhooks and DLNA

Set the webhook URL and token in deployment configuration. Kinosail sends configured event notifications to that endpoint. Keep the token in a secret file where possible.

DLNA exposes direct media URLs to devices on the local network. Set `KINOSAIL_DLNA_URL`, then select **Enable DLNA** in **Settings → DLNA**. Disabling it uses **Disable and revoke** and revokes discovery capability. DLNA is local-network access; it is not a remote relay.

## Configure MCP and API clients

Open **AI agent connections** from Settings for optional MCP connections. Follow the [MCP client guide]({{ '/developer-guide/mcp/' | relative_url }}) for host and HTTPS connection choices. The authenticated `/api/v1` surface and MCP operations reuse the same Owner and Viewer policy checks as the web app. Create a narrowly scoped API key in **Settings → API keys** for an HTTP API client and copy it once. Revoke it when the client no longer needs it.

## Protect integration secrets

The Settings page hides secrets. Environment- and YAML-managed values are **Read-only** until you remove the external value. Encrypted recovery backups contain secret state; a portable unencrypted archive does not. Never publish tokens, callback secrets, private hostnames, or provider responses.

## Source of truth

Sources: `README.md`, `kinosail.example.yaml`, `compose.release.yaml`, `internal/server/settings_http.go`, `internal/server/settings_configuration.go`, `internal/server/settings_dlna.go`, and `docs/research/documentation-information-architecture.md`.
