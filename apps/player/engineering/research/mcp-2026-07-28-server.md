# MCP 2026-07-28 Streamable HTTP server

Research snapshot: 2026-08-23.

## Conclusion

MCP `2026-07-28` is the current GA revision and the first revision the specification calls **Modern**. A modern-only Kinosail endpoint should use the official Go SDK at `v1.7.0`, configure `StreamableHTTPOptions.Stateless = true`, and accept only the stateless `2026-07-28` lifecycle. The official Go SDK release says `v1.7.0` fully supports this revision and that Streamable HTTP accepts it only in stateless mode ([Go SDK v1.7.0 release](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.7.0)).

This is a breaking protocol era, not merely a new version string: `initialize`, `notifications/initialized`, protocol sessions, standalone GET streams, DELETE session termination, and resumable SSE are absent. Every operation is an independent POST carrying its protocol context ([versioning](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning), [Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http)).

## Modern-only server contract

### Lifecycle and discovery

- There is no negotiation handshake. Each request declares its version independently, and unsupported versions receive `UnsupportedProtocolVersionError` (`-32022`) with the supported versions. Servers MUST implement `server/discover`; clients MAY call it before another method but do not have to ([versioning](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning)).
- `server/discover` has only the standard `_meta` request fields and returns supported versions, capabilities, optional instructions, and server identity in result `_meta`. Its complete result also carries caching hints ([discovery](https://modelcontextprotocol.io/specification/2026-07-28/server/discover)).
- Every request `_meta` MUST contain `io.modelcontextprotocol/protocolVersion` and `io.modelcontextprotocol/clientCapabilities`. `io.modelcontextprotocol/clientInfo` is optional but clients SHOULD send it. Missing required metadata is `-32602` plus HTTP 400; servers SHOULD identify themselves in each result with `_meta.io.modelcontextprotocol/serverInfo` ([base protocol `_meta`](https://modelcontextprotocol.io/specification/2026-07-28/basic/index#_meta)).
- A server MUST NOT infer protocol version, capabilities, identity, task, thread, or conversation from a prior request or connection. Application state spanning calls needs an explicit identifier in each call ([statelessness](https://modelcontextprotocol.io/specification/2026-07-28/basic/index#statelessness)).

### Streamable HTTP

- Expose one MCP endpoint, such as `/mcp`, that supports POST. Every JSON-RPC request is a fresh POST containing one request or notification. Requests receive either one `application/json` object or a request-scoped `text/event-stream`; accepted notifications receive HTTP 202 with no body ([Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http)).
- Clients send `Accept: application/json, text/event-stream`. On request-scoped SSE, notifications must relate to that request, independent server-to-client JSON-RPC requests are forbidden, and the final response should end the stream. Closing the stream cancels the request ([Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http)).
- Every JSON-RPC request POST MUST carry `MCP-Protocol-Version` and `Mcp-Method`; `tools/call`, `resources/read`, and `prompts/get` MUST also carry `Mcp-Name`. Header values must match the corresponding body values. Missing, malformed, or mismatched routing headers produce HTTP 400 and `HeaderMismatch` (`-32020`). This revision does not define header requirements for notification POSTs ([request metadata](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http#request-metadata)).
- Servers MUST validate `Origin` on every incoming connection and return HTTP 403 when a present origin is invalid. Local servers SHOULD bind only to `127.0.0.1`, and servers SHOULD authenticate all connections ([transport security](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http#security--endpoint)).
- A modern-only endpoint returns 405 for GET and DELETE, ignores `Mcp-Session-Id` without minting or echoing a session ID, ignores `Last-Event-ID`, and provides no resumability ([earlier Streamable HTTP revisions](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http#earlier-streamable-http-revisions)).

### Caching

Complete results from `server/discover`, `tools/list`, `prompts/list`, `resources/list`, `resources/templates/list`, and `resources/read` MUST include `ttlMs >= 0` and `cacheScope` of `public` or `private`. `input_required` and MRTR-retry results are not cacheable ([caching](https://modelcontextprotocol.io/specification/2026-07-28/server/utilities/caching)).

Use `cacheScope: "private"` whenever a result contains user-specific data or a list is authorization-filtered. A private entry may be reused only within the same authorization context; caches MUST NOT share it across tokens. Cache scope is not an access-control mechanism, so authorization must still be checked on every primitive ([cache scope](https://modelcontextprotocol.io/specification/2026-07-28/server/utilities/caching#cache-scope-field), [caching security](https://modelcontextprotocol.io/specification/2026-07-28/server/utilities/caching#security-considerations)).

### Tools and results

- Each tool has a unique name and a non-null JSON Schema object as `inputSchema`. The default dialect is JSON Schema 2020-12; implementations MUST support that dialect. An optional `outputSchema` may describe any JSON value, and a server that advertises one MUST return conforming `structuredContent` ([tools](https://modelcontextprotocol.io/specification/2026-07-28/server/tools), [JSON Schema usage](https://modelcontextprotocol.io/specification/2026-07-28/basic/index#json-schema-usage)).
- Successful final results include `resultType: "complete"`. Structural/protocol failures are JSON-RPC errors; recoverable tool-execution or business errors are complete tool results with `isError: true` ([base results](https://modelcontextprotocol.io/specification/2026-07-28/basic/index#result-responses), [tool error handling](https://modelcontextprotocol.io/specification/2026-07-28/server/tools#error-handling)).
- Servers MUST validate tool inputs, enforce access control, rate-limit invocations, and sanitize outputs. The specification also recommends human denial/confirmation for sensitive operations ([tool security](https://modelcontextprotocol.io/specification/2026-07-28/server/tools#security-considerations), [tool interaction model](https://modelcontextprotocol.io/specification/2026-07-28/server/tools#user-interaction-model)).

## OAuth profile

Authorization is optional in MCP generally, but an HTTP MCP implementation that offers it SHOULD conform to the `2026-07-28` authorization specification. A protected MCP server is the OAuth resource server; the authorization server may be a separate service **or hosted with the resource server**, so a same-origin Kinosail authorization server is valid ([authorization roles](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#roles)).

### Discovery and endpoint placement

- The MCP resource server MUST publish RFC 9728 Protected Resource Metadata containing `authorization_servers` with at least one authorization-server issuer ([authorization-server discovery](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery)).
- The server MUST expose the metadata location through at least one of two mechanisms: point to it from a 401 with `WWW-Authenticate: Bearer resource_metadata="..."`, or serve an RFC 9728 well-known location specific to the MCP endpoint or at the origin root. For `https://example.com/public/mcp`, the endpoint-specific path is `https://example.com/.well-known/oauth-protected-resource/public/mcp`; the root fallback is `https://example.com/.well-known/oauth-protected-resource` ([Protected Resource Metadata discovery](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery#protected-resource-metadata-discovery-requirements)).
- The authorization server MUST publish either RFC 8414 OAuth Authorization Server Metadata or OpenID Connect Discovery metadata. For a root issuer such as `https://example.com`, the OAuth metadata path is `https://example.com/.well-known/oauth-authorization-server`; clients must validate that the returned `issuer` is identical to the issuer used for discovery ([authorization-server metadata discovery](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery#authorization-server-metadata-discovery)).
- All authorization-server endpoints MUST use HTTPS. Redirect URIs must use HTTPS or localhost ([authorization security](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#communication-security)).

### Client registration

The revision supports three registration mechanisms in this priority order: existing pre-registration, Client ID Metadata Documents (CIMD) when the authorization server advertises `client_id_metadata_document_supported: true`, then Dynamic Client Registration (DCR) only as a fallback. A server/client may instead prompt for statically registered client information ([client registration](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration)).

- CIMD is the preferred mechanism when client and server have no prior relationship. Supporting clients use an HTTPS URL with a path as `client_id`; the document contains at least `client_id`, `client_name`, and `redirect_uris`, and its `client_id` must exactly equal its URL. The authorization server validates that equality and exact redirect-URI membership ([CIMD requirements](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration#implementation-requirements)).
- Pre-registration is supported for an existing relationship, using a server-specific client ID and, if applicable, credentials ([pre-registration](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration#pre-registration)).
- DCR is deprecated in `2026-07-28`; it remains optional only for backward compatibility and new implementations should use CIMD ([DCR](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration#dynamic-client-registration), [deprecated-feature registry](https://modelcontextprotocol.io/specification/2026-07-28/deprecated)).
- Pre-registered and DCR credentials MUST be keyed to the issuer that issued them and never reused with another authorization server. CIMD identifiers are portable because the authorization server resolves the self-hosted HTTPS document ([authorization-server binding](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration#authorization-server-binding)).

### Authorization-code and token requirements

- Authorization servers MUST implement OAuth 2.1 security for both public and confidential clients. MCP clients MUST implement PKCE, verify `code_challenge_methods_supported` in authorization-server metadata before proceeding, and use `S256` when technically capable ([authorization overview](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#overview), [PKCE requirements](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#authorization-code-protection)).
- Before redirecting, clients record the validated metadata `issuer` with the PKCE verifier. If `authorization_response_iss_parameter_supported` is true, a response without `iss` is rejected; whenever `iss` is present, it is compared to the recorded issuer using exact/simple string comparison before the code is sent to a token endpoint ([authorization-response validation](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#authorization-response-validation)).
- Clients MUST include the canonical MCP server URI as `resource` in both authorization and token requests, even if the authorization server does not advertise Resource Indicators. The URI should be as specific as possible, including the MCP path where needed ([resource parameter](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#resource-parameter-implementation)).
- Bearer authorization MUST be present on every MCP HTTP request and MUST NOT be sent in a query string. The resource server MUST validate token expiry/validity and that the token was issued specifically for it as the intended audience; it must reject foreign-token passthrough ([access-token usage](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization#access-token-usage), [audience binding](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations#token-audience-binding-and-validation)).

## Official Go SDK implementation notes

As of this snapshot, GitHub marks `github.com/modelcontextprotocol/go-sdk` `v1.7.0` (published 2026-07-28) as its latest release. That release states full support for MCP `2026-07-28`, automatically registers `server/discover`, handles per-request metadata and standardized headers, and requires `StreamableHTTPOptions{Stateless: true}` for this revision ([release](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.7.0), [protocol guide](https://github.com/modelcontextprotocol/go-sdk/blob/v1.7.0/docs/protocol.md)).

For a modern-only Kinosail server:

```go
handler := mcp.NewStreamableHTTPHandler(
    func(*http.Request) *mcp.Server { return server },
    &mcp.StreamableHTTPOptions{Stateless: true},
)
```

`Stateless: true` is necessary but not by itself a modern-only version allowlist: the v1.7.0 transport source says a stateless handler also supports every legacy SDK protocol version. Kinosail must therefore gate the endpoint to `MCP-Protocol-Version: 2026-07-28` and the required modern `_meta`, reject `initialize`/legacy envelopes with an actionable error naming `2026-07-28`, and advertise only `2026-07-28` from `server/discover` ([Go SDK transport source](https://github.com/modelcontextprotocol/go-sdk/blob/v1.7.0/mcp/streamable.go#L868-L875), [modern-only compatibility guidance](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning#backward-compatibility-with-initialization-based-versions)).

Do not enable the SDK's legacy keepalive/ping, event-store resumability, or session-ID compatibility paths for this endpoint. The SDK documents `ping` as removed for `2026-07-28`, and its v1.7.0 default stateless behavior ignores session IDs and returns 405 for DELETE ([Go SDK protocol guide](https://github.com/modelcontextprotocol/go-sdk/blob/v1.7.0/docs/protocol.md), [v1.7.0 release notes](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.7.0)).

The SDK supplies bearer-token middleware and Protected Resource Metadata helpers, but token audience validation remains the application's verifier responsibility. Origin policy also remains an application deployment decision even where the SDK provides DNS-rebinding protection ([Go SDK authorization and security](https://github.com/modelcontextprotocol/go-sdk/blob/v1.7.0/docs/protocol.md#authorization)).

## Do not adopt from legacy MCP

New code should not implement the legacy `initialize` lifecycle, `Mcp-Session-Id`, HTTP GET/DELETE MCP transport operations, `Last-Event-ID` replay, or the old HTTP+SSE transport. Roots, sampling, protocol logging, and DCR are formally deprecated in this revision; new implementations should use tool parameters/resource URIs/configuration, direct model-provider APIs, OpenTelemetry or process logging, and CIMD respectively ([deprecated features](https://modelcontextprotocol.io/specification/2026-07-28/deprecated), [Streamable HTTP compatibility](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http#backward-compatibility)).

## Conformance gate

The official conformance project provides a frozen requirement set for the revision. Run the server against exactly the requirements that existed at release:

```sh
npx @modelcontextprotocol/conformance server \
  --url http://localhost:3000/mcp \
  --spec-version 2026-07-28
```

The suite validates the exchanged messages against the revision's wire schema. This verifies MCP resource-server behavior but deliberately does not certify the separately hosted authorization-server implementation, which the MCP specification places outside its scope ([official conformance README](https://github.com/modelcontextprotocol/conformance#conformance-requirements)).

## Verification boundary

This note establishes the published protocol and official SDK contract as of 2026-08-23. It does not prove interoperability with a particular MCP client, authorization server, reverse proxy, or Kinosail implementation. Completion requires focused HTTP/API tests, the official frozen conformance requirement set, and an end-to-end OAuth authorization-code flow against each intended client.
