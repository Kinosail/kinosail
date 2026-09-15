# Kinosail Dashboard architecture and security

Status: Proposed
Source snapshot: Kinosail `925f488ef6005a154d8ccc4f6c236b9f45ecbb62`

## Decision

Build Kinosail Dashboard as one Go executable and one container. Keep one canonical board in embedded SQLite. Render that board for every screen size. Do not store separate desktop and mobile layouts.

Use server-rendered HTML for the first view. Use small embedded JavaScript modules for edit mode, Pointer Events, keyboard moves, conflict handling, and live status. Keep every capability in the versioned HTTP API. Make the web and Model Context Protocol (MCP) adapters call the same board operation.

The first release should provide:

- One board with ordered sections and application cards.
- Local embedded icons and generated monograms.
- Owner editing and Viewer Profile visibility.
- Atomic add, update, move, resize, hide, and remove changes.
- Unauthenticated same-origin health checks.
- Search, keyboard launch, and recent application shortcuts.
- Host STDIO MCP and scoped remote MCP.
- A single hardened container with no Docker socket.

Do not add arbitrary widgets, remote icon downloads, custom JavaScript, iframes, LAN scans, Docker discovery, or app credentials in the first release. These features add large security surfaces before the board model is stable.

## Existing Kinosail patterns to preserve

Kinosail already has the correct outer shape:

- `server.New` is the composition root. It opens state, creates modules, registers HTML and versioned API routes, registers MCP, then applies shared middleware. [K1]
- Browser assets use `go:embed`. Versioned asset URLs receive immutable cache headers. [K2]
- The API uses Go 1.22 route patterns. Unknown methods receive `Allow`. JSON input has a 1 MiB limit, rejects unknown fields, and rejects trailing values. [K3]
- SQLite uses a CGO-free driver, an immediate transaction lock, Write-Ahead Logging (WAL), full synchronous writes, a busy timeout, schema checks, integrity checks, and owner-only permissions. [K4]
- Authentication resolves a Viewer Profile before authorization. Owner operations, API-key scopes, Multi-Factor Authentication (MFA), and remote restrictions sit in one protection path. [K5]
- Browser mutations receive security headers, origin checks, Cross-Site Request Forgery (CSRF) checks, and request-body limits. [K6]
- Outbound clients resolve names before dialing, reject mixed prohibited results, pin the selected address for the connection, set timeouts, and reject redirects. [K7]
- MCP uses explicit read, write, and manage scopes. It accepts only bounded relative `/api/v1` routes from an allowlist and records the MCP actor. [K8]
- Host STDIO MCP uses an owner-selected `0600` Unix socket and bounded concurrent connections. [K9]
- Secrets are split from public configuration, written as `0600`, and never returned through public configuration views. [K10]
- The container uses a non-root user. Compose uses a read-only root filesystem, drops all capabilities, limits process count, and enables `no-new-privileges`. [K11]

The Dashboard should copy these policies and tests. It should not copy the current `internal/server` size. The new application can start with deeper modules.

## Module map

```text
cmd/kinosail-dashboard
        |
        v
internal/server  ---- HTML adapter
        |        ---- /api/v1 adapter
        |        ---- MCP adapter
        v
internal/board        one deep mutation interface
   |      |   \
   |      |    --> internal/catalog
   |      --> internal/health
   --> internal/database

internal/identity     profiles, sessions, MFA, passkeys, audit
internal/configuration
internal/privatefile
```

### `internal/board`

This is the main deep module. Its external interface should stay small:

```go
type Module interface {
    Snapshot(context.Context, Actor) (Snapshot, error)
    Apply(context.Context, Actor, uint64, string, []Change) (Snapshot, error)
}
```

`Actor` contains the authenticated profile and Owner state. `Apply` accepts the expected revision, a mutation ID, and at most 50 changes. It validates all changes before the transaction. It then applies every change or none.

The implementation owns these rules:

