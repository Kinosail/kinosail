---
title: Get started with Docker
description: Pull the Kinosail Player container with Docker Compose, add your media folder, and play from your browser.
section: Start here
---

# Get Kinosail running

Kinosail Server is free to run on your own hardware. The web Player runs in a published container. Docker Compose pulls it from GitHub Container Registry; you do not need to build it from source.

## Docker Compose (recommended)

Install Docker Engine with the Compose plugin. Kinosail supports 64-bit Linux (`amd64` or `arm64`). Docker Desktop works for local use on macOS.

Create a folder for the Compose file. In `.env`, replace `/path/to/your/media` with the full path to an existing media folder on the Docker host. Keep the default address and port for first setup:

```sh
mkdir -p ~/kinosail
cd ~/kinosail

cat > .env <<'EOF'
KINOSAIL_MEDIA_PATH=/path/to/your/media
KINOSAIL_BIND=127.0.0.1
KINOSAIL_PORT=38127
EOF

cat > compose.yaml <<'EOF'
name: kinosail

services:
  kinosail:
    image: ghcr.io/kinosail/kinosail-player:latest
    container_name: kinosail
    restart: unless-stopped
    init: true
    user: "10001:10001"
    read_only: true
    cap_drop: [ALL]
    security_opt:
      - no-new-privileges:true
    ports:
      - "${KINOSAIL_BIND:-127.0.0.1}:${KINOSAIL_PORT:-38127}:38127"
    environment:
      KINOSAIL_DATA_DIR: /config
      KINOSAIL_CACHE_DIR: /cache
      KINOSAIL_MEDIA_DIR: /media
    volumes:
      - config:/config
      - cache:/cache
      - backups:/backups
      - "${KINOSAIL_MEDIA_PATH}:/media:ro"
    tmpfs:
      - /tmp:rw,noexec,nosuid,nodev,size=256m
      - /run:rw,noexec,nosuid,nodev,size=16m

volumes:
  config:
  cache:
  backups:
EOF

docker compose pull
docker compose up -d
```

The media folder is mounted read-only. The Server saves its settings in Docker volumes named `config`, `cache`, and `backups`. Keep those volume names when you update or recreate the container.

To follow the startup log, run:

```sh
docker compose logs --follow kinosail
```

On the same computer, open <https://localhost:38127>. Your browser will warn you about the local certificate. Trust it only for your own Server. Create the first Owner with a unique password that has at least 12 characters. Then add a passkey or TOTP authenticator.

## Docker CLI

Use this if you prefer `docker run` to Compose. This pulls the same published container image. Change the media path to an existing folder on the Docker host before running the command:

```sh
export KINOSAIL_MEDIA_PATH="/absolute/path/to/your/media"

docker run --detach \
  --name kinosail \
  --restart unless-stopped \
  --init \
  --user 10001:10001 \
  --read-only \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --publish 127.0.0.1:38127:38127 \
  --mount type=volume,source=kinosail-config,target=/config \
  --mount type=volume,source=kinosail-cache,target=/cache \
  --mount type=volume,source=kinosail-backups,target=/backups \
  --mount "type=bind,source=${KINOSAIL_MEDIA_PATH},target=/media,readonly" \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=256m \
  --tmpfs /run:rw,noexec,nosuid,nodev,size=16m \
  --env KINOSAIL_DATA_DIR=/config \
  --env KINOSAIL_CACHE_DIR=/cache \
  --env KINOSAIL_MEDIA_DIR=/media \
  ghcr.io/kinosail/kinosail-player:latest
```

## Verified installer

For signature verification and a pinned image digest, use the install script instead of starting with the Compose example. It also creates a backup key and starts the Server on this computer.

You need Docker Engine with the Compose plugin, Git, and Cosign 3.1.3 or newer. The script pulls the published image; it does not build from source. Run these commands:

```sh
git clone --depth 1 https://github.com/Kinosail/kinosail.git
cd kinosail/apps/player
./scripts/install.sh /path/to/your/media
```

Replace `/path/to/your/media` with the full path to an existing folder on the Docker host. The default web port is `38127`. If another service uses this port, give the installer a different port as its second argument.

## Settings most installs need

| Setting | Default in these examples | Change it when |
| --- | --- | --- |
| Media folder | The path set in `.env`, mounted as `/media:ro` | Your collection is stored somewhere else. Keep the mount read-only. |
| Web port | `38127` | Another service already uses that host port. In Compose, change `KINOSAIL_PORT`. |
| First-run access | `127.0.0.1` | Keep setup on this computer until you create the Owner. Then follow [Connect your devices]({{ '/getting-started/connect-devices/' | relative_url }}) to enable trusted home-network access. |
| Persistent state | Named `config`, `cache`, and `backups` volumes | Keep the same volume names when recreating the container. |
| Container user | `10001:10001` | This is fixed by the image; `PUID` and `PGID` are not needed. |

Set household preferences, scan behavior, and playback options in Owner Settings. Use Docker environment values for paths and container setup. Before you rely on encrypted backups, set up a backup key. Follow [Backups and updates]({{ '/owner-guide/backups-and-updates/' | relative_url }}). The verified installer creates a key for you.

## Add your library and play

1. In first setup, create the Owner and secure the account.
2. Add a library folder under `/media`, such as `/media/Movies` or `/media/Shows`.
3. Let the scan finish, open an item, and play it in your browser.

To connect another computer or device, follow [Connect your devices]({{ '/getting-started/connect-devices/' | relative_url }}). Keep the Server private until its home-network address and HTTPS identity are set up. For access away from home, use the [Remote access guide]({{ '/owner-guide/remote-access/' | relative_url }}). Do not forward the setup port to the internet.

For startup help, see [Install troubleshooting]({{ '/troubleshooting/install-and-startup/' | relative_url }}).
