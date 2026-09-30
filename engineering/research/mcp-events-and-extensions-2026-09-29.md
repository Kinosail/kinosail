# MCP Events and extension audit

Checked on September 29, 2026 against current `origin/main` and primary sources.

## Existing protocol support

Player and Subtitles use Go SDK `v1.8.0` and MCP `2026-07-28`.
The shared gateway implements stateless discovery, request metadata, private
tool-catalog caching, structured results, scoped OAuth, revocation, and host STDIO.
Dashboard is no longer in this repository's current main branch.

The [MCP changelog](https://modelcontextprotocol.io/specification/2026-07-28/changelog)
defines the modern protocol changes. The
[extension overview](https://modelcontextprotocol.io/extensions/overview)
separates optional extensions from core requirements.

## Events implementation

The [Events design sketch](https://github.com/modelcontextprotocol/experimental-ext-triggers-events/blob/main/docs/design-sketch-proposal.md)
is experimental. The [ChatGPT Events guide](https://developers.openai.com/plugins/build/mcp-events)
implements its webhook profile; ChatGPT does not support its poll and push modes.
Before this change, Kinosail had only an application SSE feed.

This change adds discovery, list, subscribe, refresh, and unsubscribe methods.
It connects all existing live sources: `library.updated`, `download.updated`,
and `home-assistant.command`. Subtitles also publishes `subtitles.updated` after
a durable acquisition, edit, replacement, or Hide state change.
Subtitle notices require an Owner and management access.
Download and Home Assistant notices remain bound to their Viewer Profile.
Payloads contain only the affected relative API resource path.

Subscriptions persist in protected application connection state. Identity includes
the authenticated connection, callback, event name, and normalized resource filter.
HTTPS callbacks use signed, bounded verification challenges and Standard Webhooks
HMAC signatures. The existing outbound client validates destination IPs when dialing,
blocks prohibited networks, disables proxies, preserves TLS hostname checks, and
refuses redirects. Callback response bodies never enter protocol error messages.

Subscriptions have finite lifetimes of at most 24 hours. A requested lifetime below
one minute receives the one-minute minimum. No-expiry requests receive a finite grant.
Built-in grants, Profile revisions, and access policy are checked before delivery.
Host subscriptions retain the selected Owner's authority and require host access
for management. External OAuth requires an introspected `client_id`; its grant
cannot outlive the current access token. External provider revocation is checked
on subsequent authenticated requests, while delivery rechecks local access and expiry.
No bearer token is stored with a subscription.

Delivery uses bounded queues and eight concurrent outbound requests. Failed events
receive at most three attempts with backoff. `410` and `413` suspend delivery immediately.
A refresh reactivates delivery and reports skipped events with `truncated: true`.
Event IDs and body bytes remain stable across retries; signatures are regenerated.
Signing-key refresh supports a five-minute period with both keys.

These events have no durable replay source: cursors are null. Events during process
downtime, queue pressure, or exhausted delivery attempts cannot be recovered.
A live ChatGPT plugin callback and end-user reaction require separate external proof.
This implementation does not claim poll, push, or control-notification support.

## Other feature gaps

| Feature | Current support | Kinosail use |
| --- | --- | --- |
| Tasks extension | Absent | Most useful next: scans and maintenance with durable status and cancellation |
| Resources and resource templates | Absent | Discoverable media and subtitle context |
| Prompts and argument completion | Absent | Reusable recommendation and troubleshooting workflows |
| Skills over MCP | Absent | Server-published instructions and supporting workflow resources |
| MCP Apps | Absent | Interactive library or subtitle views in a conversation |
| Elicitation using multi-round-trip requests | SDK support only; no application workflow | Scoped user choices and approvals for future interactive operations |
| OpenTelemetry context propagation | Not connected to application diagnostics | Cross-request troubleshooting without exposing credentials |
| Machine-to-machine and enterprise authorization extensions | Absent | Optional future administrative integrations |

These are optional features, not failures to implement the core protocol.
Legacy Roots, Sampling, and Logging are deprecated; adding them is not a priority.

## OpenAI-specific extensions

The [OpenAI repository](https://github.com/openai/mcp-extensions) publishes
TypeScript and Python SDKs. Its
[specification](https://github.com/openai/mcp-extensions/blob/main/docs/spec.md)
adds ChatGPT-specific behavior. Kinosail currently advertises none of it.

| Extension group | Kinosail opportunity |
| --- | --- |
| Composer mentions | Search for a title and attach it to a message; a useful small follow-up |
| Global and thread app entrypoints | Open the library or subtitle queue from ChatGPT |
| Structured plugin settings and onboarding | Present approved non-secret preferences with native controls |
| File entrypoints and local file resources | Review subtitle files; desktop-specific and requires an explicit file boundary |
| Display modes, app deep links, model context, and messages | Support the app experience after MCP Apps exists |
| Extended forms | Offer descriptive options, thumbnails, and resource choices |

The published platform table describes expected launch support, not verification
of an installed client. Several file and mention features are desktop-only.
Go can implement the wire contracts without replacing Kinosail's server.
Adding an app requires an actual resource, UI, and authorization design; metadata
alone would advertise behavior Kinosail does not provide.
