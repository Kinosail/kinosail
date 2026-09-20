---
title: API reference
description: Authenticate to the versioned Kinosail HTTP API and use the live OpenAPI document.
section: Build with Kinosail
last_reviewed: 2026-08-29
---

# API reference

The Kinosail API is a versioned JSON API. The running Server publishes its current OpenAPI document at `/api/v1/openapi.json`.

## Discover the contract

Use these endpoints on the Server that you operate:

```text
GET /api/v1
GET /api/v1/openapi.json
```

`/api/v1` reports the API name, version, and OpenAPI URL. `/api/v1/openapi.json` returns an OpenAPI 3.1 document with the title `Kinosail Server API`, document version `1.0.0`, and the versioned `/api/v1` paths. Treat the live document as the contract for request and response details. The checked-in source copy is [`internal/server/api_openapi.json`](https://github.com/Kinosail/kinosail/blob/main/apps/player/internal/server/api_openapi.json).

The Server checks that every registered versioned route appears in the OpenAPI document. This keeps the document and the implementation aligned.

## Authenticate

Send an API key or session token as a Bearer token:

```sh
curl --fail \
  -H "Authorization: Bearer $KINOSAIL_API_TOKEN" \
  https://kinosail.example.test/api/v1/library
```

Create an Owner or Viewer session with `POST /api/v1/session`. A local password login can require a second-factor `code`. Public password login is disabled. Public HTTPS users must use a passkey or Owner-approved Quick Connect request.

Owners create scoped API keys in the Owner settings. A key is shown once and Kinosail stores only its hash. Normal API keys expire after 30 days. The supported scopes are:

| Scope | Grants |
| --- | --- |
| `library` | Read the library, metadata views, history, and the OpenAPI document |
| `write` | Change viewing progress, lists, playlists, and Watch Rooms |
| `stream` | Stream media, subtitles, reader assets, and Watch Room events |
| `download` | Prepare, list, retrieve, and delete offline downloads |
| `admin` | Owner administration and maintenance routes |

API keys cannot use session-only routes, such as passkey enrollment, MFA changes, or sign-out. Profile policy still applies to every request.

## Resource groups

The live document includes these resource groups:

- setup, sessions, passkeys, MFA, OIDC linking, and Quick Connect;
- the library, Shows, albums, items, playback plans, markers, and subtitles;
- progress, history, My List, playlists, smart playlists, collections, and Watch Rooms;
- books and reader progress;
- offline downloads and media shares;
- metadata edits, metadata refresh, and viewing activity import or sync;
- Owner settings, configuration, libraries, Profiles, devices, sessions, API keys, and tasks;
- remote access, activity, diagnostics, metrics, maintenance, and encrypted backups; and
- optional supporter and agent-connection operations.

The direct media adapters use paths such as `/media/{id}`, `/hls/{id}/{file}`, `/subtitle/{id}`, `/read/{id}/file`, and `/download/{id}`. They use the same Viewer policy and API-key scope checks as versioned operations. Library Content remains direct-only: Kinosail does not send it through a hosted media proxy.

## Request rules

JSON requests must contain one object. Unknown fields are rejected. Request bodies are limited to 1 MiB before decoding. Each operation also validates its own required fields, ranges, and permissions. Do not use a successful HTTP status as proof that an operation is safe for a different Profile; the Server evaluates the authenticated Profile on every request.

## Errors and compatibility

Errors use a JSON object with an `error` string. Common statuses are `400` for invalid input, `401` for missing or invalid credentials, `403` for a policy denial, `404` for an unknown resource, `409` for a state or configuration conflict, `429` for throttled login attempts, and `500` or `503` for Server failure or unavailable work.

The bundled web interface and optional Jellyfin-compatible surface call the same application operations as the API. Jellyfin protocol coverage is tested for supported flows; it is not a certification of every physical client.

## Keep clients stable

Call only `/api/v1` routes and inspect the live OpenAPI document at startup. Do not depend on HTML markup, private JSON fields, filesystem paths, or unversioned implementation routes. Store tokens outside source control and rotate or revoke them when a device or automation is no longer trusted.
