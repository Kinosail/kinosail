# Public HTTPS setup and security audit — 2026-09-14

Scope: Player's standard TCP 443 public HTTPS installation, the shared remote-access setup model, and the matching Subtitles authentication boundary. Source review only. No live router changes, attack traffic, certificate issuance, deployment acceptance, or physical-device certification were performed. `.gates-disabled` remains in place; all tests and browser checks are unrun.

## Findings fixed

1. **High: provisioning was reachable on the public listener.** Both apps registered every SCIM route in `publicRoutes`, bypassing normal Viewer authorization. The remote deny list omitted SCIM. `scim.protect` still required a valid, unexpired provisioning token, so this was not anonymous provisioning; possession of that separate token could nevertheless allow public Profile enumeration and lifecycle changes. A shared, explicit allowlist now restricts which bootstrap/capability routes may bypass Viewer authorization on public ingress. SCIM remains available locally/WireGuard. A future local integration does not automatically become public by joining `publicRoutes`.
2. **Recovery/UX: local browser navigation could be redirected into the public boundary.** The HTTPS wizard sets the canonical authentication address to the public DuckDNS origin. Canonical page redirects then sent local browser requests there, where Owner administration is intentionally blocked. Dedicated public HTTPS now leaves local page navigation local. Host allowlists, TLS, CSRF, and WebAuthn's exact-origin checks remain enforced.
3. **Setup dead end: public passkey enrollment was instructed but prohibited.** The previous checklist instructed registration on the public listener, while the route policy blocks registration. First-device instructions use a local authenticator-secured Viewer to approve Quick Connect in a browser or native/compatible app. Readiness accepts either an authenticator or a passkey and excludes disabled Viewers. Public registration remains blocked.
4. **Setup clarity:** the shared web/API checklist includes router field/value pairs, six official manufacturer guides, the configured and strictly validated public address, cellular testing, failure-specific help, and an off-state that does not tell the Owner to reopen ports. Scripts and owner guides use the same mapping and approval flow.

5. **Browser sign-in dead end:** the public page showed a password form that the server rejects. Both apps now show a browser Quick Connect flow with a code, local approval instructions, cancellation, expiry, retry, and accessible status. The polling secret is kept in a short-lived Secure, HttpOnly, SameSite=Strict host cookie; it is absent from the page, URL, and API response. The three new public POST routes require exact HTTPS Origin, same-origin fetch metadata when present, and an empty body/query. They do not accept a caller-supplied polling secret. Public password sign-in and remote approval/enrollment stay blocked; existing passkey and configured SSO choices remain available.
6. **Quick Connect revocation race:** the Profile revision check preceded session issuance under a separate lock. Public native and browser Quick Connect now check the approved revision under the same Profile/session lock that persists the new session. A revocation between lookup and issuance produces no session, token, or cookie.
7. **Shutdown/re-enable race:** resetting the kill switch cleared the running Manager's stop flag, so an outstanding startup retry could reopen it without the required restart. Kill/reset marker operations are now serialized; resetting requires a stopped Manager, keeps that process stopped, and leaves a new process to restart. Pending DNS retries also stop when killed. Listener addresses reject malformed/named/zero/out-of-range ports and nonliteral network hosts before certificate state or network work; empty bind host and localhost remain supported.
8. **Recovery scripts:** setup reruns preserve the original local authentication address, including the automatic/empty default. Disable selects the installed source or release Compose file and stops the old service before recreation, so a failed recreation cannot leave the previous public listener serving. It stops active public HTTPS before restarting locally. The CLI does not claim to revoke persisted sessions: use the Settings kill switch for authorization revocation, and remove the router rule when stopping access.

## Security controls traced in current source

| Boundary | Evidence and limitation |
| --- | --- |
| Public ingress | `packages/remoteaccess/transport.go` uses a dedicated TLS listener, exact public Host, SNI-constrained certificate selection, TLS 1.2/1.3, header/read/idle timeouts and explicit HTTP/2 budgets. `identitycore.Remote` marks context; client forwarding headers do not make a public request local. |
| Viewer authorization | `auth.go`, `auth_remote_routes.go`, and `identitycore/access.go`: Owners and API keys denied publicly; strong public-channel sessions required; remote permission, schedule, and route policy enforced. API scope maps include personal playlists/progress and optional downloads, so this is broader than literally only playback. |
| Unauthenticated/capability routes | New `identitycore/public_routes.go` contains exact patterns for sign-in, assets, compatibility bootstrap, and media capabilities. Capability handlers still perform their own grant/session checks. SCIM, MCP/OAuth management, setup, local discovery/casting, and pairing approval are not public entry points. OIDC/SAML sign-in remains intentional. |
| Browser isolation | `identitycore/http.go`, app `security.go`, and `identitycore/session_http.go`: CSP, no-store, no-referrer, frame denial, secure HttpOnly SameSite cookies, same-origin checks, CSRF enforcement, and bounded bodies. Public/local sessions use the same cookie name but different server-side channel authorization. |
| Abuse and resource controls | `httpguard/public*.go` and `remoteaccess/transport.go`: per-source/prefix and global request limits, scanner quarantine, strict Range handling, connection/HTTP2 budgets. These do not protect upstream bandwidth from volumetric attacks. |
| Revocation | `remoteaccess/http.go`, `remoteaccess.go`, app `api_remote.go`, and session stores: kill closes the public listener, persists an off marker, and attempts all public authorization revocations. Failure to persist/revoke is reported; LAN remains the recovery path. |
| Player container | `apps/player/compose.release.yaml`: UID/GID 10001, read-only root filesystem and media mount, all Linux capabilities dropped, no-new-privileges, bounded resources, protected secret mounts, no Docker socket. Writable configuration/cache/backup mounts still matter under process compromise. Subtitles intentionally writes subtitle sidecars; do not describe its media mount as read-only. |
| Public egress | `identitycore/network.go`: provider connections reject loopback, private, link-local, multicast, unspecified, and CGNAT destinations; installation-owned local integrations have a separate policy. Not an OS network sandbox for arbitrary process execution. |

