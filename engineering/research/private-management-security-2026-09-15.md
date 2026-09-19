# Private management and public gateway boundary

Implemented source scope: Player and Subtitles. Both retain local management. Public viewing and Owner management while away are independent opt-ins.

## Decisions

- Public HTTP always uses the closed public route policy. Neither an Owner session nor an API key grants administration there. Direct internet requests to the LAN listener also receive that policy, based on the socket peer address; headers cannot establish locality. A router/proxy that hides internet clients behind a private source address still needs correct deployment configuration.
- Optional Owner management runs on a separate userspace WireGuard network. Only virtual HTTPS and bounded A/AAAA DNS are served. No host TUN, network administration capability, LAN forwarding, peer forwarding, or container socket is available. Device identity never comes from HTTP headers.
- Pairing requires an active secured Owner, fresh sign-in and a home-network connection. Each profile binds one device key to that Owner's revision. Public, ordinary LAN, other-device, and other-Owner replays of a management session fail before token-use persistence. API keys cannot authenticate on the tunnel. Revocation closes existing connections; credential changes invalidate pairings. New device addresses are not reused while an old runtime exists.
- The public HTTPS Compose override moves TLS/HTTP ingress into a separate container. It receives only two Unix sockets: the application with mandatory public policy, and a certificate service exporting only the public TLS identity. Its root filesystem is read-only, all capabilities are dropped, no-new-privileges is set, and resource limits bound connections, processes, memory and CPU. After opening the listener, a Linux amd64/arm64 seccomp filter applies to all threads and permits only Unix sockets; unsupported platforms or filter failures refuse startup. io_uring and cross-process memory/FD bypass paths are denied. The container has no media, Owner state, backups or DNS credentials.
- Private HTTPS for the public DuckDNS hostname renews separately through DNS-01. Its virtual address is never published in public DNS. Endpoint DNS maintenance continues while private management is enabled even if public viewing is killed. No public TCP listener is required to renew private HTTPS.
- Disable erases all pairings and rotates the Server identity on later enable. A separate durable recovery marker wins over persisted pairing state. Failed revocation closes management and records the marker. A local `owner-access-disable` command handles login loss or corrupt pairing state.
- Plain backup export is an Owner operation requiring authentication within ten minutes even though it uses GET/HEAD. Pairing keys are absent from portable backups.

## Input and failure review

Management accepts bounded, unique-field JSON or URL-encoded forms; duplicate keys, unknown fields, query/body ambiguity, malformed names/keys/ports, missing required values and excessive sizes are rejected before changes. Persisted peer state is bounded and validated before opening UDP. Imported WireGuard text is generated from validated values and random per-device keys; it is returned once with no-store headers.

A source pass covered startup cancellation, certificate delays, storage errors, pairing failures, revocation, stopped public access, peer revision changes, active-connection closure, spoofed headers, and session replay. Route policy fixtures classify all ten added routes in each app as private Owner routes. Public bootstrap and Viewer permissions stay on their existing allowlists.

## Limits and verification boundary

This is public ingress isolation, not a separate copy of the entire playback application. The main application still handles approved public viewing requests and holds management state. Exploitation of that application, media tooling, or the host kernel could exceed Viewer permissions. Do not describe this as proof that every full compromise can only play movies. An authenticated Viewer can use its allowed media and activity features; downloads depend on Profile policy.

All quality gates remain disabled by the repository's `.gates-disabled` policy. Focused tests were authored but **not executed**. No compile, lint, race, container, browser, deployment, physical-device or router acceptance was performed for this change. Dependency metadata was refreshed and source formatted; those actions are not runtime validation. Remote ancestry proves source publication only.

When gates are explicitly enabled, run the focused suites in `packages/owneraccess`, `packages/publicgateway`, `packages/identitycore`, `packages/httpguard`, `packages/trustedhttps`, `packages/remoteaccess`, `packages/validation`, and both apps' server/configuration packages; then the affected app gates. Exercise Linux amd64/arm64 containers, normal Viewer playback/seek/download policy, Owner pairing from home, cellular management, public kill with private management still active, session step-up, credential rotation, revocation during a stream, recovery/restart, DNS/certificate renewal, and source/release setup and mode switching. Render populated desktop/mobile management pages and inspect keyboard/focus/accessibility. Confirm the public gateway cannot read private volumes or dial LAN addresses.

## Primary references

- [wireguard-go userspace HTTP example](https://git.zx2c4.com/wireguard-go/tree/tun/netstack/examples/http_server.go) and [network-stack implementation](https://git.zx2c4.com/wireguard-go/tree/tun/netstack/tun.go).
- [WireGuard Quick Start](https://www.wireguard.com/quickstart/).
- [Linux seccomp filter documentation](https://docs.kernel.org/userspace-api/seccomp_filter.html) and [seccomp system call](https://man7.org/linux/man-pages/man2/seccomp.2.html). The filter documentation explicitly distinguishes filtering from a complete sandbox; container restrictions remain part of the design.
