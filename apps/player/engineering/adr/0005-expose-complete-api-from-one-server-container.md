# Expose the complete API from one Server container

Kinosail is an API-driven monolith: every user-visible capability is available through a versioned HTTP API, while the bundled web interface remains an HTMX adapter over the same application operations. The supported self-hosted installation starts exactly one Kinosail Server container containing the API, web interface, background work, and required data services; API completeness must not introduce separately deployed application or data services.

## Consequences

A capability is complete only when a non-browser client can perform it through the HTTP API and focused tests cover that path; presentation-only rendering and static assets do not need API equivalents. Existing browser-only operations and optional sidecar profiles are migration debt rather than precedent, and new work must not add more. User-managed network infrastructure may remain outside the Server container, but Kinosail does not define or manage a local sidecar.
