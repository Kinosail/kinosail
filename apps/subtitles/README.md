# Kinosail Subtitles

Kinosail Subtitles finds, validates, and adds subtitle sidecar files to movies and episodes you control. It is the subtitle automation app in the Kinosail family.

[Repository home](../../README.md) · [User documentation](docs/README.md) · [Support](../../SUPPORT.md) · [Contributing](../../CONTRIBUTING.md)

The application uses the same foundation as Kinosail Player: a Go API-driven monolith, server-rendered HTML with small browser enhancements, embedded SQLite, one hardened container, local-first authentication, and the shared Kinosail design and verification system.

## What works

- Scans movie and episode folders directly.
- Detects untagged and language-tagged SRT or WebVTT sidecar files.
- Shows preferred-language coverage, wanted items, and every scanned video.
- Uses an existing sidecar or embedded text track before a provider request.
- Searches the SubDL, OpenSubtitles.com, and SubSource APIs for movies and individual episodes.
- Ranks exact IDs, the OpenSubtitles file hash, release names, episodes, language, accessibility, and translation source.
- Cleans safe SRT or WebVTT, rejects invalid timing, and aligns weak release matches against local speech before writing. SubSource files stay unchanged under its terms.
- Validates provider responses, download hosts, archive cardinality, file size, subtitle timing content, and null bytes.
- Writes `Title.en.srt` atomically beside the source video.
- Protects unknown sidecars unless an exact OpenSubtitles hash proves a better release match.
- Automatically fills missing subtitles and upgrades managed files after scans, with a 15-minute minimum between bounded cycles.
- Searches up to ten wanted items from the dashboard, or a validated maximum of 50 through the API.
- Keeps provider credentials, application state, and media files on the owner-hosted Server.

The default is deliberately conservative. Kinosail upgrades its own sidecars only after a candidate gains at least ten score points. It upgrades an unknown sidecar only for an exact OpenSubtitles hash match and keeps the original as a `.kinosail.bak` recovery file.

## Install from source

Install Git and Docker Compose or Podman with a Compose provider. Use a 64-bit Linux container host; macOS can run the image in the engine's Linux VM. Clone the complete monorepo and run these commands from `apps/subtitles/`.

Kinosail Subtitles needs write access to the media folders where it creates sidecar files. The image runs as UID/GID 10001; grant appropriate access to the chosen folders without making the whole library world-writable. Keep backups of existing sidecars and media.

```sh
cp .env.example .env
chmod 600 .env
# Set KINOSAIL_MEDIA_PATH to an existing absolute movie/episode directory.
# Set KINOSAIL_BACKUP_KEY_FILE= to empty: source Compose does not mount that secret.
podman compose up --build --detach
```

Docker users can replace `podman compose` with `docker compose`.

Open `https://localhost:38128`. Create the first Owner with a unique password of at least 12 characters. Kinosail requires a passkey or time-based one-time password for every Owner.

The container remains non-root, read-only, capability-free, and protected by `no-new-privileges`. Application state, cache, backups, and the media mount have separate writable locations. Host permissions still control which folders the container can change. The web port stays on localhost until you change its binding; finish Owner setup before doing so. Automatic encrypted backups require a configured key; see [backups](docs/owner-guide/backups-and-updates.md).

Useful commands:

```sh
podman compose logs --follow kinosail
podman compose down
```

## Configure subtitle search

Open Settings to configure provider credentials and the preferred language. Use the guided setup links, or follow the [provider guide](docs/owner-guide/integrations.md). Configure one or more providers; availability, quotas, and account terms belong to each provider. One provider is enough. Kinosail searches every configured provider and selects one result automatically.

```sh
KINOSAIL_SUBDL_API_KEY=your-key
# Optional custom or test endpoint.
KINOSAIL_SUBDL_URL=https://api.subdl.com/api/v1

KINOSAIL_OPENSUBTITLES_API_KEY=your-application-key
KINOSAIL_OPENSUBTITLES_USERNAME=your-username
KINOSAIL_OPENSUBTITLES_PASSWORD=your-password
# Optional custom or test endpoint.
KINOSAIL_OPENSUBTITLES_URL=https://api.opensubtitles.com/api/v1

# SubSource is for personal household use. Set both values after accepting its terms.
KINOSAIL_SUBSOURCE_API_KEY=your-key
KINOSAIL_SUBSOURCE_PERSONAL_USE=true
# Optional custom or test endpoint.
KINOSAIL_SUBSOURCE_URL=https://api.subsource.net/api/v1

KINOSAIL_SUBTITLE_LANGUAGE=en
```

The variables above are native-process configuration names. A Compose `.env` value reaches the container only when `compose.yaml` forwards it. The supplied Compose files forward SubDL and OpenSubtitles settings; they currently do not forward SubSource settings. Configure SubSource through Owner Settings, or explicitly add its environment/secret mappings in a local Compose override. Do not assume that adding an unforwarded key to `.env` configures the app.

