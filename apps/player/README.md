# Kinosail

Kinosail is a private, self-hosted media server for the movies, Shows, music, audiobooks, books, photos, and live television you control. It combines a Go server, an HTMX web interface, embedded SQLite, and FFmpeg in one container. Library Content is mounted read-only and never needs to pass through a Kinosail-operated service.

The quickest production path is the release installer below.

## Core features

- **One-container library:** automatic and on-demand scanning for Movies, Shows and Episodes, music and albums, audiobooks, books, photos, and mixed Libraries. Kinosail stores application state in embedded SQLite and does not require a database sidecar.
- **Fast browsing:** responsive and installable web/PWA interface with search, filters, long-library pagination, Continue Watching, recommendations, private history and ratings, My List, playlists, smart playlists, collections, queues, and shuffle.
- **Rich metadata:** local NFO and embedded tags, folder artwork, optional TMDB enrichment and artwork, cast, seasons and Episodes, chapters, lyrics, photo dates and albums, and playback markers.
- **Adaptive playback:** direct play and byte-range seeking when the device supports the source; remux, audio conversion, or HLS transcoding when it does not. The Linux images include QSV, NVENC/NVDEC, VA-API, and RKMPP where applicable; native macOS and Windows FFmpeg installations provide the VideoToolbox and AMF paths that a Linux container cannot access.
- **Viewer playback tools:** audio and subtitle selection, sidecar and embedded text subtitles, preferred subtitle language, chapter navigation, intro/credits skipping, resume positions, watched state, next-Episode playback, low-gap music queues, audiobook speed and sleep controls, and integrity-checked offline downloads.
- **Shared viewing:** Direct synchronized Watch Rooms for authenticated Viewers.
- **Broad clients:** the bundled web app, a versioned JSON API, and a tested Jellyfin-compatible surface for common Android, Android TV, iOS, and Swiftfin flows. Protocol coverage is not universal physical-device certification.
- **Operations included:** automatic maintenance, encrypted scheduled backups, structured logs, Prometheus metrics, diagnostics, an integrity-linked activity journal, and import or one-way sync of compatible viewing activity.

## Sharing and access control

Kinosail sharing uses persistent Viewer Profiles and direct connections. Kinosail does not operate a media proxy, tunnel, or relay.

### Viewer Profile controls

Kinosail sharing is built around local **Owner** and **Viewer Profiles**. A Server may have multiple Owners, must always retain at least one, and can give each Viewer only the access they need. Changing or removing a Profile takes effect at the Server and revokes sessions when required.

| Viewer control | What the Owner can enforce |
| --- | --- |
| Libraries | No Library access, every Library, or selected Library folders |
| Content rating | Family, teen, or unrestricted content ceilings |
| Viewing hours | Optional daily start and end times |
| Remote access | Whether the Viewer may connect through the public HTTPS boundary |
| Transcoding | Whether the Viewer may consume transcoding capacity |
| Downloads | Whether original or prepared offline files may be downloaded |

Remote access is **off by default**. When an Owner enables it, media travels directly between the Viewer and the owner-hosted Server.

Public HTTPS is the simplest no-VPN path for browsers and compatible Jellyfin apps. Public TCP 443 must reach the Server through each network address translation (NAT) router. Carrier-grade NAT (CGNAT) or blocked inbound traffic requires the Owner to obtain public reachability from the internet provider. Kinosail cannot remove this network limit without operating a relay.

| Connection | Intended use | Public Kinosail surface |
| --- | --- | --- |
| LAN HTTPS | Normal use on a trusted home network | No internet exposure |
| WireGuard | Owner administration and paired managed devices | No public Kinosail HTTP listener |
| Public HTTPS (recommended for ordinary Viewers) | Browsers and compatible Jellyfin apps without a VPN | Dedicated, Viewer-only listener on the configured hostname |

Run the guided setup only after the local Server is installed and the first Owner is secured:

```sh
./scripts/setup-remote-access.sh
```

