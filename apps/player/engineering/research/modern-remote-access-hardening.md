# Modern remote-access hardening: second research pass

Research snapshot: 2026-08-24.

## Decision

Kinosail's current direct-HTTPS boundary is already substantially stronger than a conventional self-hosted media server: it has an explicit public listener, phishing-resistant public authentication, a deny-by-default route inventory, short channel-bound sessions, bounded public work, source-scoped quarantine, and a persistent kill switch. The next security gains should deepen identity binding, revocation, protocol robustness, and egress isolation. They should not add another public service or turn hostile traffic into an attacker-controlled global shutdown.

The best immediate additions, in priority order, are:

1. close the remaining public-cookie CSRF/origin gap and add a session-bound CSRF token;
2. add a persisted authorization generation to every public session and derived playback capability;
3. bind each WireGuard peer to one Kinosail Viewer and add a unique per-peer WireGuard pre-shared key;
4. make abuse accounting compound and IPv6-prefix-aware;
5. separate public-only outbound HTTP from explicitly local-capable outbound HTTP;
6. set explicit HTTP/2 resource budgets and add raw HTTP/1.1 desynchronization regression tests;
7. turn WebAuthn counter and backup-state changes into auditable risk signals, while expanding adversarial passkey tests;
8. continuously fuzz the security parsers and authentication/capability state machines.

These are application changes. This report excludes image pinning, container privileges, host firewall rules, router policy, and generic operating-system hardening.

## Priority and disposition

| Control | Disposition | Primary threat | Complexity | Compatibility risk |
| --- | --- | --- | --- | --- |
| Exact-origin CSRF plus session token | Implement now | Cookie-authenticated request forgery | Low | Low |
| Authorization generation and capability lineage | Implement now | Stale session/capability reuse after revocation | Medium | Low |
| WireGuard peer-to-Viewer binding | Implement now, after source-address proof | Stolen peer becoming a trusted network principal | Medium | Medium |
| Unique WireGuard per-peer PSK | Implement now | Later compromise of static public-key cryptography | Low | Low |
| Compound IPv4/IPv6 abuse principals | Implement now | Rate-limit evasion and attacker-induced collateral lockout | Medium | Low |
| Public-only outbound HTTP policy | Implement now | SSRF and DNS rebinding into LAN/loopback services | Medium | Low if adapters are classified correctly |
| Explicit HTTP/2 budgets and raw HTTP/1.1 tests | Implement now | Stream-reset DoS, compression state growth, request desynchronization | Low/medium | Low after client matrix |
| WebAuthn lifecycle signals and adversarial tests | Implement now | Passkey cloning, implementation mistakes, lockout | Medium | Low if signals do not auto-lock |
| Continuous state-machine fuzzing | Implement now | Parser and multi-step protocol edge cases | Test-only | None at runtime |
| Preserve hybrid post-quantum TLS defaults | Preserve and verify | Store-now/decrypt-later cryptanalysis | Low | Low with classical fallback |
| DPoP sender-constrained API tokens | Later, first-party clients only | Stolen native-client bearer token replay | High | High for Jellyfin clients |
| Externally anchored audit checkpoints | Later | On-host attacker rewriting audit history | Medium/high | Optional integration needed |
| Risk-adaptive step-up | Later and deterministic | Account takeover that still passes primary authentication | Medium | False positives |
| Mandatory attestation, mTLS, proof-of-work, AI kill decisions | Reject as defaults | Various | High | High |

## Implement now

### 1. Complete CSRF and exact-origin enforcement

The current [`unsafeCrossOrigin`](../../internal/server/security.go) fallback looks only for `kinosail_session` when `Origin` is absent. A direct-public browser uses `__Host-kinosail_session`, so a cookie-authenticated mutation with no `Origin` and no trustworthy `Sec-Fetch-Site` is not rejected by that branch. The same function compares only `origin.Host` to `request.Host`; it does not compare the complete scheme, host, and effective port.

