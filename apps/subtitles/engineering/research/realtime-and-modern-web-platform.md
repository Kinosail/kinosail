# Real-time and modern web-platform direction

Research snapshot: 2026-08-29

Implementation baseline: `c3199b23e8f36ef54451678242490d8a321cb375`

## Decision

Kinosail will benefit from real-time updates. It does not need a WebSocket-first architecture.

## Implementation status

This change implements the safe production recommendations:

- profile-scoped Server-Sent Events with bounded replay, queues, heartbeats, and access rechecks;
- a reconnecting Watch Together browser WebSocket client;
- event-driven download and Home Assistant command refresh with polling fallback;
- storage estimates, persistence requests, Web Locks, Broadcast Channel, and OPFS-backed offline media;
- removal of authenticated player-page caches and a neutral profile-bound offline library;
- source-sized Media Capabilities checks, richer Media Session data, and displayed-frame timing.

The search benchmark stays near 12 milliseconds for 10,000 items on the test machine. Full-text search is not justified yet. Cross-document View Transitions caused skipped-transition errors during rapid navigation, so they remain excluded. WebTransport, WebRTC, Web Push, WebCodecs playback, and required Background Sync remain excluded by design.

The Nox edge measurement negotiated HTTP/2 and found no UDP listener on port 38128. HTTP/3 cannot improve this path without edge and firewall work, so it remains disabled. The live Server-Sent Events route returned `text/event-stream`, replayed a bounded event by `Last-Event-ID`, and rejected an invalid cursor with HTTP 400 through the real Nox path.

Aggregate metrics now cover live-event subscribers, publications, reconnects, rejections, and slow-client drops. They also cover Watch Room connections, joins, reconnects, slow-client drops, and bounded client drift observations. These metrics contain no Viewer or media identifiers.

Use this package:

1. Add Server-Sent Events for low-rate server state changes.
2. Keep normal HTTP operations for all commands.
3. Keep WebSockets only for bidirectional Watch Together control.
4. Improve offline storage and media capability detection before adding a new transport.

Do not send media through Server-Sent Events, WebSockets, WebTransport, WebRTC, or Push. Keep original files on HTTP byte ranges. Keep adaptive media on HTTP Live Streaming (HLS). This preserves direct-media privacy and the one-container monolith.

## Baseline Kinosail seams

- Direct files use `http.ServeFile` in [`files.go`](../../internal/server/files.go). HTTP byte ranges already support seek and recovery.
- HLS publishes playable segment lists in [`hls_playlist.go`](../../internal/server/hls_playlist.go).
- Watch Together exposes a WebSocket route in [`watch_together.go`](../../internal/server/watch_together.go). It limits messages to 4 KiB and rechecks access each second.
- The first-party browser code did not create the existing Watch Together WebSocket before this change.
- Library monitoring uses file events, a stability delay, and polling fallback in [`library_watch.go`](../../../../packages/catalog/watch.go).
- Offline preparation is a bounded server job in [`downloads.go`](../../internal/server/downloads.go).
- Browser downloads use 8 MiB IndexedDB chunks in [`downloads.js`](../../../../packages/webassets/static/downloads.js).
- The prior service worker verified chunks, created one full `Blob`, and served local ranges in [`service-worker.js`](../../internal/server/static/service-worker.js).
- The prior service worker cached authenticated `/watch/` HTML. That created stale-session and profile-isolation risk.
- The prior player used Media Capabilities with one fixed 1080p sample. It also used Media Session in [`player.js`](../../../../packages/webassets/static/player-core.js).
- Kinosail already sends optional outbound webhook events in [`notification.go`](../../internal/server/notification.go).

