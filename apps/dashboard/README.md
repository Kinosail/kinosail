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

## Run with Compose

Docker Compose and Podman Compose can build and run the same source image:

```sh
podman compose up --build --detach
```

Open <http://localhost:38400>. The named `dashboard-config` volume stores the database. The service binds to localhost by default.

Set `KINOSAIL_DASHBOARD_BIND=0.0.0.0` only when the host firewall and network are ready for local-network access. Use a trusted reverse proxy for Transport Layer Security (TLS) before public exposure.

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

Run focused Go checks while developing:

```sh
make verify-changed
```

Run the complete local gate before release:

```sh
make check
```

The browser and container checks require pnpm and Podman or Docker. Automated checks do not prove behavior on a physical mobile device or an externally routed network.

## Account recovery

Change the Owner password from Board settings while signed in. Forgotten-password recovery is not automated in this release. Keep a current board export and a backup of the `dashboard-config` volume. Do not delete the database to reset access.

## Connect an MCP client

Open Board settings and create an MCP token. Configure the trusted client with the Streamable HTTP endpoint `http://SERVER:38400/mcp` and the header `Authorization: Bearer TOKEN`.

The token has full Owner access. It can list, add, update, remove, reorder, and check applications. Creating a token replaces the previous MCP token. Revoke it from Board settings when access is no longer required. Use Transport Layer Security (TLS) and set `KINOSAIL_DASHBOARD_SECURE_COOKIES=true` before access crosses an untrusted network.

## Privacy and license

Board configuration, health evidence, owner data, and audit records stay on the owner-hosted Server. Configured application addresses receive only direct browser visits and explicitly enabled health probes.

Health checks allow private and loopback destinations by default. Public destinations require an exact hostname in `KINOSAIL_DASHBOARD_PUBLIC_PROBE_HOSTS`. Separate multiple hostnames with commas.

The Server accepts local and private Internet Protocol (IP) host headers. Add trusted proxy or domain names to `KINOSAIL_DASHBOARD_TRUSTED_HOSTS` before using them.

Set `KINOSAIL_SUPPORTER_ACTIVATION_URL` to the authorized certificate endpoint. Set `KINOSAIL_SUPPORTER_URL` to the purchase page. Supporter recognition never gates Dashboard features.

Kinosail Dashboard is source-available under the [PolyForm Perimeter License 1.0.1](LICENSE). It is not open source. Read [LICENSING.md](LICENSING.md) for the complete terms.
