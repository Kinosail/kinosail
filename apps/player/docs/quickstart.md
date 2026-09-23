---
title: Get started with Docker
description: Copy a Docker or Docker Compose example, create your Owner, add media, and play from your browser.
section: Start here
---

# Get Kinosail running

Kinosail Server is free to run on your own hardware, and the web Player is included. You can also connect compatible Jellyfin mobile apps after setting up trusted HTTPS and enabling the optional integration in Owner Settings.

The examples below keep your media read-only, persist the Server's state, and bind the first setup page to this computer. For Jellyfin app setup, see [connect your devices]({{ '/getting-started/connect-devices/' | relative_url }}).

## Recommended: verified install

The installer verifies Kinosail's signed image, pins the image digest, creates the backup key, and starts the Server on localhost.

You need Docker Engine with the Compose plugin, Git, and `cosign`. Kinosail supports 64-bit Linux (`amd64` or `arm64`); Docker Desktop also works for local use on macOS. Then run:

```sh
git clone --depth 1 https://github.com/Kinosail/kinosail.git
cd kinosail/apps/player
./scripts/install.sh /absolute/path/to/your/media
```

Replace `/absolute/path/to/your/media` with an existing absolute path on the Docker host. The default web port is `38127`; pass another port as the second argument if that port is already in use.

## Docker CLI

These direct examples use the published `latest` tag. For signature verification and a pinned image digest, use the installer above. You need Docker Engine and an existing media folder on the Docker host. Change the path in the first line, then paste the block:

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

## Docker Compose

Create a folder for the Compose file and its settings. Change the media path to an existing folder on the Docker host:

```sh
mkdir -p ~/kinosail
cd ~/kinosail
cat > .env <<'EOF'
KINOSAIL_MEDIA_PATH=/absolute/path/to/your/media
KINOSAIL_BIND=127.0.0.1
KINOSAIL_PORT=38127
EOF
```

The media path must already exist on the Docker host. These direct examples leave automatic encrypted backups unconfigured; set up a backup key before relying on backups, or use the installer, which creates one for you.

Save this as `compose.yaml` in that folder:

```yaml
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
```

Start Player and watch its first-start logs:

```sh
docker compose up -d
docker compose logs --follow kinosail
```

Open <https://localhost:38127> on the same computer. Your browser will warn about the locally generated certificate. Accept it only for your own Server, then create the first Owner with a unique password of at least 12 characters and enroll a passkey or TOTP authenticator.

## Settings most installs need

| Setting | Default in these examples | Change it when |
| --- | --- | --- |
| Media folder | `/absolute/path/to/your/media` on the host, mounted as `/media:ro` | Your collection is stored somewhere else. Keep the mount read-only. |
| Web port | `38127` | Another service already uses that host port. In Compose, change `KINOSAIL_PORT`. |
| First-run access | `127.0.0.1` | Keep setup on this computer until the Owner is created. Then follow [connect devices]({{ '/getting-started/connect-devices/' | relative_url }}) to enable trusted LAN access. |
| Jellyfin mobile apps | Off by default | Configure trusted HTTPS, enable **Allow compatible Jellyfin apps to connect** in Owner Settings, and restart Kinosail. |
| Persistent state | Named `config` and `cache` volumes | Keep the same volume names when recreating the container. |
| Container user | `10001:10001` | This is fixed by the image; `PUID` and `PGID` are not needed. |

Choose household preferences, scan behavior, and playback options in Player's Owner settings. The Docker environment is for paths and container deployment. Before relying on encrypted automatic backups, configure the backup key using [backups and updates]({{ '/owner-guide/backups-and-updates/' | relative_url }}). The verified installer creates that key for you.

## Add your library and play

1. In first setup, create the Owner and secure the account.
2. Add a library folder under `/media`, such as `/media/Movies` or `/media/Shows`.
3. Let the scan finish, open an item, and play it in your browser.

For a phone, TV, or another computer, follow [connect devices]({{ '/getting-started/connect-devices/' | relative_url }}). Keep the Server private until its LAN address and HTTPS identity are configured. For remote access, use the [remote access guide]({{ '/owner-guide/remote-access/' | relative_url }}); do not forward the setup port directly to the internet.

For the maintained signed-image install, health checks, and update flow, use the [verified installer](#recommended-verified-install). For startup help, see [install troubleshooting]({{ '/troubleshooting/install-and-startup/' | relative_url }}).
