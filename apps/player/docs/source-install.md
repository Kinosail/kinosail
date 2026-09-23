---
title: Build from source
description: Build a local container from the Kinosail repository for development or early evaluation.
section: Build & contribute
---
# Build from source

Most users should use the [published Docker installation]({{ '/quickstart/' | relative_url }}). This alternative builds a local image from a source checkout for contributors and early evaluation. It does not establish that the image was published or signed.

## Prerequisites

Install Git and Docker with Compose (or Podman with a Compose provider). Use a 64-bit Linux host or start your container engine's Linux VM on macOS. Have an existing absolute media path ready. The container build includes the media tools.

## Build Player

```sh
git clone https://github.com/Kinosail/kinosail.git
cd kinosail/apps/player
cp .env.example .env
chmod 600 .env
```

Edit `.env` and set `KINOSAIL_MEDIA_PATH` to the existing absolute media directory. Clear `KINOSAIL_BACKUP_KEY_FILE=` for this source quickstart: source Compose does not mount the release installer's secret file. Automatic encrypted backups remain unavailable until you [configure a key]({{ '/owner-guide/backups-and-updates/' | relative_url }}).

```sh
docker compose up --build --detach
docker compose logs --tail 50 kinosail
```

Open **<https://localhost:38127>**, create the Owner, enroll a passkey or TOTP authenticator, and [complete first setup]({{ '/getting-started/first-setup/' | relative_url }}).

## Keep your state

Run updates from the same app directory with the same Compose project name. `docker compose down` preserves named volumes; `down --volumes` deletes them. Back up before rebuilding to a new source revision.

See [contributing](https://github.com/Kinosail/kinosail/blob/main/CONTRIBUTING.md) for direct Go builds, tests, and repository conventions.