OWASP recommends a synchronizer token for stateful applications, says `SameSite` is defense in depth rather than a general substitute, requires a fallback when Fetch Metadata is absent, and recommends rejecting requests when neither `Origin` nor `Referer` can establish the source ([OWASP CSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)).

Concrete design:

- Treat either session cookie name as cookie authentication.
- For every cookie-authenticated unsafe method, require both an exact expected origin and a session-bound unpredictable CSRF token. Compare the normalized scheme, ASCII host, and effective port; do not infer the public target from an unauthenticated forwarded header.
- Put the token in rendered forms and a custom header for HTMX/API requests. Never put it in a URL or audit record.
- Retain `SameSite=Strict`, Fetch Metadata, CSP, COOP, and CORP as independent layers.
- Use a separate `__Host-` ceremony cookie for public passkey ceremonies if the cookie path can become `/`; otherwise retain the narrowly scoped path and document why the prefix is unavailable.

Tests:

- Public `__Host-kinosail_session` plus missing `Origin`, missing Fetch Metadata, wrong scheme, wrong effective port, `Origin: null`, malformed origin, absent token, wrong token, and another session's token all fail before the handler runs.
- A valid public form and HTMX request succeed; bearer-token and Jellyfin header clients remain outside browser-CSRF handling.
- LAN aliases and the canonical public origin cannot authorize one another's cookies.

### 2. Use an authorization generation for immediate, compositional revocation

