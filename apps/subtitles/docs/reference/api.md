---
title: Subtitle HTTP API
description: Use bounded subtitle inventory and mutation operations.
section: Reference
last_reviewed: 2026-09-15
---

# Subtitle HTTP API

The web dashboard and HTTP API call the same validated application operations. Use `/api/v1/openapi.json` on your running Server for its complete contract. All subtitle routes require Owner authority. For API keys, inventory uses the `library` scope; management operations use `admin`. Browser session, same-origin, and recent-authentication rules still apply.

| Method and route | Purpose |
| --- | --- |
| `GET /api/v1/subtitle-library` | Read subtitle inventory and coverage. |
| `POST /api/v1/subtitle-library/{id}/fetch` | Fetch for a scanned item. |
| `POST /api/v1/subtitle-library/fetch-wanted` | Fetch a bounded wanted batch. |
| `POST /api/v1/subtitle-library/maintain` | Fill missing subtitles and consider safe upgrades. |
| `POST /api/v1/subtitle-library/{id}/restore` | Restore an available preserved original for a language. |
| `POST /api/v1/subtitle-library/{id}/replacement` | Permit or freeze replacement for an item. |
| `POST /api/v1/subtitle-providers/test` | Check configured provider credentials and report health. |
| `GET /api/v1/subtitle-library/{id}/inspect` | Inspect installed subtitle content and matching evidence. |
| `GET /api/v1/subtitle-library/{id}/export` | Export a subtitle in a supported format. |
| `GET` / `POST /api/v1/subtitle-library/{id}/draft` | Inspect or create a reviewable local draft. |
| `POST /api/v1/subtitle-library/{id}/audio` | Run the supported local audio-draft operation. |
| `POST /api/v1/subtitle-library/{id}/preview` | Preview a subtitle edit. |
| `POST /api/v1/subtitle-library/{id}/apply` | Apply a validated reviewed edit. |

Use an item ID returned by inventory; do not submit a filesystem path. For a single fetch, send `{}` to use the configured language, or `{"language":"es"}` for an explicit language. Batch operations accept, for example:

```json
{"language":"en","limit":10}
```

Limits must be integers from 1 through 50. Unknown fields, malformed JSON, invalid languages, and invalid limits are rejected before provider requests or file writes. Read operation results; a successful batch request does not imply that every wanted item received a file.

See [API quickstart]({{ '/developer-guide/api-quickstart/' | relative_url }}) and [API workflows]({{ '/developer-guide/api-workflows/' | relative_url }}).

## Recovery and replacement payloads

Restore accepts `{}` for the configured language or `{"language":"en"}`. Replacement requires a boolean, for example `{"replaceable":false}` to freeze replacement. Provider testing accepts `{}`. Read current inspect/preview/draft schemas from OpenAPI before editing; do not invent stale fingerprints or approval data.

Inspect accepts the `language` query field. Export additionally accepts `format=srt`, `vtt`, or `original`. Duplicate/unknown query fields and invalid values are rejected. Export returns bytes directly to the authenticated client and is not exposed through MCP.
