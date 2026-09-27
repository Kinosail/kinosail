---
title: Install Kinosail Subtitles with Docker
description: Run the published Kinosail Subtitles container with a writable media folder and persistent app state.
section: Start here
last_reviewed: 2026-09-27
---

# Install Kinosail Subtitles

Install Docker Engine on a 64-bit Linux host, or use Docker Desktop on macOS. Choose an existing movie or episode folder that container user 10001 can read and write. Replace `/path/to/your/media` with its absolute path on the host.

Use either Docker CLI or Docker Compose to run the published image.

## Docker CLI

```sh
docker run --detach --name kinosail-subtitles --restart unless-stopped \
  --init --user 10001:10001 --read-only --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --publish 127.0.0.1:38128:38128 \
  --mount type=volume,source=kinosail-subtitles-config,target=/config \
  --mount type=volume,source=kinosail-subtitles-cache,target=/cache \
  --mount type=volume,source=kinosail-subtitles-backups,target=/backups \
  --mount "type=bind,source=/path/to/your/media,target=/media" \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=256m \
  --env KINOSAIL_DATA_DIR=/config \
  --env KINOSAIL_CACHE_DIR=/cache \
  --env KINOSAIL_MEDIA_DIR=/media \
  ghcr.io/kinosail/kinosail-subtitles:latest
```

## Docker Compose

Save this file as `compose.yaml` in a new directory. Replace the media path with the same absolute path used by the Docker CLI example.

```yaml
services:
  kinosail:
    image: ghcr.io/kinosail/kinosail-subtitles:latest
    restart: unless-stopped
    init: true
    user: "10001:10001"
    read_only: true
    cap_drop:
      - ALL
    security_opt:
      - no-new-privileges:true
    ports:
      - "127.0.0.1:38128:38128"
    environment:
      KINOSAIL_DATA_DIR: /config
      KINOSAIL_CACHE_DIR: /cache
      KINOSAIL_MEDIA_DIR: /media
    volumes:
      - config:/config
      - cache:/cache
      - backups:/backups
      - /path/to/your/media:/media:rw
    tmpfs:
      - /tmp:rw,noexec,nosuid,nodev,size=256m
volumes:
  config:
    name: kinosail-subtitles-config
  cache:
    name: kinosail-subtitles-cache
  backups:
    name: kinosail-subtitles-backups
```

In that directory, start the container and check its health:

```sh
docker compose up --detach
docker compose logs --tail 50 kinosail
docker compose exec -T kinosail kinosail healthcheck
```

The two methods use the same named volumes. Stop the existing container before switching methods.

Open **https://localhost:38128** on that host. The initial certificate is generated locally; trust only your own installation. Create the first Owner with a unique password of at least 12 characters, then set up a passkey or time-based one-time password.

The media mount is writable because Subtitles saves sidecar files beside videos. App state, cache, and backups use separate volumes. The web port stays on localhost until you deliberately change its binding. Configure an encryption key before relying on automatic backups; see [Back up and update]({{ '/owner-guide/backups-and-updates/' | relative_url }}).

To update a Docker CLI install, pull the latest published image and recreate the container with the same mounts and settings. For Docker Compose, run `docker compose pull` and `docker compose up --detach`. Keep the three named volumes and the media folder. The [release checklist](https://github.com/Kinosail/kinosail/blob/main/apps/subtitles/engineering/release-checklist.md) explains signed images and immutable digests.

## Build from source

Install Git and a working Docker Compose or Podman Compose engine. Start its Linux VM on macOS. Clone the complete monorepo; the image needs shared `packages/` outside this app directory.

```sh
git clone https://github.com/Kinosail/kinosail.git
cd kinosail/apps/subtitles
cp .env.example .env
chmod 600 .env
```

Edit `.env` before starting:

- Set `KINOSAIL_MEDIA_PATH` to an existing absolute directory containing your movies and episodes.
- Set `KINOSAIL_BACKUP_KEY_FILE=` to empty for the source quickstart. The supplied source Compose file does not mount the release installer's secret file. Configure encrypted backups before keeping important state.
- Keep the web binding at its localhost default until the first Owner is secured.

```sh
docker compose up --build --detach
docker compose logs --tail 50 kinosail
docker compose exec -T kinosail kinosail healthcheck
```

Replace `docker compose` with `podman compose` when using Podman. Open **https://localhost:38128** on the host. The initial certificate is generated locally; trust only your own installation. Use a private SSH port forward for a headless host.

The image runs as UID/GID 10001. Grant that account appropriate read/write access to the selected media tree. Check host ACLs, NAS permissions, and container file-sharing configuration when writes fail; do not make the entire library world-writable.

Compose keeps configuration, cache, and backups in persistent locations and mounts media at `/media` with write access. `docker compose down` preserves named volumes; adding `--volumes` deletes stored application state. Keep the same Compose project when updating.

Next: [Complete first setup]({{ '/getting-started/first-setup/' | relative_url }}) and [configure backups]({{ '/owner-guide/backups-and-updates/' | relative_url }}).
