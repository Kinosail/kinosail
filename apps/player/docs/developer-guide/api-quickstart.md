---
title: API quickstart
description: Make your first authenticated Kinosail API request.
section: Build with Kinosail
last_reviewed: 2026-08-29
---

# API quickstart

Use this guide to connect a script or small client to a Kinosail Server.

## Before you begin

You need:

- the HTTPS or local address of the Kinosail Server;
- an Owner who can create an API key; and
- `curl` and `jq` for the examples below.

In **Settings → API keys**, create a key for this integration. Start with **Browse library**. Add **Manage personal library state** only when the client must change progress, My List, playlists, or Watch Rooms. Copy the key when Kinosail shows it. It is not shown again.

## Set local variables

Keep the key in your shell environment or a secret manager. Do not commit it to a script.

```sh
KINOSAIL_URL='https://kinosail.example.test'
KINOSAIL_TOKEN='paste-the-key-only-in-your-local-shell'
```

## Discover the Server

The root response identifies the API version and the OpenAPI URL:

```sh
curl --fail-with-body \
  -H "Authorization: Bearer $KINOSAIL_TOKEN" \
  "$KINOSAIL_URL/api/v1"
```

Inspect the live contract when the client starts:

```sh
curl --fail-with-body \
  -H "Authorization: Bearer $KINOSAIL_TOKEN" \
  "$KINOSAIL_URL/api/v1/openapi.json" | jq .
```

## List library items

Use a bounded page while testing. The response includes item identifiers for later requests.

```sh
curl --fail-with-body \
  -H "Authorization: Bearer $KINOSAIL_TOKEN" \
  "$KINOSAIL_URL/api/v1/library?limit=20&sort=title" | jq .
```

Save an item identifier from the response, then request its details:

```sh
KINOSAIL_ITEM_ID='item-id-from-the-library-response'

curl --fail-with-body \
  -H "Authorization: Bearer $KINOSAIL_TOKEN" \
  "$KINOSAIL_URL/api/v1/items/$KINOSAIL_ITEM_ID" | jq .
```

## Save viewing progress

This request needs the **Manage personal library state** scope. Progress belongs to the authenticated Profile.

```sh
curl --fail-with-body \
  -X PUT \
  -H "Authorization: Bearer $KINOSAIL_TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{"seconds":600}' \
  "$KINOSAIL_URL/api/v1/items/$KINOSAIL_ITEM_ID/progress"
```

The Server validates the item, Profile policy, request shape, and progress range. Handle non-2xx responses as normal integration states.

## Handle errors

Errors use a JSON object with an `error` string. Common responses include:

- `400` for invalid input;
- `401` for a missing, expired, or invalid token;
- `403` when the Profile or key lacks permission;
- `404` when the item or route is unknown; and
- `429` when login or another protected operation is throttled.

Never retry a `401` with the same token. Never log the token, cookies, playback URLs, or full request URLs.

Continue with [API workflows]({{ '/developer-guide/api-workflows/' | relative_url }}) or inspect the [API reference]({{ '/reference/api/' | relative_url }}).
