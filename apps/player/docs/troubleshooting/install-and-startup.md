---
title: Install and startup problems
description: Diagnose installer, container, HTTPS, health, storage, and startup failures.
section: Fix a problem
---

# Install and startup problems

Use this page when the installer, container, HTTPS page, or health check does not become ready.

## The installer stops with a validation message

Check the command and inputs first. The installer requires an existing absolute media directory and a port from 1 through 65,535.

```sh
./scripts/install.sh /absolute/path/to/media 38127
```

The optional third argument is `--lan`. Install on localhost first. Create and secure the first Owner, then rerun with `--lan`.

The installer also requires Podman Compose or Docker Compose and `cosign` for release verification. Install the missing prerequisite, then rerun the same command. Do not bypass signature or digest verification.

## The container is not running

Run these read-only checks from the directory that contains `compose.release.yaml`:

```sh
podman compose --file compose.release.yaml ps
podman compose --file compose.release.yaml logs --tail 50 kinosail
```

Use `docker compose` instead of `podman compose` when Docker runs the installation. Look for a missing bind path, port conflict, invalid configuration, permission error, or image pull error.

If the process exits, correct one reported cause and start it again:

```sh
podman compose --file compose.release.yaml up --detach
```

Do not remove the config or cache volume to fix a startup problem. Those volumes contain Server state.

## The health check fails

Run the same bounded check that the container uses:

```sh
podman compose --file compose.release.yaml exec -T kinosail kinosail healthcheck
```

Then inspect the last log lines. A failed health check can follow an invalid YAML file, an unavailable data directory, a missing backup key, or a startup task that has not completed.

If you use YAML, validate it before restarting:

```sh
podman compose --file compose.release.yaml run --rm --no-deps kinosail config validate
```

The same validation applies to stored Owner settings and environment overrides. Environment and YAML values are read-only in the web UI. Edit the deployment source instead.

## The browser cannot connect

Check the bind address and published port:

```sh
podman compose --file compose.release.yaml ps
```

Open the exact configured address. A default local install uses `https://localhost:38127`. A local certificate can produce a browser trust warning. Accept it only when you know that you reached your own Server.

If the page works on the Server host but not on another device, the install may still be bound to localhost. Complete first setup, then rerun the installer with `--lan`, or change the deployment bind setting as an Owner.

If another process uses the port, choose an unused port and keep the same port in the browser address and deployment configuration. Do not expose a new port to the internet while diagnosing local access.

## LAN or public HTTPS does not work

Test locally first. Then check the address, TLS host name, router forward, firewall, and external network separately. Public HTTPS requires a reachable TCP 443 path and the exact configured TLS name. CGNAT or blocked inbound traffic cannot be repaired by Kinosail.

Owners and administrative routes are not available through the public Viewer boundary. Use the LAN address or WireGuard for Owner administration.

For the guided remote setup, review its final output and the remote-access status in **Settings**. Do not send a DuckDNS token or private key in a support request.

## Media or backup storage is unavailable

Confirm that the host path exists and that the container can read Library Content. The library mount is read-only by design. The backup key and backup directory are separate from Library Content.

If backups report that setup is required, confirm that a backup key is configured and that the backup directory is writable by the container. Keep an independent copy of both backups and the key. A backup without its key cannot be restored.

## Recover after a failed release update

The release installer creates a recovery backup before updating a running installation. If the new image fails its health check, follow the installer’s rollback output. Do not start a second update while rollback is running.

If the Server is still unavailable, preserve the installer output and the last 50 log lines. Stop before restoring or deleting state. Use the [backup and recovery guide]({{ '/owner-guide/backups-and-updates/' | relative_url }}) when you have a verified backup and its key.


Source of truth: `scripts/install.sh`, Compose files, and system settings handlers.
