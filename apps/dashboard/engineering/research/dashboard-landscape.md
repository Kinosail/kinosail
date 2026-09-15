# Self-hosted dashboard landscape

Reviewed: 2026-08-30

## Decision summary

Kinosail Dashboard should not be a prettier bookmark grid. The strongest opening is a local-first network cockpit with one semantic board, safe direct manipulation, and an automation contract shared by the UI, HTTP API, and Model Context Protocol (MCP).

Homarr and Homepage already expose MCP servers. Basic natural-language app creation is now a parity feature, not a differentiator. Homarr exposes more than 50 tools over its application API, while Homepage can add services and widgets through an opt-in write mode. Kinosail should differentiate through safe previews, atomic changes, secret isolation, responsive intent, useful diagnostics, and reversible operations. [Homarr MCP](https://homarr.dev/docs/management/mcp/) [Homepage MCP](https://gethomepage.dev/configs/mcp/)

The recommended first release is a dashboard, not an application installer or reverse proxy. It can discover and describe existing services without taking ownership of their containers or network path.

## Comparative matrix

| Product | Setup and app model | Board and editing model | Network visibility | Authentication | Automation |
| --- | --- | --- | --- | --- | --- |
| Homarr | Container deployment. Apps, integrations, widgets, boards, and Docker endpoints are first-class records. Docker discovery suggests apps and integrations for review. | Direct edit mode with drag, resize, containers, rails, multi-select, and protected Mobile/Base layouts. Positions and sizes are stored independently per layout. | App ping, integration status, container discovery, logs, resource data, and start/stop/restart/remove actions. Integration credentials remain server-side. | Local credentials, Lightweight Directory Access Protocol (LDAP), OpenID Connect (OIDC), users, groups, resource permissions, and API keys. | OpenAPI and tRPC APIs. MCP exposes 50+ read and write tools, including apps, boards, integrations, Docker, media, DNS, and smart-home actions. |
| Homepage | Docker, Kubernetes, source, Unraid, YAML files, Docker labels, and Kubernetes annotations. It advertises more than 100 service integrations. | Configuration-driven responsive groups. There is no documented visual board editor. Layout comes from YAML, discovery metadata, and settings. | Server-side service widgets, site checks, container health, and CPU, memory, and network statistics. Docker label discovery can add services automatically. | Simple password or OIDC gate. Official guidance still recommends a reverse proxy with authentication and TLS, or a virtual private network (VPN). | A purpose-built MCP endpoint is off and read-only by default. Opt-in writes can replace config files or add services and information widgets. |
| Dashy | Docker or static deployment. A YAML configuration defines pages, sections, items, and widgets. | YAML, a validated JSON editor, and a live visual editor. Responsive auto and masonry grids are available. | Server-side status and ping checks. Widgets cover hosts, network interfaces, traffic, metrics, and many service APIs. | Built-in auth plus optional server enforcement, header auth, Keycloak, and OIDC. Documentation warns against built-in auth as the only public perimeter. | Optional REST API can read and write whole files or nested config paths. It has schema validation and backups, but writes are last-write-wins. No MCP is documented. |
| Heimdall | A focused application launcher. Foundation apps provide known icons and colors. Enhanced apps connect to service APIs for live tile data. | Simple tile management and tags. The project emphasizes a start page instead of a composable operations board. | Enhanced tiles show selected service data, such as download queue size and speed. Internal requests require an explicit setting because of server-side request forgery risk. | Local users exist. The project README does not document a current first-party OIDC flow. | No documented MCP or general management API was found in the reviewed first-party material. |
| Organizr | Docker, Windows, Linux, and web-server installs. Apps usually appear as categorized tabs, iframes, or new windows. | A settings-based tab editor supports ordering, categories, groups, preload, local URLs, and ping URLs. Homepage items and tabs use separate configuration paths. | Per-tab ping and separate local and external URLs. Organizr does not proxy tab content; browsers must reach each target. | Built-in users, Plex and LDAP backends, group levels, JSON Web Token cookies, and reverse-proxy authorization endpoints. | Its `/api/v2/auth` path supports proxy authorization. No MCP or broad dashboard-management API is documented. |
| Cosmos Cloud | A server management plane with an app market. Market installs can create containers, networks, volumes, and proxy routes. | Form-based app and route management. It is closer to a home-server platform than a flexible information board. | Container state, exposed routes and ports, CPU and memory, reverse proxy, certificate automation, DNS, and a Nebula-based mesh VPN. | Users, invitations, password reset, two-factor authentication reset, route authentication, OIDC provider support, and network restrictions. | No MCP was found in the reviewed first-party documentation. Its value is integrated infrastructure control, not a public dashboard automation surface. |
| Umbrel | A complete home-server operating system. Its store provides more than 300 containerized apps with one-click installation and browser-first setup. | An operating-system home screen with app shortcuts. It is not a general widget board. | Installed-app lifecycle and an app proxy. Remote access currently uses built-in Tor or an app such as Tailscale. | An Umbrel account protects the home screen. The app proxy protects most installed applications with the same login. | An internal RPC server supports app installation. No public MCP contract is documented. |
| CasaOS | A personal-cloud management platform with a Docker app store, custom stores, file management, and system widgets. | A friendly app grid with drag ordering and widgets. It is an operating environment rather than a multi-purpose board. | App state and host resource widgets. Its backend contains versioned HTTP APIs and separate management services. | Built-in account model. The reviewed project overview does not present the identity features as a dashboard differentiator. | Versioned HTTP APIs exist. No MCP surface was found in the reviewed first-party material. |

“No MCP found” means that the reviewed official documentation and repositories did not document one. It is not proof that no private or experimental endpoint exists.

## Evidence by product

### Homarr

- Homarr has the strongest direct-edit model in this group. Its official board guide describes static view mode, deferred editor loading, collision-aware moves, resize handles, multi-select, undo for deletion, containers, and fixed rails. [Boards](https://homarr.dev/docs/management/boards/)
- A board contains separate Mobile and Base layouts. Each layout stores independent positions and sizes. This solves precision but creates repeated work and possible layout drift. [Boards](https://v2.preview.homarr.dev/docs/management/boards/)
- Docker discovery is read-only and asks the user to confirm app or integration creation. It continues with healthy endpoints when one endpoint fails. [Docker integration](https://v2.preview.homarr.dev/docs/integrations/docker/)
- Integration requests originate from the Homarr host. Secrets are not sent to the browser. Permissions distinguish use, interaction, and full control. [Integrations](https://homarr.dev/docs/management/integrations/)
- The MCP server inherits the API-key owner's permissions. It can manage apps, boards, integrations, Docker containers, media servers, DNS, downloads, and smart-home actions. [MCP](https://homarr.dev/docs/management/mcp/)
- The OpenAPI and MCP surfaces include board creation, duplication, visibility, settings, and home-board automation. [API](https://homarr.dev/docs/management/api/)

### Homepage

- Homepage supports Docker, Kubernetes, source, and Unraid deployment. Its service catalog documents more than 100 integrations. [Installation](https://gethomepage.dev/installation/) [Service widgets](https://gethomepage.dev/widgets/services/)
- Docker labels can discover services and widgets. Direct Docker-socket use is discouraged and normally requires root access. [Docker integration](https://gethomepage.dev/configs/docker/) [Docker installation](https://gethomepage.dev/installation/docker/)
- Service cards can combine site monitoring, container statistics, and application integrations. [Services](https://gethomepage.dev/configs/services/)
- Homepage's MCP endpoint is disabled by default and read-only by default. Write mode can add a service or information widget. Its documentation warns that the token can read configured service credentials and, with writes enabled, control browser-executed `custom.js`. [MCP](https://gethomepage.dev/configs/mcp/)
- The core authoring model remains YAML and discovery labels. This makes infrastructure-as-code easy, but it leaves room for a strong browser editor. [Homepage](https://gethomepage.dev/)

### Dashy

- Dashy offers YAML, a schema-aware JSON editor, a visual editor with live preview, and a REST API. [Configuration](https://dashy.to/docs/configuring/)
- Its responsive layouts include automatic grid and masonry modes. [Configuration](https://dashy.to/docs/configuring/)
- Status checks and ping checks run from the Dashy server and can poll on an interval. Accessibility mode uses both shapes and colors for state. [Status indicators](https://dashy.to/docs/status-indicators/)
- The REST API can add items and update nested configuration. Writes create backups and validate the main schema, but concurrent writes use last-write-wins behavior. [REST API](https://dashy.to/docs/api/)
- Authentication ranges from local accounts to header auth, Keycloak, and OIDC. The official guide recommends a stronger perimeter for public deployments. [Authentication](https://dashy.to/docs/authentication/)

### Heimdall

- Heimdall is intentionally narrow: links, search, and application tiles. Foundation apps add known presentation data. Enhanced apps add live API data. [Project README](https://github.com/linuxserver/Heimdall)
- Its official repository warns that internal IP requests can create server-side request forgery exposure on public instances. [Project README](https://github.com/linuxserver/Heimdall)
- The separate first-party app repository supplies the application definitions. [Heimdall Apps](https://github.com/linuxserver/Heimdall-Apps/)

### Organizr

- Organizr centers tabs. Tabs can load an iframe or new window and can use different local and public URLs. Organizr does not proxy tab content. [Tab management](https://docs.organizr.app/tab-management)
- The homepage has service-specific modules, while homepage items and tabs are configured separately. [Homepage](https://docs.organizr.app/features/homepage)
- It supports built-in users and secondary authentication backends, including Plex and LDAP. [Authentication backends](https://docs.organizr.app/features/authentication-backends) [LDAP](https://docs.organizr.app/features/authentication-backends/ldap-backend)
- Reverse proxies can authorize requests through Organizr's API or validate its signed user cookie. [Server authentication](https://docs.organizr.app/features/server-authentication)

### Cosmos Cloud

- Cosmos Market can create containers, networks, volumes, links, and reverse-proxy routes from reviewed manifests. [Market](https://cosmos-cloud.io/docs/market/)
- The service manager exposes container state, public-port warnings, URLs, auto-update, CPU, and memory. [ServApps](https://cosmos-cloud.io/docs/servapps/)
- Its built-in reverse proxy supports automatic HTTPS, single sign-on, route authentication, IP restrictions, and network-only routes. [URLs](https://cosmos-cloud.io/docs/urls/)
- Constellation adds multi-server mesh networking, DNS, device ownership, and private or tunneled routes. [Constellation VPN](https://cosmos-cloud.io/docs/constellation-vpn/)

### Umbrel

- Umbrel presents a polished home-server operating system with more than 300 apps, not only a dashboard. [umbrelOS](https://github.com/getumbrel/umbrel)
- App packages use Docker Compose and a manifest. Dependencies and lifecycle hooks are part of the app framework. [App framework](https://github.com/getumbrel/umbrel-apps)
- The framework requires a useful browser experience and avoids SSH as the default user path. It also exposes an internal RPC installation command. [App framework](https://github.com/getumbrel/umbrel-apps)
- Most apps can sit behind the Umbrel app proxy. Current remote-access guidance points users to Tailscale or built-in Tor. [Installing an app](https://umbrel.com/support/apps/how-to-install-an-app) [Remote access](https://umbrel.com/support/basics/remote-access)

### CasaOS

- CasaOS combines a home-oriented interface, one-click apps, Docker app installation, file management, and resource widgets. [Project README](https://github.com/IceWhaleTech/CasaOS)
- Its app-store protocol uses Compose plus `x-casaos` metadata and produces static, localized store catalogs. [Store protocol](https://github.com/IceWhaleTech/CasaOS-AppStore/blob/main/docs/specs/overview.md)
- The backend embeds an OpenAPI document and serves versioned HTTP routers. [Backend entrypoint](https://github.com/IceWhaleTech/CasaOS/blob/main/main.go)

## Product opportunities for Kinosail Dashboard

### 1. One semantic board, not two manual canvases

Store intent instead of separate desktop and mobile coordinates:

- Each item has order, importance, minimum span, preferred span, and supported display modes.
- The layout engine derives desktop, tablet, compact, and 320-pixel layouts from that intent.
- Users can pin a deliberate exception at a breakpoint, but the default never requires duplicate editing.
- Containers use container queries, not global viewport assumptions.
- A phone preview is always available beside the desktop editor.

This combines Homepage's low-maintenance responsive flow with Homarr's visual control. It avoids layout drift between independent mobile and desktop canvases.

### 2. Deliberate edit mode for mouse, keyboard, and touch

Use a visible Edit board action. Do not let ordinary scrolling rearrange content.

- Drag handles are explicit and large enough for touch.
- Keyboard users can move and resize items with commands and announce results through a live region.
- An inspector edits the selected item without hiding the board.
- Undo and redo cover the complete draft session.
- Autosave stores a private draft. Publish applies one atomic revision.
- Concurrent edits use a revision token and show a merge choice instead of last-write-wins.
- Desktop, tablet, and phone previews share the same pending draft.

### 3. Service cards that explain the network

Most competitors show only “up” or “down.” Kinosail can show the path that matters:

- Browser URL: the address the current device will open.
- Server URL: the private address used for health and integration calls.
- Reachability: browser, Kinosail Server, or both.
- Transport: local network, private VPN, or public HTTPS.
- Health evidence: last check, latency, response class, certificate expiry, and error category.
- Runtime evidence: discovered container identity, image, state, health, and resource use.
- Ownership: “linked only” versus “managed by another platform.”

Keep Kinosail out of the media and application data path. It should observe direct connections and open the target. It should not relay application traffic.

### 4. MCP as a safe application interface

Expose the same versioned application operations to the browser, HTTP API, and MCP. Do not make MCP a config-file editor.

Initial read tools:

- `dashboard_get_board`
- `dashboard_list_apps`
- `dashboard_discover_apps`
- `dashboard_search_catalog`
- `dashboard_test_connection`
- `dashboard_explain_reachability`
- `dashboard_preview_change`

Initial write tools:

- `dashboard_add_app`
- `dashboard_update_app`
- `dashboard_remove_app`
- `dashboard_place_app`
- `dashboard_add_integration`
- `dashboard_publish_draft`
- `dashboard_undo_revision`

Every mutation should:

1. Validate and normalize at the shared application boundary.
2. Accept an expected board revision.
3. Return a human-readable preview before risky effects.
4. Apply atomically.
5. Record an actor, source, timestamp, and redacted change summary.
6. Return the new revision and a reversible operation identifier.

MCP credentials should carry narrow scopes such as `apps:read`, `apps:write`, `layout:write`, `integrations:test`, and `runtime:operate`. Read tools must never return integration secrets. Secret setup should use a short-lived, same-user browser handoff.

### 5. Discovery without automatic ownership

Discover services from Docker or Podman first. Add Kubernetes and external catalogs later.

- Read labels, published ports, health, and image metadata.
- Match known integrations and icons locally.
- Explain ambiguous matches.
- Let the user review the proposed name, browser URL, server URL, icon, integration, and placement.
- Never mutate or restart a discovered container during dashboard setup.
- Track the discovery source so stale suggestions can be reconciled.

This retains Homarr's review step while giving the MCP enough structured context to complete setup safely.

### 6. A small, coherent integration system

Do not start by cloning a catalog of hundreds of fragile widgets. Start with reusable capabilities:

- HTTP health and latency.
- Container state and resource use.
- JSON metric extraction with strict response limits.
- Media activity.
- Download activity.
- Storage and host capacity.
- Calendar and recent events.

Each adapter should declare inputs, secret fields, permissions, request limits, cache policy, supported card modes, empty and error states, and test fixtures. One integration can then power a compact tile, expanded card, detail sheet, search action, and MCP query.

### 7. Household-ready access

Use Kinosail's existing household model rather than an administrator-only homelab model:

- One owner completes initial setup without a log-derived code.
- Viewer Profiles receive curated views of the same board.
- An app can be visible, launchable, or operable as separate permissions.
- Public anonymous boards remain an explicit opt-in.
- Local accounts work without an identity provider. OIDC can follow after the core household flow.

### 8. Calm operational design

The main board should answer three questions quickly:

1. What can I open?
2. What needs attention?
3. What changed?

Use a stable app field, restrained live data, and an attention tray. Avoid a wall of constantly animated metrics. Expand a card for detail instead of making every tile a miniature monitoring console.

## Recommended first release boundary

Ship these capabilities together:

- One household board with derived responsive layouts.
- App links, sections, search, favorites, and compact or expanded cards.
- Direct edit mode with keyboard and touch support, preview, undo, draft, and publish.
- Docker and Podman discovery with review before add.
- HTTP status, container health, resource use, and clear reachability diagnostics.
- Local owner authentication and Viewer Profile visibility rules.
- Versioned HTTP API for every operation.
- Scoped MCP tools for discovery, preview, add, update, place, test, publish, and undo.
- An audit timeline for UI, API, and MCP changes.
- Backup and restore of board state without exporting plaintext secrets.

Defer container installation, reverse proxy, VPN, DNS, and arbitrary browser JavaScript. Cosmos, Umbrel, and CasaOS show how quickly those features turn a dashboard into an operating platform. Kinosail Dashboard can integrate with those platforms without owning their risk.

## Adversarial product checks

- Can an MCP read token reveal an integration credential? The answer must be no.
- Can a stale MCP client overwrite a newer board? The revision check must reject it.
- Can a dragged item move when the user intended to scroll? Only explicit edit mode can move it.
- Can one inaccessible integration delay the whole board? Each card needs an independent timeout and stale-data state.
- Can a server URL make a request to metadata, loopback, or an unexpected private network? Apply strict server-side request forgery controls and explicit network policy.
- Can a discovered container become managed accidentally? Discovery must remain read-only until a separate authorized operation.
- Can the mobile board silently lose important apps? Responsive derivation must preserve order and visibility unless the user sets a rule.
- Can a custom metric return unbounded data? Limit response size, depth, fields, redirects, duration, and refresh rate.
- Can a Viewer Profile infer hidden app names through search, errors, icons, or MCP? Apply visibility before query, rendering, and indexing.
- Can rollback restore old secrets? Revisions should reference secret records, never copy secret values into history.

## Conclusion

Homarr sets the direct-edit and broad-MCP benchmark. Homepage sets the configuration, discovery, and integration breadth benchmark. Cosmos, Umbrel, and CasaOS show the appeal and risk of becoming a full server control plane.

Kinosail Dashboard can lead with a narrower promise: one beautiful board that understands the local network, works naturally on every screen, and is equally safe to edit by a person or an agent.