- One board exists.
- A board contains no more than 32 sections and 256 applications.
- Names contain 1 to 64 characters.
- Descriptions contain no more than 160 characters.
- A launch URL contains no more than 2048 bytes.
- A launch URL contains no user information, query, or fragment.
- Each application appears exactly once.
- Every application refers to one existing section.
- Card size is `small` or `wide`.
- Visibility is `everyone`, `owners`, or a bounded profile list.
- Only an Owner can mutate the board.
- The expected revision must equal the current revision.
- A repeated mutation ID returns the original result.
- Reuse of a mutation ID with a different canonical payload fails.

Do not expose a persistence interface only to make SQLite replaceable. There is one production persistence adapter. Use package-private helpers and transaction tests.

### `internal/database`

Use normalized SQLite tables. The board has ordering and concurrency invariants that are clearer in rows than in one JSON document.

```text
board(id=1, title, revision, updated_at)
sections(id, title, position, created_at, updated_at)
apps(id, section_id, name, description, launch_url, icon_key,
     size, position, visibility, health_mode, health_path,
     created_at, updated_at)
app_profiles(app_id, profile_id)
board_mutations(actor_id, id, request_hash, resulting_revision,
                response_json, created_at)
```

Use strict tables, foreign keys, uniqueness constraints, and check constraints. Use `(actor_id, id)` as the mutation key. Reject reuse with a different request hash. Use one immediate transaction for `Apply`. Update `board.revision` with `WHERE revision = ?`. If no row changes, return a stale revision error. Keep the latest 1,024 mutation records or 30 days, whichever is smaller.

Open the database with WAL, `synchronous=FULL`, a five-second busy timeout, schema version checks, `quick_check`, and `0600` permissions. Do not write health samples on every poll. Keep current health in memory. A restart can show `unknown` until the first probe.

### `internal/catalog`

Embed a small reviewed application catalog. A catalog entry contains:

- Stable catalog ID.
- Display name.
- Local icon key.
- Search aliases.
- Optional same-origin health path.

The catalog must not contain credentials, remote icon URLs, executable templates, or arbitrary headers. A custom application receives a generated monogram.

The catalog owns each health path. A path contains no more than 256 bytes. It starts with one `/`, has no scheme, authority, query, fragment, backslash, or control character, and is bound to its catalog ID. `health_mode` is only `off` or `scheduled`. Custom applications remain `off`.

This module lets an MCP client translate “add Plex” into reviewed defaults without network discovery.

### `internal/health`

This module owns all server-side egress. Its narrow interface should be:

```go
type Module interface {
    Snapshot([]AppID) map[AppID]Status
    Test(context.Context, Actor, AppID) (Status, error)
}
```

Its implementation also runs the schedule. Use these limits:

- Health is disabled unless the Owner selects it.
- Adding an application never causes network access.
- A health URL uses the launch URL origin and a relative catalog path.
- Only `http` and `https` are valid.
- Only `GET` is sent. No custom body or user-defined header is sent.
- Redirects are rejected.
- DNS is checked on every connection. All answers must pass the local integration policy.
- Unspecified, multicast, link-local, loopback, and this Dashboard listener are rejected.
- The default policy allows private, Unique Local Address (ULA), and Carrier-Grade NAT ranges. Deployment configuration can narrow these CIDRs.
- Public targets require a deployment allowlist. The UI and MCP cannot change network policy.
- The transport ignores proxy environment variables and sets `Proxy` to `nil`.
- The selected address is pinned for the connection.
- System certificate validation remains enabled.
- Automatic response decompression is disabled.
- Response headers are limited to 16 KiB. At most 8 KiB of wire body is read.
- The total timeout is three seconds.
- One global semaphore limits scheduled and manual probes to eight total.
- One application has at most one active probe.
- Scheduled probes run no faster than once per minute, with jitter and failure backoff.
- The implementation closes the response after the bounded wire read.
- Results expose only state, latency, last check, and a coarse failure class.
- Each result carries the application ID, URL generation, and start time. Publish it only if all three still match.

