---
title: Developer guide
description: Build clients, automations, and integrations for a Kinosail Server.
section: Build with Kinosail
last_reviewed: 2026-08-29
---

# Developer guide

Build against the Kinosail Server that a household operates. The versioned HTTP API supports library access, personal state, playback planning, downloads, collections, and approved administration.

## Choose a starting point

- [API quickstart](api-quickstart) makes an authenticated request and updates viewing progress.
- [API workflows](api-workflows) maps common integration tasks to safe request sequences.
- [Connect an MCP client](mcp) connects an assistant through the Model Context Protocol.
- [API reference](../reference/api) explains authentication, scopes, media boundaries, and the live contract.
- [Home Assistant integration](../owner-guide/integrations#configure-home-assistant) covers the supported household integration.

## Use the right documentation layer

The API document and these guides have different jobs:

| You need to... | Use... |
| --- | --- |
| Generate requests or inspect every operation | The live [OpenAPI document](../reference/api#discover-the-contract) |
| Learn a complete integration task | A developer guide |
| Check authentication, scopes, limits, or privacy rules | The [API reference](../reference/api) |
| Configure an Owner-controlled integration | The [Owner guide](../owner-guide/integrations) |

## Know the boundary

Kinosail is self-hosted. An integration asks the household for its Server address and uses that address for API and media requests. Kinosail does not provide a hosted API endpoint or media relay.

Use only `/api/v1` routes. Discover the Server with `GET /api/v1`, then inspect `GET /api/v1/openapi.json` at startup. Do not depend on HTML markup, private JSON fields, filesystem paths, or unversioned routes.

## Choose an interface

- Use the versioned HTTP API for clients, scripts, dashboards, and household automations.
- Use the tested Jellyfin-compatible surface when an existing client requires that protocol.
- Use MCP for assistant-oriented connections. MCP and HTTP still apply the same Profile and capability policy.

## Build safely

Create a separate API key for each integration. Select the smallest scope that works, keep the key outside source control, and revoke it when the integration is removed.

Treat the authenticated Profile as part of every request. A valid token does not grant access to libraries, media, administration, transcoding, or downloads that the Profile does not permit.

Source of truth: the running Server's OpenAPI document and the implementation tested in the Kinosail repository.
