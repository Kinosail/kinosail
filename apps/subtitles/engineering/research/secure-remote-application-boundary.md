# Secure remote application boundary

Research snapshot: 2026-08-24.

## Decision

Kinosail should treat direct public HTTPS as a separate, smaller product boundary, not as the LAN application with TLS added. The secure default remains **off**. When enabled, the public listener should admit only explicitly remote-enabled Viewer capabilities, require phishing-resistant authentication, reject every Owner and automation credential, and bound every operation that can consume CPU, memory, storage, connections, or bandwidth.

WireGuard is the higher-assurance option because it reduces the publicly reachable application surface. It must not replace Kinosail authentication or authorization: NIST's zero-trust guidance says network location alone must not create implicit trust ([NIST SP 800-207](https://csrc.nist.gov/pubs/sp/800/207/final)). A WireGuard peer may reach the normal authenticated Kinosail service; it does not become an Owner merely by joining the tunnel.

The requested honeypot should be a **low-interaction tripwire**, not a decoy service. It may return a generic response, record and aggregate signals, temporarily quarantine the verified source, and alert the Owner. It must never let an unauthenticated request trigger a global kill switch. NIST describes decoys as isolated components for detection and deflection, and warns that supporting isolation is required ([NIST SP 800-53 Rev. 5.1, SC-26](https://csrc.nist.gov/CSRC/media/Projects/risk-management/800-53%20Downloads/800-53r5/SP_800-53_v5_1-derived-OSCAL.pdf)). NIST authentication guidance also recommends throttling designs that reduce attacker-induced lockout ([NIST SP 800-63B-4, rate limiting](https://pages.nist.gov/800-63-4/sp800-63b.html#rate-limiting-throttling)).

An application kill switch is still valuable. It should atomically close only the public listener and revoke remote-derived sessions and pending capabilities while leaving LAN and WireGuard access available. Automatic activation is appropriate only when Kinosail itself knows it cannot enforce the remote security policy, not when an untrusted client claims or appears to be hostile.

## Scope

This report covers application behavior for:

- Kinosail's dedicated public HTTPS listener;
- authenticated requests received through WireGuard;
- browser, versioned API, Jellyfin-compatible, media-range, WebSocket, and long-lived request paths;
- authentication, sessions, authorization, throttling, tripwires, audit events, alerts, certificate state, and an in-application kill switch.

It deliberately excludes container-image or dependency pinning, Compose, container privileges, host firewalling, router configuration, and operating-system hardening. It does not claim that an application can withstand a volumetric network attack; those packets consume upstream capacity before application controls run.

## Existing foundation observed in the repository

The current code already provides a strong base:

- [`packages/remoteaccess/remoteaccess.go`](../../../../packages/remoteaccess/remoteaccess.go) uses a TLS-only listener, TLS 1.2 and 1.3, forward-secret AEAD TLS 1.2 suites, a hostname-restricted ACME manager, bounded DuckDNS responses, request timeouts, and no redirect following.
- [`internal/server/security.go`](../../internal/server/security.go) provides an out-of-band public-listener context marker, Host checks, `no-store`, a strict CSP without inline script, clickjacking and MIME-sniffing defenses, same-origin mutation checks, and a global request-body bound.
- [`internal/server/auth.go`](../../internal/server/auth.go) hides setup, Owner, API-key, MCP, OAuth, OIDC, and health operations from the public listener; profile policy is enforced after identity resolution.
- [`internal/server/passkeys.go`](../../internal/server/passkeys.go) uses discoverable WebAuthn credentials, `userVerification=required`, bounded expiring ceremonies, and a canonical origin.
- [`internal/server/sessions.go`](../../internal/server/sessions.go) generates opaque random tokens, stores only token hashes, rotates tokens on login, uses host-only `HttpOnly`/`SameSite=Strict` cookies, and gives browser sessions an eight-hour absolute lifetime. [`internal/server/profiles.go`](../../internal/server/profiles.go) enforces a 15-minute browser inactivity timeout and promptly rechecks profile policy.
- [`internal/server/credentials.go`](../../internal/server/credentials.go) uses Argon2id with the OWASP baseline parameters and a dummy verification path.
- [`internal/server/audit.go`](../../internal/server/audit.go) records structured security events in an integrity-linked journal and uses a bounded asynchronous notification queue.
- [`internal/server/watch_together.go`](../../internal/server/watch_together.go) authenticates WebSocket upgrades, revalidates authorization every second, caps messages at 4 KiB, and bounds writes.
- [`internal/server/workloads.go`](../../internal/server/workloads.go) globally bounds heavyweight playback/background work and reserves capacity for playback.

These controls should be preserved. The following work closes the remaining public-boundary gaps.

## Required controls before direct HTTPS is called highly secure

### 1. Preserve the actual socket identity

On a direct listener, `RemoteAddr` is the only available source address supplied by the transport. Strip `Forwarded` and every `X-Forwarded-*` field before abuse controls or audit attribution. Honor them only after authenticating a separately configured trusted proxy.

This is a concrete issue in the reviewed snapshot: [`trustedProxy` in `internal/server/security.go`](../../internal/server/security.go) enters the forwarded-header path when either the proxy capability is valid **or** the request has the direct-listener remote marker. Consequently, a direct client can currently supply `X-Forwarded-For`, replace `RemoteAddr`, evade per-IP login/tripwire windows, and poison audit attribution. The remote marker should select the public policy only; it must not confer proxy trust.

RFC 7239 says forwarded identity cannot be relied upon because the client itself can modify it; trusted proxy relationships are required ([RFC 7239, Section 8.1](https://www.rfc-editor.org/rfc/rfc7239.html#section-8.1)). Add negative tests for IPv4, IPv6, multiple fields, malformed values, `Forwarded`, every supported `X-Forwarded-*` spelling, and a valid proxy-capability control case.

The public listener should also bind every layer to the one configured public name: reject unknown SNI during TLS negotiation and require the exact public `Host`/HTTP/2 `:authority` after negotiation. Do not inherit the LAN alias allowlist on that listener. RFC 9325 recommends TLS 1.2 and 1.3, preference for 1.3, strict TLS, SNI, forward secrecy, and AEAD suites ([RFC 9325, Sections 3 and 4](https://www.rfc-editor.org/rfc/rfc9325.html)); RFC 9110 permits `421 Misdirected Request` when the connection is not authoritative for the target origin ([RFC 9110, Section 15.5.20](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.5.20)). Preserve the current TLS versions and suites, disable 0-RTT if the runtime ever exposes it, and send HSTS only for the exact public hostname.

### 2. Allowlist the public route and capability surface

The public boundary should be `deny unless explicitly authorized`, including for future routes. Saltzer and Schroeder's foundational protection paper states both fail-safe defaults (base decisions on permission, not exclusion) and complete mediation (check authority on every access) ([The Protection of Information in Computer Systems, Section I.A.3](https://web.mit.edu/Saltzer/www/publications/protection/Basic.html)). NIST carries the secure-default and secure-failure principles into current systems engineering guidance ([NIST SP 800-160 Vol. 1 Rev. 1](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-160v1r1.pdf)); OWASP ASVS requires explicit function-, object-, and field-level authorization at a trusted service layer ([ASVS 5.0.0 V8](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x17-V8-Authorization.md)).

The current `remotePublicRouteDenied` list denies known sensitive routes. That is valuable but fails open for a newly registered route. Replace or precede it with a public-listener allowlist whose entries name:

- exact method and route pattern;
- required identity type (`anonymous`, `remote Viewer`, or short-lived playback capability);
- required Viewer permission;
- whether the operation may mutate state or start heavy work;
- endpoint-specific limits.

Anonymous public access should be limited to static login assets, passkey/Quick Connect initiation and completion, and the minimum Jellyfin discovery response necessary for compatible clients. Authenticated public access should be limited to the Viewer library, playback, progress/list operations that are already covered by the Viewer policy, logout, and session self-management. Owner, setup, recovery, administration, backup, diagnostics, scanning, configuration, MCP/OAuth/OIDC, API keys, DLNA, and local-CA/WireGuard provisioning remain unavailable. Unknown routes and methods return the same generic `404`.

Add a route-inventory test that fails whenever a new registered route has no explicit public classification. Also test every public route with anonymous, local-only Viewer, remote Viewer, Owner, API-key, expired-session, revoked-profile, and short-lived media-capability identities.

### 3. Make direct-HTTPS authentication phishing-resistant by default

NIST recognizes WebAuthn as phishing-resistant because the credential is bound to the verifier name, while manually entered OTP codes are not phishing-resistant ([NIST SP 800-63B-4, phishing resistance](https://pages.nist.gov/800-63-4/sp800-63b.html#phishing-resistance)). WebAuthn requires the RP ID to match the effective domain and requires HTTPS except for localhost ([W3C WebAuthn Level 3, RP ID](https://www.w3.org/TR/webauthn-3/#relying-party-identifier)).

For the highly secure direct mode:

- require a passkey with user verification for browser and versioned-API login;
- permit Quick Connect only as a short, one-use bootstrap approved by a recently phishing-resistant local or WireGuard Viewer session; keep every approval page/API route unavailable on the public listener;
- never let public Quick Connect inherit an Owner or local-only profile;
- keep passkey registration, recovery, TOTP enrollment, and remote-permission changes local or WireGuard only;
- disable password-plus-TOTP login on the public listener by default. It can exist as an explicit compatibility downgrade, visibly labeled as not phishing-resistant.

Quick Connect must be throttled independently for creation, code approval, status polling, and token exchange. Approval must show the requesting device/client, target Viewer, and that the request originated on the public boundary. Consumption must be one-use and must recheck the Viewer's current `Remote` permission immediately before minting a session.

The existing six-entry local password denylist is insufficient for any remaining password path. ASVS requires at least the top 3,000 policy-compatible passwords at Level 1 and a breached-password set at Level 2 ([ASVS 5.0.0 V6.2](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x15-V6-Authentication.md)). Use a shipped, deterministic offline blocklist so password creation does not disclose candidates to an external service or make account management depend on the internet.

### 4. Give public sessions a distinct lifecycle

NIST recommends cookies that are secure-only, host- and path-scoped, inaccessible to JavaScript, same-site, opaque, and preferably use the `__Host-` prefix ([NIST SP 800-63B-4, browser cookies](https://pages.nist.gov/800-63-4/sp800-63b.html#browser-cookies)). ASVS requires at least 128 bits of random session-token entropy, rotation at authentication/reauthentication, idle and absolute expiry, and prompt revocation ([ASVS 5.0.0 V7](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x16-V7-Session-Management.md)).

Keep the current 15-minute browser idle timeout and eight-hour absolute timeout. Add a `channel=public` attribute to sessions minted on the direct listener and:

- use a public-host-only `__Host-` secure cookie, with browser expiry aligned to server expiry;
- rotate the session after every full authentication and step-up;
- revoke public sessions immediately when remote access or the Viewer's remote permission is disabled, the password/passkey changes, the profile is removed, or the kill switch runs;
- list the creation channel, device, creation/last-use time, and expiry in the Owner's local session view;
- cap concurrent public sessions per Viewer and make excess-session behavior explicit;
- use anomaly signals only to require step-up or notify; do not hard-bind a session to an IP address that can legitimately change.

Preserve the current `SameSite=Strict`, Origin, and Fetch Metadata checks, and add a per-session CSRF token for browser-cookie state changes as defense in depth. Origin validation should compare the complete expected origin (scheme, host, and effective port), not only the Host text. Machine clients presenting an authorization token remain outside the browser-CSRF path.

Native Jellyfin device tokens should also be tagged as public-derived. Thirty-day bearer sessions are a large theft window for an internet-facing TV; use a shorter configurable public lifetime with visible renewal/revocation, or a rotating device credential where the client protocol permits it.

Do not accept a long-lived session or API key in the public URL. ASVS prohibits session/API tokens in URLs ([ASVS 5.0.0 V14.2.1](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x23-V14-Data-Protection.md)); RFC 6750 warns that query bearer tokens leak into histories and logs and recommends short-lived, scoped tokens ([RFC 6750, Sections 2.3 and 5](https://www.rfc-editor.org/rfc/rfc6750.html)). Jellyfin compatibility media URLs that cannot use an authorization header should carry a separate unguessable playback capability, stored hashed, bound to Viewer, item, operation, and session, refreshable during active playback, and revoked with the parent session. The reviewed eight-hour anonymous `playSessionId` window should be shortened or made rolling with an absolute cap.

### 5. Bound public resource use at every expensive seam

OWASP API4:2023 calls for maximum input sizes and cardinalities, pagination bounds, endpoint-specific request-rate limits, and operation-frequency limits ([OWASP API4:2023](https://owasp.org/API-Security/editions/2023/en/0xa4-unrestricted-resource-consumption/)). RFC 6585 defines `429 Too Many Requests` and optional `Retry-After` ([RFC 6585, Section 4](https://www.rfc-editor.org/rfc/rfc6585.html#section-4)).

Keep the existing one-MiB application body limit, but use narrower limits at login, passkey, Quick Connect, search, progress, and policy endpoints. On the public listener add bounded, expiring limiters for:

- unauthenticated requests by verified socket IP/prefix and endpoint class;
- credential attempts by verified IP, normalized account, and authenticator;
- authenticated requests by session and Viewer;
- concurrent HTTP requests by source and Viewer;
- concurrent direct streams, HLS sessions, downloads, and transcodes by Viewer/session/source;
- WebSocket handshakes, open connections, and messages;
- Quick Connect starts, polling, and invalid approvals;
- notification and audit-event coalescing, so an attacker cannot create an alert flood or exhaust a paid notification provider.

Prefer temporary increasing delays and expiring source quarantine over permanent account lockout. NIST explicitly identifies bot challenges, progressive waits, and risk signals as ways to keep throttling from becoming attacker-induced lockout ([NIST SP 800-63B-4, rate limiting](https://pages.nist.gov/800-63-4/sp800-63b.html#rate-limiting-throttling)). Keep maps cardinality-bounded, remove expired entries opportunistically, and make saturated limiters fail closed for the public request without affecting LAN service.

The listener's current one-MiB header allowance is unnecessarily broad for this API. Reduce the public limit to the smallest value demonstrated compatible by real clients (64 KiB is a reasonable test target), while retaining explicit `431` behavior where possible ([RFC 6585, Section 5](https://www.rfc-editor.org/rfc/rfc6585.html#section-5)). Do not add a single short global write timeout: it would terminate legitimate long media responses. Instead set short write deadlines for ordinary responses and apply stream-specific concurrency, idle, cancellation, and bandwidth policies.

### 6. Constrain byte ranges and media capabilities

RFC 9110 permits servers to reject invalid ranges, more than two overlapping ranges, and many small unsorted ranges because they can signal deliberate denial of service ([RFC 9110, Section 14.2](https://www.rfc-editor.org/rfc/rfc9110.html#section-14.2)). Go's `http.ServeFile` can produce multipart responses for multiple ranges, so the application should validate before handing a public media request to it.

For direct public media delivery:

- accept only the `bytes` unit and at most one satisfiable range unless a proven client requires more;
- strictly parse decimal offsets with overflow checks and reject ambiguous duplicate `Range` fields;
- bound suffix ranges and reject overlapping, unsorted, or many-small-range requests;
- return the correct `416` and `Content-Range: bytes */size` without opening a transcode;
- authorize the item and playback capability before filesystem access, probing, HLS work, or download creation;
- cancel reads, probes, and transcodes immediately when the request is canceled;
- recheck the parent session/profile policy for long-lived or renewed playback capabilities.

Keep original downloads disabled unless the Viewer has an explicit download entitlement. Even permitted downloads need independent concurrent-download and response-byte accounting so they cannot starve interactive playback.

### 7. Finish WebSocket and long-lived-request hardening

RFC 6455 requires origin consideration for browser clients and implementation-specific frame/message limits; it notes that servers may reject excessive connections and disconnect resource-hogging peers ([RFC 6455, Sections 10.2 and 10.4](https://www.rfc-editor.org/rfc/rfc6455.html#section-10.2)). ASVS requires WSS, an Origin allowlist, and authenticated transition from the HTTPS session ([ASVS 5.0.0 V4.4](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x13-V4-API-and-Web-Service.md)).

The current WebSocket library validates same-host Origin by default and compression is disabled by default ([coder/websocket `AcceptOptions`](https://pkg.go.dev/github.com/coder/websocket#AcceptOptions)); make the exact canonical-origin policy explicit rather than relying on a default. Preserve the 4-KiB message limit and one-second authorization recheck, then add:

- a global, per-source, per-Viewer, and per-room connection cap;
- a per-connection and per-Viewer message-rate limit;
- heartbeat/idle closure and a bounded connection lifetime;
- immediate closure on logout, session expiry, remote-permission change, public kill, or room expiry;
- a bounded outbound queue/backpressure policy so one slow client cannot hold room resources;
- generic close reasons for policy failures and structured audit events without message contents.

Apply equivalent session revalidation, connection caps, cancellation, heartbeat, and `no-store` behavior to any future SSE endpoint. Neither WebSocket nor SSE URL should contain a session token.

## Safer tripwire and quarantine

The tripwire should recognize only paths or protocol patterns that legitimate Kinosail clients never use, such as `/.env`, `/.git/config`, `/wp-login.php`, traversal probes, malformed authority/forwarding attempts, and repeated requests for remotely forbidden Owner routes. It is a detector attached to the real public boundary, not a second application.

Required behavior:

1. Return the same generic `404` as an unknown route. Do not present a fake login, filesystem, shell, database, or credential.
2. Record a structured category, verified socket source, request ID, and time. Never record the query, body, cookies, authorization fields, or supplied forwarding headers.
3. Coalesce repeated signals and send at most one Owner alert per source/category/cooldown window.
4. After multiple independent high-confidence signals, temporarily quarantine only that verified source. Return `429` with `Retry-After` after quarantine is active; do not disclose which request triggered it.
5. Use a bounded, expiring quarantine map. Avoid permanent IP bans because NAT, carrier networks, privacy relays, and address reassignment can make an IP represent multiple people.
6. Never change account state, revoke unrelated sessions, disable WireGuard, or close the public listener from a tripwire hit.
7. Keep LAN and WireGuard traffic outside the public tripwire policy even when a public source is quarantined.

This is intentionally less ambitious than a honeypot. NIST SC-26 says full decoys require isolation so malicious code cannot affect operational systems. A few impossible-path canaries provide high-signal detection without creating a new parser, state machine, or attack surface.

The reviewed tripwire's primary source map is cardinality-bounded, but its shared overflow bucket cannot quarantine a previously unseen source once the map is saturated. Saturation therefore needs an explicit, tested policy: reject new public sources temporarily, evict only expired entries, or use a bounded approximate limiter. It must not silently report a source as blocked while allowing that source's next request.

## Application kill switch and fail-closed rules

Expose an idempotent `Disable public access now` application operation through the local/WireGuard Owner UI and versioned API. Require a recently phishing-resistant Owner session when normal authentication is available, but preserve a documented local recovery path for an active incident.

The operation should, in order:

1. atomically put the public policy into `disabled` so new requests fail;
2. stop accepting new public connections and close public WebSockets;
3. cancel or close public streams according to an explicit emergency policy;
4. revoke all `channel=public` browser/device sessions and playback capabilities;
5. remove pending public passkey ceremonies and Quick Connect requests;
6. preserve and append an integrity-protected audit event;
7. leave the LAN server, WireGuard access, local sessions, library, and background work running.

Re-enabling public access should require recent strong Owner authentication and rerun all startup invariants before the listener opens.

Automatic public-only shutdown is justified when Kinosail positively detects one of these internal conditions:

- the remote authorization policy did not initialize or cannot classify a route;
- profile/credential state cannot be read or authorization cannot be enforced;
- the configured TLS identity is invalid for the public hostname, expired, or private-key integrity fails;
- the remote-listener marker or exact authority invariant cannot be established;
- an integrity failure affects security policy or remote credential state.

Do **not** auto-disable on a scan, tripwire hit, login burst, limiter saturation, ordinary `403`/`404`, transient DuckDNS/ACME/notification failure, audit notification queue overflow, or suspicious WebAuthn counter alone. These events call for reject/throttle/quarantine/alert behavior. OWASP requires secure failure without fail-open processing and also graceful degradation when an external dependency fails ([ASVS 5.0.0 V16.5](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x25-V16-Security-Logging-and-Error-Handling.md)).

A lost DuckDNS refresh does not prove that the active DNS address is unsafe, so keep established TLS service while showing degraded state. A certificate validation failure already prevents a secure handshake; expose it locally and refuse new public connections without stopping the LAN service.

## Audit, alerting, and certificate health

ASVS requires authentication events, failed authorization, security-control bypass attempts, and security-control failures to be logged; it also requires structured metadata, secret redaction, log protection, and generic external errors ([ASVS 5.0.0 V16](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x25-V16-Security-Logging-and-Error-Handling.md)). Extend the current journal with stable event types for:

- public listener enable/disable/start/failure;
- certificate issuance/renewal/expiry/integrity failure and last successful DuckDNS update;
- public session, Quick Connect, passkey, and playback-capability creation/revocation;
- public authentication success/failure and authorization denial;
- throttle and tripwire activation/expiry;
- malformed authority, forwarding, range, WebSocket, and oversized-input attempts;
- remote permission and remote security-policy changes.

Never log passwords, TOTP values/seeds, recovery codes, WebAuthn challenges, cookies, session/device/playback tokens, DuckDNS tokens, WireGuard private/public configuration, media titles/paths, request bodies, or raw URLs containing queries. Encode all untrusted fields. The HMAC-linked local journal detects accidental or offline modification only while its key remains trustworthy; because the key and journal share the application data boundary, describe that limitation honestly. An optional authenticated export to a logically separate destination can raise assurance, but notification delivery must remain asynchronous, bounded, redacted, and unable to block a request.

The local Owner status view should show hostname, mode, listener state, exact remote policy version, certificate subject/issuer/expiry, last renewal, last DDNS success, active public sessions/streams, current quarantines, dropped-alert count, and the last high-signal security event. It should never render secrets or full media activity.

## Direct HTTPS and WireGuard posture

| Control | Direct public HTTPS | WireGuard |
| --- | --- | --- |
| Network reachability | Public TLS listener only | WireGuard peer key required before Kinosail is reachable |
| Kinosail identity | Remote-enabled Viewer only | Normal Kinosail profile authentication still required |
| Owner operations | Never | Allowed only after normal Owner authorization and recent step-up |
| Browser authentication | Passkey required by secure default | Passkey preferred; local recovery policy may be used |
| Jellyfin authentication | Locally approved Quick Connect; no public password login | Normal compatible flow subject to profile MFA policy |
| API keys | Denied | Allowed only by existing scope and Owner policy |
| Media access | Viewer policy plus short playback capability and public resource limits | Viewer policy and normal workload limits |
| Tripwire/quarantine | Enabled against verified socket source | Public tripwire disabled |
| Kill switch effect | Listener and public-derived credentials revoked | Tunnel and LAN service remain usable |

WireGuard's cryptokey routing associates each peer public key with allowed source IPs and drops a decrypted packet whose inner source does not match that peer ([WireGuard design paper, Section 2](https://www.wireguard.com/papers/wireguard.pdf)). Kinosail peer provisioning should therefore remain per-Viewer/per-device with unique peer keys, a single Kinosail `/32` route rather than household-LAN access, explicit revocation, and an audit event. Application authorization changes must take effect immediately for tunnel requests just as they do for LAN/public sessions.

## Later higher-assurance controls

For a future first-party installed Kinosail client, sender-constrain its access and refresh tokens to a device-held key. DPoP is an application-layer proof-of-possession standard that binds a token to a public key and requires a fresh, request-specific proof, reducing the usefulness of a stolen token ([RFC 9449](https://www.rfc-editor.org/rfc/rfc9449.html)). Use server nonces and replay detection, keep the key in platform-protected non-exportable storage where available, and continue to require HTTPS and normal authorization. Do not require DPoP from third-party Jellyfin clients or silently fall back from DPoP to bearer use for a token issued as sender-constrained.

Record WebAuthn backup eligibility/state and signature-counter observations as risk signals. A counter regression or backup-state change can justify an Owner alert or step-up, but it is not sufficiently conclusive to disable public access or delete a credential automatically; the WebAuthn specification explicitly describes signature counters as a signal whose interpretation depends on the authenticator ([W3C WebAuthn Level 3, signature counter considerations](https://www.w3.org/TR/webauthn-3/#sctn-sign-counter)).

## Controls not to add

- **No scanner-triggered global kill.** It is a trivial unauthenticated denial-of-service primitive.
- **No interactive honeypot in the Kinosail process.** It adds parsers, state, forensic data, and isolation requirements without protecting the real route surface.
- **No permanent IP/account lockout from remote failures.** Use expiring progressive delay and source quarantine so attackers cannot lock out a household.
- **No browser HPKP header.** Browser public-key pinning is operationally brittle, can make a hostname unavailable after key loss/rotation, and is no longer recommended by modern browser guidance ([RFC 7469 operational risks](https://www.rfc-editor.org/rfc/rfc7469.html#section-4), [OWASP TLS guidance](https://cheatsheetseries.owasp.org/cheatsheets/Transport_Layer_Security_Cheat_Sheet.html#public-key-pinning)). Normal public PKI, strict hostname validation, HSTS, certificate-health alerts, and WireGuard's explicit peer keys are the appropriate mechanisms here.
- **No `includeSubDomains` or preload for a DuckDNS hostname.** Kinosail has no required descendants below its assigned hostname, so `includeSubDomains` adds no useful protection. Avoid preload because a self-hosted DuckDNS name can be lost, reassigned, or need an emergency recovery path; an effectively irreversible browser preload would create avoidable availability and reassignment risk. Keep HSTS scoped to the exact hostname. RFC 6797 defines `includeSubDomains` as applying policy to subordinate hosts ([RFC 6797, Section 6.1.2](https://www.rfc-editor.org/rfc/rfc6797.html#section-6.1.2)).
- **No trust based only on WireGuard/LAN source.** The network narrows reachability but does not replace application identity and resource authorization.

## Verification plan

Treat the public listener as a separately testable product boundary.

1. **Route inventory:** mechanically enumerate every method/pattern and fail the test for an unclassified public route. Exercise every identity class and permission transition.
2. **Header/authority fuzzing:** invalid/mismatched SNI and Host, absolute-form targets, duplicate Host, HTTP/2 `:authority`, `Forwarded`, all `X-Forwarded-*` variants, spoofed public marker, oversized and duplicate headers.
3. **Authentication/session:** WebAuthn origin/RP/user-verification/challenge replay, public password denial, Quick Connect brute/poll/replay/Owner inheritance, session fixation/rotation/idle/absolute expiry, permission-change and kill-switch revocation.
4. **Resource limits:** oversized and ambiguous JSON/form/query inputs, cardinality and pagination, concurrent browse/stream/download/transcode load, slow headers, canceled requests, saturated limiter maps, alert floods.
5. **Media:** invalid/overflow/suffix/multiple/overlapping/unsorted ranges, capability wrong item/profile/session, expired/revoked capability, seek renewal, disconnect cancellation, full-duration playback.
6. **WebSocket:** missing/wrong/null/cross-site Origin, excess connections/messages, oversized and fragmented messages, slow reader, idle peer, logout/expiry/policy/kill closure.
7. **Tripwire:** generic first response, source-only temporary quarantine, forwarded-header spoof resistance, bounded state, cooldown/coalescing, no LAN impact, no global shutdown.
8. **Failure injection:** unreadable/corrupt profile and remote-policy state, invalid/expired/wrong-host certificate, key-integrity failure, DuckDNS/ACME timeout/malformed/oversized/redirect responses, audit/notification failure, graceful local survival.
9. **Protocol checks:** TLS 1.0/1.1 rejection; TLS 1.2/1.3 success with expected suites; unknown SNI rejection; HSTS exact-host behavior; browser header/CSP tests.
10. **Race/load/browser:** race detector under login/tripwire/session revocation/WebSocket/kill concurrency, load tests demonstrating bounded memory/goroutines/FFmpeg work, and browser/Jellyfin-client tests on the exact public hostname.

This research establishes a design and verification target, not public-exposure certification. Certification still requires the implemented code, settled-tree automated gates, adversarial protocol tests, real public-PKI renewal, off-LAN browser testing, and representative physical Jellyfin clients.

## Primary sources

- [OWASP Application Security Verification Standard 5.0.0](https://github.com/OWASP/ASVS/tree/v5.0.0/5.0/en)
- [OWASP API Security Top 10 — 2023](https://owasp.org/API-Security/editions/2023/en/0x11-t10/)
- [NIST SP 800-63B-4, Authentication and Authenticator Management](https://pages.nist.gov/800-63-4/sp800-63b.html)
- [NIST SP 800-160 Vol. 1 Rev. 1, Engineering Trustworthy Secure Systems](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-160v1r1.pdf)
- [NIST SP 800-207, Zero Trust Architecture](https://csrc.nist.gov/pubs/sp/800/207/final)
- [NIST SP 800-53 Rev. 5.1, SC-26 Decoys](https://csrc.nist.gov/CSRC/media/Projects/risk-management/800-53%20Downloads/800-53r5/SP_800-53_v5_1-derived-OSCAL.pdf)
- [Saltzer and Schroeder, The Protection of Information in Computer Systems](https://web.mit.edu/Saltzer/www/publications/protection/)
- [Donenfeld, WireGuard: Next Generation Kernel Network Tunnel](https://www.wireguard.com/papers/wireguard.pdf)
- [W3C Web Authentication Level 3](https://www.w3.org/TR/webauthn-3/)
- [RFC 9325, Recommendations for Secure Use of TLS](https://www.rfc-editor.org/rfc/rfc9325.html)
- [RFC 9110, HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110.html)
- [RFC 7239, Forwarded HTTP Extension](https://www.rfc-editor.org/rfc/rfc7239.html)
- [RFC 6750, Bearer Token Usage](https://www.rfc-editor.org/rfc/rfc6750.html)
- [RFC 9449, OAuth 2.0 Demonstrating Proof of Possession](https://www.rfc-editor.org/rfc/rfc9449.html)
- [RFC 6585, Additional HTTP Status Codes](https://www.rfc-editor.org/rfc/rfc6585.html)
- [RFC 6455, The WebSocket Protocol](https://www.rfc-editor.org/rfc/rfc6455.html)
- [RFC 6797, HTTP Strict Transport Security](https://www.rfc-editor.org/rfc/rfc6797.html)
