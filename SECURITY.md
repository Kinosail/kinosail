# Security policy

This policy covers Player, Subtitles, Dashboard, the native Apple clients, shared packages, and repository tooling.

## Report privately

Use this repository's **Security → Advisories → Report a vulnerability** form when private vulnerability reporting is available. If it is unavailable, open a non-sensitive issue asking the maintainer to establish a private contact channel. Include no exploit details in that request.

Do not post credentials, tokens, backup archives, private URLs, library paths, media titles, or viewing activity in an issue. Redact logs and screenshots before sharing them.

A useful private report includes:

- affected app and version or full source commit;
- host OS, architecture, container engine, and client version;
- affected feature or endpoint and the minimum reproduction;
- expected and observed behavior, impact, and required access; and
- whether the issue requires an Owner, Viewer, local network, or public listener.

The maintainer coordinates validation, remediation, and disclosure privately. There is no published response-time SLA.

## Supported revisions

Security maintenance targets current `main` and the latest published app release when one exists. This monorepo has no published GitHub releases as of September 15, 2026. Report source-build findings with the exact commit; older app versions are not a long-term-support promise.

## Deployment boundaries

- Create the first Owner over a private local connection before opening network access.
- Player and Subtitles require Owner strong authentication. Their default generated local HTTPS certificate needs explicit trust; public access uses a separately restricted gateway.
- Player's Jellyfin compatibility is off by default. Enable only the integrations you need.
- Player mounts media read-only. Subtitles intentionally writes sidecars beside videos; protect originals and recovery copies separately.
- Dashboard uses localhost HTTP by default. Configure trusted HTTPS, its exact public URL, trusted hosts, and secure cookies before crossing an untrusted network. Dashboard MCP tokens grant Owner access.
- Back up application state, external configuration, secret material, and media separately. Protect backup keys independently from encrypted archives.

Read the app's setup and security guides for details. Host firewall rules, certificate trust, DNS account security, disk encryption, physical access, and off-host backup custody remain operator responsibilities.

## API authorization matrix

This matrix is the authorization contract for Player and Subtitles. The route inventory is exact; a new route is protected until it is explicitly classified and added to the corresponding policy and test matrix. “Session” means a valid Kinosail browser or bearer session. API keys never inherit session-only permissions.

| Route class | Representative routes | Anonymous | Valid session | API key | Remote/public rule |
| --- | --- | --- | --- | --- | --- |
| Unknown route or unsupported method | `/api/v1/not-registered`, wrong method on a known API route | Denied; JSON `404` or `405` | Same | Same | No HTML fallback or authorization bypass |
| Bootstrap and sign-in | `/api/v1/session`, passkey login, Quick Connect, login/static assets | Explicit allowlist only | May continue the flow | Not accepted as a bootstrap credential | Only the closed public-bootstrap allowlist; setup and administration stay private |
| Capability delivery | Media shares, playback capabilities, cast, DLNA, HLS/media URLs | Only with the exact unguessable capability, bound to its resource and purpose | Allowed when the normal Viewer policy also permits it | A matching `stream` key may authorize a classified protected route, but never turns an arbitrary URL into a capability | Capability routes are independently restricted on public listeners |
| Local compatibility | Jellyfin image routes such as `/Items/{id}/Images/{type}` | Local anonymous only | Allowed locally when authorized | Query credentials do not authorize these routes | Remote access is denied by default |
| Library and read API | `/api/v1/library`, item/history/show/album reads | Denied | Viewer or Owner, subject to profile, schedule, MFA, and remote policy | `library` scope; `admin` alone does not grant library scope | Public access requires the separate remote Viewer policy |
| Viewer-owned writes | Progress, bookmarks, lists, playlists, and watch-room writes | Denied | Viewer or Owner, subject to the same policy | `write` scope | No public mutation; origin/CSRF and strict input validation apply |
| Streaming and live access | `/media`, `/hls`, subtitles, events, cast sessions | Denied unless the route has its own capability | Viewer or Owner when permitted | `stream` scope for classified API routes | Public media requires the route-specific capability/session policy |
| Downloads | `/api/v1/downloads/*`, `/download/*` | Denied | Profile download permission plus normal authorization | `download` scope | Never exposed by anonymous public access |
| Owner administration | Settings, profiles, backups, diagnostics, configuration, API-key management | Denied | Owner only; sensitive actions require recent strong authentication | `admin` only where the API route is explicitly admin-scoped; Owner middleware and step-up still apply | Owner administration is unavailable on public listeners |
| Session-only identity and security | Session deletion, MFA, passkey registration, identity linking, management access | Denied except the exact ceremony start/finish routes | Valid Kinosail session and role policy | Explicitly denied | Public sign-in ceremonies do not make management routes public |
| Feature-gated integrations | Home Assistant, MCP/OAuth, SCIM | Discovery or handshake only where explicitly listed; disabled features return `404` | Feature-specific authorization | `home-assistant` only for its explicitly scoped API routes; no generic API-key access to MCP/SCIM | Public transport entry does not imply public data or tool access |

Every mutation also passes bounded-body, strict content-type/JSON or form parsing, origin/CSRF, and route authorization checks before side effects. Invalid, expired, or revoked credentials are rejected; role, schedule, remote-policy, and scope failures cannot fall through to a handler.

The enforcement references are [Player's route authorization matrix](apps/player/internal/server/auth_all_routes_test.go), [Subtitles' route authorization matrix](apps/subtitles/internal/server/auth_all_routes_test.go), the shared [API scope policy](packages/identitycore/access.go), and the shared [fail-closed API contract](packages/servertest/api_security.go).

## Verification and release evidence

Signed-release installer checks must remain enabled. A checksum establishes file consistency; it does not establish a publisher's identity.

GitHub Actions is disabled. While `.gates-disabled` exists, local quality suites are also disabled. Neither retained workflow definitions nor skipped checks establish security certification. The [Player release checklist](apps/player/engineering/release-checklist.md) records the required release evidence and current blockers.