Private, ULA, and Carrier-Grade NAT addresses are intentional product targets. Any port in the approved CIDRs is reachable after explicit probe authorization. This means health checks are controlled internal requests, not general URL fetching. A separate scope, same-origin catalog paths, no secrets, and the limits above reduce the remaining risk.

### `internal/identity`

Reuse the Kinosail Owner and Viewer Profile model. Require authentication for the board. Only Owners can edit, test health, or use manage MCP tools. Filter the read snapshot by card visibility before HTML, API, or MCP serialization.

Keep first-owner creation serialized. Keep session cookies secure, HTTP-only, and strict SameSite. Keep passkeys, MFA, login limits, recent-authentication checks, CSRF, origin checks, host checks, and audit actor tracking.

Bind to loopback by default. Make setup unavailable on a public-marked listener. Commit the first Owner and setup completion in one transaction. Do not add direct public exposure in the first release. Users can use a trusted reverse proxy or private network after setup.

### `internal/server`

This package should contain adapters and composition only. Keep HTML rendering, JSON decoding, status translation, and route registration here. Put semantic validation in `internal/board`.

The HTML and API adapters call `board.Module`. The MCP adapter creates an internal request to the same API handler. It discards caller-supplied `Authorization`, `Cookie`, `Origin`, `Host`, CSRF, and forwarding headers. It injects the authenticated actor through a private context type. This keeps status translation, authorization, audit, and negative input behavior identical.

## One responsive board

Store semantic order, not pixel coordinates. Store section order, application order, and `small` or `wide` size. CSS Grid chooses the columns from available width.

This creates one board across devices:

- Desktop shows several columns.
- Tablet shows fewer columns.
- Mobile shows one or two columns.
- A wide card spans available columns but never causes horizontal scroll.
- The DOM order always matches the visual and keyboard order.

Use an explicit Edit button. Do not overload normal card activation with a long press. In edit mode:

- Use Pointer Events on a visible drag handle.
- Set `touch-action: none` only on that handle.
- Keep page scrolling active outside the handle.
- Provide Move before, Move after, Move to section, Resize, Edit, and Remove controls.
- Support Space, arrow keys, Enter, and Escape.
- Announce moves with an `aria-live` status.
- Save one draft atomically.
- Keep an Undo action until the save completes.
- Respect reduced motion and forced colors.

Every move becomes `moveApp(appID, sectionID, beforeAppID)` or an explicit append. Pointer and keyboard moves create the same stable-ID operation. CSS columns never enter the payload. Rotation can change columns but cannot change the draft order.

The editor reads the board revision when it opens. Save sends that revision. A stale save must not merge silently. Show the current board and offer Reload or Reapply. Reapply checks each stable-ID precondition against the current board. If an application or anchor was deleted, moved, or changed, report that operation as unresolved. Never commit a partial reapply.

Server-Sent Events (SSE) can notify viewers about board revisions and health transitions. Events contain only the new revision and visible application IDs. Use `Cache-Control: no-store`, disable proxy buffering, and close streams on session or profile revocation. Close every stream after five minutes so reconnect performs authentication again. Do not replay events. On reconnect or `Last-Event-ID`, fetch a fresh board snapshot. If an edit draft is open, the client shows “Board changed elsewhere” and keeps the local draft. SSE does not replace the revision check.

Use local assets only. Do not hotlink application icons. Hotlinks disclose the Viewer IP, weaken Content Security Policy (CSP), and make the board depend on third parties.

Set `min-width: 0` on grid children. Wrap hostile long text with `overflow-wrap: anywhere`. Truncate secondary labels visually without changing accessible names.

## URL policy

A launch URL is browser-visible board content. It is not a secret.

Normalize once in the board module:

