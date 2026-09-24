---
title: Docker installation details
description: Install one Kinosail Server container and open it safely for first setup.
section: Start here
---

# Docker installation details

Start with [Install with Docker]({{ '/quickstart/' | relative_url }}) to get the current deployment files and install the prebuilt Player container. Successful changes on `main` publish signed images; numbered releases are optional.

Run the installer from `apps/player` in that checkout. It verifies the image, creates protected recovery material, starts one Server container, and binds it to localhost until you create the first Owner.

## Prerequisites

You need:

- a 64-bit Intel/AMD or Arm Linux host, or Docker/Podman on macOS for local use;
- Podman Compose or Docker Compose;
- Cosign 3.1.3 or newer to verify the signed container image;
- `curl` when you later enable LAN mode;
- an absolute path to an existing folder that contains media you control; and
- a free TCP port, such as `38127`.

The Player directory must include `scripts/install.sh` and `compose.release.yaml`. Do not place credentials or private hostnames in documentation, shell history, or a public repository.

## Install on localhost

From `apps/player`, run:

```sh
./scripts/install.sh /absolute/path/to/media 38127
```

The second argument is optional. The default port is `38127`. The path must be absolute, must exist, and must not contain a newline or a single quote.

The installer uses `podman compose` when it is available. Otherwise, it uses `docker compose`. If needed, it creates `secrets/backup_key` with file mode `600`. Keep this key with your encrypted backups.

The installer pulls `ghcr.io/kinosail/kinosail-player:latest`. It checks the image signature against `publish.yml@refs/heads/main`. It pins the SHA-256 digest in `.env` and starts the Server. It waits for `kinosail healthcheck` to pass.


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

The installer checks that `/setup` redirects. It changes `KINOSAIL_BIND` to `0.0.0.0`. If `KINOSAIL_TLS_HOSTS` or `KINOSAIL_AUTH_URL` is empty, it adds the detected home-network address. This does not enable public internet access.

Open the reported LAN address from another device. Use the exact HTTPS name in the address when you add passkeys. If your host has more than one network interface, verify the detected address before sharing it.

## Update an existing installation

Run `git pull --ff-only` to update the deployment files. Then run the installer with the same media path and port. Keep the same checkout and Compose project name. If the Server is running, the installer saves a recovery archive in `backups/` before it pulls the new image. It keeps your existing `.env` values.

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

`stop` and `start` keep the container configuration, including installer settings. Use the installer to update Kinosail. `down` removes containers but keeps named volumes. Do not add `--volumes` unless you want to delete the saved Kinosail data.

## Source of truth

Sources: `README.md`, `scripts/install.sh`, and `compose.release.yaml`.
