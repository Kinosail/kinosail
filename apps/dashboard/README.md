# Kinosail Dashboard

Kinosail Dashboard is a private, self-hosted home for the applications on your network. One responsive board provides direct links, search, service health, and safe editing on desktop and mobile.

The Server uses one Go binary, an embedded SQLite database, and one container. It does not proxy application traffic. Your browser opens each configured application directly.

## Features

- One ordered board that adapts from wide desktop screens to 320-pixel mobile screens.
- A local application catalog with useful defaults for common self-hosted services.
- Add, edit, remove, reorder, search, and favorite operations.
- Bounded service-health checks with clear evidence and timestamps.
- Owner authentication and a local audit history.
- Signed-in Owner password rotation that revokes other sessions and MCP access.
- A versioned `/api/v1` interface used by the web application.
- Model Context Protocol (MCP) tools that use the same validated application operations.
- Atomic SQLite persistence in the single Kinosail Dashboard container.
- Optional signed Supporter honors with a Dashboard-specific Badge Case.

[Repository home](../../README.md) · [Support](../../SUPPORT.md) · [Contributing](../../CONTRIBUTING.md) · [Security](../../SECURITY.md)

## Run with Compose

Install Docker Compose or Podman with a Compose provider. On macOS, start the container engine VM. Clone the complete monorepo and run these commands from `apps/dashboard/`; the build needs the root `packages/` directory. Docker Compose and Podman Compose can build and run the same source image:

```sh
podman compose up --build --detach
```

Open <http://localhost:38400>. The named `dashboard-config` volume stores the database. The service binds to localhost by default.

Create the Owner before enabling LAN access. The setup form requires a name, a unique password of at least 12 characters (at most 72 bytes), and matching confirmation. Add an application from the catalog or enter its reachable address, then confirm that the link opens on the device you will use.

