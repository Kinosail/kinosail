---
title: Back up and update
description: Protect Server state, verify backups, restore safely, and update the one-container installation.
section: Own the Server
---

# Back up and update

Protect the Kinosail state before an update or host change. Backups contain portable Server state. They do not contain Library Content or reproducible transcode cache data.

## Configure encrypted automatic backups

The release installer creates `secrets/backup_key` with mode `600`. Configure a private backup directory on another disk or NAS with `KINOSAIL_BACKUP_PATH`. Keep an independent copy of both the backup archives and the key.

In **Settings → Automatic maintenance**, open **Backup and recovery**. Confirm **Encryption is on**, the destination, schedule, retention, latest archive, and last verification time. The release defaults are daily backups and seven retained files.

Select **Back up now** for an immediate encrypted archive. Select **Verify latest backup** after the archive appears. A failed backup stays visible as a last error; automatic creation fails closed without a key.

{% include screenshot.html title="Backup and recovery" alt="Future screenshot of the Backup and recovery page showing encryption, destination, schedule, latest archive, and verification actions." description="Show placeholder paths only. Never capture the backup key or real host path." %}

## Understand what a backup contains

Portable state includes configuration managed by the UI, credential hashes, sessions, playback state, and retained activity. Encrypted recovery backups can include hidden secrets. Backups do not include:

- Library Content;
- YAML files or environment files;
- secret files outside the backup state; or
- transcode cache data.

Back up media and deployment files separately. Protect unencrypted archives as sensitive account data.

## Create a portable archive from Compose

From the installation root, run:

```sh
podman compose --file compose.release.yaml run --rm --no-deps kinosail backup > kinosail-backup.tar.gz
```

Use `docker compose` when Docker runs the service. Keep the output in a private location. Do not upload it to a public issue or place it in a documentation repository.

## Restore after a failure

Stop the service, restore the archive, then start it:

```sh
podman compose --file compose.release.yaml stop kinosail
podman compose --file compose.release.yaml run --rm --no-deps --no-tty kinosail restore < kinosail-backup.tar.gz
podman compose --file compose.release.yaml up --detach
```

Restore validates the manifest and every entry before writing. It rejects unknown paths, duplicate or malformed state, and oversized entries. Restore into the correct Kinosail data location. Keep the backup key available for encrypted recovery material.

After restart, run `kinosail healthcheck`, sign in, check Profiles, and verify one Library item. Reconnect external deployment values because YAML and environment files are outside the archive.

## Update a release

Run the release installer again with the same media path and port. When the service is running, the installer creates `backups/kinosail-before-update-<timestamp>.tar.gz`, pulls the signed image, pins its digest, recreates the service, and waits for health.

Check the service:

```sh
podman compose --file compose.release.yaml ps
podman compose --file compose.release.yaml exec -T kinosail kinosail healthcheck
```

If the new service is unhealthy, preserve the recovery archive and logs. Do not remove volumes or run a destructive cleanup while diagnosing. Use the [install and startup troubleshooting guide]({{ '/troubleshooting/install-and-startup/' | relative_url }}) for focused checks.

## Source of truth

Sources: `README.md`, `compose.release.yaml`, `internal/server/settings_backups.go`, `internal/backup/backup.go`, `internal/backup/encrypted.go`, and `docs/research/documentation-information-architecture.md`.