- Trim outer whitespace.
- Reject control characters and invalid UTF-8.
- Parse one absolute URL.
- Allow only `http` and `https`.
- Require a hostname.
- Reject user information, such as `user:password@host`.
- Reject every query and fragment in the first release.
- Lowercase the scheme and canonical hostname.
- Remove only a default port.
- Preserve the path.
- Reject values longer than 2048 bytes.

Launch URLs are visible through HTML, the API, MCP, backups, and browser history. The UI must state that they are not secret fields. A future credential feature must use a separate write-only secret operation.

Render links through `html/template`. Use `rel="noreferrer"` for new-window links. Keep `Referrer-Policy: no-referrer`, `object-src 'none'`, `frame-ancestors 'none'`, and local-only script, style, image, and connection sources.

## Versioned HTTP API

Recommended first-release routes:

| Method and path | Access | Result |
|---|---|---|
| `GET /api/v1` | Viewer | API identity and OpenAPI link. |
| `GET /api/v1/openapi.json` | Viewer | OpenAPI 3.1 document. |
| `GET /api/v1/board` | Viewer | Filtered board snapshot, revision, and current health. |
| `POST /api/v1/board/changes` | Owner | Atomic board change list. |
| `PUT /api/v1/apps/{id}/health` | Owner; tokens also need probe scope | Enable or disable reviewed scheduled health checks. |
| `POST /api/v1/apps/{id}/probe` | Owner; tokens also need probe scope | One rate-limited health test. |
| `GET /api/v1/board/events` | Viewer | SSE revisions and health transitions. |
| `GET /healthz` | Public local probe | Process and database readiness only. |

`GET /api/v1/board` returns `ETag: "board-<revision>"`. Every route that changes board state requires `If-Match` and `Idempotency-Key`.

- Missing `If-Match` returns `428 Precondition Required`.
- `If-Match` must contain one strong, quoted board ETag. Reject weak tags, `*`, lists, duplicates, and malformed values.
- Stale `If-Match` returns `412 Precondition Failed` and the current ETag.
- A repeated idempotency key returns the saved response only for the same actor, endpoint, and canonical payload.
- Successful changes return the new snapshot and ETag.
- All failure responses use bounded JSON and stable error codes.

The request body contains at most 50 tagged changes. Supported tags are `addApp`, `updateApp`, `moveApp`, `removeApp`, `addSection`, `updateSection`, `moveSection`, and `removeSection`. These tags do not accept health fields. Reject unknown fields and tags. Reject the complete request before any write if one change is invalid.

The health-setting and manual probe routes are separate because they cause outbound network access. A board save never enables or waits for a probe.

## MCP tools

Keep tools task-specific. Do not expose a general URL fetcher or an unrestricted API tunnel.

### `get_board`

Returns the authenticated profile view, current revision, section IDs, application IDs, normalized launch URLs, and health. Mark it read-only, idempotent, and closed-world.

### `search_app_catalog`

Accepts a bounded text query and returns local catalog matches. Mark it read-only, idempotent, and closed-world.

### `apply_board_changes`

Accepts:

- `expectedRevision`.
- `mutationId` as a UUID.
- One to 50 tagged changes.

It calls `POST /api/v1/board/changes` through the internal API adapter. Give it manage scope only. Mark it destructive and closed-world. Return the complete new board snapshot so the agent can verify the result.

This tool supports requests such as “add Plex at this URL,” “move Calendar before Home Assistant,” and “hide Admin from children” without separate shallow tools.

### `test_app_health`

Accepts one application ID. It calls the manual probe route. Require manage and probe scopes. Mark it open-world because it causes network access. Rate-limit it per Owner and application.

### `set_app_health`

Accepts one application ID, an enabled flag, the expected board revision, and a mutation ID. It calls the dedicated health-setting route. Require manage and probe scopes. Mark it destructive and open-world because enabling schedules future network access. Do not permit health changes through `apply_board_changes`.

### MCP access policy

Use `kinosail-dashboard.read`, `kinosail-dashboard.manage`, and `kinosail-dashboard.probe` scopes. Probe tools require both manage and probe. Board mutation alone cannot cause network access.

