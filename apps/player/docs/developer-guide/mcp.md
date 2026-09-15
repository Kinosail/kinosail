---
title: Connect an MCP client
description: Connect an assistant to Kinosail through the Model Context Protocol.
section: Build with Kinosail
last_reviewed: 2026-08-29
---

# Connect an MCP client

Kinosail provides an optional Model Context Protocol (MCP) connection for assistants that need to search a household library, prepare recommendations, or perform approved API operations.

MCP is an additional interface. It does not replace the [versioned HTTP API]({{ '/reference/api/' | relative_url }}), and it does not move media through Kinosail or another hosted service.

## Choose a connection

Use the connection that matches where the MCP client runs:

| Client location | Connection | Access model |
| --- | --- | --- |
| The Kinosail host | Host STDIO | Recommended. Uses local Docker or Podman access. |
| A different device | HTTPS / OAuth | Uses a browser-approved, revocable MCP grant. |

Open **Settings → AI agent connections** on the Kinosail Server. The page shows the exact commands and resource URL for that installation.

## Use host STDIO

Host STDIO keeps the MCP transport on the Server host. With Codex, run this on the host:

```sh
codex mcp add kinosail -- docker exec -i kinosail kinosail mcp-stdio
```

If the client runs on another device with an SSH host alias, use:

```sh
codex mcp add kinosail -- ssh -T SERVER docker exec -i kinosail kinosail mcp-stdio
```

Replace `SERVER` with the SSH alias. Use the equivalent Podman command when that is how the Server runs. If the Server has more than one Owner, select the Owner Profile ID shown in **AI agent connections**.

Host STDIO has full Owner access. It is recorded as **Host MCP** in activity. Remove the MCP client entry or revoke the underlying Docker, Podman, or SSH access when the client no longer needs it.

## Use HTTPS and OAuth

Use HTTPS / OAuth when the MCP client cannot access the Kinosail host.

1. Open **Settings → AI agent connections**.
2. Copy the HTTPS connection command or resource URL.
3. Add the resource URL to the MCP client.
4. Complete the browser approval flow.
5. Select the Profile and grants that the client should use.

With Codex, the commands are:

```sh
codex mcp add kinosail-http --url https://kinosail.example.test/mcp
codex mcp login kinosail-http
```

Use the exact resource URL shown by the Server. A locally generated certificate may require the client to trust the Kinosail local certificate. A publicly trusted HTTPS certificate avoids that step.

The Owner can revoke an HTTPS connection from **AI agent connections**. Access tokens expire, and refresh access is stored by the Server for the approved connection.

## Understand MCP grants

MCP grants are separate from HTTP API-key scopes:

- `kinosail.read` allows library and viewing-context tools;
- `kinosail.write` adds personal state, playlists, collections, and language changes; and
- `kinosail.manage` adds approved Owner operations when the selected Profile is an Owner.

Read access is the default. Write access is optional. Management access requires both an Owner Profile and the management grant.

The Owner chooses the Profile and grants during HTTPS approval. Host STDIO uses the selected Owner Profile and does not use an API key.

## Use the available tools

The MCP server provides these tools according to the grant:

- `search_media` finds visible media by query, section, sort, and viewing state;
- `recommendation_context` returns personal history and matching candidates;
- `read_api` reads an approved Viewer API path;
- `write_api` performs approved Viewer mutations;
- `create_playlist` creates a Profile-owned playlist; and
- `manage_api` performs approved Owner operations.

The MCP adapter accepts relative `/api/v1` paths only. It rejects absolute URLs and operations outside the approved route set. Responses are JSON and are bounded before they return to the client.

## Know what MCP cannot do

MCP does not expose media bytes, credentials, sessions, passkey operations, interactive identity operations, or unrestricted administration. It also does not expose the HTTP API's download, API-key, device, or session-management routes through the MCP tools.

MCP may return a media path or playback plan through an approved read operation, but an assistant should keep media requests direct to the household Server. It should not proxy media or place tokens in prompts or logs.

## Protocol and security

The HTTPS endpoint is `POST /mcp` and uses stateless Streamable HTTP. Kinosail currently accepts MCP protocol version `2026-07-28`. `GET /mcp` and `DELETE /mcp` are not supported.

Bearer tokens travel in the `Authorization` header. Do not put them in query strings. Kinosail validates the OAuth resource audience, Profile grant, token expiry, and request origin.

For deployment-managed OAuth settings, see the [configuration reference]({{ '/reference/configuration/' | relative_url }}) and the [Owner integration guide]({{ '/owner-guide/integrations/' | relative_url }}). For exact MCP behavior, use the connection details shown by the Server and the live API reference.