Closing sockets and deleting current sessions is necessary but does not express a single invariant across sessions, Quick Connect grants, playback IDs, HLS manifests, WebSockets, and future capabilities. NIST zero trust requires access decisions to depend on current subject, device, and resource policy rather than network location ([NIST SP 800-207](https://csrc.nist.gov/pubs/sp/800/207/final)); the 2025 OpenID CAEP final specification defines continuous signals such as session revocation so cooperating components can attenuate access without waiting for natural expiry ([OpenID CAEP 1.0 Final](https://openid.net/specs/openid-caep-1_0-final.html)). Kinosail is a monolith, so it can implement the same principle without a distributed event protocol.

Concrete design, inferred from those continuous-evaluation principles:

- Persist `publicAuthorizationGeneration` with security state. Increment it atomically when the public kill switch runs, a profile's remote permission changes, a passkey/password changes, a profile is removed, or authorization state becomes unreadable.
- Stamp every public session, pending Quick Connect request, playback capability, HLS authorization, and WebSocket with the generation at creation. Require equality at every use and renewal.
- Give each profile its own authorization revision as well as the global public generation. This avoids revoking every household Viewer when only one Viewer changes.
- Keep opaque random tokens and stored hashes. A generation is metadata, not a replacement for entropy.
- On persistence failure, fail closed for public creation/use and keep LAN recovery available. Do not increment only in memory and later reopen with an older persisted value.

Tests:

- Race kill/profile change against session validation, HLS renewal, range streaming, Quick Connect exchange, and WebSocket messages; no capability survives the committed generation change.
- Restart after kill and after a profile-policy change; old tokens still fail.
- Simulate state-write failure and corrupt generation state; the public boundary fails closed without breaking LAN recovery.

### 3. Bind WireGuard peers to application identities

WireGuard proves possession of a tunnel key; it does not identify which Kinosail profile may be used. NIST explicitly rejects implicit trust based on network location, and SP 800-207A calls for application/service identities in addition to network and user identity ([NIST SP 800-207A](https://csrc.nist.gov/pubs/sp/800/207/a/final)).

Concrete design:

- Pair a WireGuard peer to exactly one non-Owner Viewer profile and its fixed `10.91.0.x` source address. Preserve ordinary Kinosail authentication; the tunnel is not a login.
- When the application can cryptographically rely on WireGuard's `AllowedIPs` and can observe the original tunnel source address, tag the request with a server-created `wireguardPeerID`. Reject authentication as another Viewer or as an Owner on that channel.
- Keep Owner administration on local access by default. If remote Owner access is ever offered, make it a separately enabled policy requiring a recently verified passkey and a dedicated Owner peer—not a side effect of joining the tunnel.
- Fail closed when a request claims the WireGuard range but source preservation or peer mapping is ambiguous. The setup check must prove the source that reaches the application before enabling identity binding.
- Revoking a peer must also revoke its bound sessions/capabilities and advance its peer revision.

Tests:

- A paired source can authenticate only its bound Viewer; attempts to use another Viewer, Owner, API key, or an unbound source fail before a session is created.
- Duplicate addresses/keys, missing profile, profile conversion to Owner, revoked peer, source NAT collapse, and state corruption have no side effects.
- Confirm source preservation on the supported one-container Linux deployment before making this policy mandatory; otherwise leave WireGuard available only as transport while reporting the reduced assurance.

### 4. Add a unique WireGuard pre-shared key per peer

WireGuard supports an optional symmetric pre-shared key mixed into its handshake for an additional layer of post-quantum resistance ([WireGuard protocol](https://www.wireguard.com/protocol/)). WireGuard's own limitations page is careful that this does not provide forward-secure post-quantum secrecy by itself, so it should be described as an extra layer rather than “quantum-proof” ([WireGuard known limitations](https://www.wireguard.com/known-limitations/)).

Concrete design:

- Generate an independent 32-byte PSK for every pairing, render it as `PresharedKey` in both peer sections, store it only in the private WireGuard state/server configuration, and return it to the Viewer only in the one-time profile.
- Never include it in status JSON, HTML, logs, audit details, filenames, errors, or backups unless the backup is encrypted.
- Rotation means issuing a fresh peer profile with a new public key and PSK, then revoking the old peer. Avoid a protocol of Kinosail's own for PSK updates.

Tests:

- Client and server output contain the same valid PSK; two peers never share one.
- Redaction tests cover every API, settings, audit, notification, backup, and error surface.
- Failed persistence produces no returned profile and no partial server config.

### 5. Make abuse identity compound and IPv6-aware

Exact-IP-only accounting can be evaded with IPv6 temporary addresses. RFC 8981 deliberately rotates randomized interface identifiers while retaining a network prefix, and RFC 9099 notes that privacy addresses can obscure malicious activity and require suitable attribution procedures ([RFC 8981](https://www.rfc-editor.org/rfc/rfc8981.html), [RFC 9099](https://www.rfc-editor.org/rfc/rfc9099.html)). A prefix-only block, however, can punish many legitimate users behind one provider or carrier network.

Concrete design:

- Maintain bounded, expiring buckets for exact socket IP, IPv6 `/64` prefix, authenticated session, Viewer, credential ID/account, and endpoint class. For IPv4, keep exact-IP accounting and add only a coarser prefix signal after evidence from multiple addresses.
- Quarantine an exact source quickly for tripwire paths. Escalate a prefix only after failures from several distinct addresses in that prefix and never let prefix evidence trigger the global kill switch.
- Apply stricter limits to unauthenticated passkey starts, Quick Connect starts/polls, invalid capability use, and WebSocket handshakes than to an authenticated media stream.
- Bound both the number of buckets and distinct-address samples. Saturation rejects only public requests and coalesces alerts.
- Store privacy-minimized prefix/account hashes in metrics where full addresses are not needed; retain raw source information only in the protected audit retention window.

Tests:

- Many temporary IPv6 addresses in one `/64` cannot bypass login/tripwire limits; one bad address does not instantly quarantine the household/provider prefix.
- IPv4 NAT users, IPv4-mapped IPv6, zone identifiers, canonical-equivalent IPv6 text, malformed addresses, and bucket saturation behave deterministically.
- A successful authenticated stream is not terminated because unrelated anonymous sources fill the limiter.

### 6. Split outbound HTTP into public-only and explicitly local policies

[`allowedOutboundIP`](../../internal/server/security.go) currently rejects unspecified, multicast, and link-local addresses but permits loopback and private addresses. The shared client is used by metadata, subtitle, artwork, and notification adapters. Some installation-owned integrations intentionally need a local destination; remote-service responses and public-provider DNS must not receive that privilege.

This is a real trust-boundary class, not a hypothetical style concern. A 2024 USENIX Security measurement found almost all detected attacker-controlled server-request flows in its sample lacked effective SSRF defenses ([Wessels et al., “SSRF vs. Developers”](https://www.usenix.org/conference/usenixsecurity24/presentation/wessels)). OWASP recommends strict hostname allowlists plus validation of every resolved IPv4/IPv6 address to prevent private-network access and DNS pinning/rebinding ([OWASP SSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html)).

Concrete design:

- Create two explicit constructors: `publicProviderHTTPClient`, which rejects loopback, private, link-local, multicast, unspecified, CGNAT, IPv4-mapped private, and other special-purpose destinations; and `localIntegrationHTTPClient`, which permits a strictly configured local endpoint because the Owner requested it.
- Public-provider adapters must validate an exact HTTPS host allowlist before DNS, validate every A and AAAA answer, dial only an approved resolved address, repeat validation on every new connection, and reject redirects unless the destination independently passes the same policy.
- Never forward provider credentials across a redirect or to a download hostname unless that hostname is separately allowlisted for that credential.
- Keep DuckDNS on its fixed endpoint. Live TV/local webhooks remain explicit local integrations rather than silently weakening every outbound client.

Tests:

- Reject loopback, RFC 1918, CGNAT, IPv6 ULA, link-local, multicast, unspecified, IPv4-mapped forms, mixed public/private answer sets, DNS answer changes, userinfo, fragments, non-HTTPS, alternate ports, and redirect pivots.
- Prove no DNS lookup or request occurs for malformed/unallowlisted hosts.
- Prove explicitly local integrations still work only under their narrow configuration and cannot inherit public-provider credentials.

### 7. Set explicit HTTP/2 budgets and test HTTP/1.1 framing

The public middleware limits executing requests, but protocol state is allocated before and around handlers. Current Go exposes server controls for HTTP/2 concurrent streams and HPACK decoder/encoder table sizes ([Go `HTTP2Config`](https://go.dev/src/net/http/http.go?s=1536%3A1576)). Go's HTTP/2 Rapid Reset fix bounds simultaneously executing handler goroutines to the configured stream limit, which makes an explicit, measured limit useful defense in depth ([GO-2023-2102](https://pkg.go.dev/vuln/GO-2023-2102)).

HTTP parsing deserves wire-level regression tests even without a reverse proxy. RFC 9112 requires rejection/connection closure for invalid message framing and identifies `Transfer-Encoding` plus `Content-Length` ambiguity as a request-smuggling signal ([RFC 9112, Sections 6.3 and 11.2](https://www.rfc-editor.org/rfc/rfc9112.html#section-6.3)). Recent differential research found 17 HTTP desynchronization flaws across 19 implementations, with nine assigned CVEs, demonstrating that parser agreement cannot be assumed from ordinary handler tests ([Mu et al., HDHunter, USENIX Security 2025](https://www.usenix.org/system/files/usenixsecurity25-mu.pdf)).

Concrete design:

- Configure a measured public `MaxConcurrentStreams` and small HPACK decoder/encoder table limits. Keep the existing application-wide and per-source handler caps because HTTP/2 settings are per connection.
- Reject request bodies on public `GET`/`HEAD` unless an exact endpoint has a demonstrated need. Apply endpoint-specific body sizes below the global maximum.
- Keep HTTP/1.1 for client compatibility, but add raw-TCP/TLS tests for duplicate/conflicting `Content-Length`, `Transfer-Encoding` plus `Content-Length`, invalid chunking, obs-fold, whitespace variants, absolute-form targets, malformed authority, pipelined bytes after an error, and oversized trailers.
- Add HTTP/2 tests for excessive concurrent streams, reset loops, oversized header lists, continuation-frame abuse, slow headers, and connection shutdown during the kill switch. Rely on the standard library for parsing; do not add a second custom HTTP parser.

Tests should assert connection closure and absence of handler side effects, not only an HTTP status. Run them against the exact listener stack used by the application and any supported optional proxy mode.

### 8. Treat WebAuthn metadata as risk signals and test the whole ceremony

WebAuthn Level 3 defines backup eligibility/state and signature counters. A non-increasing nonzero counter can mean a clone, malfunction, or request race; the specification does not say it proves which authenticator is malicious ([W3C WebAuthn Level 3, signature-counter considerations](https://www.w3.org/TR/webauthn-3/#sctn-sign-counter)). NIST says public-facing services should not generally reject syncable authenticators solely because of backup state due to user-experience concerns, while recommending that relying parties understand these flags and retain appropriate recovery options ([NIST SP 800-63B-4, syncable authenticators](https://pages.nist.gov/800-63-4/sp800-63b/syncable/)).

Implementation quality matters as much as choosing WebAuthn. A 2026 USENIX Security study manipulated WebAuthn messages across 15 attack types; 18 of 103 evaluated relying parties had a critical finding and 53 had at least one high-severity finding ([Jannett et al., “The State of Passkeys”](https://www.usenix.org/system/files/conference/usenixsecurity26/sec26_prepub_jannett.pdf)).

Concrete design:

- Persist and display credential creation time, last use, backup eligibility/state, transports, and a privacy-safe label. Allow local/WireGuard removal and require a recent passkey ceremony for adding/removing another credential.
- Audit a non-increasing nonzero counter or an unexpected backup-state transition, notify the Owner once, and require a fresh passkey ceremony where practical. Do not auto-delete the credential or activate the global kill switch from this ambiguous signal.
- Recommend at least two Owner recovery authenticators locally. Do not require attestation or a device-bound passkey for ordinary Viewer public access.
- Confirm the library rejects wrong RP ID hash, wrong exact origin, wrong challenge, wrong ceremony/profile, missing user verification, replayed response, unregistered credential ID, malformed CBOR, disallowed algorithms, and expired state.

Tests:

- Build adversarial ceremony fixtures for every item above and prove failure before session creation or credential mutation.
- Exercise concurrent assertions from a syncable credential so a counter race cannot lock out a legitimate user.
- Verify passkey details and alerts never expose credential public-key material unnecessarily.

### 9. Continuously fuzz the security seams and state machines

Go's native coverage-guided fuzzing is intended to reach security-relevant edge cases humans miss, and minimized findings become reproducible corpus tests ([Go security fuzzing](https://go.dev/doc/security/fuzz/)). Stateful-protocol research found state-aware feedback explored an order of magnitude more state/transition sequences and found eight CVE-assigned bugs, supporting sequence fuzzing in addition to isolated parser fuzzing ([Ba et al., “Stateful Greybox Fuzzing,” USENIX Security 2022](https://www.usenix.org/system/files/sec22-ba.pdf)).

Start with deterministic, side-effect-free fuzz targets for:

- public Host/SNI/origin/forwarded-header normalization;
- `Range` parsing and authorization-before-I/O;
- WireGuard state/config parsing and peer/profile invariants;
- remote configuration and DuckDNS response parsing;
- playback capability parsing and generation checks;
- subtitle/provider URLs and public-only IP classification.

Then add model-based sequence fuzzers for:

- passkey begin/finish/replay/expiry/profile changes;
- Quick Connect begin/poll/approve/exchange/kill/restart;
- session creation/rotation/revocation/profile mutation;
- playback create/range/renew/logout/kill;
- tripwire strikes, expiry, prefix escalation, and map saturation.

Each sequence model should assert that unauthorized state never appears, rejected input produces no persistence or external request, maps remain bounded, secrets never enter output, and kill/revocation is monotonic. Run seed corpora in every test invocation and bounded fuzz campaigns in a scheduled security job; a short deterministic CI budget is more useful than an unbounded per-commit run.

## Preserve and verify

### 10. Keep Go's hybrid post-quantum TLS defaults

Kinosail's TLS configuration currently leaves `CurvePreferences` unset. That is the right shape for cryptographic agility. Go 1.24 enabled the hybrid `X25519MLKEM768` key exchange by default when curve preferences are nil, and Go 1.26 added hybrid P-256/ML-KEM and P-384/ML-KEM defaults ([Go 1.24 release notes](https://go.dev/doc/go1.24#crypto/tls), [Go 1.26 release notes](https://go.dev/doc/go1.26#crypto/tls)). ML-KEM is standardized by NIST in FIPS 203 ([NIST FIPS 203](https://csrc.nist.gov/pubs/fips/203/final)).

Do not pin only a post-quantum group or implement a KEM in Kinosail. Preserve runtime defaults with classical fallback, record the negotiated TLS version/cipher/group only in privacy-bounded aggregate diagnostics, and add an interoperability test using one hybrid-capable and one classical-only client. A test should fail if a later refactor populates `CurvePreferences` without an explicit compatibility/security review.

## Later or experimental

### 11. Sender-constrain first-party native API tokens with DPoP

DPoP binds an access token to a client's public key and a signed proof of the method, target URI, time, unique identifier, and token hash; the resource refuses requests whose proof or key binding is wrong ([RFC 9449](https://www.rfc-editor.org/rfc/rfc9449.html)). The current OAuth security BCP recommends sender-constrained tokens where feasible while noting that compromise of both token and key still defeats the property ([RFC 9700](https://www.rfc-editor.org/rfc/rfc9700.html)).

This is valuable for a future first-party native Kinosail client or automation API, not for today's browser cookie and Jellyfin compatibility paths:

- Issue a short opaque token bound to a client public-key thumbprint.
- Require `htm`, normalized `htu`, `iat`, unique `jti`, `ath`, and a server nonce; retain a bounded replay cache and rotate refresh tokens.
- Scope the token to one Viewer, channel, audience, and capability set; advance the authorization generation on revocation.
- Keep bearer compatibility disabled publicly by default. Do not silently accept DPoP as bearer when the proof is absent or invalid.

DPoP does not stop malicious script already running in the first-party browser from asking a non-extractable key to sign, and third-party Jellyfin clients do not implement it. HttpOnly public cookies and short opaque playback capabilities remain the simpler browser-compatible protection.

### 12. Add externally anchored audit checkpoints only as an optional integration

The current integrity-linked local journal can detect damage while its key and trusted state remain intact, but an attacker controlling the host and key can rewrite or truncate both. Forward-secure logging research uses evolving/erased keys to protect older records after later compromise ([Kelsey and Schneier, “Cryptographic Support for Secure Logs on Untrusted Machines”](https://www.usenix.org/conference/7th-usenix-security-symposium/cryptographic-support-secure-logs-untrusted-machines)). Merkle consistency proofs can establish that a later tree extends an earlier checkpoint, although an external witness is still needed to prevent a compromised host presenting a rewritten view ([RFC 9162, Merkle consistency proofs](https://www.rfc-editor.org/rfc/rfc9162.html#section-2.1.4)).

A later design could periodically sign a compact audit tree head and send it to an Owner-controlled notification target or export it for a companion verifier. Keep media titles, URLs, tokens, and full IPs out of the checkpoint. Do not claim that a Merkle tree stored only beside its own log protects against full host compromise.

### 13. Use deterministic risk signals only for step-up and notification

Risk signals such as a new client label, repeated invalid capabilities, counter regression, unusual request rate, or abrupt channel change can justify notification or a fresh passkey ceremony. They must not grant access, permanently lock an account, or trigger the global kill switch. NIST's authentication guidance treats risk indicators as additions to—not replacements for—required authentication and warns about throttling designs that enable denial of service ([NIST SP 800-63B-4, rate limiting](https://pages.nist.gov/800-63-4/sp800-63b.html#rate-limiting-throttling)).

Start with explainable rules, bounded state, explicit expiry, and a local Owner-visible reason. Do not add a remotely trained ML classifier: Kinosail lacks the representative per-installation data needed to validate false-positive and evasion behavior, and household travel/mobile networks make IP/geolocation signals unstable.

## Controls to reject as defaults

### Mandatory WebAuthn attestation or device-bound passkeys

Attestation can support managed-enterprise authenticator policy, but NIST says its absence should not block syncable authenticators for broad public-facing applications because that can push users toward weaker alternatives ([NIST syncable-authenticator guidance](https://pages.nist.gov/800-63-4/sp800-63b/syncable/)). Offer credential metadata and possibly an expert high-assurance policy later; do not make consumer Viewer access depend on an authenticator-vendor allowlist.

### Browser or Jellyfin mutual TLS

mTLS sender-constrains credentials but has poor consumer certificate provisioning/recovery and is not supported consistently by media clients. WireGuard already supplies the high-assurance paired-device transport. Keep mTLS for a future machine-to-machine API only if a concrete client requires it.

### Proof-of-work, CAPTCHAs, and remote attestation for every request

These add accessibility, privacy, battery, and compatibility costs while bot operators can distribute work or use solving services. Prefer bounded server work, phishing-resistant authentication, compound throttling, and source-scoped quarantine. No unauthenticated challenge result may activate the global kill switch.

### IP-bound sessions

IPv6 privacy addressing and mobile network changes make addresses unstable ([RFC 8981](https://www.rfc-editor.org/rfc/rfc8981.html)). Use IP/prefix only as an abuse/risk signal; cryptographically bind sessions to profile, channel, generation, and—where supported—a client key.

### Port knocking, “secret” URLs, and a larger honeypot

They do not replace authentication or reduce the consequences of a parsing flaw once discovered. The existing low-interaction tripwire is the safer design: no command execution, no believable data, no reflected payload, no global action controlled by a scanner.

### HTTP/3, ECH, or a custom post-quantum protocol solely for security branding

HTTP/3 adds a second public protocol implementation and UDP resource surface without removing the need for HTTP/1.1 client compatibility. Go supports server ECH, but DuckDNS setup does not provide a simple authenticated ECH key-distribution/DNS workflow, and the public hostname remains observable through DNS and certificate-transparency ecosystems ([Go 1.24 release notes](https://go.dev/doc/go1.24#crypto/tls)). Keep the public protocol surface small until a measured client or privacy requirement justifies either feature.

## Recommended implementation order

1. Exact-origin/public-cookie CSRF correction and session tokens.
2. Public/profile/peer authorization generations and derived-capability lineage.
3. Per-peer WireGuard PSKs, then peer-to-Viewer binding after proving source preservation.
4. Public-only egress client classification and complete special-address denial.
5. Compound IPv6-aware throttling.
6. Explicit HTTP/2 budgets and raw HTTP framing tests.
7. Passkey lifecycle UI/audit signals and adversarial ceremony suite.
8. Parser fuzz targets, followed by state-machine sequence fuzzers.
9. Hybrid TLS interoperability guard.
10. DPoP and external audit anchoring only when first-party clients/integrations exist.

## Completion evidence required

Do not call this tranche complete from unit tests alone. Required proof is:

- focused negative/no-side-effect tests at each new trust boundary;
- `go test ./...`, race detection for authorization/session/limiter state, and scheduled fuzz corpus execution;
- raw TLS HTTP/1.1 and HTTP/2 protocol tests against the real remote listener;
- one-container integration with original WireGuard source-address verification;
- browser tests for public passkey, CSRF, logout/revocation, and kill behavior;
- real off-LAN tests through changing IPv4/IPv6/mobile networks;
- Jellyfin client playback/seek/Quick Connect regression coverage;
- inspection proving PSKs, CSRF tokens, DPoP keys, session tokens, and audit keys never appear in status, HTML, logs, notifications, or unencrypted backups.

The residual boundary remains volumetric denial of service: application controls execute only after traffic reaches the host and cannot protect upstream bandwidth.
