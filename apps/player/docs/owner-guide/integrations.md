---
title: Connect integrations
description: Configure metadata, identity, webhooks, DLNA, and MCP.
section: Own the Server
---

# Connect integrations

Enable only the integrations you need. Open **Settings → Access** and use **Settings → Deployment configuration** for values that need a restart or an external secret.

## Configure metadata

TMDB enrichment is optional. Set its access token and addresses in deployment configuration, then select **Refresh missing metadata** from **Settings → Library discovery**. Kinosail uses local NFO files, embedded tags, and folder artwork without TMDB. TMDB attribution and licensing terms still apply.

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
