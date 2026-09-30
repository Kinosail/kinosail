# MCP security and protocol audit

Reviewed: 2026-09-29. Scope: Player, Subtitles, and `packages/mcpgateway`.
Implementation revision: `26c9dd0e0f0e75765fd973854e5eb8c3d1bcd3f7`.

Keep the connection flow simple. Prefer host STDIO through Docker or SSH for a trusted host operator. Use HTTPS OAuth for a client that needs a revocable Profile grant. Start with read access. Neither transport needs a new proxy, token format, or configuration layer.

## Protocol baseline

The current published MCP specification is [2026-07-28](https://blog.modelcontextprotocol.io/posts/2026-07-28/). The repository already uses the official [Go SDK v1.8.0](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.8.0). No dependency update was needed.

The HTTPS endpoint is stateless `POST /mcp`. It accepts only `2026-07-28`, carries per-request metadata, validates mirrored method/name/version headers, and returns complete results. `server/discover` and authorization-filtered tool lists use private caching. These match the [Streamable HTTP contract](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http) and [tool contract](https://modelcontextprotocol.io/specification/2026-07-28/server/tools).

The SDK retains STDIO compatibility with earlier clients. The adapter does not add legacy HTTPS sessions. It exposes tools only. Prompt, resource, completion, and change-subscription features are not needed for the current app operations.

## Findings and repairs

| Finding | Result |
| --- | --- |
| Refresh replay did not invalidate the rotated connection. Refreshing extended its lifetime indefinitely. | Store bounded hashes of consumed refresh tokens. Replay revokes the family, including current access. Keep an absolute 30-day approval lifetime. Report the actual remaining access lifetime. |
| A connected host session retained a stale Owner snapshot. | Recheck Owner existence, role, state availability, and revision before every request. Require reconnection after identity changes. |
| Generic configuration mutations exposed identity and credential authority. | Block generic configuration PUT/DELETE in both apps. Keep the existing dedicated settings and operational routes. |
| MCP and public token endpoints lacked bounded request rates. | Limit MCP ingress to 120 requests per minute per network address. Limit tools to 120 per Profile across transports. Token/revoke endpoints share another 120-request network limit. Rejections precede API effects. |
| Expired grants and unused client registrations could consume capacity permanently. Approval codes had no capacity check. | Reclaim expired grants and unused registrations after 24 hours. Preserve registrations with active grants, requests, or codes. Bound approval codes at 128. Unknown token revocation avoids state writes. |
| PKCE verifier validation rejected valid RFC characters and lengths. Challenges allowed noncanonical values. | Accept the RFC 7636 unreserved verifier alphabet at 43–128 characters. Require canonical S256 challenges. |
| Invalid authorization/API query encoding could be ignored. Introspection allowed ambiguous JSON and credential redirects. | Parse queries strictly before effects. Reject duplicate introspection keys while allowing provider extensions. Do not follow introspection redirects. |
| API failures looked like successful tool results. Version errors lost IDs. The SDK served undeclared empty primitives. | Set tool `isError` for API status 400 or higher while retaining structured status/body. Echo readable IDs, omit unreadable IDs, return the correct version/header codes, and reject unsupported primitives. Advertise a static tool catalog. |

Refresh rotation follows [OAuth Security BCP section 4.14.2](https://www.rfc-editor.org/rfc/rfc9700.html#section-4.14.2). Verifiers follow [RFC 7636](https://www.rfc-editor.org/rfc/rfc7636.html). Error IDs follow the [MCP base protocol](https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2026-07-28/basic/index.mdx#error-responses).

## Boundaries retained

- Host STDIO uses a private `0600` Unix socket with 32 concurrent connections. Docker/SSH access grants full host Owner authority. Browser OAuth revocation does not revoke host access; remove host credentials when needed.
- HTTPS requires a bearer token with the exact resource audience and a current mapped Profile. Built-in approval starts with read access. Write and management need explicit scopes; management also needs an Owner.
- OAuth uses browser approval, PKCE, hashed opaque tokens, Profile revision binding, and persisted revocation. Public client metadata uses bounded HTTPS fetches with private-network and redirect protections.
- Tools call the shared application API through an allowlist and the app's identity/audit boundary. Absolute URLs, media bytes, unrestricted filesystem access, sessions, API-key administration, and generic configuration mutations remain blocked.
- API paths are bounded at 2,048 bytes, API bodies at 1 MiB, and API responses at 4 MiB. The SDK bounds HTTP bodies at 4 MiB and STDIO frames at 16 MiB.
- Returned names, titles, and descriptions remain untrusted data. The server's instructions identify this boundary. Tool annotations are hints; the server enforces authorization independently.

These choices follow the [MCP authorization specification](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization) and [security considerations](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations).

## Tradeoffs and verification limits

A built-in connection requires approval again after 30 days or 1,024 rotations. A client retrying a consumed refresh token loses the whole connection. External providers control their own lifetimes and replay policies.

If saving replay revocation fails, the running process removes the connection and returns 503. Durable revocation cannot be claimed for that failure. Repair storage and revoke the connection before restarting against an older readable state. Token hashes consumed before this upgrade were not recorded.

Rate limits use the transport's remote address and a one-minute window. A shared reverse proxy or NAT can group callers. Owner/Profile tool limits still apply across different connections.

The [evidence packet](../qa/2026-09-29-mcp-audit/README.md) records source tests, official conformance checks, repeat commands, and environment limits. This audit does not prove production deployment, physical-device behavior, or a live external-provider login. It does not claim the full diagnostic conformance suite passed.
