---
title: API workflows
description: Choose safe request sequences for common Kinosail integrations.
section: Build with Kinosail
last_reviewed: 2026-08-29
---

# API workflows

Use these sequences as integration shapes. Read the live OpenAPI document for the exact request and response fields on your Server version.

## Browse and search

1. Call `GET /api/v1` to discover the API.
2. Call `GET /api/v1/library` with `limit`, `offset`, `sort`, and an optional `q` or `view`.
3. Follow an item identifier with `GET /api/v1/items/{id}`.
4. Use the returned item type to choose the next action.

Keep pagination bounded. Do not assume that every Profile sees the same items or fields.

## Start playback

1. Call `GET /api/v1/items/{id}/playback` with the client video codecs when known.
2. Read the returned playback plan and policy decision.
3. Use the media path returned by the Server. Keep media requests on the household Server.
4. Send playback events only when the integration owns a playback session.

Kinosail prefers the original media representation when the source and client support it. A compatible representation is available only when policy and Server capacity allow it. Do not assume that every item has an HLS URL or that every Viewer may transcode.

## Track personal state

Use the same token for the Profile whose state the integration represents:

- `PUT /api/v1/items/{id}/progress` saves viewing progress;
- `DELETE /api/v1/items/{id}/continue-watching` dismisses a shelf item;
- `PUT /api/v1/items/{id}/list` changes My List; and
- `GET /api/v1/history` reads that Profile's history.

Do not use an Owner token for shared automation unless the automation truly acts as the Owner.

## Work with playlists and collections

1. Call `GET /api/v1/playlists` or `GET /api/v1/collections`.
2. Create or select a named list.
3. Add or remove items with the named resource route.
4. Read the resource again after a mutation if the integration needs the canonical result.

Use the resource name and item identifier returned by the Server. Do not build names from filesystem paths.

## Prepare an offline download

1. Confirm that the authenticated Profile permits downloads.
2. Call `POST /api/v1/items/{id}/downloads`.
3. Poll `GET /api/v1/downloads/{id}` until the Server reports a terminal state.
4. Retrieve the file with `GET /api/v1/downloads/{id}/file`.
5. Delete the download with `DELETE /api/v1/downloads/{id}` when it is no longer needed.

Treat downloads as Profile-bound. Do not copy a download URL between Profiles or expose it in logs.

## Build an Owner integration

Use an Owner-scoped key only for tasks such as configuration, Profile management, library management, diagnostics, or backups. Keep Owner integrations separate from Viewer automations.

If a task changes Server configuration, prefer the Owner guide and configuration reference. The API still enforces managed settings, required fields, cross-field rules, and restart behavior.

## Protect media and credentials

Kinosail sends Library Content directly between the device and the household Server. A third-party integration should proxy neither media nor playback tokens.

Use HTTPS for remote access. Store tokens in a secret manager. Redact authorization headers, cookies, playback URLs, Media Share tokens, and private Server addresses from logs and support reports.

For exact routes, schemas, and security requirements, use the [API reference]({{ '/reference/api/' | relative_url }}).
