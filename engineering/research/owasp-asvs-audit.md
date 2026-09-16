# OWASP ASVS audit baseline

Research snapshot: 2026-08-26.

## Decision

Use **OWASP ASVS 5.0.0** as the pinned audit baseline and target Level 2 for
the self-hosted Kinosail server. ASVS calls Level 2 the appropriate goal for
most applications; it includes all Level 1 requirements plus Level 2 controls.
The application handles household identity, viewing history, media files,
owner administration, remote access, a browser UI, a versioned API, Jellyfin
compatibility, and WebSockets, so Level 1 alone is not an adequate security
bar. This is an implementation and verification target, **not** a present
claim of compliance. [OWASP ASVS scope and levels](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x03-What-is-the-ASVS.md)

Record every finding using the versioned form `v5.0.0-x.y.z`. OWASP explicitly
warns that unversioned identifiers can change. The OWASP project identifies
5.0.0 as its current stable release. [OWASP ASVS project page](https://owasp.org/www-project-application-security-verification-standard/)

## In-scope attack surface

Audit the same application operation through every adapter: browser and HTMX
routes, `/api/v1`, Jellyfin-compatible routes, media ranges/HLS/downloads,
WebSockets, passkeys/passwords/TOTP/Quick Connect, API keys and OAuth/OIDC
where enabled, import/restore, metadata/artwork fetches, and the direct public
HTTPS listener. Do not count a UI-only guard as a control: ASVS requires
trusted-service-layer validation and authorization. [V2](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x11-V2-Validation-and-Business-Logic.md) [V8](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x17-V8-Authorization.md)

ASVS evaluates application controls, including application-managed proxies and
load balancers when they implement such controls; deployment, DNS, and external
backup processes remain separate evidence boundaries. [ASVS scope](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x03-What-is-the-ASVS.md)

## Highest-value Kinosail work

| Priority | Requirement IDs | Kinosail-specific verification and expected control | Primary source |
| --- | --- | --- | --- |
| P0 | `v5.0.0-8.2.1`, `8.2.2`, `8.3.1` | Build an endpoint/object/field authorization matrix for Owner, Viewer, revoked Viewer, anonymous, API key, device token, remote viewer, and media capability. Exercise it through web, API, Jellyfin, range, HLS, download, WebSocket, and background-job entry points; profile or role changes must take effect before the next protected action. | [V8 Authorization](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x17-V8-Authorization.md) |
| P0 | `v5.0.0-2.2.1`, `2.2.2`, `2.3.1`, `2.4.1` | At each trust boundary, impose strict type/format/range/cardinality limits in shared operations, prove malformed input has no side effect, make multi-step flows replay- and skip-resistant, and limit costly login, search, scan, transcode, download, HLS, export, and notification work per subject and globally. Document the limits. | [V2 Validation and Business Logic](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x11-V2-Validation-and-Business-Logic.md) |
| P0 | `v5.0.0-6.1.1`, `6.3.1`, `6.3.3`, `6.3.4`, `6.5.1`, `6.5.3`, `6.5.5`, `6.6.2`, `6.6.3` | Inventory every authentication path and require equivalent controls. Test credential/OTP/Quick Connect brute force, replay, expiration, one-use behavior, account enumeration, recovery, factor revocation, and Owner-versus-Viewer escalation. Use CSPRNG values and bounded lifetime; avoid attacker-triggered permanent lockout. | [V6 Authentication](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x15-V6-Authentication.md) |
| P0 | `v5.0.0-7.2.1`–`7.2.4`, `7.3.1`, `7.3.2`, `7.4.1`, `7.4.2`, `7.4.3`, `7.5.1`, `7.5.2` | Verify backend-only, high-entropy session tokens; rotation on authentication; idle and absolute expiry; immediate invalidation on logout, profile disable/delete, permission change, credential reset, and remote-access disable. Provide session visibility/revocation and step-up before sensitive account changes. | [V7 Session Management](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x16-V7-Session-Management.md) |
| P0 | `v5.0.0-3.3.1`–`3.3.4`, `3.4.2`–`3.4.6`, `3.5.1`, `3.5.3` | Browser sessions must use secure host-bound HttpOnly cookies and purpose-appropriate SameSite policy. Test CSP, `nosniff`, referrer, framing, CORS, Origin/CSRF and Fetch-Metadata handling for every state-changing or expensive endpoint, including unauthenticated login and pairing flows. | [V3 Web Frontend Security](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x12-V3-Web-Frontend-Security.md) |
| P0 | `v5.0.0-4.1.1`, `4.1.3`, `4.2.1`, `4.4.1`–`4.4.4` | Validate content types and supported methods; reject client-spoofed forwarded identity headers unless a configured proxy has authenticated them; test HTTP message limits/smuggling posture. WebSockets require WSS, an explicit Origin allowlist, normal-session binding, message/connection limits, and closure on revocation. | [V4 API and Web Service](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x13-V4-API-and-Web-Service.md) |
| P0 | `v5.0.0-14.2.1`, `14.2.2`, `14.3.1`–`14.3.3` | Ensure session/API/media capability secrets never appear in URLs, referrers, logs, telemetry, or browser storage. Apply `no-store` to sensitive authenticated responses and validate that logout clears client-side authenticated data. Treat media titles, viewing history, profile data, and local filesystem paths as classified data. | [V14 Data Protection](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x23-V14-Data-Protection.md) |
| P1 | `v5.0.0-5.1.1`, `5.2.1`–`5.2.3`, `5.3.1`, `5.3.2`, `5.4.1`, `5.4.2` | For backup/restore, imports, artwork, subtitles, and any future upload, document allowed type/size/count; validate actual content, archives, and decompressed limits; prohibit attacker-directed filesystem paths and executable public serving; sanitize response filenames. Test symlinks, traversal, zip slip, extension/content mismatches, and decompression bombs. | [V5 File Handling](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x14-V5-File-Handling.md) |
| P1 | `v5.0.0-11.1.1`, `11.1.2`, `11.2.1`–`11.2.3`, `11.4.2`, `11.5.1` | Maintain a key/crypto inventory covering sessions, passkeys, API keys, backup encryption, audit integrity, TLS, OAuth/OIDC, and WireGuard. Use vetted Go cryptography, CSPRNG secrets with at least 128 bits where required, current password hashing parameters, AEAD for encrypted data, and a rotation/migration plan. | [V11 Cryptography](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x20-V11-Cryptography.md) |
| P1 | `v5.0.0-12.1.1`, `12.2.1`, `12.2.2`, `12.3.1`, `12.3.2` | Test that public routes accept only TLS 1.2/1.3, no HTTP downgrade leaks credentials, public hosts use valid public certificates, and every outbound TLS client verifies certificates. Keep the local private-CA workflow deliberately separate from public remote access. | [V12 Secure Communication](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x21-V12-Secure-Communication.md) |
| P1 | `v5.0.0-15.1.1`–`15.1.3`, `15.2.1`–`15.2.3`, `15.3.1`–`15.3.4` | Maintain dependency remediation windows and a software inventory; bound resource-demanding media workloads; remove dev-only production surfaces; return allowlisted response fields; do not follow untrusted outbound redirects; prevent mass assignment; and keep original-client identity trustworthy through proxies. | [V15 Secure Coding and Architecture](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x24-V15-Secure-Coding-and-Architecture.md) |
| P1 | `v5.0.0-16.1.1`, `16.2.1`–`16.2.5`, `16.3.1`–`16.3.4`, `16.4.1`, `16.4.2`, `16.5.1`–`16.5.3` | Keep a structured, access-controlled security-event inventory. Log authentication, authorization denials, rejected validation/anti-automation attempts, and control failures with redaction and injection-safe encoding; return generic errors; fail closed and keep LAN service safe when remote dependencies fail. | [V16 Logging and Error Handling](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x25-V16-Security-Logging-and-Error-Handling.md) |

## Initial source-inspection findings

These are high-confidence source observations, not a complete ASVS assessment;
they are the first controls that prevent a Level-1 claim for all supported
deployment modes.

| Finding | ASVS impact | Action |
| --- | --- | --- |
| `validatePassword` rejects six literal values (`internal/server/credentials.go`) rather than the required policy-compatible top 3,000 passwords. | `v5.0.0-6.2.4` requires the top 3,000 at Level 1. [V6](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x15-V6-Authentication.md) | Ship a deterministic offline denylist; test registration, reset, and all profile-creation/update adapters against a representative blocked password. |
| Normal browser sign-in writes `kinosail_session`; `Secure` is conditional so the supported plaintext-local mode writes an insecure, unprefixed session cookie (`internal/server/sessions.go`). The distinct direct-public session already uses `__Host-kinosail_session`. | `v5.0.0-3.3.1` requires `Secure` and `__Host-` or `__Secure-` naming. [V3](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x12-V3-Web-Frontend-Security.md) | Choose and document an ASVS-compatible browser transport policy: require HTTPS for all non-loopback browser sessions, use a host-prefixed secure cookie there, and make any loopback-only development exception unavailable on a network listener. |
| Jellyfin GET/HEAD routes accept `api_key` in the query string (`internal/server/session_tokens.go`). | `v5.0.0-14.2.1` prohibits API keys and session tokens in URLs/query strings. [V14](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x23-V14-Data-Protection.md) | Remove URL bearer authentication and require headers or a secure cookie. If a compatibility player cannot do that, it is incompatible with this Level-1 requirement; record an explicit exception rather than claiming ASVS compliance. |

Existing source controls such as Argon2id password hashing, centrally applied
Origin/CSRF checks, restrictive browser headers, a request-body limit, public
`__Host-` cookies, structured audit events, and route-authorization tests are
useful evidence, but none substitutes for testing every applicable requirement
and deployment mode.

## Audit method and completion bar

1. Create a versioned ASVS worksheet that marks every applicable requirement
   pass, fail, not applicable (with rationale), or unverified. Do not report
   Level 2 compliance while any applicable Level 1 or Level 2 requirement is
   unverified or failed.
2. Couple code review to executable evidence: focused unit/API/web/browser
   tests, negative/no-side-effect tests for each trust boundary, fuzzing for
   parsers and archive/range/header boundaries, `go test -race`, dependency
   scanning, and a deployed-container/browser check for TLS, cookies, headers,
   proxy behavior, WebSockets, and media cancellation.
3. Treat ASVS documentation requirements as deliverables, not optional prose:
   authorization rules, input and workload limits, authentication pathways,
   session lifetime/concurrency, sensitive-data classes, logging inventory,
   cryptographic inventory, dependency remediation window, and costly-work
   policy need a maintained home in the repository.

Some ASVS chapters are conditional. GraphQL, SOAP, WebRTC, and self-contained
token requirements can be marked not applicable only after the route and
dependency inventory proves their absence; OAuth/OIDC requirements are
applicable whenever those integrations are enabled. ASVS explicitly supports
tailoring while retaining requirement traceability. [ASVS tailoring guidance](https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x03-What-is-the-ASVS.md)
