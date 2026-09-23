---
title: Connect an MCP client
description: Connect an assistant to Kinosail through the Model Context Protocol.
section: Build with Kinosail
last_reviewed: 2026-09-20
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

Host STDIO keeps the MCP transport on the Server host. For a prebuilt Docker installation, use the Compose service name so the command does not depend on a generated container name. Replace `/absolute/path/to/player` with your installation directory. Include the same Compose overlays used at install time. With Codex, run:

```sh
codex mcp add kinosail -- docker compose \
  --file /absolute/path/to/player/compose.release.yaml \
  exec -T kinosail kinosail mcp-stdio
```

If the client runs on another device with an SSH host alias, use:

```sh
codex mcp add kinosail -- ssh -T SERVER docker compose \
  --file /absolute/path/to/player/compose.release.yaml \
  exec -T kinosail kinosail mcp-stdio
```

Replace `SERVER` with the SSH alias. Use the equivalent Podman command when that is how the Server runs. For source Compose, point `--file` at `apps/player/compose.yaml`. The service must already be running. If the Server has more than one Owner, append the Owner Profile ID shown in **AI agent connections** after `mcp-stdio`, for example `mcp-stdio OWNER_PROFILE_ID`. Do not substitute a display name. Keep STDIO free of shell banners or extra command output.

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

The client needs access to the Server’s management boundary over the local network or WireGuard. The restricted public media gateway does not expose MCP. Use the exact resource URL shown by the Server. A locally generated certificate may require the client to trust the Kinosail local certificate. A publicly trusted HTTPS certificate avoids that step.

When deployment-managed OAuth is configured, follow the identity provider registration shown by the Server instead of assuming built-in authorization.

The Owner can revoke an HTTPS connection from **AI agent connections**. Access tokens expire, and refresh access is stored by the Server for the approved connection.

## Understand MCP grants

MCP grants are separate from HTTP API-key scopes:

- `kinosail.read` allows library and viewing-context tools;
- `kinosail.write` adds personal state, playlists, Watch Rooms, and language changes; and
- `kinosail.manage` adds collection management and approved Owner operations when the selected Profile is an Owner.

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

## Try a read-only task first

After adding the connection, run `codex mcp list`, open a new client session, and inspect its MCP tool list. Ask: “Find five unwatched science-fiction films in my Kinosail library. Do not change anything.” The client should use `search_media` or `recommendation_context` and return only media visible to the selected Profile.

Example `search_media` arguments:

```json
{"query":"science fiction","view":"unwatched","sort":"year","limit":5}
```

`view` accepts `all`, `list`, `unwatched`, `history`, `movies`, `shows`, `music`, `audiobooks`, `books`, or `photos`. `sort` accepts `title`, `added`, or `year`. A query is limited to 512 bytes; `limit` is 1–200, with a default of 50. `recommendation_context` combines history and candidates and defaults to unwatched candidates.

To inspect the authenticated identity, call `read_api` with:

```json
{"path":"/api/v1/me"}
```

Once write access is approved, ask the assistant to show a proposed playlist before creating it. `create_playlist` takes `name` (1–64 characters) and an ordered `ids` array containing real IDs returned by search. Collection changes require `manage_api`, an Owner Profile, and management access. They are not part of the Viewer write grant.

For a management connection, a read-only diagnostic request is:

```json
{"method":"GET","path":"/api/v1/diagnostics"}
```

Send this through `manage_api`, without a body. For mutation bodies, consult the running Server’s [OpenAPI contract]({{ '/reference/api/' | relative_url }}) and request only an approved route. A documented HTTP API operation is not automatically exposed through MCP.

## Connect another MCP client

Choose **STDIO** with executable `docker` and arguments equivalent to the Compose command above, or **Streamable HTTP** with the Server's `/mcp` URL and OAuth. Client configuration formats vary. Use an argument array when supported rather than putting the whole command into an executable field.

The client must support the protocol version below. For Codex configuration details, see the [official MCP documentation](https://developers.openai.com/codex/mcp).

## Revoke access

In **Settings → AI agent connections**, review each HTTPS connection's client, Profile, grants, expiry, and last use. Select **Revoke** for an unwanted connection. Remove the client-side MCP entry too; removing it alone does not revoke the Server-side grant.

Host STDIO does not use a revocable OAuth connection. Remove the configured command and withdraw the client's Docker/Podman or SSH access when retiring it. That host access is powerful independently of MCP.

## Troubleshoot a connection

| Symptom | Check |
| --- | --- |
| STDIO exits immediately | Confirm the Compose file path, running service, container access, and Owner ID if multiple Owners exist. Avoid interactive TTY allocation. |
| HTTPS certificate error | Trust the Server's local certificate through the client/OS trust mechanism or use trusted HTTPS. Do not disable certificate verification. |
| Endpoint unreachable or denied remotely | Connect through the local network or approved WireGuard management access; the public media gateway is not an MCP endpoint. |
| Sign-in expired or connection revoked | Start OAuth login again and have the Owner approve the intended Profile and grants. |
| Write or management tool absent | Inspect the approved grants. Read-only connections intentionally expose fewer tools. Management also needs an Owner Profile. |
| API operation rejected | Use a relative `/api/v1` path in the proper tool; the MCP route allowlist is narrower than the HTTP API. |
| No results | Check the selected Profile, library visibility, query, and completed media scan. |
| Protocol negotiation fails | Check the client's supported MCP versions against the version below. |

## Know what MCP cannot do

MCP does not expose media bytes, credentials, sessions, passkey operations, interactive identity operations, or unrestricted administration. It also does not expose the HTTP API's download, API-key, device, or session-management routes through the MCP tools.

MCP may return a media path or playback plan through an approved read operation, but an assistant should keep media requests direct to the household Server. It should not proxy media or place tokens in prompts or logs.

## Protocol and security

The HTTPS endpoint is `POST /mcp` and uses stateless Streamable HTTP. Kinosail currently accepts MCP protocol version `2026-07-28`. `GET /mcp` and `DELETE /mcp` are not supported.

Bearer tokens travel in the `Authorization` header. Do not put them in query strings. Kinosail validates the OAuth resource audience, Profile grant, token expiry, and request origin.

For deployment-managed OAuth settings, see the [configuration reference]({{ '/reference/configuration/' | relative_url }}) and the [Owner integration guide]({{ '/owner-guide/integrations/' | relative_url }}). For exact MCP behavior, use the connection details shown by the Server and the live API reference.
