---
title: Install with Docker
description: Install the prebuilt Kinosail Player container with Docker Compose, then create your Owner and play your first item.
section: Start here
---
# Install with Docker

Docker Compose is the recommended way to run Kinosail. Green changes on `main` publish signed Player containers for Intel/AMD and Arm hosts. You do not need to wait for a numbered release or build the application locally.

The installer uses `ghcr.io/kinosail/kinosail-player:latest`, verifies its signature, and pins its immutable image digest. A numbered GitHub release or installer archive is not required.

## Container image

```sh
docker pull ghcr.io/kinosail/kinosail-player:latest
```

This downloads the image only. Follow the installer steps below to verify the signature and configure persistent storage, read-only media, HTTPS, and backup secrets. The `latest` tag advances after successful publication; an installed Server stays on its pinned digest until you update it.

## Before you start

You need:

- Docker Engine with the Compose plugin on a 64-bit Intel/AMD or Arm Linux host, or Docker Desktop on macOS with its Linux VM running.
- An existing media folder with an absolute path and permissions allowing the container account to read it.
- Local port **38127** available, plus space for application state, artwork, and playback cache.
- Git to obtain the deployment files and `cosign` to verify the container image. Use its [official installation instructions](https://docs.sigstore.dev/cosign/system_config/installation/).

Podman with a Compose provider is also supported. The installer chooses Podman if both engines are available; otherwise it uses Docker. The commands below assume Docker is the selected engine.

## 1. Get the deployment files

Clone the repository and enter the Player directory. These files configure the prebuilt container; this step does not build the application:

```sh
git clone --depth 1 https://github.com/Kinosail/kinosail.git
cd kinosail/apps/player
```

Keep this directory for updates. The installer verifies the downloaded image against `publish.yml` on the protected `main` branch before starting it.

## 2. Start the Docker container

Replace `/absolute/path/to/media` with your existing media directory:

```sh
./scripts/install.sh /absolute/path/to/media 38127
```

The installer pulls `ghcr.io/kinosail/kinosail-player`, verifies the image signature, pins its digest, creates a protected backup key, and starts the container using `compose.release.yaml`. It waits for the health check before reporting success. Keep the installation directory and `secrets/backup_key` for future updates and recovery.

Check the running service from that same directory:

```sh
docker compose --file compose.release.yaml ps
docker compose --file compose.release.yaml logs --tail 50 kinosail
```

Open **<https://localhost:38127>** on the same computer. A generated local certificate warning is expected: trust only the certificate from your own installation.

On a headless Linux host, forward the setup port from your own computer with `ssh -L 38127:127.0.0.1:38127 user@your-server`, replacing the example login and host. Then open the localhost address on your computer.

If startup fails, follow [install and startup troubleshooting]({{ '/troubleshooting/install-and-startup/' | relative_url }}). Keep your state volumes while investigating.

## 3. Create your Owner

Create the first Owner with a unique password of at least 12 characters. Enroll a passkey or TOTP authenticator and keep recovery material somewhere private. Complete setup before changing network access.

Follow [first setup]({{ '/getting-started/first-setup/' | relative_url }}) to configure your Server and [add your media]({{ '/getting-started/add-media/' | relative_url }}) to create a library. Let scanning finish, open an item, and try playback in the browser.

## 4. Connect another device

The default web port listens on localhost. On a phone, `localhost` means the phone—not your Server. Follow [device setup]({{ '/getting-started/connect-devices/' | relative_url }}) to configure reachable addresses and trusted HTTPS before connecting other devices.

For away-from-home playback, use the [remote-access guide]({{ '/owner-guide/remote-access/' | relative_url }}). Public viewing uses a separate restricted HTTPS gateway. Do not forward the administration web port to the internet.

## Stop and come back later

From the same app directory:

```sh
docker compose --file compose.release.yaml stop
docker compose --file compose.release.yaml start
```

`stop` and `start` preserve the existing container configuration, including installer-selected options. Use the installer for updates. `down` preserves named volumes but removes containers. **Adding `--volumes` deletes stored state.** Keep the same Compose project name and directory so restarts reconnect to the same volumes.

## Advanced setup

- [Configure hardware acceleration and playback]({{ '/owner-guide/playback/' | relative_url }}).
- [Build from source]({{ '/source-install/' | relative_url }}) for development or evaluation.

Next: [learn the playback controls]({{ '/user-guide/playback/' | relative_url }}) or [plan your backups]({{ '/owner-guide/backups-and-updates/' | relative_url }}).
