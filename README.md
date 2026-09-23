# Kinosail

Kinosail is a family of private, self-hosted applications for your media and home network. Run the apps you need on your own hardware. Each app includes its web interface, API, and embedded database in one independently deployed container.

**[Get started](https://kinosail.com/quickstart/)** · **[Documentation](https://kinosail.com/docs/)** · **[Contribute](CONTRIBUTING.md)** · **[Get help](SUPPORT.md)** · **[Security](SECURITY.md)**

## Choose your app

| App | What it does | Default local address |
| --- | --- | --- |
| [Kinosail Player](apps/player/README.md) | Browse and play movies, Shows, music, audiobooks, books, and photos. Includes profiles, progress, collections, and compatible playback. | `https://localhost:38127` |
| [Kinosail Subtitles](apps/subtitles/README.md) | Find, validate, and save subtitle sidecars beside your movies and episodes. | `https://localhost:38128` |
| [Kinosail Dashboard](apps/dashboard/README.md) | Organize direct links to your applications and check their reachability. | `http://localhost:38400` |
| [Player for iPhone, iPad, and Apple TV](apps/player/apps/native/README.md) | Native Swift clients that connect to a Kinosail Player Server. | Use your Server's address |

Player mounts media read-only. Subtitles needs write access to create sidecars. Dashboard opens applications directly in your browser. Kinosail does not operate a media relay, and local use does not require a Kinosail-hosted account.

The native targets are iOS/iPadOS and tvOS. Other devices can use the web app or Player's optional Jellyfin-compatible interface; compatibility depends on the client, codec, and device. See [connecting devices](apps/player/docs/getting-started/connect-devices.md).

## Getting started

For a normal web Player installation, follow [Install with Docker](https://kinosail.com/quickstart/). The installer verifies and pins the published Player container. The examples below build each app from the checked-out source for development or evaluation.

### Requirements

- Git and access to this repository for a source installation.
- Docker with Compose, or Podman with a Compose provider. On macOS, start the container engine's Linux virtual machine first.
- A 64-bit Linux container host (`amd64` or `arm64`); macOS can run the Linux containers for local use.
- An existing media directory for Player or Subtitles, with suitable host permissions. Container builds include the media tools; you do not need host Go or FFmpeg for this path.
- Free local ports from the table above and storage for application state, artwork, backups, and playback cache. Transcoding requirements depend on your media and hardware.

These commands use Docker Compose. Replace `docker compose` with `podman compose` if you use Podman. Run only the section for the app you want.

### 1. Get the source

```sh
git clone https://github.com/Kinosail/kinosail.git
cd kinosail
```

These examples build the checked-out source. Follow the [published Player installation](#published-player-installation) for a prebuilt container.

### 2. Start an app

#### Player

From the repository root:

```sh
cd apps/player
cp .env.example .env
chmod 600 .env
```

Edit `.env`: set `KINOSAIL_MEDIA_PATH` to your existing absolute media directory. Set `KINOSAIL_BACKUP_KEY_FILE=` to an empty value for this source quickstart: source Compose does not mount the release installer's secret file. Automatic encrypted backups remain unavailable until you configure a key; follow the [backup guide](apps/player/docs/owner-guide/backups-and-updates.md) before keeping important state.

```sh
docker compose up --build --detach
docker compose logs --tail 50 kinosail
```

Open **<https://localhost:38127>** on the host. A generated local certificate warning is expected on first use; trust only the certificate from your own installation. Create the first Owner with a unique password of at least 12 characters and enroll a passkey or TOTP authenticator. Complete [first setup](apps/player/docs/getting-started/first-setup.md), add your libraries, and play an item.

#### Subtitles

From the repository root:

```sh
cd apps/subtitles
cp .env.example .env
chmod 600 .env
```

Set `KINOSAIL_MEDIA_PATH` to an existing absolute movie/episode directory and clear `KINOSAIL_BACKUP_KEY_FILE=` for the source quickstart as above. The container account must be able to write sidecars in this directory.

```sh
docker compose up --build --detach
docker compose logs --tail 50 kinosail
```

Open **<https://localhost:38128>**, create and secure the Owner, then configure a preferred language and a provider in Settings. Scan the library and fetch a subtitle for one item before enabling a larger workflow. See the [Subtitles guide](apps/subtitles/docs/README.md) for credentials, matching, automation, and recovery.

#### Dashboard

From the repository root:

```sh
cd apps/dashboard
docker compose up --build --detach
docker compose logs --tail 50 dashboard
```

Open **<http://localhost:38400>**, create the Owner, and add your first application's address. Use addresses that the browser on each household device can reach: `localhost` on a phone means the phone itself. See the [Dashboard README](apps/dashboard/README.md) for health probes, HTTPS, backups, and MCP access.

### 3. Keep your data and connect other devices

Each source Compose project stores state in its own named volumes. Keep the same project name and directory when restarting or updating so Compose reconnects to those volumes.

From the app directory:

```sh
docker compose ps
docker compose down
docker compose up --detach
```

`down` stops the app and preserves named volumes. Adding `--volumes` deletes those volumes and their stored state.

The web ports bind to localhost by default. Finish Owner setup before changing access. For a headless host, use a private SSH port forward to reach its localhost setup page. Player and Subtitles also map a UDP port for optional private management (51821 and 51822 respectively); publishing a port does not configure or enable the feature.

Use [Player device setup](apps/player/docs/getting-started/connect-devices.md) for LAN HTTPS and clients, [Player remote access](apps/player/docs/owner-guide/remote-access.md) for away-from-home viewing, and the app-specific guides for administration. Public Player viewing uses a separate restricted HTTPS gateway. Do not forward the administration web port to the internet.

### Published Player installation

Passing `main` builds publish signed `latest` containers for affected apps. Player's [Docker quickstart](https://kinosail.com/quickstart/) uses the published image, verifies its signature, and pins its digest. A numbered GitHub release or installer archive is not required for this path. A successful source build does not establish that a published image is available.

For a numbered release, use the matching app bundle from [Releases](https://github.com/Kinosail/kinosail/releases) and verify its supplied checksum and signature. The Player installer requires `cosign` and verifies the image identity before pinning its digest. Do not bypass that check to install a development image.

## Documentation

Visit **[Kinosail Player Docs](https://kinosail.com/docs/)** for searchable web Player guides, starting with Docker installation.

| Task | Start here |
| --- | --- |
| Install, use, or troubleshoot Player | [Player documentation](apps/player/docs/README.md) |
| Configure subtitle providers and automation | [Subtitles documentation](apps/subtitles/docs/README.md) |
| Operate Dashboard | [Dashboard README](apps/dashboard/README.md) |
| Build the Apple clients | [Native client README](apps/player/apps/native/README.md) |
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

Apple client builds require macOS and Xcode with the iOS/tvOS 26 SDKs. The Swift apps build directly with Xcode.

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

GitHub-hosted runners enforce repository quality, app tests, race checks, browser tests, native client builds, dependency scanning, secret scanning, and CodeQL. Required checks protect `main`; failed checks block merging and releases.

Affected Player, Subtitles, and Dashboard containers build on native Linux AMD64 and ARM64 runners in parallel. Passing `main` builds publish signed `latest` images to `ghcr.io/kinosail/kinosail-player`, `ghcr.io/kinosail/kinosail-subtitles`, and `ghcr.io/kinosail/kinosail-dashboard`. Numbered releases use the `player-vMAJOR.MINOR.PATCH`, `subtitles-vMAJOR.MINOR.PATCH`, and `dashboard-vMAJOR.MINOR.PATCH` tags and require successful CI for the exact commit on `main`. Each architecture is scanned before its combined manifest is signed and attested. Builds include SBOMs and provenance.

Release publication does not configure a production deployment target. The existing local deployment watcher remains separate.