Choose WireGuard for the smallest application exposure. Choose public HTTPS for ordinary Viewers and Jellyfin apps. The guided setup defaults to public HTTPS. It configures DuckDNS, protects its token, and can restart a standard Compose installation. The Owner must configure the TCP 443 router forward and test the address from an external network.

Jellyfin compatibility is off by default and stays behind its integration flag. Enable it in Owner settings or set `KINOSAIL_JELLYFIN_ENABLED=true`. Disabling it hides Jellyfin-compatible routes without disabling the Kinosail web app or `/api/v1`. Enter the displayed HTTPS address manually in the client. Configure trusted HTTPS through DuckDNS or deSEC so clients need no certificate install. DuckDNS is easiest. deSEC supports narrower tokens for more privacy. Use Quick Connect for limited-input devices. Swiftfin supports this flow on Apple TV.

Follow [Connect phones, TVs, and Jellyfin apps](docs/getting-started/connect-devices.md) for the complete Server URL, trusted HTTPS provider, and troubleshooting steps.

Home Assistant support is off by default. Enable it in the setup wizard or Owner settings. Then create a ten-minute pairing code and add the [Kinosail Home Assistant integration](https://github.com/MikeO7/kinosail-home-assistant). The connection can browse Library Content and control active Kinosail browser players. Media streams directly from this Server. Turning the setting off hides every Home Assistant route and revokes every paired connection. Operators can manage the same setting with `KINOSAIL_HOME_ASSISTANT_ENABLED`.

Turn either remote mode off without changing local access:

```sh
./scripts/disable-remote-access.sh
```

### Security controls for public sharing

Public HTTPS is a separate application boundary, not the LAN interface with a port forwarded:

- only Profiles explicitly allowed remote access can sign in;
- Owners, API keys, setup, Profile management, backups, diagnostics, MCP/OAuth administration, and other administrative routes are unavailable;
- password login is disabled; a remote Viewer uses a user-verified passkey or a short-lived, one-use Quick Connect request approved from a strongly authenticated LAN or WireGuard session;
- public sessions use a separate secure, host-only cookie, expire after eight hours, are capped per Viewer, and are revoked when remote permission is removed or the public kill switch is used;
- the listener requires the exact configured TLS name and HTTP Host, serves TLS 1.2 or 1.3 with forward-secret AEAD ciphers, and applies bounded HTTP/2, header, request, connection, and per-source capacity;
- the public route surface is allowlisted, media ranges are parsed strictly, and Profile policy is rechecked before Viewer operations;
- scanner tripwires and repeated credential failures quarantine only the verified source, while the Owner-visible emergency kill switch closes public connections and persists across restart without interrupting LAN access; and
- security events and denied access are recorded in the local integrity-linked activity journal without passwords, tokens, secret values, request bodies, or URL queries.

Kinosail cannot stop a volumetric attack before traffic reaches the home connection. Router configuration, firewall policy, DNS account security, disk encryption, physical security, and off-host backup custody remain Owner responsibilities.

## Install a release

Kinosail supports 64-bit Intel/AMD and Arm Linux. Docker Compose and Podman Compose can run the same container on macOS for local use. Release bundles include a checksummed installer; the installer verifies the published image's keyless signature, pins its digest, generates the automatic-backup key, and initially binds Kinosail to localhost so another device cannot claim the first Owner Profile.

The central install command is:

```sh
./scripts/install.sh /absolute/path/to/media 38127
```

Open `https://localhost:38127`, accept the generated local-certificate warning, and create the first Owner Profile with a unique password of at least 12 characters. Every Owner must then enroll one passkey or TOTP authenticator before using the rest of Kinosail. After setup, rerun the installer with `--lan` to expose that initialized Server only to the private network.

## Run from source

Copy the environment template, set the absolute media path, and start the development image:

```sh
cp .env.example .env
# Edit KINOSAIL_MEDIA_PATH in .env.
# The source Compose file does not mount the release installer's backup key.
KINOSAIL_BACKUP_KEY_FILE= podman compose up --build --detach
```

Open `https://localhost:38127`. Configuration, TLS material, backups, cache data, and Library Content use separate persistent locations; the Library Content mount is read-only.

For hardware transcoding on a supported host:

```sh
KINOSAIL_BACKUP_KEY_FILE= podman compose --file compose.yaml --file compose.gpu.yaml up --build --detach
```

Select Automatic in Settings. Source installs can override `KINOSAIL_GPU_DEVICE` and `KINOSAIL_GPU_GROUP` for a different device or host group.

Release installs detect Intel, AMD, NVIDIA CDI, and standard RK3588 devices automatically. The installer also adds the detected host device groups to the non-root container user. Set `KINOSAIL_GPU_BACKEND=off` in `.env` only when you want software transcoding. VideoToolbox and Windows AMF require a native Server process and cannot be enabled inside Kinosail's Linux container.

Useful lifecycle commands:

```sh
podman compose logs --follow kinosail
podman compose down
```

Docker users can replace `podman compose` with `docker compose`.

## Configuration and integrations

Every Server setting has one validated definition. Precedence is environment variables, optional YAML, Owner-persisted Settings, then built-in defaults. Environment- and YAML-managed values remain visible but read-only in the web interface and versioned API.

- Copy [kinosail.example.yaml](kinosail.example.yaml) to `kinosail.yaml`, then uncomment only settings that the file should manage. Omitted settings stay editable in Owner Settings. The release installer detects the file automatically; manual Compose runs add `compose.config.yaml`.
- Run `kinosail config validate [FILE]` to check the same file, stored UI settings, and environment overrides before startup.
- Use `_FILE` variants for supported secrets so credentials can be mounted as files instead of placed in the environment.
- Set `KINOSAIL_AUTH_URL` to the canonical HTTPS/passkey origin and list accepted LAN DNS names or IP addresses in `KINOSAIL_TLS_HOSTS`.
- Set `KINOSAIL_TMDB_TOKEN` for optional movie and Show enrichment. TMDB attribution and licensing requirements still apply.
- Configure OpenID Connect for identities that a Profile explicitly links after local sign-in. Unknown provider identities cannot create Profiles.
- Open **Settings → Deployment configuration → SCIM provisioning** to copy the provider base URL, create a bearer token, and choose its expiration date. Docker and YAML can instead set `KINOSAIL_SCIM_TOKEN` with `KINOSAIL_SCIM_TOKEN_EXPIRES_AT`. The configured sign-in address must be a trusted HTTPS origin that the identity provider can reach. Kinosail supports SCIM 2.0 user discovery, provisioning, updates, disable, restore, core and enterprise attributes, standard filters, schema discovery, and attribute projection at `/scim/v2`. Configure user provisioning only because group provisioning is not supported. SCIM-managed Viewer Profiles are passwordless and receive the least-privilege default policy. To allow OpenID Connect or SAML sign-in, map `externalId` to the configured stable identity claim or assertion attribute. Names and email addresses never link identities.
- Kinosail also provides an authenticated `/api/v1` surface and optional MCP access; both reuse the same Owner/Viewer application operations and policy checks as the web adapter.

### OpenID Connect single sign-on

1. Create a confidential web application in your identity provider.
2. Register `https://your-kinosail-address/login/oidc/callback` as its exact return address.
3. Open **Settings → Access → Deployment configuration → Single sign-on**.
4. Enter the provider issuer URL, client ID, client secret, registered return address, and stable identity claim together.
5. Restart Kinosail Server. Sign in locally once, then link OpenID Connect from the Profile page.

Kinosail uses provider discovery and the Authorization Code flow with PKCE. The default identity claim is `sub`. It requests `openid`, plus `profile` when another claim is selected. Microsoft Entra deployments can use `oid`. Map the same claim to SCIM `externalId`. The provider must support a confidential client with `client_secret_basic` or `client_secret_post`. Use the exact issuer from the provider's discovery document. A private provider certificate authority must be trusted inside the Kinosail container.

### SAML single sign-on

1. Set the trusted public Kinosail address with `KINOSAIL_AUTH_URL`.
2. Open **Settings → Access → Deployment configuration → SAML single sign-on**.
3. Copy the service-provider metadata URL into your identity provider.
4. Enter the provider metadata URL. If the provider supplies a download, paste its metadata XML instead.
5. Keep `NameID`, or enter one stable signed assertion attribute. Map the same value to SCIM `externalId`.
6. Restart Kinosail Server. Sign in locally once, then link SAML from the Profile page.

Kinosail uses signed SAML 2.0 authentication requests and validates signed responses, issuer, audience, destination, expiry, and request correlation. It supports service-provider-initiated sign-in with HTTP Redirect or HTTP POST requests and HTTP POST responses. The provider NameID is the default stable identity key. Metadata URL certificates refresh hourly, with the last valid metadata retained during a temporary fetch failure.

See [.env.example](.env.example) and [kinosail.example.yaml](kinosail.example.yaml) for the complete setting names.

## Backups and recovery

The release installer configures daily authenticated encrypted backups with seven-file retention. Automatic backup creation fails closed without a key. Point `KINOSAIL_BACKUP_PATH` at a private directory on another disk or NAS for host-loss recovery, and keep an independent copy of both the backups and key.

Backups contain portable UI-managed configuration, credential hashes, sessions, playback state, and retained activity. They do not contain externally managed YAML, environment or secret files, Library Content, or reproducible transcode cache data. Protect those deployment files separately.

```sh
podman compose --file compose.release.yaml run --rm --no-deps kinosail backup > kinosail-backup.tar.gz
podman compose --file compose.release.yaml stop kinosail
podman compose --file compose.release.yaml run --rm --no-deps --no-tty kinosail restore < kinosail-backup.tar.gz
podman compose --file compose.release.yaml up --detach
```

Restore validates the manifest and every entry before writing. It rejects unknown paths, malformed or duplicate state, and oversized entries.

## Try the synthetic test Server

The public test fixture creates original synthetic movies, Shows, music, an audiobook, a book, photos, and a local TMDB-compatible catalogue without downloading third-party creative media.

```sh
./scripts/test-instance.sh up
./scripts/test-instance.sh verify
# Optional after `make bootstrap`:
./scripts/test-instance.sh browser
./scripts/test-instance.sh down --volumes
```

Open `https://localhost:38127` and sign in as `Owner` with password `test-instance-password`; obtain the rotating test code with `./scripts/test-instance.sh totp`. The fixture remains bound to localhost.

## Development and verification

Kinosail's authoritative repository gate is:

```sh
make check
```

Container, browser, full browser-engine, and populated-fixture checks cover separate boundaries:

```sh
make container-test
make browser-test
KINOSAIL_BROWSER_MATRIX=full make browser-test
make test-instance-check
```

Run the isolated playback regression harness when player behavior changes:

```sh
make playback-test
```

It tests three browser engines, repeated bandwidth changes, offline recovery, responsive layouts, and accessibility. Failure traces and screenshots remain under `.kinosail-test/playback/playback-results/`.

See the [release checklist](engineering/release-checklist.md) for the verification required before publishing. Physical devices, real GPUs, public DNS/TLS, router reachability, and off-LAN networks remain explicit external certification steps.

## Privacy, license, and contributing

Kinosail is complete for local use without a hosted account or subscription. Library Content, Profile policy, credentials, and Viewing Activity stay on the owner-hosted Server. Optional DNS updates, metadata, identity, or notification integrations receive only the requests needed for the Owner-enabled integration; no Kinosail-operated service relays media.

Kinosail Server is source-available under the [PolyForm Perimeter License 1.0.1](LICENSE), not an OSI-approved open-source license. Read [LICENSING.md](LICENSING.md) for Server, third-party, contribution, and trademark boundaries. Supporters who explicitly choose public recognition appear in [SUPPORTERS.md](SUPPORTERS.md); anonymous support remains private.

Use GitHub Issues for non-sensitive bugs and feature requests. Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md). Contributions follow [CONTRIBUTING.md](CONTRIBUTING.md) and the applicable contributor agreement.