HTTP remains the correct media transport. HTTP range requests support efficient recovery from partial transfers ([RFC 9110, section 14](https://www.rfc-editor.org/rfc/rfc9110.html#name-range-requests)). HLS clients fetch listed segments and reload playlists for new segments ([RFC 8216](https://www.rfc-editor.org/rfc/rfc8216.html)).

## Technology decisions

| Technology | Decision | Value | Cost and reach | Kinosail seam |
|---|---|---|---|---|
| Server-Sent Events (SSE) | **Adopt now** | Immediate scan, job, library, and Owner status updates | Low to medium. It is HTTP-native and one-way. `EventSource` reconnects and supports event IDs. | Add `GET /api/v1/events`. Publish revisions from library scans, download jobs, metadata work, and update work. |
| WebSockets | **Keep narrowly** | Bidirectional play, pause, seek, and room control | Medium. The browser WebSocket API has no backpressure. The server must bound every queue. | Keep Watch Together. Add Jellyfin `/socket` only for measured client compatibility. |
| Fetch response streaming | **Adopt when request-scoped** | Streams export, import, diagnostics, or one job response | Low to medium. Kinosail must define framing and resume rules. | Use for a user-started log or export stream that needs `Authorization` or `POST`. |
| HTTP ranges and HLS | **Keep as media core** | Broad playback, seeking, resume, caching, and receiver compatibility | Existing cost. It already matches the media resource model. | Keep `/media`, `/download`, offline file ranges, and `/hls`. |
| Media Capabilities | **Improve now** | Better direct-play and compatibility choices | Low. Results are hints, not proof. | Query exact file and HLS facts. Include audio, codec profile, size, bitrate, frame rate, and high dynamic range facts. |
| Media Session | **Improve now** | Better lock-screen, headset, keyboard, and system controls | Low. Action support varies, so keep feature checks. | Add artist, album, playback state, stop, next, previous, and chapters where supported. |
| Storage API, Web Locks, and Broadcast Channel | **Adopt now** | Safer offline downloads across low storage and multiple tabs | Low. Use feature checks and keep current fallback. | Check quota, request persistence after a user action, lock each download job, and broadcast progress to other tabs. |
| Service worker | **Keep, then narrow** | Offline shell and verified offline playback | Medium. Lifecycle and cache mistakes can expose stale private state. | Stop caching authenticated `/watch/` HTML. Cache a neutral offline player projection and explicit profile-bound manifests. Clear private data on sign-out or revocation. |
| View Transitions | **Experiment** | Clearer movement between library and detail views | Low. It is optional visual polish. | Test same-document transitions with current Hypertext Markup Language (HTML) swaps. Exclude the player and honor reduced motion. |
| `requestVideoFrameCallback` | **Experiment** | Measures the first displayed frame and supports precise drift checks | Low. It must not run expensive work per frame. | Replace time-only startup evidence with displayed-frame evidence. Sample Watch Together drift at a low rate. |
| Web Push | **Optional experiment** | Alerts when the app is closed | High for its small value. It needs permission, subscriptions, a service worker, and browser-vendor push services. | Limit to generic Owner alerts. Never send titles, filenames, artwork, tokens, or media. |
| Background Sync | **Avoid as a required path** | Can retry small writes after connectivity returns | Limited and browser-controlled. It cannot guarantee timing or long work. | At most, retry small idempotent progress writes. Keep foreground HTTP as the guaranteed path. |
| Background Fetch | **Do not depend on it** | Fits long movie downloads in concept | Experimental and incomplete across browsers. | Keep chunked foreground download and native-client background transfer. Revisit only as an enhancement. |
| File System Access | **Optional desktop experiment** | Can stream a user-selected export without one large browser `Blob` | Picker support is uneven. It accesses the viewer device, not server media roots. | Offer “Save a copy” only after feature detection. Never use it for server library setup. |
| Origin private file system (OPFS) | **Prototype for offline scale** | Avoids loading all chunks into one full `Blob` | Medium. Quota and eviction still apply. | Compare OPFS streaming playback with current IndexedDB assembly on multi-gigabyte files. |
| WebCodecs | **Experiment only** | Local thumbnail, frame analysis, or precise preview work | High. It is low-level and provides no container demuxer. Codec support varies. | Test one local trick-play prototype. Do not replace `<video>`, HLS, or server compatibility work. |
| HTTP/3 | **Measure at the edge** | Can reduce cross-request stalls on lossy remote links | Medium to high. It adds QUIC, User Datagram Protocol (UDP), certificate, and proxy work. | Test through the actual public endpoint. Keep application HTTP semantics unchanged. |
| WebTransport | **Avoid for now** | Adds multiplexed streams and unreliable datagrams | High. The standard is still a draft. It needs HTTP/3 and a separate Go server stack. | No current event or media use needs unordered delivery or custom streams. |
| WebRTC data channels | **Avoid** | Useful only for browser-to-browser data | Very high. It needs signaling, Interactive Connectivity Establishment (ICE), and often a relay. | Do not use it for Watch Together or private media. Revisit only for an approved peer feature. |
| `WebSocketStream` | **Avoid** | Adds browser-side backpressure | Non-standard and narrow. | Use bounded application queues on standard WebSockets. |

## Recommended SSE design

Create one authenticated, versioned event stream. Keep it inside the Server container.

Events should carry invalidations and small state facts. They must not carry full library records or private media facts.

Suggested events:

- `library.changed` with a monotonic library revision;
- `scan.state` with queued, running, complete, or failed;
- `download.state` with job identifier and state;
- `metadata.state` with job identifier and state;
- `server.notice` for an Owner-visible recovery action.

Send an initial snapshot revision. Set an event identifier on every change. Accept `Last-Event-ID`. Resend from a small bounded history or require a fresh snapshot.

Use cookie authentication for the browser `EventSource`. Use Fetch streaming when a client needs an authorization header. The EventSource interface defines a persistent `text/event-stream` connection and reconnection behavior ([WHATWG HTML](https://html.spec.whatwg.org/multipage/server-sent-events.html)).

The server must:

- use a bounded connection count;
- use a bounded queue for each client;
- disconnect slow clients when their bounded queue fills;
- send a heartbeat;
- close on sign-out, expiry, policy change, or remote-access shutdown;
- set `Cache-Control: no-store`;
- stop on request cancellation;
- test proxy buffering on the live path.

Go supports HTTP/1.x and HTTP/2 flushing. `ResponseController.Flush` also reports unsupported wrappers. A proxy can still buffer flushed data ([Go `net/http`](https://pkg.go.dev/net/http#ResponseController)).

## WebSocket direction

RFC 6455 defines two-way browser-to-server communication ([RFC 6455](https://www.rfc-editor.org/rfc/rfc6455.html)). This matches Watch Together commands. It does not improve library invalidation or job progress enough to justify a global socket.

Before expanding the current socket:

1. Prove that the populated browser route opens it.
2. Add an authoritative snapshot and monotonic room revision.
3. Include server time for drift calculation.
4. Bound outbound queues and disconnect slow readers.
5. Add ping, write deadline, idle timeout, and reconnect tests.
6. Configure the canonical Origin policy explicitly.
7. Prove closure after profile disable, sign-out, and remote shutdown.

The current Go package checks the request host by default and supports explicit allowed Origin patterns. It disables compression by default ([`coder/websocket` options](https://pkg.go.dev/github.com/coder/websocket#AcceptOptions)).

## Offline priority

Offline reliability has more user value than a new network protocol.

The Storage Standard lets an origin estimate storage and request persistent storage ([WHATWG Storage](https://storage.spec.whatwg.org/)). Web Locks coordinate work across same-origin tabs and workers ([Web Locks](https://www.w3.org/TR/web-locks/)). Broadcast Channel shares state between same-origin browsing contexts ([WHATWG HTML](https://html.spec.whatwg.org/multipage/web-messaging.html#broadcasting-to-other-browsing-contexts)).

Apply them in this order:

1. Estimate quota before a transfer.
2. Explain when the file cannot fit.
3. Request persistence only after the user selects offline storage.
4. Hold one lock per download job.
5. Send progress through Broadcast Channel.
6. Avoid creating one full-file `Blob` for large media.
7. Prototype OPFS or chunk-stream assembly.
8. Clear profile-bound data on sign-out and revocation.

Background Sync cannot own movie download correctness. The draft gives the browser control over deferred execution ([Web Background Sync](https://wicg.github.io/background-sync/spec/)). Background Fetch targets long downloads, but remains an incubating draft ([Background Fetch](https://wicg.github.io/background-fetch/)).

## Playback priority

The current Media Capabilities call uses one fixed HLS-oriented sample. The API accepts exact codec, size, bitrate, frame rate, color gamut, and transfer facts. It returns support, smoothness, and power-efficiency hints ([Media Capabilities](https://www.w3.org/TR/media-capabilities/)).

Use this order:

1. Ask about the exact original file with `type: "file"`.
2. Ask about exact HLS choices with `type: "media-source"`.
3. Keep original-file playback first when the result supports it.
4. Treat actual playback success or failure as stronger evidence.
5. Do not use slow Wi-Fi or buffering as a codec failure.

Media Session can expose metadata, system actions, position, and chapters ([Media Session](https://www.w3.org/TR/mediasession/)). Keep every action optional.

WebCodecs provides direct codec access but does not provide a media container layer ([WebCodecs](https://www.w3.org/TR/webcodecs/)). It is not a replacement for the current player.

## Privacy boundary

Push messages travel through a push service. Payloads are encrypted, but the push service can observe timing, frequency, and size ([Push API privacy section](https://www.w3.org/TR/push-api/#security-and-privacy-considerations)). Apple also routes Safari push through Apple Push Notification service ([WebKit](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)). Use generic text only if Kinosail adds Push.

WebRTC data channels are peer-to-peer and use Stream Control Transmission Protocol (SCTP) over Datagram Transport Layer Security (DTLS). They need separate signaling ([WebRTC](https://www.w3.org/TR/webrtc/#peer-to-peer-data-api)). This conflicts with Kinosail's simple central-server model. A relay can also become necessary across difficult Network Address Translation (NAT) paths.

WebTransport supports reliable streams, unreliable datagrams, and multiple streams. It runs over HTTP/3 or HTTP/2 in the current draft ([WebTransport](https://www.w3.org/TR/webtransport/)). Kinosail does not need those semantics. HTTP/3 itself can remove cross-stream stalls caused by Transmission Control Protocol (TCP) packet loss, but blocked UDP must fall back to TCP ([RFC 9114](https://www.rfc-editor.org/rfc/rfc9114.html)). Measure it at the public edge before changing the Server.

## Delivery order

1. Audit the live Watch Together browser connection.
2. Fix offline quota, persistence, multi-tab locking, and private cache isolation.
3. Make Media Capabilities queries match real source facts.
4. Add the small SSE event broker and versioned endpoint.
5. Improve Media Session details and actions.
6. Test View Transitions and displayed-frame timing.
7. Run bounded Push, OPFS, or HTTP/3 experiments only after measured need.

This order improves the visible product without changing the direct-media path.
