# Home Assistant integration direction

Research snapshot: **2026-08-27 (America/Denver)**.

This note uses **Home Assistant** for the product sometimes called HASS.

## Outcome

Kinosail should ship a native, local Home Assistant integration. It should follow the proven Plex and Jellyfin shape:

- expose Kinosail as one service and Home Assistant device;
- expose the Kinosail library as a searchable media source; and
- expose each stable, controllable playback client as a `media_player` entity.

The integration should use Kinosail's versioned `/api/v1` contract. It should not depend on Kinosail Cloud, MQTT, DLNA, or an embedded web page.

Home Assistant integration must be disabled by default. One Owner-only setting should control discovery and connection availability.

The first useful release needs work on both sides. Home Assistant needs a Python integration. Kinosail needs pairing, discovery, live client state, remote commands, events, and short-lived playback URLs.

The built-in Home Assistant cards can provide a polished first experience. A custom dashboard card should remain optional.

## Desired owner experience

The normal setup should contain these steps:

1. During first-launch **Devices** setup, Kinosail offers **Allow Home Assistant connections** as an optional, off-by-default choice.
2. The same control remains available under **Settings → Integrations** after setup.
3. After the owner enables it, Home Assistant discovers **Kinosail — _Server name_** on the local network.
4. The owner selects **Configure**.
5. Kinosail opens an approval page in the browser.
6. The owner selects the Viewer Profile and approves library, playback-state, streaming, and control access.
7. Home Assistant returns to a success screen. It does not store a Kinosail password.
8. Kinosail appears as one device. Its connected clients appear as media players.
9. **Browse media** shows real artwork, Continue Watching, Movies, Shows, Music, Collections, and search.
10. Standard Home Assistant cards show current artwork, title, position, and only the controls that the client supports.

Manual URL setup must remain available. Containers, virtual local area networks, and segmented networks can block multicast discovery.

Kinosail can also offer an **Add to Home Assistant** link in Owner settings. My Home Assistant links can open a page in the user's instance without sending that instance address to the redirect service ([My Home Assistant](https://www.home-assistant.io/integrations/my)).

## Exposure boundary

Use two explicit gates.

The first gate is the Owner setting. While it is off, Kinosail must:

- publish no `_kinosail._tcp.local.` record;
- reject new Home Assistant pairing requests;
- keep Home Assistant event, command, and playback-capability operations unavailable; and
- start no Home Assistant-specific background work.

Enabling the setting exposes only public discovery facts and the pairing start. It does not expose library, profile, or playback data.

The second gate is the Owner-approved grant. Only that narrow grant can read the selected Viewer Profile's library and playback state or control approved clients.

Disabling the setting must withdraw discovery, revoke all Home Assistant grants, close event streams, and invalidate outstanding Home Assistant playback capabilities. Label the action **Disable and revoke** and explain that Home Assistant must pair again before it can reconnect.

The first-launch wizard should place this choice in the existing **Step 2 of 4 · Devices** screen, after the trusted-address choice. Do not add a fifth step. Use one short privacy statement: Home Assistant sees nothing until the Owner enables access and approves a connection. Skipping the choice must preserve the disabled state and continue setup.

## Why this architecture fits Home Assistant

Home Assistant's Jellyfin integration exposes a library as a media source. It creates media players for connected sessions ([Jellyfin integration](https://www.home-assistant.io/integrations/jellyfin/)). Plex also creates media players for active clients and supports browse and playback ([Plex integration](https://www.home-assistant.io/integrations/plex)).

A Kinosail server is not itself a player. It should be a service-type integration and device. Actual playback clients should be media players.

Use one config entry for each Kinosail server. Use a stable server identifier as its unique ID. Permit multiple servers.

