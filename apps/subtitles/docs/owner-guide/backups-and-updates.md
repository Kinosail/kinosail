---
title: Back up and update
description: Create and verify backups, restore application state, and update safely.
section: Own the Server
last_reviewed: 2026-09-15
---

# Back up and update

Back up before changing versions, storage, or configuration. Keep app-state archives, external deployment files, media, and backup keys separately. For Subtitles, sidecars and `.kinosail.bak` originals belong to the media backup, not the app-state archive.

## Configure encryption

Automatic backups require a key and fail closed without one. The signed-release installer creates `secrets/backup_key`; a source Compose installation does not mount that file automatically.

For source Compose, use a strong random backup key stored privately in your password manager. Put it in `KINOSAIL_BACKUP_KEY` in the app's uncommitted `.env` and leave `KINOSAIL_BACKUP_KEY_FILE=` empty. Protect `.env` with `chmod 600 .env`. Alternatively, configure a mounted secret file and set only its `_FILE` variable; the container must be able to read it. Do not set both forms.

Recreate the source service with `docker compose up --detach`. Confirm encryption, backup destination, interval, and retention in Owner Settings. Use **Back up now** and **Verify latest backup**, then keep an independent off-host copy of the archive and key. `KINOSAIL_BACKUP_PATH` can point to a private host directory mounted at `/backups`; check container write permissions.

Scheduled defaults are daily backups with seven retained files. Verify your effective settings; deployment overrides can change them.

## What the archive contains

Application backups contain portable stored state such as UI-managed settings, credential hashes, sessions, and retained activity. Encrypted recovery archives can include private stored secrets. They do not contain the external media tree, reproducible cache, deployment YAML, `.env`, or independently mounted secret files.

The CLI `backup` command uses encryption when a key is configured. Without one, it can produce an unencrypted portable archive that omits private secret state; that is not a complete encrypted recovery backup. Treat every archive as sensitive.

## Create and verify an archive

Run from `apps/subtitles/` for source Compose. Use the exact Compose files/project that own your installation. Release installations must add `--file compose.release.yaml` and their applicable overrides to every command below. Replace Docker with Podman if appropriate.

Choose a new private output filename so redirection cannot overwrite your only backup:

```sh
umask 077
docker compose run --rm --no-deps -T kinosail backup > before-update.kinosail-backup
docker compose run --rm --no-deps -T kinosail backup verify < before-update.kinosail-backup
```

Proceed only if both commands succeed. Retain the key, source/image revision, configuration, and archive together in your recovery records, with the key stored separately.

## Restore deliberately

Restore replaces stored application state. Confirm the target Compose project/volume, retain a copy of its current state, and verify the archive first. Use a known-compatible app revision and the correct backup key. Stop the running service before restoration:

```sh
docker compose stop kinosail
docker compose run --rm --no-deps -T kinosail restore < before-update.kinosail-backup
docker compose up --detach
docker compose exec -T kinosail kinosail healthcheck
```

If restore fails, keep the service stopped while investigating the error and preserve the original state/backup. Restore validates the archive before writing, including unknown paths, duplicates, malformed state, and size limits.

After restart, verify sign-in, configuration, and provider configuration, library scope, and a one-item subtitle operation. Reapply externally managed environment/YAML/secret files separately. Verify recovery on a disposable installation before depending on it for host-loss recovery.

## Update or roll back

For source installs, back up, fetch/review the desired revision, and run `docker compose up --build --detach` from the same app directory. Keep the same project name and volumes. Check health, sign-in, and the primary workflow after the update.

When signed releases are available, use the matching release installer. It verifies the image and, for a running installation, creates and verifies an encrypted `backups/kinosail-before-update-<timestamp>.kinosail-backup` before the update. Check the app release notes and installer output; do not bypass signature checks.

For rollback, retain the previous image/source and its matching state backup. Restore that pair deliberately; do not assume an older binary can read newly migrated state. Never use `down --volumes` as a troubleshooting step.

Source of truth: app `compose.yaml`, `compose.release.yaml`, `scripts/install.sh`, and shared `packages/backup/`.