## What “at most they can play movies” means

A compromised Viewer credential/session should stay within that Viewer's permitted Libraries and personal activity. Leave downloads off when the Owner wants playback-only use, while recognizing that someone able to play content can still record it. Revoke the Viewer/session or disable public access after suspected credential theft.

An arbitrary-code-execution, host, or container escape vulnerability has a different impact. Both listeners share one process and its writable state/secrets; an application-level route policy cannot contain full process compromise. The container defaults reduce host and media-write exposure but do not make the service invulnerable. No zero-risk or “only movies under any breach” claim is justified.

## Authored regression coverage — not executed

- Both apps: `TestPublicListenerBlocksProvisioningEvenWithValidCredentials` exercises every SCIM route (including HEAD), absent/invalid/valid credentials, spoofed forwarding metadata, and unchanged directory state. Local creation/listing remains a positive control.
- Both apps: `TestPublicHTTPSKeepsLANBrowserRecoveryLocal` covers local browser GET/HEAD recovery, public setup denial, and rejection of an incorrect passkey origin without ceremony cookies.
- Shared identity: exact bootstrap allowlist positives and unknown/malformed/oversized pattern denial.
- Both setup scripts: interactive fixtures for TCP-only port 443 guidance, token redaction, repeated setup, automatic/local origin restoration, source installs, and failure after stopping the old service. Host service commands are stubbed.
- Shared setup: TCP 443 field mapping, manufacturer links, off state, credential readiness, disabled Profiles, malicious hostname rejection, and escaped/accessibly labelled HTML.

- Both apps: full HTTP browser Quick Connect sign-in, public login/asset routing, public approval denial, Viewer library access, administrative denial, and permission revocation.
- Shared Quick Connect: browser cookies and one-time replay, origin/body/query/cookie validation without state changes, rate limits, weak/Owner approval denial, expiry, cancellation, persistence failures, and public native/browser issuance concurrent with Profile revision changes.
- Shared listener: strict bind validation without file creation, stop-before-reset, and a stopped Manager remaining stopped until replacement.
- Shared browser script: request/status/recovery, malformed responses, expiry, cancellation with a late poll, same-origin return paths, and back-forward cache recovery. These are authored DOM simulations, not observed browser renders.

When gates are explicitly enabled, run the affected suites first:

```sh
(cd packages && go test ./identitycore ./remoteaccess ./httpguard ./quickconnect ./passkeys ./scim)
(cd apps/player && go test ./internal/server)
(cd apps/subtitles && go test ./internal/server)
bash apps/player/scripts/test-remote-setup.sh
bash apps/subtitles/scripts/test-remote-setup.sh
node --test packages/webassets/public-login.test.mjs
```

Run these commands from the repository root. Then run the affected app checks and populated desktop/mobile browser flow. Exercise actual public Quick Connect, library restriction, playback/seek, denied administration, session/permission revocation, and the kill switch over cellular data. Run appropriate race and real-listener protocol checks before making a security-readiness claim. None of those outcomes is established by this patch.

## External references checked

The perimeter design follows OWASP's [deny-by-default and per-request authorization guidance](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html). Router setup links use official [TP-Link](https://www.tp-link.com/us/support/faq/1379/), [ASUS](https://www.asus.com/us/support/faq/1037906/), [NETGEAR](https://kb.netgear.com/24289/How-do-I-set-up-port-forwarding-to-a-local-server-on-my-NETGEAR-router), [eero](https://eero.com/support/articles/how-do-i-set-up-port-forwarding), [Google](https://support.google.com/googlehome/answer/6274503?hl=en), and [Xfinity](https://www.xfinity.com/support/articles/xfi-port-forwarding) documentation. Router examples must not override Kinosail's TCP 443 → host TCP 443 mapping.
