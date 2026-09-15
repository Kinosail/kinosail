---
title: Subtitle API workflows
description: Use one-item fetches and bounded maintenance with visible results.
section: Build with Subtitles
last_reviewed: 2026-09-15
---

# Subtitle API workflows

## Fetch a known item

Read inventory, select the returned item ID, then POST a language or an empty object to its fetch route. The Server derives the sidecar target from the scanned video; clients cannot choose an arbitrary write path.

## Fill wanted items

POST `{"language":"en","limit":10}` to `/api/v1/subtitle-library/fetch-wanted`. Read per-operation outcomes and provider errors. Respect provider quotas; do not create a tight retry loop.

## Maintain coverage

POST a bounded request to `/api/v1/subtitle-library/maintain` to use the same shared operation as automatic maintenance. It can add missing sidecars and safely upgrade eligible files. Preserve `.kinosail.bak` originals until replacements are checked.

## Handle failure

Treat invalid input as a correction task. Treat permission errors as an access/configuration issue. Back off on temporary provider or quota failures. Do not retry file writes blindly after an uncertain network result: refresh inventory first.

See [API reference]({{ '/reference/api/' | relative_url }}) and [provider setup]({{ '/owner-guide/integrations/' | relative_url }}).
