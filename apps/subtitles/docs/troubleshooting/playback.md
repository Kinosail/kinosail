---
title: Fix subtitle timing and save failures
description: Check the candidate, filesystem permissions, and preserved original.
section: Fix a problem
last_reviewed: 2026-09-15
---

# Fix subtitle timing and save failures

## A subtitle does not fit

Check language, episode, accessibility role, and release cut. Test both early and later dialogue in a player. Do not assume every subtitle for the same title has the same timing. Keep any `.kinosail.bak` original until satisfied with the replacement.

## A fetch finds a result but cannot save

Check write permissions for UID/GID 10001, host/NAS ACLs, container file sharing, free disk, and whether the selected media mount is read-only. Inspect the operation result for validation failures or protection of an existing sidecar.

## A download is rejected

Invalid archives, untrusted destinations, oversized content, malformed timing, or invalid text can be rejected intentionally. Do not disable validation; try an appropriate different candidate/provider and report a redacted reproducible example if valid content is rejected.

SubSource files stay unchanged under its terms. See [provider setup]({{ '/owner-guide/integrations/' | relative_url }}) and [recovery]({{ '/owner-guide/backups-and-updates/' | relative_url }}).
