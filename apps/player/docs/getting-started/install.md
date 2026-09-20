---
title: Docker installation details
description: Install one Kinosail Server container and open it safely for first setup.
section: Start here
---

# Docker installation details

As of September 15, 2026, this monorepo has no published GitHub releases. Start with [Install with Docker]({{ '/quickstart/' | relative_url }}) for the complete release path, or use [source installation]({{ '/source-install/' | relative_url }}) for early evaluation. The steps below apply when a signed Player release is available.

Download the matching Player installer from [Releases](https://github.com/Kinosail/kinosail/releases), verify the supplied checksum and signature, extract it, and run commands from that bundle directory. Run the release installer from the Kinosail release bundle. It verifies the image, creates protected recovery material, starts one Server container, and binds it to localhost until you create the first Owner.

## Prerequisites

You need:

- a 64-bit Intel/AMD or Arm Linux host, or Docker/Podman on macOS for local use;
- Podman Compose or Docker Compose;
- `cosign` to verify the signed release image;
- `curl` when you later enable LAN mode;
- an absolute path to an existing folder that contains media you control; and
- a free TCP port, such as `38127`.

The release bundle must include `scripts/install.sh` and `compose.release.yaml`. Do not place credentials or private hostnames in documentation, shell history, or a public repository.

## Install on localhost

From the bundle root, run:

```sh
./scripts/install.sh /absolute/path/to/media 38127
```

The second argument is optional. The default port is `38127`. The path must be absolute, must exist, and must not contain a newline or a single quote.

The installer selects `podman compose` when available. It falls back to `docker compose`. It creates `secrets/backup_key` with mode `600` when the key does not exist. Keep this key with your encrypted backups.

The installer pulls `ghcr.io/kinosail/kinosail-player:<version>`, verifies its keyless signature, pins the resolved SHA-256 digest in `.env`, and starts the service. It waits for the `kinosail healthcheck` command to pass.


## Open the Server

Open:

```text
https://localhost:38127
```

Your browser may warn about the local certificate. Accept the warning only for this local Server. Create the first Owner from the setup page before you expose the Server to the LAN.

If the installer reports that the Server is not healthy, inspect the last lines of the service log:

```sh
docker compose --file compose.release.yaml logs --tail 50 kinosail
```

Use `podman compose` in place of `docker compose` if the installer selected Podman. Do not delete the `config`, `cache`, or backup locations while investigating.

## Expose the initialized Server on the LAN

Create the first Owner at the local address. Then run the same command with `--lan`:

```sh
./scripts/install.sh /absolute/path/to/media 38127 --lan
```

The installer checks that `/setup` redirects, changes `KINOSAIL_BIND` to `0.0.0.0`, and records a detected LAN address in `KINOSAIL_TLS_HOSTS` and `KINOSAIL_AUTH_URL` when those values are empty. It does not enable public internet access.

Open the reported LAN address from another device. Use the exact HTTPS name in the address when you add passkeys. If your host has more than one network interface, verify the detected address before sharing it.

## Update an existing release

Run the installer again with the same media path and port. If the service is running, the installer writes a recovery archive under `backups/kinosail-before-update-<timestamp>.kinosail-backup` before it pulls the new image. It keeps the existing `.env` values.

Check the resulting state:

```sh
docker compose --file compose.release.yaml ps
docker compose --file compose.release.yaml exec -T kinosail kinosail healthcheck
```

Read [Back up and update]({{ '/owner-guide/backups-and-updates/' | relative_url }}) before a planned host or storage change.

## Stop or restart the Server

```sh
docker compose --file compose.release.yaml logs --follow kinosail
docker compose --file compose.release.yaml stop
docker compose --file compose.release.yaml start
```

`stop` and `start` preserve the existing container configuration, including installer-selected overlays. Use the installer for updates. `down` removes containers but does not remove named volumes. Do not add `--volumes` unless you intend to remove the stored Kinosail state.

## Source of truth

Sources: `README.md`, `scripts/install.sh`, and `compose.release.yaml`.
