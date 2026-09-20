---
title: Install Subtitles
description: Build the source container with writable media and persistent state.
section: Start here
last_reviewed: 2026-09-15
---

# Install Subtitles

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
