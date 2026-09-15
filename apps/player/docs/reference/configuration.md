---
title: Configuration reference
description: Understand Kinosail setting sources, precedence, validation, restart behavior, and secret handling.
section: Reference
last_reviewed: 2026-08-28
---

# Configuration reference

Kinosail accepts configuration from built-in defaults, Owner settings, a YAML file, and environment variables. The higher source wins.

## Choose a source

Use the Owner settings page for values that you want to change in the app. Use YAML or environment variables for deployment-owned values.

The precedence order is:

```text
environment > YAML > Owner settings (GUI) > built-in default
```

Kinosail reads the file named by `KINOSAIL_CONFIG_FILE`. If that variable is empty, the Server can use `kinosail.yaml` below its data directory. The example file is [`kinosail.example.yaml`](https://github.com/MikeO7/kinosail/blob/main/apps/player/kinosail.example.yaml). Copy it to `kinosail.yaml` and set only the values that you own.

If YAML or an environment variable supplies a value, the Owner interface shows that value as managed and read-only. Resetting a managed value cannot change it. An omitted YAML value does not lock the corresponding Owner setting.

The `paths.data` value is resolved before stored Owner settings load. This lets a YAML or environment value select the directory that contains persisted configuration.

## File and environment rules

- YAML must contain `version: 1`.
- YAML uses nested keys, such as `playback.mode` under `playback`.
- List values use YAML lists. Environment list values use a JSON array, such as `[".","Shows"]`.
- A YAML secret can use a `<key>_file` value, such as `token_file`. The path is relative to the YAML file unless it is absolute.
- An environment secret can use a variable ending in `_FILE`. The file must be regular and no larger than 16 KiB.
- A direct secret variable and its `_FILE` form cannot both be set.
- `_FILE` is valid only for secret settings.
- Unknown keys, malformed values, and invalid cross-field combinations stop configuration loading.
- Settings marked “restart” take effect after a Server restart. Other settings are live or are applied by their owning operation.

Do not place secrets in a committed YAML file. Prefer a mounted secret file or the Owner settings flow. Secret values are never returned by the configuration API.

## Compose and application settings

The table lists application-process variables. Compose only passes variables explicitly declared in the chosen Compose files; setting an arbitrary variable in `.env` does not forward it. Host mount/binding variables such as `KINOSAIL_MEDIA_PATH`, `KINOSAIL_BACKUP_PATH`, `KINOSAIL_BIND`, and `KINOSAIL_PORT` configure Compose itself. Read `.env.example` alongside `compose.yaml` and any overrides.

## Settings

The following table describes typed application settings. Use the running configuration API and the source definitions for the exact inventory of your installed version. A blank default means that the feature is not configured by default.

| Key | Environment variable | Type | Default | Restart |
| --- | --- | --- | --- | --- |
| `listen` | `KINOSAIL_LISTEN` | address | `127.0.0.1:38127` | yes |
| `tls.enabled` | `KINOSAIL_TLS_ENABLED` | boolean | `true` | yes |
| `tls.hosts` | `KINOSAIL_TLS_HOSTS` | JSON list | blank | yes |
| `tls.duckdns` | `KINOSAIL_DUCKDNS_HTTPS` | trusted HTTPS secret text | blank | yes |
| `paths.media` | `KINOSAIL_MEDIA_DIR` | path | `/media` | yes |
| `paths.data` | `KINOSAIL_DATA_DIR` | path | `/config` | yes |
| `paths.cache` | `KINOSAIL_CACHE_DIR` | path | `/cache` | yes |
| `binaries.ffmpeg` | `KINOSAIL_FFMPEG` | text | `ffmpeg` | yes |
| `binaries.ffprobe` | `KINOSAIL_FFPROBE` | text | `ffprobe` | yes |
| `binaries.fpcalc` | `KINOSAIL_FPCALC` | text | `fpcalc` | yes |
| `server.name` | `KINOSAIL_SERVER_NAME` | text | blank | no |
| `supporter.activation_url` | `KINOSAIL_SUPPORTER_ACTIVATION_URL` | URL | Kinosail supporter endpoint | yes |
| `supporter.url` | `KINOSAIL_SUPPORT_URL` | URL | Kinosail repository | yes |
| `security.require_mfa` | `KINOSAIL_REQUIRE_MFA` | boolean | `true` | no |
| `auth.url` | `KINOSAIL_AUTH_URL` | URL | blank | yes |
| `libraries` | `KINOSAIL_LIBRARIES` | JSON list | blank | no |
| `playback.mode` | `KINOSAIL_PLAYBACK_MODE` | enum | blank | no |
| `playback.autoplay` | `KINOSAIL_AUTOPLAY` | boolean | blank | no |
| `playback.subtitles` | `KINOSAIL_SUBTITLES` | enum | blank | no |
| `playback.auto_skip` | `KINOSAIL_AUTO_SKIP` | JSON list | blank | no |
| `transcoding.quality` | `KINOSAIL_TRANSCODE_QUALITY` | enum | blank | no |
| `transcoding.accelerator` | `KINOSAIL_TRANSCODE_ACCELERATOR` | enum | blank | no |
| `transcoding.codec` | `KINOSAIL_TRANSCODE_CODEC` | enum | blank | no |
| `transcoding.tone_map` | `KINOSAIL_TONE_MAP` | boolean | blank | no |
| `subtitles.language` | `KINOSAIL_SUBTITLE_LANGUAGE` | 2–3 letter code | blank | no |
| `scanning.interval` | `KINOSAIL_SCAN_INTERVAL` | duration | `10m` | yes |
| `scanning.frequency` | `KINOSAIL_SCAN_FREQUENCY` | enum | blank | no |
| `dlna.url` | `KINOSAIL_DLNA_URL` | URL | blank | yes |
| `dlna.enabled` | `KINOSAIL_DLNA_ENABLED` | boolean | blank | no |
| `backup.directory` | `KINOSAIL_BACKUP_DIR` | path | `/backups` | yes |
| `backup.key` | `KINOSAIL_BACKUP_KEY` | secret text | blank | yes |
| `backup.interval` | `KINOSAIL_BACKUP_INTERVAL` | duration | `24h` | yes |
| `backup.retention` | `KINOSAIL_BACKUP_RETENTION` | positive integer | `7` | yes |
| `logging.level` | `KINOSAIL_LOG_LEVEL` | `debug`, `info`, `warn`, `error` | `info` | yes |
| `logging.audit_retention` | `KINOSAIL_AUDIT_RETENTION` | positive duration | `8760h` | yes |
| `logging.playback_retention` | `KINOSAIL_PLAYBACK_RETENTION` | positive duration | `2160h` | yes |
| `remote.proxy_token` | `KINOSAIL_PROXY_TOKEN` | secret text | blank | yes |
| `remote.mode` | `KINOSAIL_REMOTE_MODE` | `off`, `wireguard`, `https` | `off` | yes |
| `remote.gateway` | `KINOSAIL_PUBLIC_GATEWAY` | boolean; requires `https` mode | `false` | yes |
| `remote.duckdns_domain` | `KINOSAIL_DUCKDNS_DOMAIN` | DNS label | blank | yes |
| `remote.duckdns_token` | `KINOSAIL_DUCKDNS_TOKEN` | secret text | blank | yes |
| `remote.listen` | `KINOSAIL_REMOTE_LISTEN` | address | `:8443` | yes |
| `remote.wireguard_dir` | `KINOSAIL_WIREGUARD_DIR` | absolute path | `/wireguard` | yes |
| `integrations.tmdb.url` | `KINOSAIL_TMDB_URL` | URL | blank | yes |
| `integrations.tmdb.image_url` | `KINOSAIL_TMDB_IMAGE_URL` | URL | blank | yes |
| `integrations.tmdb.token` | `KINOSAIL_TMDB_TOKEN` | secret text | blank | yes |
| `integrations.jellyfin.enabled` | `KINOSAIL_JELLYFIN_ENABLED` | boolean | `false` | no |
| `integrations.home_assistant.enabled` | `KINOSAIL_HOME_ASSISTANT_ENABLED` | boolean | blank | no |
| `integrations.oidc.issuer` | `KINOSAIL_OIDC_ISSUER` | URL | blank | yes |
| `integrations.oidc.client_id` | `KINOSAIL_OIDC_CLIENT_ID` | text | blank | yes |
| `integrations.oidc.client_secret` | `KINOSAIL_OIDC_CLIENT_SECRET` | secret text | blank | yes |
| `integrations.oidc.redirect_url` | `KINOSAIL_OIDC_REDIRECT_URL` | URL | blank | yes |
| `integrations.oidc.identity_claim` | `KINOSAIL_OIDC_IDENTITY_CLAIM` | text | `sub` | yes |
| `integrations.saml.metadata_url` | `KINOSAIL_SAML_METADATA_URL` | URL | blank | yes |
| `integrations.saml.metadata_xml` | `KINOSAIL_SAML_METADATA_XML` | text | blank | yes |
| `integrations.saml.identity_attribute` | `KINOSAIL_SAML_IDENTITY_ATTRIBUTE` | text | `NameID` | yes |
| `integrations.scim.token` | `KINOSAIL_SCIM_TOKEN` | secret text | blank | yes |
| `integrations.scim.token_expires_at` | `KINOSAIL_SCIM_TOKEN_EXPIRES_AT` | RFC3339 time | blank | yes |
| `integrations.mcp.resource_url` | `KINOSAIL_MCP_RESOURCE_URL` | HTTPS URL | blank | yes |
| `integrations.mcp.authorization_server` | `KINOSAIL_MCP_AUTHORIZATION_SERVER` | HTTPS URL | blank | yes |
| `integrations.mcp.introspection_url` | `KINOSAIL_MCP_INTROSPECTION_URL` | HTTPS URL | blank | yes |
| `integrations.mcp.client_id` | `KINOSAIL_MCP_CLIENT_ID` | text | blank | yes |
| `integrations.mcp.client_secret` | `KINOSAIL_MCP_CLIENT_SECRET` | secret text | blank | yes |
| `integrations.webhook.url` | `KINOSAIL_WEBHOOK_URL` | URL | blank | yes |
| `integrations.webhook.token` | `KINOSAIL_WEBHOOK_TOKEN` | secret text | blank | yes |

## Enum values and validation

- `playback.mode`: `automatic`, `direct`, or `compatible`.
- `playback.subtitles`: `on` or `off`.
- `transcoding.quality`: `automatic`, `speed`, or `quality`.
- `transcoding.accelerator`: `none`, `auto`, `vaapi`, `qsv`, `cuda`, `videotoolbox`, `rkmpp`, `v4l2m2m`, `amf`, or `mf`.
- `transcoding.codec`: `auto`, `h264`, `hevc`, `av1`, or `vp9`.
- `scanning.frequency`: `default`, `off`, `5m`, `15m`, or `1h`.
- `remote.mode`: `off`, `wireguard`, or `https`.

Remote access requires a DuckDNS domain and token. HTTPS mode also requires `auth.url` to be the exact `https://<domain>.duckdns.org` origin. WireGuard mode requires an absolute clean directory. Enabling DLNA requires `dlna.url`. SCIM requires both its token and a future expiration time. MCP requires all five MCP OAuth values and a resource URL ending in `/mcp`.

`integrations.jellyfin.enabled` is `false` when it is unset. Setting it to `true` requires a valid trusted HTTPS value. Kinosail rejects the configuration before it exposes Jellyfin routes when trusted HTTPS is missing.

The Owner settings flow is the preferred setup. The historical `tls.duckdns` key now stores every supported trusted HTTPS provider. Its name remains for automatic upgrade compatibility.

A deployment-managed deSEC value is one secret JSON object:

```json
{"provider":"desec","domain":"myhome.dedyn.io","token":"REDACTED_PROVIDER_TOKEN","address":"192.168.1.10","termsAccepted":true}
```

`provider` is `duckdns` or `desec`. `domain` is the lowercase subdomain label for DuckDNS. It is a lowercase full hostname for deSEC.

`address` must be a private IPv4 address or a local hostname that resolves to one private IPv4 address. `termsAccepted` records acceptance of the Let's Encrypt subscriber agreement. Store the complete object through the environment variable `_FILE` form or another protected secret source.

Kinosail still accepts the previous DuckDNS object without `provider`. It treats that object as `duckdns` and requires no Owner action.

LAN trusted HTTPS requires TLS. It cannot run with public HTTPS remote mode. A DuckDNS-based WireGuard deployment must use a different DuckDNS label for its remote connection. Follow [Connect devices]({{ '/getting-started/connect-devices/' | relative_url }}) for the Owner workflow and privacy effects.

## Owner-managed configuration API

Owners can inspect configuration with `GET /api/v1/configuration`. Use `PUT /api/v1/configuration/{key}` to save an editable value and `DELETE` to reset it to the next source. A managed key returns a conflict instead of changing state. See the [API reference]({{ '/reference/api/' | relative_url }}) for authentication and the live schema.

## Source of truth

This page follows `internal/configuration/configuration.go`, `internal/configuration/parse.go`, and `kinosail.example.yaml`. The application tests the example YAML and precedence rules during its configuration test suite.

The supported public HTTPS Compose override sets `remote.gateway=true` and adds
a restricted public gateway container using the same image as the Server. The
main application serves public requests only over a private Unix socket. Set this
only with that deployment; an ordinary direct binary listener has no gateway
process isolation. Owner device management is separate from `remote.mode` and is
explicitly enabled and paired through **Settings → Manage while away**. See the
[remote-access guide]({{ '/owner-guide/remote-access/' | relative_url }}).