Secrets also support `_FILE` variants; the referenced file must be mounted into the container. Deployment-managed values stay visible but read-only in Owner Settings.

SubSource downloads stay unchanged on disk. Kinosail validates them, but does not rewrite, synchronize, shift, or convert the stored sidecar.

The dashboard uses a lowercase ISO language code such as `en` or `pt-br`. An exact sidecar such as `Arrival.srt` counts as a default subtitle. A language sidecar such as `Arrival.en.srt` covers only that language.

## First successful subtitle

1. Open the local address and create the Owner with a unique password and required authenticator.
2. Choose movie/episode folders in setup and scan them.
3. Set your preferred language and at least one provider.
4. Select a wanted item, fetch a subtitle, and check the resulting language sidecar beside that video.
5. Review the dashboard status before relying on automatic maintenance.

Scanning can show local coverage without a provider. A provider quota, unmatched episode, read-only folder, or invalid download can prevent a fetch; see [troubleshooting](docs/troubleshooting/index.md). Exact naming matters: `Film.en.srt` covers English for `Film.mkv`.

## Updates and recovery

Keep the same Compose project and volumes when updating. Back up application state, deployment secrets, and media/sidecars separately; the app backup does not contain the media tree. Read [Back up and update](docs/owner-guide/backups-and-updates.md) before `podman compose up --build --detach`. `podman compose down` preserves volumes; adding `--volumes` deletes app state.

A managed sidecar may be upgraded; an unknown sidecar requires an exact OpenSubtitles hash match and is preserved as a `.kinosail.bak` copy. Keep that original until you have checked the replacement. SubSource files remain unchanged under its terms.

## HTTP API

The web and API adapters call the same application operations.

```text
GET  /api/v1/subtitle-library
POST /api/v1/subtitle-library/{id}/fetch
POST /api/v1/subtitle-library/fetch-wanted
POST /api/v1/subtitle-library/maintain
```

`maintain` runs the same bounded operation as background automation. It fills missing files and applies safe upgrades.

Fetch one subtitle with the configured language:

```json
{}
```

Or select a language explicitly:

```json
{"language":"es"}
```

Run a bounded batch:

```json
{"language":"en","limit":10}
```

Unknown fields, malformed JSON, invalid languages, and limits outside 1–50 are rejected before provider requests or file writes.

## Architecture

Kinosail Subtitles deliberately retains the Kinosail repository framework:

- `AGENTS.md` defines implementation, validation, UI, delivery, security, and performance policy.
- `CONTEXT.md` owns subtitle-domain language.
- `DESIGN.md` owns the sister-app visual contract.
- `../../.codex/skills/` contains the shared design, implementation, testing, review, and domain skills.
- `engineering/` contains architecture decisions, research, release procedures, and agent guidance.
- `docs/` is reserved for published user documentation.
- `internal/server` keeps the web and versioned API adapters on shared operations.
- `../../packages/library` owns bounded local-media discovery.

The binary enables subtitle-app mode. The user-facing product is the subtitle command center; use Player for household media playback. Shared internals do not imply that Player features are supported Subtitles workflows.

## Development and verification

Follow the [root contribution guide](../../CONTRIBUTING.md). While `.gates-disabled` exists, do not run the suites below or count skipped checks as passes. GitHub Actions is disabled. When gates are enabled, run focused server coverage while editing:

```sh
go test ./internal/server -run 'TestSubtitle(App|Dashboard|Mutations)'
```

Run the repository gates before publication:

```sh
make max-loc
KINOSAIL_VERIFY_WORKTREE=1 make verify-changed
make check
```

Visible changes also require the populated browser gate:

```sh
make test-instance-check
```

Use `KINOSAIL_BROWSER_MATRIX=full` when shared CSS, navigation, authentication, or another cross-browser boundary changes.

## Privacy and safety

Media and subtitles stay on the owner-hosted Server. Kinosail does not relay media or subtitle files. A provider receives bounded search metadata such as title, language, filename, media type, release IDs, episode identity, and the OpenSubtitles file hash. The hash is calculated locally from the file size and boundary blocks; the media file is not uploaded.

Provider data is untrusted. The application bounds and validates JSON, candidate counts, URLs, redirects, archives, downloaded bytes, and subtitle content before an atomic write. Sidecar targets are derived from an already scanned video path and a validated language code; callers cannot supply a filesystem path.

Kinosail Subtitles is source-available under the [PolyForm Perimeter License 1.0.1](LICENSE). Read [LICENSING.md](LICENSING.md), [SECURITY.md](SECURITY.md), and [CONTRIBUTING.md](CONTRIBUTING.md) before distribution or contribution.