Remote MCP reuses the Kinosail OAuth flow. Validate the exact resource audience, issuer, expiry, scopes, revocation state, and bound Viewer Profile on every request. Validate the algorithm and signature for signed tokens. Use authenticated introspection for opaque external tokens. Publish protected-resource metadata. Use authorization code with Proof Key for Code Exchange (PKCE), exact redirect matching, state, short-lived codes, and revocable tokens. Streamable HTTP remains stateless and JSON-only. Apply origin protection before bearer processing and reject conflicting authentication headers.

Host STDIO uses the running container and a `0600` Unix socket. Treat access to that socket as full Owner bearer authority. The Owner selector controls authorization context and audit attribution; it does not authenticate a person. Require container-exec or socket access to stay restricted to the installation administrator. Require an explicit Owner Profile when several Owners exist.

Do not expose session management, profile management, authentication setup, raw audit export, configuration secrets, or future app credentials through MCP. Log the actor, tool, mutation ID, affected IDs, result, and revision. Never log full launch URLs or request bodies.

## Stored secrets

The first release must not store target application credentials. Application links use the target application's own sign-in. Health checks are unauthenticated.

Dashboard deployment secrets still use the Kinosail configuration pattern:

- Accept direct or `_FILE` input, but reject conflicting forms.
- Bound secret files and require regular files.
- Reject symlinks. Verify owner-only permissions and the opened file identity after `Lstat`.
- Store GUI secrets separately in an owner-only file.
- Replace secret files atomically.
- Return only `configured: true`, never the value.
- Exclude secrets from normal exports.
- Exclude them from every first-release backup.

A `0600` file reduces accidental disclosure. It does not protect secrets after host or installation-account compromise. A later authenticated integration must add a write-only credential operation. A reviewed catalog entry must define the exact header and health path. Never allow arbitrary header names, cross-origin credential forwarding, secret reads, or MCP secret entry.

## Threat model

| Threat | Example | Required control |
|---|---|---|
| Scriptable launch URL | `javascript:` or control characters | Strict HTTP(S) normalization in the board module. Template escaping and CSP remain mandatory. |
| Secret in URL | API token in user information, query, or fragment | Reject all three. State that launch URLs are public board content. Never log full URLs. |
| Server-Side Request Forgery (SSRF) | A manage agent adds a metadata endpoint and enables health | Adding never probes. Health needs a separate open-world action. Use same-origin catalog paths and the pinned local client. |
| DNS rebinding | A name changes to link-local after validation | Resolve at dial time. Validate every answer. Dial the selected IP directly. Reject redirects. |
| Probe resource exhaustion | Hundreds of slow applications | Bound applications, workers, timeouts, rate, response drain, and backoff. |
| Remote icon tracking | Icon host learns every view | Embed icons. Generate monograms. Do not fetch or hotlink remote icons. |
| Credential disclosure | Secret appears in API, MCP, audit, or backup | Keep app credentials out of release one. Redact deployment secrets and exclude them from backups. |
| MCP confused deputy | Read-scoped agent calls a mutation | Use dedicated tools, exact scopes, Owner checks, route allowlists, and audit actors. |
| MCP retry duplication | Tool transport retries `addApp` | Require mutation IDs and persist bounded idempotency results. |
| Lost board update | Desktop and mobile save revision 8 | Compare the revision inside the same write transaction. One save wins. The other gets 412. |
| Broken mobile order | Visual drag order differs from focus order | Make DOM order canonical. Store semantic order only. Test pointer, keyboard, and screen-reader paths. |
| Accidental mobile deletion | A touch gesture removes a card | Use explicit edit mode, a labeled action, confirmation, and Undo. |
| Cross-site mutation | Another site posts to the board | Keep same-origin checks, CSRF tokens, strict cookies, and host validation. |
| Bootstrap takeover | Another network user races first setup | Bind to loopback, block setup on public listeners, serialize creation, and close setup atomically. |
| Insecure target certificate | Probe accepts any local certificate | Use system trust. Do not add a global `skipVerify` option. |
| Container takeover | Dashboard receives Docker control | Do not mount the Docker socket. Run non-root, read-only, without capabilities. |
| Profile information leak | Child profile sees an admin card | Filter visibility in the board module before every adapter serializes data. |

