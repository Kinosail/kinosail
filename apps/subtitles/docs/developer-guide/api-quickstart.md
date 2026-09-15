---
title: API quickstart
description: Read the live API contract before sending subtitle mutations.
section: Build with Subtitles
last_reviewed: 2026-09-15
---

# API quickstart

Start a local Server, finish Owner setup, and configure a library. Use trusted HTTPS and the authentication mechanism documented by the live OpenAPI contract. Do not put an API key in a URL or share browser cookies in bug reports.

1. Read `/api/v1/openapi.json` from the intended Server.
2. Obtain an appropriately scoped credential through Owner controls, or use an authenticated session in your integration's secure credential store.
3. Read `GET /api/v1/subtitle-library` to obtain scanned item IDs.
4. Check the intended item and language before issuing a mutation.
5. Send `POST /api/v1/subtitle-library/{id}/fetch` with `Content-Type: application/json` and `{}` or an explicit language.
6. Inspect the response and resulting file before scheduling batch work.

API-key, Owner, and recent-authentication requirements vary by route. Do not assume a read credential can perform a management mutation. Follow the current contract and return errors to the operator.

See [route and payload reference]({{ '/reference/api/' | relative_url }}).

## Example read request

Use an Owner-bound API key with the `library` scope for inventory. Load its value into `KINOSAIL_TOKEN` from a private secret store; do not paste it into a committed script or shell history. `KINOSAIL_URL` is the trusted HTTPS origin of your own Server.

```sh
curl --fail-with-body \
  -H "Authorization: Bearer $KINOSAIL_TOKEN" \
  "$KINOSAIL_URL/api/v1/subtitle-library"
```

Subtitle mutations need the `admin` API-key scope as well as Owner authority. A browser session is subject to recent-authentication and same-origin rules. A `403` is not a reason to disable those controls; check the credential and intended operation. The API-key scope names differ from MCP grants.