Set `KINOSAIL_DASHBOARD_BIND=0.0.0.0` only when the host firewall and network are ready for LAN access. Use a trusted reverse proxy for HTTPS before access crosses an untrusted network; see [configuration](#configuration).

```sh
podman compose logs --follow dashboard
podman compose down
```

Docker users can replace `podman` with `docker`.

## Run from source

Go 1.27 or newer is required.

```sh
export KINOSAIL_DASHBOARD_DATA_DIR="$PWD/.kinosail-dashboard"
export KINOSAIL_DASHBOARD_LISTEN="127.0.0.1:38400"
go run ./cmd/kinosail-dashboard
```

## Verify

Follow the [root verification policy](../../CONTRIBUTING.md#verification). While `.gates-disabled` exists, do not run disabled suites or treat skipped results as passes. GitHub Actions is disabled.

When gates are enabled, run focused Go checks while developing:

```sh
make verify-changed
```

Run the complete local gate before release:

```sh
make check
```

The browser and container checks require pnpm and Podman or Docker. Automated checks do not prove behavior on a physical mobile device or an externally routed network.

## Configuration

Set these deployment variables in an uncommitted `.env` beside `compose.yaml`, then recreate the service with `podman compose up --detach`.

| Variable | Purpose and default |
| --- | --- |
| `KINOSAIL_DASHBOARD_BIND` | Host web-port binding; defaults to `127.0.0.1`. |
| `KINOSAIL_DASHBOARD_PORT` | Host port; defaults to `38400`. |
| `KINOSAIL_DASHBOARD_PUBLIC_URL` | Exact browser origin, including scheme and nonstandard port. Defaults to `http://localhost:38400` in Compose. No path, query, fragment, or credentials. |
| `KINOSAIL_DASHBOARD_SECURE_COOKIES` | `false` for local HTTP; must be `true` when the public URL uses HTTPS. |
| `KINOSAIL_DASHBOARD_TRUSTED_HOSTS` | Comma-separated exact trusted DNS names or IP addresses; no wildcards. Add the hostname used by your browser/proxy. |
| `KINOSAIL_DASHBOARD_PUBLIC_PROBE_HOSTS` | Comma-separated exact public hostnames that health checks may contact. Empty by default; private and loopback destinations are allowed. |

For a trusted HTTPS proxy, set all three of `KINOSAIL_DASHBOARD_PUBLIC_URL=https://dashboard.example.test`, `KINOSAIL_DASHBOARD_TRUSTED_HOSTS=dashboard.example.test`, and `KINOSAIL_DASHBOARD_SECURE_COOKIES=true`, replacing the example with your real hostname. Provision the proxy and certificate separately and restrict access to the backend. The public URL scheme and cookie setting must match or startup fails.

Native source runs additionally accept `KINOSAIL_DASHBOARD_LISTEN`, `KINOSAIL_DASHBOARD_DATA_DIR`, and `KINOSAIL_DASHBOARD_PROBE_INTERVAL` (default `30s`). The supplied Compose file fixes the first two to `:38400` and `/config` and does not forward the probe interval; a custom deployment must pass it explicitly. See [runtime configuration](cmd/kinosail-dashboard/main.go).

## Backups and updates

Export the board from Board settings for a portable copy of applications. An export is not a full account/session backup. Keep a private backup of the complete `/config` volume as well, including the database and any SQLite companion files. Stop Dashboard before taking a filesystem copy, use your container engine's volume backup procedure, then restart it.

To locate the actual volume, use `podman volume ls` and inspect the volume belonging to this Compose project; Compose may prefix `dashboard-config` with the project name. Do not assume an app export can restore Owner credentials.

For a source update, back up first, fetch and review the desired source revision, then run from the same app directory:

```sh
podman compose up --build --detach
podman compose exec -T dashboard /kinosail-dashboard healthcheck
```

Keep the previous source/image revision and full state backup. Restore into a stopped service using the same known-compatible revision before testing sign-in, applications, and health checks. Do not run `down --volumes` unless intentionally deleting stored state.

## Account recovery

Change the Owner password from Board settings while signed in. Forgotten-password recovery is not automated in this release. Keep a current board export and a backup of the `dashboard-config` volume. Do not delete the database to reset access.

## Connect an MCP client

Open Board settings and create an MCP token. Configure the trusted client with the Streamable HTTP endpoint `http://SERVER:38400/mcp` and the header `Authorization: Bearer TOKEN`.

The token has full Owner access. It can list, add, update, remove, reorder, and check applications. Creating a token replaces the previous MCP token. Revoke it from Board settings when access is no longer required. Use Transport Layer Security (TLS) and set `KINOSAIL_DASHBOARD_SECURE_COOKIES=true` before access crosses an untrusted network.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| The page does not load | Confirm the engine is running, `podman compose ps` shows the service, and port 38400 is free. The default bind only accepts the host. |
| Setup appears after an update | Stop and check the Compose project name and volume mapping before creating a new Owner; you may be using a different empty volume. |
| HTTPS startup or sign-in fails | Match the public URL scheme to secure cookies and add the browser hostname to trusted hosts. |
| A tile opens on one device only | Use a network-reachable application URL. `localhost` and container-only DNS names are usually wrong for other devices. |
| A health probe fails but the browser works | Probes originate inside the Server container, which has its own DNS/network view. Check connectivity and the exact public probe allowlist. |
| MCP returns unauthorized | Use the latest token; creating a replacement or changing the Owner password revokes old access. |

Inspect `podman compose logs --tail 100 dashboard` and redact private addresses and tokens before [reporting a problem](../../SUPPORT.md).

## Privacy and license

Board configuration, health evidence, owner data, and audit records stay on the owner-hosted Server. Configured application addresses receive only direct browser visits and explicitly enabled health probes.

Health checks allow private and loopback destinations by default. Public destinations require an exact hostname in `KINOSAIL_DASHBOARD_PUBLIC_PROBE_HOSTS`. Separate multiple hostnames with commas.

The Server accepts local and private Internet Protocol (IP) host headers. Add trusted proxy or domain names to `KINOSAIL_DASHBOARD_TRUSTED_HOSTS` before using them.

Set `KINOSAIL_SUPPORTER_ACTIVATION_URL` to the authorized certificate endpoint. Set `KINOSAIL_SUPPORTER_URL` to the purchase page. Supporter recognition never gates Dashboard features.

Kinosail Dashboard is source-available under the [PolyForm Perimeter License 1.0.1](LICENSE). It is not open source. Read [LICENSING.md](LICENSING.md) for the complete terms.