## High-risk test plan

### Board input and atomicity

- Reject missing, malformed, unknown, oversized, out-of-range, duplicate, and conflicting values.
- Prove every rejected change list causes no database or audit mutation.
- Reject unknown change tags and unknown JSON fields.
- Reject more than 50 changes, 32 sections, or 256 applications.
- Prove section removal cannot orphan applications.
- Prove reorder accepts every expected ID exactly once.
- Prove visibility references only existing profiles.
- Fuzz URL normalization and tagged change decoding.

### URL and browser safety

- Reject `javascript:`, `data:`, `file:`, scheme-relative, relative, empty-host, user-information, invalid UTF-8, control-character, and oversized URLs.
- Cover IPv4, bracketed IPv6, Internationalized Domain Name (IDN), and default-port normalization.
- Reject every query and fragment. Cover percent-encoded separators and credentials.
- Render hostile names, descriptions, URLs, and catalog fields without executable HTML.
- Prove external navigation sends no referrer.
- Prove no remote icon, script, style, image, or frame request is possible under CSP.

### Health and SSRF

- Prove adding, updating, moving, and importing applications causes zero network calls.
- Reject absolute health URLs and cross-origin paths.
- Reject link-local, multicast, unspecified, mixed-answer, and IPv4-mapped prohibited addresses.
- Prove public targets fail unless deployment policy allows their CIDR.
- Prove proxy environment variables cannot affect probe routing.
- Cover DNS rebinding between configuration and connection.
- Reject all redirects, including redirects to prohibited addresses.
- Reject loopback and the Dashboard listener, including aliases that resolve to the same address and port.
- Verify TLS certificate and hostname failures remain failures.
- Bound stalled headers, stalled bodies, large bodies, compressed bodies, and connection leaks.
- Prove header limits, disabled decompression, and the 8 KiB wire limit.
- Prove one shared worker limit, per-app single flight, rate limit, jitter, backoff, cancellation, and shutdown behavior.
- Update or delete an application during a probe. Prove the stale result is discarded.
- Prove health failure never changes container readiness.

### Authentication and adapter parity

- Require authentication for HTML, API, SSE, and MCP board reads.
- Deny Viewer mutations and manual probes.
- Enforce card visibility before HTML, API, SSE, and MCP output.
- Cover missing, stale, weak, wildcard, list, malformed, duplicate, and conflicting `If-Match` values.
- Prove HTML and API adapters produce the same semantic result.
- Prove CSRF and cross-origin requests fail before any side effect.
- Prove step-up authentication applies to destructive Owner changes.
- Prove SSE sends only visible IDs, never replays state, and closes on revocation or five-minute expiry.
- Reconnect with `Last-Event-ID`. Prove the client fetches an authorized fresh snapshot.

### MCP

- Deny manage tools without manage scope and an Owner Profile.
- Prove Host STDIO rejects non-Owner and ambiguous Owner selection.
- Prove duplicate mutation IDs do not duplicate applications. Reject the same ID with a different payload.
- Bound query text, mutation count, identifiers, request size, response size, and concurrent STDIO sessions.
- Prove MCP cannot read or set secrets, profiles, sessions, or arbitrary routes.
- Prove `apply_board_changes` cannot trigger a health probe.
- Prove probe tools require both manage and probe scopes.
- Prove the internal bridge drops caller authentication, origin, host, CSRF, and forwarding headers.
- Test OAuth audience, issuer, expiry, PKCE, redirect, state, scope, revocation, profile binding, and conflicting authentication evidence.
- Prove Unix socket access is full Owner authority and cannot cross the installation-account permission seam.
- Prove audit records contain the MCP actor and IDs but omit bodies and full URLs.

