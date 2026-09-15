# Simple, secure, broadly reachable remote streaming

Research snapshot: 2026-08-26. Repository implementation inspected at `3d41c965c926ce36643eee079bf0b2901d3a4a61`.

## Decision

A normal browser or Jellyfin-compatible client cannot be guaranteed to reach a Kinosail Server behind arbitrary residential NAT, carrier-grade NAT (CGNAT), and inbound firewalls unless at least one of these is true:

1. the Server has owner- or protocol-created public inbound reachability;
2. the Viewer joins a VPN or application-specific overlay;
3. the owner operates a reachable reverse proxy or tunnel; or
4. a reachable intermediary relays the connection.

This is a protocol constraint, not a missing router trick. The current TURN standard says direct communication can be impossible behind some NATs and an intermediate relay is then necessary; ICE uses relayed candidates for that case ([RFC 8656](https://www.rfc-editor.org/rfc/rfc8656.html#section-1), [RFC 8445](https://www.rfc-editor.org/rfc/rfc8445.html)). Peer-reviewed NAT-traversal work reached the same conclusion: hole punching is not universal, while relaying works whenever both endpoints can connect to the relay, at the cost of relay bandwidth and latency ([Ford, Srisuresh, and Kegel, USENIX 2005](https://www.usenix.org/legacy/events/usenix05/tech/general/full_papers/ford/ford_html/); [Guha and Francis, IMC 2005](https://www.usenix.org/legacy/event/imc05/tech/guha.html)).

For the requested “turn it on, share a URL, no Viewer VPN” experience, Kinosail should add an **optional managed L4 tunnel**:

- the Kinosail Server initiates an authenticated outbound tunnel to a reachable relay;
- a Viewer opens one stable, publicly trusted `https://` hostname in an ordinary browser or client;
- the relay routes opaque TCP/TLS bytes to the Server but does not terminate Viewer TLS;
- the Server remains the TLS endpoint and retains its certificate private key, authentication, authorization, playback, and audit state;
- the relay sees unavoidable connection metadata, but not HTTP, credentials, titles, artwork, subtitles, or media plaintext;
- disabling managed remote access closes the tunnel and revokes all public sessions and derived capabilities without changing LAN use.

This is still a media relay at the network/data-plane level. It changes Kinosail's current strict “no Kinosail-operated intermediary carries Library Content” decision into the narrower and defensible promise “no Kinosail service can read, terminate, cache, transform, or store Library Content.” It should not be described as direct access.

If Kinosail will not make that policy change, it must retain direct HTTPS and state plainly that remote access is **not universal**: CGNAT, double NAT, ISP filtering, IPv6-only mismatches, and owner router policy will leave some Servers unreachable.

“Universal” should itself be qualified. A relay makes access broadly reliable when both Server and Viewer can make ordinary Internet connections. It cannot overcome a total Internet outage, a network that blocks all usable outbound transports, an unsupported client, insufficient home upload capacity, or a relay/control-plane outage.

## Why the other reachability choices are not universal

| Path | Router work | CGNAT | Ordinary browser / Jellyfin HTTPS client | Privacy and operational result |
| --- | --- | --- | --- | --- |
| Public IPv4 plus manual port mapping | Required | Usually fails | Yes | Direct and simple for the Viewer, but exposes the home IP and makes setup router-specific. |
| Public IPv4 plus UPnP, NAT-PMP, or PCP | Automatic where supported | Usually fails | Yes after mapping | Router support and policy vary; the mapping deliberately admits inbound traffic. PCP explicitly permits gateways/ISPs to refuse popular ports, including 443 ([RFC 6887](https://www.rfc-editor.org/rfc/rfc6887.html)). Plex documents automatic mapping failures and a manual-forward fallback; Jellyfin recommends disabling UPnP unless specifically needed ([Plex Remote Access](https://support.plex.tv/articles/200289506-remote-access/), [Jellyfin setup](https://jellyfin.org/docs/general/post-install/setup-wizard/)). |
| Globally routed IPv6 plus firewall exception | Required on typical residential CPE | Avoids IPv4 CGNAT only | Only where the Viewer also has a usable IPv6 path | IPv6 addressability is not reachability. Residential gateways normally block unsolicited inbound connections until the owner creates an exception ([RFC 6092](https://www.rfc-editor.org/rfc/rfc6092.html), [RFC 7368](https://www.rfc-editor.org/rfc/rfc7368.html#section-2.3)). |
| ICE/STUN hole punching | No manual mapping when it succeeds | Sometimes | No, not transparently | Both endpoints need ICE signaling and candidate checks. A stock HTTPS client does not implement that Kinosail-specific exchange, and ICE still needs TURN relay candidates for hard NATs ([RFC 8445](https://www.rfc-editor.org/rfc/rfc8445.html), [RFC 8656](https://www.rfc-editor.org/rfc/rfc8656.html)). |
| Viewer VPN/overlay | No public app port | Yes | Only after installing/joining it | Strong and private, but violates the no-Viewer-VPN requirement. Keep WireGuard for Owners and higher-assurance managed devices. |
| Owner-operated VPS/reverse tunnel | No home inbound port | Yes | Yes | Technically sound, but not a simple mass-market setup; the owner must operate another Internet service. |
| Managed outbound L4 tunnel | None | Yes, assuming outbound Internet | Yes | Broadest compatibility and easiest setup. Relay bandwidth, service availability, metadata, and abuse handling become Kinosail responsibilities. |

The comparable products confirm the product tradeoff. Home Assistant says CGNAT leaves a public address or Home Assistant Cloud as the practical choices, and its Cloud path tunnels traffic without opening inbound home ports ([Home Assistant networking](https://companion.home-assistant.io/docs/troubleshooting/networking/#the-basics-how-the-app-talks-to-your-home-assistant)). Plex uses direct access when possible and falls back to an encrypted relay; Plex states that relay TLS remains end-to-end and the certificate stays on the media Server ([Plex Relay](https://support.plex.tv/articles/216766168-accessing-a-server-through-relay/)). Cloudflare Tunnel likewise demonstrates that an outbound-only connector can publish a service without a public origin address, though its usual HTTP product terminates traffic at Cloudflare and is therefore not the proposed Kinosail privacy boundary ([Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/networks/connectivity-options/)).

## Jellyfin-client compatibility constraint

An L4 tunnel can be transparent to an unmodified Jellyfin client because it changes only the route to the Server. It must not add a relay login, redirect, HTML interstitial, client SDK, VPN, or Kinosail-specific endpoint negotiation. Direct and relayed modes must present the same stable public HTTPS base URL, SNI name, Server identity, certificate chain, Jellyfin paths, authentication tokens, byte-range/HLS behavior, and WebSocket behavior. The Server—not the relay—must remain the HTTPS and Jellyfin API endpoint.

That stable origin has two distinct security and usability jobs. Browsers bind cookies and passkeys to an origin/RP ID. Native clients retain a Server base URL and token; changing between a LAN URL, direct-public URL, and relay URL can force another connection/login and can break absolute media URLs. DNS may route the stable name directly or through the relay, but the application origin and API contract must not change.

The current upstream client landscape is:

| Client | Current upstream status and device | Authentication constraint | Kinosail support boundary before a remote claim |
| --- | --- | --- | --- |
| Browser / bundled web | Jellyfin supports current Firefox, Chrome, Safari, and Edge families ([Jellyfin clients](https://jellyfin.org/docs/general/clients/)). | Kinosail passkey plus its server-side public cookie; same-origin HTTPS is mandatory. | Kinosail web/API remote access is independent of Jellyfin compatibility and should work when that integration is off. |
| Jellyfin for iOS | Official open-source iOS/iPadOS app; it is an Expo/web-wrapper client with native enhancements ([official downloads](https://jellyfin.org/downloads/clients/all/), [client repository](https://github.com/jellyfin/jellyfin-ios)). It is not the Apple TV app. | Upstream documents Quick Connect login and approval. | Kinosail records protocol coverage, but no physical iPhone/iPad remote certification. |
| Swiftfin | Official beta native client for iOS, iPadOS, and tvOS/Apple TV ([official downloads](https://jellyfin.org/downloads/clients/all/), [Swiftfin](https://github.com/jellyfin/Swiftfin)). | Upstream documents Quick Connect login on iOS and tvOS; only iOS can approve another device. | Kinosail records protocol-surface coverage only; no physical iOS or Apple TV certification. |
| Jellyfin for Android | Official Android mobile app, implemented around Jellyfin Web ([official downloads](https://jellyfin.org/downloads/clients/all/), [Android repository](https://github.com/jellyfin/jellyfin-android)). | Upstream documents Quick Connect login and approval. | Kinosail records protocol coverage; physical-device and remote fault testing remain pending. |
| Jellyfin for Android TV | Official Android TV, Nvidia Shield, and Fire TV client ([Android TV repository](https://github.com/jellyfin/jellyfin-androidtv)). | Upstream documents Quick Connect login, not approval. | Kinosail records stable-URL/auth/playback protocol coverage; physical-TV certification remains pending. |
| Jellyfin for Roku | Official Roku client ([Roku repository](https://github.com/jellyfin/jellyfin-roku)). | Upstream documents Quick Connect login, not approval. | Kinosail's current compatibility matrix does not claim Roku protocol or physical-device coverage. |
| Jellyfin for webOS | Official LG webOS client. It probes the manifest/public server info, then hands control to the web UI hosted by the Server ([webOS repository](https://github.com/jellyfin/jellyfin-webos)). | Upstream documents Quick Connect login and approval. | Currently unsupported because Kinosail does not host the Jellyfin Web application the released wrapper expects. A tunnel cannot repair this application-layer gap. |
| Jellyfin for Tizen | Official Samsung Tizen client; its package embeds a matching build of Jellyfin Web ([Tizen repository](https://github.com/jellyfin/jellyfin-tizen)). | Treat as a Jellyfin Web API consumer and test the exact store build. | Kinosail currently claims neither complete Web API parity nor physical Tizen certification. The packaged client's contract must be captured before support is claimed. |

Jellyfin's current Quick Connect matrix is the source of the login/approval distinctions above ([Jellyfin Quick Connect](https://jellyfin.org/docs/general/server/quick-connect/)). It is the right television login shape: the limited-input client displays a code, and an already authenticated device approves it. Kinosail must retain its stricter rule that public Quick Connect can create only a remote-enabled Viewer session, never an Owner session.

### Compatibility flag must fail closed

All Jellyfin compatibility remains behind the existing `JellyfinCompatibility` setting. Disabling it must make discovery/public info, password authentication, Jellyfin Quick Connect, user/library/item/artwork, playback negotiation, media/range/HLS/subtitle delivery, progress/favorite mutations, Live TV, and any future compatibility route return the same generic unavailable response on both LAN and the public listener, before authentication state, counters, playback work, or other side effects. It must not disable Kinosail's own web or `/api/v1` remote surface.

The current middleware mostly provides that boundary by rejecting `jellyfinPath` before the protected application handler. The inspected baseline had two exact gaps:

- `GET /system/info/public` is registered as a lowercase native-client probe, but `jellyfinPath` recognized only the uppercase `/System/` prefix. The focused patch accompanying this note closes that leak and expands disabled-flag coverage across all current compatibility route families.
- Disabling the setting only persists a Boolean. It does not invalidate already issued Jellyfin sessions, pending Quick Connect records, or in-memory playback/capability state. The route gate makes most of that state inert while disabled, but re-enabling can revive still-valid credentials or capabilities. A fail-closed lifecycle should revoke or generation-invalidate all compatibility-derived state when the switch goes off.

Before release, use a table-driven inventory generated from every registered Jellyfin route and run every method/path twice—LAN and public listener—with the flag off. Assert a generic `404`, no token/session/Quick Connect/playback creation, no progress/list mutation, no file read or transcode, no audit-secret leak, and unchanged Kinosail web/API behavior. Repeat client acceptance for both direct and L4-relayed routing; transport transparency does not establish client compatibility.

## Current Kinosail implementation

The present direct-public design is much stronger than forwarding the LAN listener indiscriminately:

- [`packages/remoteaccess`](../../../../packages/remoteaccess) owns a dedicated public HTTPS listener, DuckDNS updates, exact SNI and Host matching, ACME certificate issuance, TLS 1.2/1.3, bounded connections, explicit HTTP/2 budgets, and a persistent kill marker.
- [`internal/server/security.go`](../../internal/server/security.go) marks public requests through server-created context rather than a Viewer-supplied header, validates exact request origins, applies session CSRF, strict range parsing, request limits, IPv6-aware source limits, and local/public outbound-network policies.
- [`internal/server/auth_remote_routes.go`](../../internal/server/auth_remote_routes.go) and [`internal/server/auth.go`](../../internal/server/auth.go) apply a public route inventory, reject Owner and API-key sessions, require a distinct public session, and re-evaluate Viewer policy.
- [`internal/server/passkeys.go`](../../internal/server/passkeys.go) permits public passkey login only for remote-enabled non-Owner Viewers; [`internal/server/auth_login.go`](../../internal/server/auth_login.go) rejects public password login. Quick Connect is the compatibility path for devices that cannot perform WebAuthn.
- [`internal/server/sessions.go`](../../internal/server/sessions.go) creates server-side opaque public sessions with an eight-hour absolute limit, a ten-session per-Viewer cap, strong-authentication state, and the current Profile revision.
- [`internal/server/media_shares.go`](../../internal/server/media_shares.go) has selected-item, expiring, revocable, device-limited grants with hashed claim/session tokens. The browser claim secret is carried in the URL fragment and removed from browser history before it is submitted.
- [`internal/server/api_remote.go`](../../internal/server/api_remote.go) makes the kill switch close the public listener and revoke public sessions, public passkey ceremonies, remote Quick Connect state, and all Media Shares.

These are valuable application controls and should be reused behind a tunnel. Network location must not replace them: NIST zero trust explicitly rejects implicit trust based solely on LAN versus Internet location and requires authentication and authorization before a resource session ([NIST SP 800-207](https://csrc.nist.gov/pubs/sp/800/207/final)).

### Exact current gaps against the requested experience

1. **It is direct-only.** [`internal/server/api_remote.go`](../../internal/server/api_remote.go) reports `directOnly: true`; there is no connector, reverse tunnel, or relay fallback. It therefore cannot solve CGNAT or blocked inbound traffic.
2. **Setup is not one-click.** [`scripts/setup-remote-access.sh`](../../scripts/setup-remote-access.sh) sends the Owner to DuckDNS, asks them to create a name and copy a long-lived account token, rewrites environment configuration, requires a restart, and instructs them to forward public TCP 443. Each of those is a support and security failure point.
3. **“Ready” is not an external reachability proof.** [`internal/server/remote_readiness.go`](../../internal/server/remote_readiness.go) checks local state, listener state, certificate lifetime, policy, and a Viewer passkey, but does not prove a fresh connection from outside the home network. A correct local listener and DDNS update can still be unreachable behind CGNAT, double NAT, or a firewall.
4. **The first public certificate depends on inbound reachability.** The direct listener uses an ACME TLS challenge on public 443. DNS-01 can issue without inbound HTTP/TLS reachability because control is proven with a DNS TXT record ([RFC 8555, section 8.4](https://www.rfc-editor.org/rfc/rfc8555.html#section-8.4)). Managed tunnel mode needs automated DNS-01 with a credential scoped to exactly one Server name, not a reusable DuckDNS account token.
5. **The home address and access link are exposed to Internet scanning and volumetric traffic.** Application limits help after traffic reaches Kinosail, but they cannot keep a flood off the home connection. CISA recommends minimizing and continuously assessing Internet-exposed services ([CISA Internet Exposure Reduction Guidance](https://www.cisa.gov/resources-tools/resources/exposure-reduction)). A relay can absorb connection floods upstream, subject to its own quotas and capacity.
6. **There is no tunnel identity, rotation, revocation, regional failover, or relay privacy contract.** Those become required security state as soon as a managed connector exists.
7. **Keep management integrations off Viewer ingress.** The inspected baseline exposed SCIM discovery and profile provisioning through the public listener. The accompanying change blocks every SCIM route before bearer processing and proves that a rejected mutation has no effect. OWASP recommends avoiding Internet-exposed management endpoints and using a deny-by-default authorization policy checked on every request ([OWASP REST Security](https://cheatsheetseries.owasp.org/cheatsheets/REST_Security_Cheat_Sheet.html#management-endpoints), [OWASP Authorization](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html)). Keep SCIM on the local or separately protected integration path.
8. **Disabling direct configuration is host-script driven.** The existing in-app kill switch is strong once the listener exists, but full enable/disable still depends on Compose/environment lifecycle. Managed mode needs an Owner-visible claim, connect, disable, revoke, and delete lifecycle through the shared application/API operation.

## Recommended architecture

```text
ordinary browser or Jellyfin client
              |
        HTTPS / TLS 1.2-1.3
              |
     Kinosail L4 relay on TCP 443
       (cannot terminate Viewer TLS)
              |
 authenticated multiplexed outbound tunnel
       initiated by the Server container
              |
   dedicated public Kinosail handler
              |
 local identity, policy, media, audit, keys
```

### 1. One stable remote origin

Assign an opaque random name such as `s-<random-128-bits>.remote.kinosail.example`, not an Owner name, address, or Library name. The same origin is used for browser login, passkeys, Media Shares, HLS/range playback, WebSockets, and compatible clients. Stable origin matters because WebAuthn credentials are scoped to an RP ID and the relying party must validate the exact expected origin ([W3C WebAuthn Level 3](https://www.w3.org/TR/webauthn-3/#sctn-validating-origin)).

The Server generates and retains the TLS private key. It obtains and renews a publicly trusted certificate with ACME DNS-01 through a narrowly scoped, signed challenge API that can modify only `_acme-challenge` for that Server name. ACME standardizes automated issuance and revocation, and DNS-01 proves control with a challenge-specific TXT record ([RFC 8555](https://www.rfc-editor.org/rfc/rfc8555.html)). Never give the Server a zone-wide DNS credential and never send the private key to the relay.

Use TLS 1.3 preferentially and retain hardened TLS 1.2 only for the required embedded-client matrix. The IETF TLS BCP requires TLS 1.2 support for broad interoperability, prefers TLS 1.3, forbids older versions, and recommends strict TLS/HSTS ([RFC 9325](https://www.rfc-editor.org/rfc/rfc9325.html#section-3.1.1)).

### 2. Owner claim without secrets or router work

Remote access stays off until an Owner reauthenticates locally with a passkey and presses **Enable remote streaming**. The Server creates a hardware-independent asymmetric connector identity locally. Use an OAuth device-style claim flow: a short code and QR/deep link, explicit Server name, clear approve/deny choice, bounded polling, and short expiry. RFC 8628 defines that human-friendly pattern and its phishing mitigations ([RFC 8628](https://www.rfc-editor.org/rfc/rfc8628.html)).

After approval, issue short-lived, audience-restricted, sender-constrained connector credentials. OAuth's current security BCP recommends asymmetric client authentication, least-privilege audiences, and mTLS- or DPoP-bound tokens so a stolen bearer value is not sufficient ([RFC 9700](https://www.rfc-editor.org/rfc/rfc9700.html#section-2.2), [RFC 8705](https://www.rfc-editor.org/rfc/rfc8705.html)). Persist only the Server private identity and renewable grant in the existing private configuration volume. Support key rotation by overlap, proof of possession, and explicit retirement of the old key.

### 3. Server-originated, opaque L4 transport

The single Kinosail container opens at least two outbound connector sessions to independent relay failure domains. Prefer QUIC for independently flow-controlled multiplexed streams and connection migration, with a TCP/TLS 443 fallback for networks that block UDP. QUIC is a secure multiplexed transport with per-stream and connection flow control ([RFC 9000](https://www.rfc-editor.org/rfc/rfc9000.html)).

For every Viewer TCP connection, the relay opens one tunnel stream and copies bytes unchanged. HTTP defines a tunnel as a blind relay that does not change messages and can carry end-to-end TLS through an intermediary ([RFC 9110, section 3.7](https://www.rfc-editor.org/rfc/rfc9110.html#section-3.7)). The outer connector encryption protects the tunnel control plane; the inner Viewer-to-Server TLS protects application content from the relay.

The authenticated stream envelope may carry the client socket IP and connection identifier. Kinosail must obtain that metadata from the authenticated tunnel object and synthesize the connection's `RemoteAddr`; it must not trust Viewer-provided `Forwarded` or `X-Forwarded-For`. The existing `Remote` context marking, dedicated public handler, and public route policy then apply unchanged.

Do not add relay-side HTTP parsing, TLS termination, caching, HLS awareness, transcoding, thumbnail generation, URL logging, or content moderation hooks. Those would turn the relay into a plaintext media processor and enlarge both breach impact and compliance scope.

### 4. Authentication that is secure and understandable

Keep public password login disabled. A stable HTTPS origin plus a discoverable passkey gives phishing-resistant authentication through verifier-name binding; NIST identifies WebAuthn as phishing-resistant and requires account-level throttling and immediate authenticator invalidation capability ([NIST SP 800-63B-4](https://pages.nist.gov/800-63-4/sp800-63b/authenticators/)).

Passkeys are not automatically safe merely because a library is used. A 2026 USENIX study found critical findings at 18 of 103 evaluated relying parties and high-severity findings at 53, including session fixation, credential-management, and account-takeover weaknesses ([Jannett et al., “The State of Passkeys,” USENIX Security 2026](https://www.usenix.org/conference/usenixsecurity26/presentation/jannett)). Retain exact RP ID/origin validation, one-use bounded ceremonies, user verification, duplicate-credential rejection, credential lifecycle/audit, and adversarial ceremony tests.

Keep Quick Connect for TVs and compatibility clients that cannot perform WebAuthn, but require approval from a recently passkey-authenticated local/WireGuard session and bind the resulting session to the requested non-Owner Viewer, device, public channel, Profile revision, and short expiry. Never make an Owner remotely approvable through the same public listener.

For a new persistent Viewer, add an Owner-created invitation that creates no authority until claimed. Use a cryptographically random, hashed, single-use, short-lived token; after claim, require the Viewer to enroll a passkey or finish locally approved Quick Connect. OWASP's analogous account-recovery guidance requires URL tokens to be random, stored securely, single-use, expiring, and rate-limited ([OWASP Forgot Password](https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html)). Reuse the current Media Share fragment pattern so claim secrets do not enter ordinary request URLs, history, referrers, or relay logs.

### 5. Authorization and sharing

Preserve separate authorization forms:

- **Viewer Profile:** persistent identity with explicit remote permission, selected Libraries, content rating, viewing hours, download permission, transcoding permission, and concurrency/bandwidth policy.
- **Quick Connect:** one-time device authorization that ends in a Viewer session, never in an Owner session.
- **Media Share:** an Owner-created capability for explicit item IDs, short expiry, small device/concurrency cap, no Library browsing, no downloads, and immediate revoke.

Every request, range, HLS manifest/segment, subtitle, artwork response, WebSocket message, and playback renewal must re-evaluate the current Profile/share policy or a capability stamped with the current authorization revision. OWASP requires deny-by-default authorization and validation on every request; NIST zero trust says access decisions cannot rest on network location ([OWASP Authorization](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html), [NIST SP 800-207](https://csrc.nist.gov/pubs/sp/800/207/final)).

Maintain global public and per-Viewer authorization generations. Enabling, disabling, remote-permission change, Profile disable/delete, credential compromise, connector revoke, or Media Share revoke advances the appropriate generation. A stale cookie, playback ID, HLS URL, Quick Connect result, share session, or WebSocket must then fail on its next use; disabling remote access also actively closes long-lived connections.

### 6. Abuse and availability

The blind relay can enforce only transport-level policy: valid registered SNI, TCP/TLS handshake deadlines, exact IP and IPv6-prefix connection limits, per-Server connection/byte ceilings, idle deadlines, maximum tunnel streams, egress quotas, and regional circuit breakers. Because it cannot see HTTP, it cannot replace Kinosail's endpoint, account, credential, session, range, or transcoder limits.

Kinosail should retain bounded request bodies, strict single-range parsing, connection and HTTP/2 budgets, scanner quarantine, and expensive-work admission. Replace fixed-window login accounting with bounded token/sliding buckets across exact source, IPv6 prefix, Viewer/account, credential, session, endpoint class, and Server. OWASP describes rate limiting as a multilayer control and recommends keys beyond IP; NIST requires effective per-account failed-authentication throttling ([OWASP Bot Management](https://cheatsheetseries.owasp.org/cheatsheets/Bot_Management_and_Anti-Automation_Cheat_Sheet.html#rate-limiting-and-quotas), [NIST SP 800-63B-4](https://pages.nist.gov/800-63-4/sp800-63b/authenticators/#rate-limiting-throttling)).

An attacker must never be able to trigger a global kill switch by sending hostile traffic. Quarantine the source/prefix, shed public load, and notify the Owner. Reserve Server CPU, PIDs, memory, disk I/O, transcode slots, and upload bandwidth for active authenticated playback and local recovery. Relay plans must have explicit per-Server bandwidth and concurrency bounds; Plex's published remote controls similarly treat home upload and per-stream limits as core product inputs ([Plex bandwidth controls](https://support.plex.tv/articles/227715247-server-settings-bandwidth-and-transcoding-limits/)).

### 7. Session, CSRF, and browser boundary

Keep opaque server-side session tokens in `Secure; HttpOnly; SameSite=Strict; Path=/; __Host-` cookies, with server-enforced idle and absolute expiry, session listing, per-device revoke, and rotation after authentication or privilege change. This follows both NIST and OWASP session guidance ([NIST session management](https://pages.nist.gov/800-63-4/sp800-63b/session/), [OWASP Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)).

For every cookie-authenticated unsafe method, require an exact canonical origin plus a session-bound unpredictable CSRF token. `SameSite`, Fetch Metadata, CSP, and Origin/Referer validation are independent layers, not substitutes for the token ([OWASP CSRF Prevention](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)). Keep public error responses generic and do not reveal whether a Viewer name, passkey, share, or Server exists.

### 8. Disable, revoke, recover, and delete

One Owner action must do all of the following atomically or fail closed:

1. stop accepting new relay streams and direct-public connections;
2. close active public connections;
3. revoke the connector grant and its renewable credentials;
4. advance global public authorization generation;
5. revoke public Viewer sessions, Quick Connect requests/results, passkey ceremonies, Media Shares, playback sessions, HLS capabilities, and WebSockets;
6. remove public DNS routing and request ACME certificate revocation when the remote name is permanently deleted; and
7. preserve LAN access and local recovery.

Temporary relay loss must not erase configuration or disable LAN playback. Connector reconnect uses bounded exponential backoff with jitter and resumes only after authenticating both sides. A compromised connector key is rotated from a local Owner session; a lost control-plane account cannot silently claim a Server without local proof of possession.

### 9. Privacy and trust statement

End-to-end TLS is a meaningful improvement over an HTTP reverse proxy, but it is not “zero knowledge.” TLS 1.3 protects content confidentiality and integrity but does not hide record length; a relay necessarily observes Viewer and relay IPs, the Server routing name/SNI unless a future compatible mechanism hides it, connection times, duration, direction, byte counts, and failures ([RFC 8446](https://www.rfc-editor.org/rfc/rfc8446.html#section-1)).

The relay/control plane should therefore store only:

- opaque Server and account identifiers;
- connector key/certificate identifiers and revocation state;
- chosen region and coarse health state;
- security/abuse counters and coarse byte totals with a short published retention; and
- billing state if the service is metered.

It should not receive Profiles, credentials, passkey public keys, item IDs, titles, paths, artwork, subtitles, request URLs, query strings, playback position, Library inventory, or audit content. No media caching or replay is possible without the Server TLS key. Logs use opaque connection IDs and truncated/pseudonymized network identifiers where incident response permits.

There is still a DNS/Web-PKI trust limit: if Kinosail controls the parent DNS zone, a malicious or compromised control plane could redirect the name and seek another public certificate. Browser WebAuthn is verifier-name-bound, not pinned to one Server public key. Mitigations are tightly separated DNS/relay privileges, hardware-backed control-plane keys, CAA and Certificate Transparency monitoring, short-lived certs, incident revocation, optional Owner-controlled custom domains, and certificate/server-identity pinning in first-party native clients. These reduce but do not erase the hosted domain owner's power. This limitation must appear in the threat model and privacy promise.

## Product shape

Offer three explicit modes, not one ambiguous “Remote” switch:

1. **Managed remote streaming — recommended for most households.** One Owner claim, no router changes, stable URL, outbound opaque tunnel, ordinary Viewer browser/client, optional paid bandwidth because Kinosail carries encrypted bytes.
2. **Direct public HTTPS — advanced.** Preserve the existing DuckDNS/ACME listener for owners who have public reachability and accept port/firewall management, home-IP disclosure, and no reachability guarantee.
3. **WireGuard — higher assurance.** Preserve for Owner administration and managed devices; do not present it as the simple Viewer path.

Managed mode should say “traffic is relayed when you are away from home; Kinosail's relay can see connection metadata but cannot decrypt or cache your media.” Direct mode should say “no Kinosail relay carries traffic, but this may not work behind CGNAT or restrictive routers.” Those are materially different privacy and availability choices.

For first-party Kinosail clients, a later direct-first optimization can probe a separately authenticated direct endpoint and fall back to the managed relay. Do not make it the initial architecture: ordinary browsers and third-party clients need one stable HTTPS origin and cannot perform a Kinosail-specific ICE or endpoint-selection protocol. The stable managed hostname should always work through the relay; optimization is secondary to correctness.

## Required evidence before calling it secure and easy

### Reachability matrix

- public IPv4 with manual mapping, PCP/UPnP available, PCP/UPnP unavailable, double NAT, and RFC 6598 CGNAT;
- global IPv6 with default-deny firewall, IPv6-only Server with IPv4-only Viewer, and changing IPv6 prefix;
- Server outbound UDP allowed, UDP blocked with TCP 443 allowed, authenticated proxy, and captive/restricted networks;
- cellular and residential Viewers on IPv4, IPv6, and dual stack; and
- at least two relay regions plus loss and recovery of each tunnel.

For every case, prove fresh off-LAN sign-in, original and adaptive playback, seek-to-moving-picture, range resume, HLS renewal, subtitle/audio tracks, WebSocket reconnect, and kill/revoke behavior. “DDNS updated” or “tunnel connected” is not proof that a Viewer can stream.

### Security and privacy matrix

- packet capture at the relay proves no HTTP path, header, cookie, token, title, or media plaintext is available;
- only the Server possesses the Viewer-facing TLS private key;
- stolen connector tokens fail without the connector private key; old keys fail after rotation;
- untrusted forwarding headers cannot spoof public/local identity or source address;
- Owner, setup, MCP, OAuth administration, backups, diagnostics, SCIM, and configuration remain unavailable on managed Viewer ingress;
- policy and generation changes invalidate cookies, HLS, range, playback, share, Quick Connect, and WebSocket state during concurrent use and after restart;
- malformed TLS, tunnel frames, HTTP/1.1 framing, HTTP/2 streams, ranges, bodies, and state files fail before side effects;
- an external penetration test covers the relay, claim flow, ACME/DNS delegation, passkeys, client compatibility, and application boundary; and
- control-plane storage and logs are inspected to prove the documented data-minimization contract.

### Performance and operations matrix

- measure direct versus relay click-to-first-moving-frame, seek recovery, sustained bitrate, CPU, allocations, tunnel overhead, and home/relay bandwidth;
- test direct play and bounded transcoding across representative 1080p and 4K sources without starving local playback;
- rotate connector keys and certificates, revoke and reclaim a Server, restore a backup, update the one-container Server, and recover from relay/DNS/ACME outages; and
- run the complete API, browser, container, populated-instance, race, fuzz, and physical-client gates already required by Kinosail.

## Primary sources

- [RFC 8656 — TURN](https://www.rfc-editor.org/rfc/rfc8656.html)
- [RFC 8445 — ICE](https://www.rfc-editor.org/rfc/rfc8445.html)
- [RFC 6887 — Port Control Protocol](https://www.rfc-editor.org/rfc/rfc6887.html)
- [RFC 6092 — residential IPv6 gateway security](https://www.rfc-editor.org/rfc/rfc6092.html)
- [RFC 7368 — IPv6 home networking architecture](https://www.rfc-editor.org/rfc/rfc7368.html)
- [RFC 9110 — HTTP semantics and blind tunnels](https://www.rfc-editor.org/rfc/rfc9110.html#section-3.7)
- [RFC 9000 — QUIC](https://www.rfc-editor.org/rfc/rfc9000.html)
- [RFC 8446 — TLS 1.3](https://www.rfc-editor.org/rfc/rfc8446.html)
- [RFC 9325 — TLS deployment best current practice](https://www.rfc-editor.org/rfc/rfc9325.html)
- [RFC 8555 — ACME](https://www.rfc-editor.org/rfc/rfc8555.html)
- [RFC 8628 — OAuth Device Authorization Grant](https://www.rfc-editor.org/rfc/rfc8628.html)
- [RFC 8705 — OAuth mTLS and certificate-bound tokens](https://www.rfc-editor.org/rfc/rfc8705.html)
- [RFC 9700 — OAuth security best current practice](https://www.rfc-editor.org/rfc/rfc9700.html)
- [W3C WebAuthn Level 3](https://www.w3.org/TR/webauthn-3/)
- [NIST SP 800-63B-4 — Authentication and Authenticator Management](https://pages.nist.gov/800-63-4/sp800-63b.html)
- [NIST SP 800-207 — Zero Trust Architecture](https://csrc.nist.gov/pubs/sp/800/207/final)
- [CISA Internet Exposure Reduction Guidance](https://www.cisa.gov/resources-tools/resources/exposure-reduction)
- [OWASP Authorization Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html)
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [OWASP CSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)
- [OWASP Bot Management and Anti-Automation Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Bot_Management_and_Anti-Automation_Cheat_Sheet.html)
- [Ford, Srisuresh, and Kegel — “Peer-to-Peer Communication Across Network Address Translators,” USENIX 2005](https://www.usenix.org/legacy/events/usenix05/tech/general/full_papers/ford/ford_html/)
- [Guha and Francis — “Characterization and Measurement of TCP Traversal Through NATs and Firewalls,” IMC 2005](https://www.usenix.org/legacy/event/imc05/tech/guha.html)
- [Jannett et al. — “The State of Passkeys,” USENIX Security 2026](https://www.usenix.org/conference/usenixsecurity26/presentation/jannett)
- [Plex Remote Access](https://support.plex.tv/articles/200289506-remote-access/)
- [Plex Relay](https://support.plex.tv/articles/216766168-accessing-a-server-through-relay/)
- [Home Assistant networking](https://companion.home-assistant.io/docs/troubleshooting/networking/)
- [Jellyfin networking](https://jellyfin.org/docs/general/post-install/networking/)
- [Cloudflare Tunnel connectivity](https://developers.cloudflare.com/cloudflare-one/networks/connectivity-options/)