Use `ConfigEntry.runtime_data` for the API client and coordinator. Entity properties must only read cached state. Home Assistant forbids network input/output from entity properties ([entity model](https://developers.home-assistant.io/docs/core/entity/)).

Use a push coordinator after Kinosail has an event stream. Push reduces unnecessary requests and provides immediate changes. Until then, use one bounded `DataUpdateCoordinator` poll for all state ([fetching data](https://developers.home-assistant.io/docs/integration_fetching_data/)).

The target manifest should declare `integration_type: service`. It should declare `iot_class: local_push` only after push is real. Before that point, it is local polling.

This remains compatible with Kinosail's one-container boundary. The Home Assistant component runs inside Home Assistant. It is not a Kinosail sidecar ([Kinosail API-container ADR](../adr/0005-expose-complete-api-from-one-server-container.md)).

## Current Kinosail foundation

Kinosail already has most catalog and stream primitives:

- `/api/v1/library` supports bounded search, views, sorting, pagination, and title buckets;
- `/api/v1/items/{id}` returns viewer-filtered item data;
- `/api/v1/items/{id}/playback` returns direct and compatible paths, MIME information, position, chapters, tracks, markers, and the next item;
- `/api/v1/shows`, `/api/v1/albums`, playlists, and collections provide useful browse roots;
- artwork uses opaque item identifiers under `/art/{id}` and `/backdrop/{id}`; and
- bearer API keys already have explicit `library`, `write`, `stream`, `download`, and `admin` scopes.

See the current route and scope definitions in [`api_keys.go`](../../internal/server/api_keys.go), [`api_product.go`](../../internal/server/api_product.go), [`api_browse.go`](../../internal/server/api_browse.go), and [`api_media.go`](../../internal/server/api_media.go).

Current API keys are not yet an ideal Home Assistant credential. Normal keys expire after 30 days. Home Assistant needs a durable, revocable grant with reauthentication.

Kinosail also lacks the main control seam. It records playback progress, but it does not publish a general live-client snapshot or a remote-control channel.

The existing Watch Together socket is room-specific. It is not a server-wide device event interface. Kinosail explicitly does not support Jellyfin WebSocket remote-session control today ([compatibility matrix](../compatibility.md)).

## Stock Jellyfin integration as an interim route

The stock Jellyfin integration is useful as a protocol probe, but it is not a seamless interim solution.

Home Assistant's integration expects Jellyfin login, a session list, per-session state, and remote commands. Its source creates media players from current session identifiers and sends pause, play, stop, seek, volume, queue, browse, and search operations ([Jellyfin media-player source](https://github.com/home-assistant/core/blob/dev/homeassistant/components/jellyfin/media_player.py)).

Kinosail implements password and Quick Connect login, item browsing, playback information, streaming, and progress calls. It does not implement `GET /Sessions`. It accepts capability reports, but it has no compatible remote-session command surface. See [`jellyfin.go`](../../internal/server/jellyfin.go) and [`jellyfin_playback.go`](../../internal/server/jellyfin_playback.go).

Therefore, the stock integration cannot create or control useful Kinosail player entities today. Adding only `GET /Sessions` would still leave stale polling, remote commands, discovery, and identity behavior unresolved.

The stock route also asks Home Assistant for a Kinosail username and password. A native approval grant is safer and smoother. It also avoids expanding the compatibility adapter around Home Assistant-specific behavior.

A small compatibility experiment remains valuable. Run the stock Jellyfin integration against a disposable Kinosail profile. Record every request and failure. Do not treat that experiment as the target design.

## Kinosail API work

### Owner-controlled enablement

Add one shared application operation for the Home Assistant enabled state. The Owner settings adapter, onboarding adapter, and versioned API must call the same operation.

Use a configuration key such as `integrations.home_assistant.enabled`, defaulting to `false`. Environment or YAML ownership must remain visible and read-only in the browser, consistent with other Kinosail integration settings.

The disable transition must revoke grants and capabilities before it reports success. Invalid, conflicting, or externally managed changes must cause no partial side effects.

### Stable integration snapshot

Add one native, versioned snapshot operation. It should return:

- stable server ID, name, version, and capabilities;
- canonical local origin;
- visible client devices and their current playback state;
- library counts needed for optional diagnostics; and
- one monotonic event revision.

Reuse the persistent Jellyfin server identifier if it is a true installation identity. Otherwise, add one native installation ID and persist it.

### Stable client identity

Each Kinosail client needs a stable, opaque client ID. Do not key Home Assistant entities only to an authentication session or playback session.

The client record should include:

- user-safe client name and type;
- connected or disconnected state;
- Viewer Profile visibility allowed by the grant;
- supported command flags;
- current media ID and type;
- `playing`, `paused`, `buffering`, or `idle` state;
- title, series, season, episode, artist, and album;
- duration, position, and position update time; and
- an opaque artwork ID.

Client capabilities must control the advertised Home Assistant features. Do not display volume, queue, next, or previous controls when the client cannot perform them.

### Client command channel

The bundled player and supported Kinosail clients need a bidirectional control connection. A server command should target the stable client ID.

Support these operations when the client declares them:

- play or resume;
- pause;
- stop;
- seek;
- next and previous;
- play one selected media item; and
- queue as `add`, `next`, `play`, or `replace`.

The server must reject commands for missing, disconnected, unauthorized, or incapable clients. Rejected commands must not change playback state.

The Home Assistant `media_player` model defines these states and features. It also defines queue behavior for `play_media` ([media-player entity](https://developers.home-assistant.io/docs/core/entity/media-player/)).

### Push events

Add an authenticated `/api/v1` WebSocket or server-sent event stream. Use one initial snapshot and ordered changes.

Each event needs:

- revision;
- event type;
- stable client or server identifier;
- complete state for the changed object; and
- no secret, path, or unrestricted URL.

The stream needs bounded frames, keepalive, cancellation, reconnect backoff, and a replay cursor. If replay is impossible, it must require a full snapshot.

Home Assistant subscribes push entities during `async_added_to_hass` and unsubscribes on removal ([fetching data](https://developers.home-assistant.io/docs/integration_fetching_data/#push-vs-poll)).

### Hierarchical browse and search

The current API can supply much of the tree. The native contract should make the hierarchy explicit and consistent.

Start with:

- Continue Watching;
- Movies;
- Shows, seasons, and Episodes;
- Artists, albums, and tracks;
- Collections;
- Playlists; and
- Recently Added.

Each result needs an opaque ID, title, media class, MIME type, artwork ID, `can_play`, and `can_expand`.

Home Assistant media sources use `media-source://domain/identifier`. They support hierarchy, search, and resolution to a URL and MIME type ([media-source platform](https://developers.home-assistant.io/docs/core/platform/media_source/)).

### Short-lived playback capabilities

Do not put the Home Assistant bearer credential in artwork or playback URLs.

Add a resolver that returns a short-lived, item-scoped playback capability. Bind it to:

- one media item;
- an allowed direct or compatible representation;
- a short expiration;
- allowed HTTP methods;
- range access; and
- the approved Viewer policy.

The target speaker, television, or Cast device often fetches the URL itself. It cannot attach Home Assistant's Kinosail authorization header.

Use the target media-player identity when it helps choose direct audio, direct video, or compatible output. Home Assistant passes the target entity to media-source resolution and accepts absolute or Home Assistant-relative URLs ([media-source resolution](https://developers.home-assistant.io/docs/core/platform/media_source/#resolving-media)).

Prefer a direct device-to-Kinosail path. A Home Assistant proxy can be a fallback for compatible local devices, but it moves media through Home Assistant.

Capability URLs must fail closed after expiry or Viewer policy revocation. They must not disclose filesystem paths or general credentials.

### Artwork proxy

Home Assistant should proxy Kinosail artwork only when the dashboard request is outside the local network. This preserves mobile dashboard rendering.

Pass an opaque Kinosail artwork ID to the proxy. Never pass an arbitrary URL as `media_image_id`. Home Assistant calls out that pattern as a server-side request forgery risk ([album-art proxy](https://developers.home-assistant.io/docs/core/entity/media-player/#proxy-album-art-for-media-browser)).

## Authentication and authorization

The preferred setup is an explicit Kinosail approval grant. Do not store a Kinosail password in Home Assistant.

Reuse the security properties of Kinosail's existing OAuth approval implementation where practical:

- browser approval;
- Proof Key for Code Exchange;
- short-lived access tokens;
- renewable grants;
- revocation; and
- one-time authorization codes.

Do not reuse MCP scope names or make MCP grants valid on `/api/v1`. Generalize the grant layer or add a narrow Home Assistant audience.

Suggested capabilities are:

- library read;
- stream resolution;
- selected-profile playback-state read;
- optional household playback-state read; and
- client playback control.

Do not grant `admin` by default. A future scan button or administrative action requires separate Owner approval.

The approval page must show the Home Assistant instance, selected Viewer Profile, visible playback scope, and exact abilities. Household playback reveals personal viewing activity and needs explicit approval.

Revoking the Kinosail grant must return `401`. The integration should then start Home Assistant reauthentication. Home Assistant distinguishes temporary setup failures from invalid credentials through `ConfigEntryNotReady` and `ConfigEntryAuthFailed` ([setup failures](https://developers.home-assistant.io/docs/integration_setup_failures/)).

Removing the Home Assistant config entry should revoke its Kinosail grant when Kinosail is reachable. Unloading must close streams and remove listeners.

## Discovery and configuration

Kinosail should advertise a specific mDNS service such as `_kinosail._tcp.local.`. TXT data should contain only public discovery facts:

- stable server ID;
- server name;
- API version;
- port;
- TLS mode; and
- minimal capability version.

Do not advertise credentials, media facts, Viewer names, or library names.

Advertise this service only while the Owner setting is enabled. Withdraw it immediately when the setting is disabled.

The Home Assistant manifest can match this service. Discovery should set the stable server ID as the flow unique ID. Home Assistant requires unique IDs for mDNS discovery and uses them to prevent duplicate entries ([config-flow discovery](https://developers.home-assistant.io/docs/core/integration/config_flow/), [manifest discovery](https://developers.home-assistant.io/docs/creating_integration_manifest/#zeroconf)).

Discovery must always require user confirmation. It must update saved connection information when the same server moves to a new local address.

The UI config flow needs:

- discovery confirmation;
- manual full URL input;
- connection and API-version validation;
- pairing approval;
- duplicate-server rejection;
- translated error messages;
- reauthentication; and
- host reconfiguration.

Home Assistant requires service integrations to use a UI config flow. YAML is not the normal setup route ([config flow](https://developers.home-assistant.io/docs/core/integration/config_flow/), [YAML policy](https://developers.home-assistant.io/docs/core/integration/yaml_configuration/)).

## Local networking and TLS

The default connection should be local and direct. Do not require public remote access or a Kinosail-operated relay.

Home Assistant and every target playback device must be able to route to the resolved Kinosail origin. This is a separate requirement from Home Assistant reaching Kinosail.

Verify TLS by default. The best path is Kinosail's trusted LAN HTTPS origin. It gives Home Assistant, browsers, Cast devices, and televisions a common certificate story.

Kinosail's generated private certificate authority can work only after Home Assistant trusts it. Trust in Home Assistant does not make a television trust it.

A manual **Verify certificate** option can serve expert installations. Disabling validation must be explicit and must not be the default. It also does not solve trust for target playback devices.

Plain local HTTP would simplify device access, but it exposes bearer or capability URLs to the local network. Do not recommend it as the polished path.

Store the canonical hostname, not only a current IP address. Validate the exact certificate name. Accept discovery address changes only for the same stable server ID.

Kinosail currently rejects API-key access on the public listener. Preserve that boundary. Home Assistant should use LAN, a user-managed virtual private network, or WireGuard.

## Home Assistant entity model

### Primary entities

Create one `media_player` entity for each stable, controllable Kinosail client.

Map Kinosail state to:

- `PLAYING`;
- `PAUSED`;
- `BUFFERING`;
- `IDLE`; and
- unavailable when the client or server cannot be reached.

Expose standard media properties for title, series, season, episode, artist, album, content ID, duration, position, update time, and artwork.

Expose only supported standard features:

- `PLAY`;
- `PAUSE`;
- `STOP`;
- `SEEK`;
- `PLAY_MEDIA`;
- `BROWSE_MEDIA`;
- `SEARCH_MEDIA`;
- `NEXT_TRACK`;
- `PREVIOUS_TRACK`; and
- `MEDIA_ENQUEUE`.

Create entities dynamically when a new stable client appears. Mark disconnected clients unavailable or idle according to real capability. Remove only confirmed stale clients.

### Related entities and actions

Keep the first release small.

Useful optional entities are:

- disabled-by-default diagnostic sensors for server version, library count, and last event;
- one update entity only if Kinosail exposes a supported update operation; and
- one scan button only after separate Owner authorization exists.

Do not expose Kinosail settings as many switches. Do not expose a fake server media player.

A `remote` entity can follow when Kinosail clients support navigation keys. The current Home Assistant Jellyfin integration offers this pattern for capable clients ([Jellyfin integration](https://www.home-assistant.io/integrations/jellyfin/)).

Use standard media-player actions for playback. Add Kinosail-specific actions only for operations without a standard action.

## Events, automations, and privacy

Standard entity state changes already drive automations. Typical examples are:

- dim lights when a player starts;
- restore lights when it stops;
- pause on a doorbell event; and
- start a selected Kinosail item on a room player.

Do not fire a duplicate raw event for every state change. Home Assistant already emits `state_changed` events for entities ([Home Assistant events](https://www.home-assistant.io/docs/configuration/events/)).

Viewing data leaves Kinosail's database when it becomes Home Assistant state. Home Assistant Recorder stores entity state changes and attributes by default ([Recorder](https://www.home-assistant.io/integrations/recorder)). Titles, episode names, artwork identifiers, and playback times can therefore enter Home Assistant backups and external Recorder databases.

The integration documentation must state this clearly. It should explain Recorder exclusions for owners who do not want playback history stored.

Minimize recorded data:

- expose only the current media facts required by standard `media_player` behavior;
- do not attach library, cast, path, token, or history lists as attributes;
- do not emit position state every second;
- rely on `media_position_updated_at` for smooth dashboard progress; and
- exclude integration-only noisy or sensitive attributes from Recorder where Home Assistant permits it.

Home Assistant warns that fast-changing extra attributes can grow its database quickly ([entity state guidance](https://developers.home-assistant.io/docs/core/entity/#entity-class-or-instance-attributes)).

## Dashboard experience

The first release does not need custom frontend code.

Use Kinosail artwork, correct states, accurate capabilities, translated names, and one coherent device page. These feed Home Assistant's built-in media control card ([media control card](https://www.home-assistant.io/dashboards/media-control/)).

Tile cards can show selected playback, volume, mute, shuffle, repeat, source, and sound-mode controls. Home Assistant derives defaults from entity capabilities ([tile media-player features](https://www.home-assistant.io/dashboards/features/#media-player-playback-controls)).

Provide documented dashboard examples for:

- one compact room player tile;
- one large current-media card; and
- one conditional card visible only while playing or paused.

Set the Kinosail server device's `configuration_url` to the local Owner settings page. Use official Kinosail brand assets.

Do not use an iframe as the primary integration. It duplicates Kinosail's interface instead of making media state native to automations and dashboards.

A later optional custom card can add Continue Watching shelves and a player picker. It should remain a separate HACS dashboard package. Built-in cards must remain fully supported.

## HACS and Home Assistant Core path

Use this distribution sequence:

1. Run a manual custom component against real Kinosail and Home Assistant instances.
2. Publish a separate public GitHub repository for the integration.
3. Release it as a HACS custom repository.
4. Apply for the HACS default list after stable releases and validation.
5. Submit a small Home Assistant Core integration after Kinosail is established and has users.

HACS only supports public GitHub repositories. It expects one integration under `custom_components/kinosail/`, a root `hacs.json`, a versioned manifest, documentation, issues, code owners, and brand assets ([HACS general requirements](https://hacs.xyz/docs/publish/start/), [HACS integration requirements](https://hacs.xyz/docs/publish/integration/)).

Do not place the HACS package inside the current Kinosail application repository. Use a separate repository so its structure, releases, dependencies, and review history match HACS expectations.

Home Assistant Core requires an established, available product or service. A new integration must reach Bronze quality and use a separate Python communication library. Its first pull request should be small and usually include one platform ([Core integration submission](https://developers.home-assistant.io/docs/core/integration/contributing_to_core/)).

Create a small asynchronous `aiokinosail` package before Core submission. It should accept Home Assistant's shared `aiohttp` session. Home Assistant recommends injected web sessions for efficient HTTP integrations ([web-session injection](https://developers.home-assistant.io/docs/core/integration-quality-scale/rules/inject-websession/)).

For a Core submission, start with the `media_player` platform and its browse support. Add a general `media_source.py`, diagnostics, remote, and optional entities in later pull requests.

## Verification requirements

### Kinosail tests

Add focused API and application tests for:

- stable server and client identity;
- mDNS record contents and shutdown;
- pairing approval, denial, expiry, replay, and revocation;
- every scope boundary;
- snapshot and event revision ordering;
- disconnect, reconnect, replay, and full reconciliation;
- client capability registration;
- every accepted command;
- unsupported, stale, missing, and unauthorized commands with no side effects;
- browse pagination, search, filters, and profile visibility;
- short-lived stream capability expiry, range handling, policy revocation, and method limits;
- artwork authorization and opaque identifiers;
- bounds for every new string, identifier, list, frame, and request; and
- logs, diagnostics, audit, and URLs that never expose tokens or local paths.

Run focused Go tests, the affected package suite, `make max-loc`, and `make verify-changed`. Add race coverage for the live client and event registries.

### Home Assistant tests

Use pytest and mocked API boundaries for:

- discovery and manual setup;
- duplicate server rejection;
- bad URL, bad certificate, invalid grant, and unsupported API version;
- pairing cancellation and expiry;
- reauthentication and host reconfiguration;
- initial unavailable state and recovery;
- push reconnect and snapshot reconciliation;
- clean unload and listener removal;
- dynamic client creation and stale-client handling;
- every media-player state and command;
- accurate supported-feature flags;
- browse hierarchy, search filters, pagination, and MIME type;
- invalid and oversized media identifiers;
- artwork proxy authorization and server-side request forgery rejection; and
- media capability expiry and revocation.

Home Assistant requires full config-flow coverage for Core. Silver requires more than 95 percent coverage for all integration modules ([quality rules](https://developers.home-assistant.io/docs/core/integration-quality-scale/rules/)).

Run `pytest`, coverage, Ruff, mypy, and Hassfest through the Home Assistant development workflow. Run HACS validation before every custom-integration release.

### End-to-end evidence

Run one real Kinosail container and one real Home Assistant instance. Verify:

- discovery on a normal local network;
- manual setup from a separate container network;
- trusted LAN HTTPS;
- explicit private-certificate trust;
- Home Assistant restart and Kinosail restart;
- grant revocation and reauthentication;
- browser, mobile dashboard, and external dashboard artwork;
- built-in media control and tile cards;
- play, pause, seek, stop, next, and item selection;
- direct media fetch by at least one Cast or television device; and
- Recorder contents and documented privacy exclusions.

Protocol tests cannot prove local routing, certificate trust, codecs, Cast behavior, or visual quality on physical devices.

## Phased delivery

### Phase 0 — protocol spike

- Run Home Assistant's stock Jellyfin integration against Kinosail.
- Confirm the expected failure around `GET /Sessions` and remote control.
- Capture the exact request sequence.
- Prototype one Home Assistant media source against the current `/api/v1` library.
- Prove one target device can fetch a short-lived Kinosail URL.

Exit condition: the team has evidence for the integration boundary and URL delivery model.

### Phase 1 — secure read-only integration

- Add the off-by-default Owner setting to **Settings → Integrations** and the existing **Devices** onboarding step.
- Add stable native server identity and mDNS discovery.
- Add browser-approved, revocable library and playback-state grants.
- Add a bounded snapshot endpoint.
- Ship a custom Home Assistant integration with UI setup, server device, media browsing, search, and artwork proxy.
- Use one coordinated poll if push is not ready.

Exit condition: setup needs no Kinosail password, browsing is polished, and privacy limits are documented.

### Phase 2 — live players and native controls

- Add stable client registration.
- Add the command channel and ordered event stream.
- Create dynamic `media_player` entities.
- Support play, pause, stop, seek, next, previous, and item play where declared.
- Validate built-in cards at desktop and mobile sizes.

Exit condition: automations react promptly and cards always match the actual player.

### Phase 3 — play Kinosail media on Home Assistant players

- Add short-lived, item-scoped playback capabilities.
- Resolve by target device and MIME type.
- Verify direct device reachability and trusted certificates.
- Add queue behavior.

Exit condition: a user can select Kinosail media in Home Assistant and play it on supported room devices without exposing a general token.

### Phase 4 — distribution and quality

- Publish the separate integration repository and releases.
- Add HACS and Hassfest validation.
- Reach the practical Silver rules, then Gold discovery and diagnostics.
- Extract and publish the async API library.
- Submit a small Core pull request when Kinosail meets Core eligibility.

Exit condition: installation and upgrades are reliable, documented, and maintainable outside one development machine.

## Decisions to keep

- Build a native integration. Do not make Jellyfin compatibility the long-term seam.
- Keep Home Assistant disabled by default. Require both Owner enablement and a narrow approved grant.
- Use local direct connections. Do not require Kinosail Cloud.
- Model clients as media players. Do not model the server as a fake player.
- Pair through browser approval. Do not store Owner passwords.
- Use short-lived item capabilities. Do not put API tokens in playback URLs.
- Use standard Home Assistant cards first. Do not require a custom card.
- Make viewing-history export explicit. Home Assistant Recorder changes Kinosail's privacy boundary.
- Start with a small custom integration. Treat HACS and Core as separate later publication stages.

## Primary sources

### Home Assistant

- [Jellyfin integration](https://www.home-assistant.io/integrations/jellyfin/)
- [Jellyfin media-player source](https://github.com/home-assistant/core/blob/dev/homeassistant/components/jellyfin/media_player.py)
- [Jellyfin media-source source](https://github.com/home-assistant/core/blob/dev/homeassistant/components/jellyfin/media_source.py)
- [Plex integration](https://www.home-assistant.io/integrations/plex)
- [Media-player entity](https://developers.home-assistant.io/docs/core/entity/media-player/)
- [Media-source platform](https://developers.home-assistant.io/docs/core/platform/media_source/)
- [Config flow](https://developers.home-assistant.io/docs/core/integration/config_flow/)
- [Integration manifest](https://developers.home-assistant.io/docs/creating_integration_manifest/)
- [Fetching data](https://developers.home-assistant.io/docs/integration_fetching_data/)
- [Handling setup failures](https://developers.home-assistant.io/docs/integration_setup_failures/)
- [Integration quality scale](https://developers.home-assistant.io/docs/core/integration-quality-scale/)
- [Core integration submission](https://developers.home-assistant.io/docs/core/integration/contributing_to_core/)
- [Recorder](https://www.home-assistant.io/integrations/recorder)
- [Media control card](https://www.home-assistant.io/dashboards/media-control/)
- [Card features](https://www.home-assistant.io/dashboards/features)
- [HACS integration requirements](https://hacs.xyz/docs/publish/integration/)

### Kinosail

- [`README.md`](../../README.md)
- [`DESIGN.md`](../../DESIGN.md)
- [`docs/compatibility.md`](../compatibility.md)
- [`docs/adr/0005-expose-complete-api-from-one-server-container.md`](../adr/0005-expose-complete-api-from-one-server-container.md)
- [`internal/server/api_keys.go`](../../internal/server/api_keys.go)
- [`internal/server/api_product.go`](../../internal/server/api_product.go)
- [`internal/server/api_browse.go`](../../internal/server/api_browse.go)
- [`internal/server/api_media.go`](../../internal/server/api_media.go)
- [`internal/server/jellyfin.go`](../../internal/server/jellyfin.go)
- [`internal/server/jellyfin_playback.go`](../../internal/server/jellyfin_playback.go)