### Concurrent updates

- Start two writes from one revision. Prove exactly one commits.
- Retry the winner with the same mutation ID. Prove it returns the same revision and response.
- Prove idempotency lookup, request-hash check, revision update, and result insertion share one transaction.
- Retry the loser after refresh with a new mutation ID. Prove the requested stable-ID move applies.
- Race browser, remote MCP, and Host MCP changes under the Go race detector.
- Interrupt a transaction before commit. Prove the prior board remains intact.
- Expire mutation records without breaking recent retries.

### Responsive editing

- Run populated browser checks at desktop, tablet, 390 px, and 320 px.
- Verify no horizontal document or section overflow.
- Verify pointer drag, touch drag, pointer cancellation, page scroll, and orientation change.
- Verify pointer and keyboard moves produce the same stable insertion anchors.
- Verify the complete edit flow with keyboard and screen reader semantics.
- Verify long names, missing icons, offline apps, all visibility modes, empty sections, and 256 applications.
- Verify 2,048-byte hostile strings cannot force grid or document overflow.
- Verify reduced motion, forced colors, zoom, focus visibility, and minimum touch targets.
- Inspect screenshots for default, editing, saving, stale, error, empty, and recovery states.

### Container and persistence

- Run the single container as its non-root user with a read-only root filesystem.
- Prove startup fails closed on a symlinked, non-regular, corrupt, or future-schema database.
- Prove `/config` database and secret permissions are `0600`.
- Prove the container has no Docker socket, added capability, or sibling application process.
- Verify `/healthz` checks local process and database readiness through the configured host and transport.

## Delivery order

1. Build database, identity, and board modules with API tests.
2. Add the server-rendered board and embedded design assets.
3. Add accessible responsive editing and revision conflicts.
4. Add the embedded catalog and local icons.
5. Add bounded health checks and live status.
6. Add dedicated MCP tools through the internal API adapter.
7. Run populated browser, race, container, security, and full changed-path gates.

Do not add integration credentials or arbitrary widgets until this release is stable. Their designs should start from observed needs, not a generic plugin system.

## Primary sources

All Kinosail source references use commit `925f488ef6005a154d8ccc4f6c236b9f45ecbb62`.

- **[K1]** `internal/server/server.go:53-245` — configuration, composition root, route registration, API, MCP, and lifecycle wiring.
- **[K2]** `internal/server/assets.go:8-102` — embedded assets and immutable versioned caching.
- **[K3]** `internal/server/api.go:170-258` — versioned routes, strict bounded JSON, and method-aware API failures.
- **[K4]** `internal/database/database.go:18-137,182-223` — bounded state, SQLite setup, integrity checks, permissions, and transactions.
- **[K5]** `internal/server/auth.go:45-148` and `internal/server/auth_setup.go:9-60` — profile authorization, MFA, Owner rules, and serialized setup.
- **[K6]** `internal/server/csrf.go:17-90` and `internal/server/security.go:154-238` — CSRF, origin validation, response headers, and request limits.
- **[K7]** `internal/server/security.go:29-82` — restricted DNS resolution, pinned dialing, timeouts, and redirect rejection.
- **[K8]** `internal/server/mcp.go:20-108,193-297` and `internal/server/mcp_route_policy.go:1-11` — MCP scopes, bounded API bridging, actor context, and route policy.
- **[K9]** `internal/server/mcp_stdio.go:19-260` — Host STDIO transport, Owner selection, socket permissions, and concurrency limits.
- **[K10]** `internal/configuration/mutation.go:12-175` and `internal/configuration/snapshot.go:38-45` — secret separation, atomic owner-only writes, and public redaction.
- **[K11]** `Containerfile:1-45` and `compose.yaml:1-15` — static Go build, non-root runtime, container health, and Compose hardening.
