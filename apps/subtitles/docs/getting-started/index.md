---
title: Choose an install path
description: Select the supported Kinosail installation path for your host and operating goal.
section: Start here
---

# Choose an install path

Choose one path before you begin. The release installer is the supported production path. A source install is useful for development. The synthetic test Server is useful for a safe trial.

## Choose the release installer

Use the release installer when you want a stable, one-container Server for your own media. It supports 64-bit Intel/AMD and Arm Linux. Docker Compose and Podman Compose can run the Linux container on macOS for local use.

The installer:

- checks that the media path is absolute and exists;
- creates a protected backup key;
- verifies the signed release image and records its digest;
- starts Kinosail on `127.0.0.1` first; and
- checks the Server health before it reports success.

Continue with [Install Kinosail]({{ '/getting-started/install/' | relative_url }}).

## Choose a source install

Use a source install when you are developing Kinosail or need to test a local change. You need the repository, Podman Compose or Docker Compose, and the tools required by the project. Source Compose does not mount the release installer backup key.

```sh
cp .env.example .env
# Set KINOSAIL_MEDIA_PATH to an absolute path in .env.
KINOSAIL_BACKUP_KEY_FILE= podman compose up --build --detach
```

Use `docker compose` instead when Docker is your runtime. Hardware acceleration needs the additional `compose.gpu.yaml` file and a supported host device.

Open the reported Server address. Create the first Owner from the host or another device on your local network.

## Choose the synthetic test Server

Use the test fixture when you want to explore Kinosail without downloading third-party creative media. It creates original synthetic movies, Shows, music, an audiobook, a book, photos, and a local TMDB-compatible catalogue.

```sh
./scripts/test-instance.sh up
./scripts/test-instance.sh verify
```

Open `https://localhost:38128`. Sign in as `Owner` with `test-instance-password`. Get the rotating test code with `./scripts/test-instance.sh totp` when the fixture asks for one.

Stop the fixture and remove its volumes when you finish:

```sh
./scripts/test-instance.sh down --volumes
```

## Decide where media and state live

Kinosail reads Library Content from the media mount. The container mounts this path writable so it can create validated Subtitle Files. Kinosail stores application state in embedded SQLite under its data volume. It stores transcode cache data separately and does not include either media or cache data in a portable backup.

Keep the following locations separate:

| Location | Purpose | Backup treatment |
| --- | --- | --- |
| Media path | Movies, Shows, music, books, photos, and other Library Content | Back up separately |
| `/config` | Profiles, settings, credentials, sessions, and viewing state | Included in state backups |
| `/cache` | Reproducible transcode data | Not included |
| Backup directory | Encrypted recovery archives | Protect with the backup key |

When you are ready, follow [Install Kinosail]({{ '/getting-started/install/' | relative_url }}), then [Complete first setup]({{ '/getting-started/first-setup/' | relative_url }}). Use [Connect phones, TVs, and Jellyfin apps]({{ '/getting-started/connect-devices/' | relative_url }}) when you add household devices.

## Source of truth

Sources: `README.md`, `scripts/install.sh`, `compose.release.yaml`, and `docs/research/documentation-information-architecture.md`.
