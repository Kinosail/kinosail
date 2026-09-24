# Kinosail

Kinosail is a set of self-hosted apps for your media and home network. Run the apps you need on your own hardware. Each app runs in its own container and includes a web interface, API, and database.

Kinosail Player Server is free to run on hardware you control. The web Player is included.

**[Official website](https://kinosail.com/)** · **[Get started](https://kinosail.com/quickstart/)** · **[Documentation](https://kinosail.com/docs/)** · **[Player merch](https://kinosail-shop.fourthwall.com/)** · **[Contribute](CONTRIBUTING.md)** · **[Get help](SUPPORT.md)** · **[Security](SECURITY.md)**

## Choose your app

| App | What it does | Default local address |
| --- | --- | --- |
| [Kinosail Player](apps/player/README.md) | Browse and play movies, Shows, music, audiobooks, books, and photos. Includes Profiles, playback progress, collections, and compatible playback. | `https://localhost:38127` |
| [Kinosail Subtitles](apps/subtitles/README.md) | Find, validate, and save subtitle sidecars beside your movies and episodes. | `https://localhost:38128` |
| [Kinosail Dashboard](apps/dashboard/README.md) | Organize direct links to your applications and check their reachability. | `http://localhost:38400` |

Player reads your media and does not change the source files. Subtitles needs write access to save subtitle files beside your media. Dashboard opens your apps in a browser. Kinosail does not relay media. You do not need a Kinosail account for local use.

## Get started with Kinosail Player

Use the published Player container from GitHub Container Registry. Docker pulls it from `ghcr.io`; you do not need to build from source. These examples keep setup on this computer and mount your media read-only. Replace `/path/to/your/media` with the full path to an existing media folder on the Docker host.

Install Docker Engine. The Compose example also needs the Compose plugin. Kinosail supports 64-bit Linux (`amd64` or `arm64`). Docker Desktop works for local use on macOS.

### Docker (one line)

```sh
docker run --detach --name kinosail --restart unless-stopped --init --user 10001:10001 --read-only --cap-drop ALL --security-opt no-new-privileges:true --publish 127.0.0.1:38127:38127 --mount type=volume,source=kinosail-config,target=/config --mount type=volume,source=kinosail-cache,target=/cache --mount type=volume,source=kinosail-backups,target=/backups --mount "type=bind,source=/path/to/your/media,target=/media,readonly" --tmpfs /tmp:rw,noexec,nosuid,nodev,size=256m --tmpfs /run:rw,noexec,nosuid,nodev,size=16m --env KINOSAIL_DATA_DIR=/config --env KINOSAIL_CACHE_DIR=/cache --env KINOSAIL_MEDIA_DIR=/media ghcr.io/kinosail/kinosail-player:latest
```

### Docker Compose

Create a folder for `compose.yaml` and `.env`. Set `KINOSAIL_MEDIA_PATH` to the full path of an existing media folder on the Docker host:

```dotenv
KINOSAIL_MEDIA_PATH=/path/to/your/media
KINOSAIL_BIND=127.0.0.1
KINOSAIL_PORT=38127
```

Save this as `compose.yaml` in the same folder:

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

Pull and start the container from the folder with `compose.yaml`:

```sh
docker compose pull
docker compose up -d
```

On this computer, open **<https://localhost:38127>**. Your browser will warn you about the local certificate. Trust only the certificate from your own Server. Create the first Owner with a unique password that has at least 12 characters, then add a passkey or TOTP authenticator.

The Compose example saves backup data in a volume, but it does not create an encryption key. Set up a backup key before you rely on encrypted backups. See [Backups and updates](apps/player/docs/owner-guide/backups-and-updates.md).

For signature verification and a pinned image digest, use the [verified Docker quickstart](https://kinosail.com/quickstart/).

## Build the apps from source

Use these steps only if you want to build app images from the source checkout. For a standard Player setup, use the published-container examples above.

### Requirements

- For a source install, install Git and get access to this repository.
- Install Docker Compose or Podman with a Compose provider. On macOS, start the container engine's Linux virtual machine.
- Use a 64-bit Linux host (`amd64` or `arm64`). On macOS, the container engine runs Linux for you.
- For Player or Subtitles, choose a media directory that the container can read or write as required.
- Leave the local ports in the table above free. Allow space for app data, artwork, backups, and playback cache.
- Container builds include the media tools. You do not need Go or FFmpeg on the host.

These source-build commands use Docker Compose. If you use Podman, replace `docker compose` with `podman compose`. Follow only the section for the app you want.

### 1. Get the source

```sh
git clone https://github.com/Kinosail/kinosail.git
cd kinosail
```

These examples build the source that you checked out. For a prebuilt Player container, use the examples in [Get started with Kinosail Player](#get-started-with-kinosail-player).

### 2. Start an app

#### Player

From the repository root:

```sh
cd apps/player
cp .env.example .env
chmod 600 .env
```

Edit `.env`. Set `KINOSAIL_MEDIA_PATH` to the full path of your media directory. Set `KINOSAIL_BACKUP_KEY_FILE=` to an empty value for this source install. Source Compose does not mount the installer's secret file. Automatic encrypted backups need a key. Follow the [backup guide](apps/player/docs/owner-guide/backups-and-updates.md) before you rely on backups.

```sh
docker compose up --build --detach
docker compose logs --tail 50 kinosail
```

On the host, open **<https://localhost:38127>**. Your browser may warn you about the local certificate. Trust only the certificate from your own Server. Create the first Owner with a unique password that has at least 12 characters. Add a passkey or TOTP authenticator. Then [complete first setup](apps/player/docs/getting-started/first-setup.md), add a library, and play an item.

#### Subtitles

From the repository root:

```sh
cd apps/subtitles
cp .env.example .env
chmod 600 .env
```

Set `KINOSAIL_MEDIA_PATH` to the full path of your movie or episode directory. For this source install, clear `KINOSAIL_BACKUP_KEY_FILE=` as described above. The container must be able to write subtitle files in this directory.

```sh
docker compose up --build --detach
docker compose logs --tail 50 kinosail
```

Open **<https://localhost:38128>**. Create and secure the Owner. In Settings, choose a language and a provider. Scan the library and fetch one subtitle before you set up a larger workflow. See the [Subtitles guide](apps/subtitles/docs/README.md) for credentials, matching, automation, and recovery.

#### Dashboard

From the repository root:

```sh
cd apps/dashboard
docker compose up --build --detach
docker compose logs --tail 50 dashboard
```

Open **<http://localhost:38400>**. Create the Owner and add your first app address. Each device's browser must be able to reach that address. On a phone, `localhost` means the phone itself. See the [Dashboard README](apps/dashboard/README.md) for health checks, HTTPS, backups, and MCP access.

### 3. Keep your data and connect other devices

Each source Compose project saves state in named volumes. Keep the same project name and directory when you restart or update an app. Compose uses these values to find the saved data.

From the app directory:

```sh
docker compose ps
docker compose down
docker compose up --detach
```

`down` stops the app and keeps its named volumes. If you add `--volumes`, Compose deletes those volumes and their data.

The web ports use localhost by default. Finish Owner setup before you change access. On a headless host, use a private SSH port forward to open the setup page. Player and Subtitles also map UDP ports for optional private management: 51821 and 51822. Publishing a port does not turn on this feature.

Use [Player device setup](apps/player/docs/getting-started/connect-devices.md) to set up HTTPS on your home network and connect clients. Use [Player remote access](apps/player/docs/owner-guide/remote-access.md) when you need access away from home. Read each app guide for setup details. Public Player access uses a separate, restricted HTTPS gateway. Do not forward the administration port to the internet.

### Published Player installation

Passing builds on `main` publish signed `latest` containers for changed apps. The Player [Docker quickstart](https://kinosail.com/quickstart/) checks the image signature and pins its digest. You do not need a numbered release or installer archive. A successful source build does not prove that a published image is available.

For a numbered release, get the matching app bundle from [Releases](https://github.com/Kinosail/kinosail/releases). Check its checksum and signature. The Player installer uses `cosign` to check the image before it pins the digest. Do not skip this check to install a development image.

## Documentation

Visit **[Kinosail Player Docs](https://kinosail.com/docs/)** for searchable web Player guides, starting with Docker installation.

| Task | Start here |
| --- | --- |
| Install, use, or troubleshoot Player | [Player documentation](apps/player/docs/README.md) |
| Configure subtitle providers and automation | [Subtitles documentation](apps/subtitles/docs/README.md) |
| Operate Dashboard | [Dashboard README](apps/dashboard/README.md) |
| Configure Player | [Configuration reference](apps/player/docs/reference/configuration.md) |
| Protect and restore Player state | [Backups and updates](apps/player/docs/owner-guide/backups-and-updates.md) |
| Integrate with Player's API or MCP | [Developer guide](apps/player/docs/developer-guide/index.md) |
| Understand the repository and releases | [Engineering guide](engineering/README.md) |
| Report a bug or suggest a change | [Support](SUPPORT.md) |

## Development

The Go workspace contains three app modules and [shared packages](packages/README.md). Use Go 1.27 or newer, as declared in [go.work](go.work). The [contribution guide](CONTRIBUTING.md) covers tool installation, Git workflow, app-scoped commands, and verification.

```sh
make hooks
# Compile one app without starting it:
cd apps/player
go build ./cmd/kinosail
```

**Verification status:** while `.gates-disabled` exists, quality suites and hooks are disabled by repository policy. Do not interpret a skipped command as a pass or remove the marker without maintainer authorization. See [verification policy](CONTRIBUTING.md#verification).

## Repository layout

```text
apps/player/       Player Server, web app, docs, and Apple clients
apps/subtitles/    Subtitle automation Server, web app, and docs
apps/dashboard/    Application dashboard Server and web app
packages/          Shared Go operations and contracts
engineering/       Cross-app architecture, research, and workflows
scripts/           Shared build, quality, and deployment tooling
.github/           Issue forms, PR template, and retained workflows
```

Kinosail Supporter and the Home Assistant integration live in separate repositories. App binaries, containers, versions, and release acceptance remain independent.

## License and contributions

Kinosail is **source-available**, under the [PolyForm Perimeter License 1.0.1](LICENSE). Read [LICENSING.md](LICENSING.md) for repository, third-party, and contribution boundaries. Contributions require the applicable contributor agreement; see [CONTRIBUTING.md](CONTRIBUTING.md). Report vulnerabilities privately using [SECURITY.md](SECURITY.md).

## Continuous integration and releases

GitHub-hosted runners check code quality, app tests, race conditions, browser journeys, dependencies, secrets, and CodeQL. Required checks protect `main`. A failed check blocks a merge or release.

Changed Player, Subtitles, and Dashboard containers build on Linux AMD64 and ARM64 runners at the same time. Passing builds on `main` publish signed `latest` images to `ghcr.io/kinosail/kinosail-player`, `ghcr.io/kinosail/kinosail-subtitles`, and `ghcr.io/kinosail/kinosail-dashboard`. Numbered releases use tags such as `player-v1.2.3`. CI must pass for the exact commit on `main`. CI scans each architecture before it signs and attests the combined image. Builds include a software bill of materials (SBOM) and provenance.

Release publication does not configure a production deployment target. The existing local deployment watcher remains separate.
