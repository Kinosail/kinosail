---
title: Connect an MCP client
description: Connect trusted assistants to bounded subtitle operations.
section: Build with Subtitles
last_reviewed: 2026-09-15
---

# Connect an MCP client

Subtitles supports the Model Context Protocol (MCP) through host STDIO and an HTTPS/OAuth connection. Open **Settings → AI agent connections** for the exact commands, Owner selection, and resource URL for your installation.

## Choose the access model

| Connection | Use and authority |
| --- | --- |
| Host STDIO | Runs through Docker/Podman or a private SSH connection on the Server host. Uses the selected Owner and has broad Owner authority. |
| HTTPS/OAuth | A compatible client connects to the displayed `/mcp` resource and completes browser approval for a Profile and grants. The connection can be revoked from Settings. |

For source Compose, an MCP client can launch this command with the app directory as its working directory:

```sh
docker compose exec -T kinosail kinosail mcp-stdio
```

Use Podman when appropriate. Follow the Server's displayed command when multiple Owners require explicit selection. Removing a host STDIO entry does not revoke the underlying Docker/SSH authority; manage that access separately. Host operations are recorded as **Host MCP**.

For HTTPS, copy the Server's resource URL, configure it in a client that supports Streamable HTTP and OAuth, and complete the approval flow. Use trusted HTTPS on a private administration connection. Bearer tokens belong in the Authorization header, never in URLs or prompts.

## Subtitle tools and grants

- `read_api` with `kinosail.read` can read the allowlisted `/api/v1/subtitle-library` inventory. The subtitle endpoint also requires an Owner Profile.
- `manage_api` with `kinosail.manage` and an Owner Profile can invoke the allowlisted fetch, wanted-batch, maintenance, provider-test, inspect, draft, preview, apply, replacement, and restore operations.
- `kinosail.write` covers personal library state; it does not substitute for the management grant needed to change subtitle files.

Only approved relative `/api/v1` paths are accepted. Discover the Server's tool schemas before calling them. Responses are bounded JSON. Subtitle export bytes, credentials, sessions, API-key management, and unrestricted filesystem paths are blocked from MCP.

Begin with inventory and a single explicitly requested operation. Confirm the item, language, and intended replacement before approving a mutation. Provider quotas, input validation, Owner policy, and sidecar protection apply just as they do to HTTP and web requests.

See the [HTTP API reference]({{ '/reference/api/' | relative_url }}) for routes and payloads. Source of truth: `internal/server/mcp_route_policy.go` and the connection details returned by your installed Server.
