---
title: Fix installation and startup
description: Inspect source-container startup without deleting state.
section: Fix a problem
last_reviewed: 2026-09-15
---

# Fix installation and startup

Run these commands from `apps/subtitles/` using the same Compose project and files as the installation:

```sh
docker compose ps
docker compose logs --tail 100 kinosail
docker compose exec -T kinosail kinosail healthcheck
```

Check that the engine/VM is running, port 38128 is available, the host media directory exists, and container file sharing allows access. The default source setup binds its web port to localhost.

If startup names `/run/secrets/kinosail_backup_key` as missing, the release secret-file setting was copied without a corresponding mount. Clear that setting for the source quickstart, or mount and configure the actual key as described in [backups]({{ '/owner-guide/backups-and-updates/' | relative_url }}).

If setup reappears unexpectedly after an update, check the Compose project and `/config` volume before creating another Owner. Do not remove volumes while diagnosing.
