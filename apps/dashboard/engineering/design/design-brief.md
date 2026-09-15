# Kinosail Dashboard design brief

## Product

Kinosail Dashboard gives a household one responsive board for opening, checking, and organizing self-hosted applications.

## User and priorities

- Primary user: a household owner who runs local services.
- First priority: application destinations.
- Second priority: current service reachability.
- Third priority: fast editing and recovery.
- Trust requirement: high. Status must come from real checks and private configuration must stay local.

## Main actions

- Open an application.
- Find an application.
- Add, edit, move, check, or remove an application.
- Let a host-authorized MCP client perform the same operations.

## Direction and layout

- Direction: Household Network Console.
- Pattern: one operational object with a compact service pulse and adaptive launcher grid.
- Density: balanced on touch and compact on wide pointers.
- Editing: direct manipulation plus explicit accessible move controls.
- Recovery: inline form errors, persistent input, and a global live status region.

## Product-specific constraints

- One board and one order across every viewport.
- No separate mobile configuration.
- No proxied application content.
- No fake network statistics.
- No hosted fonts, icons, telemetry, or design dependencies.
- No generic sidebar, metric-card, chart, and activity composition.

## State inventory

- Authentication: first setup, login, invalid credentials, expired session.
- Board: loading, empty, populated, filtered-empty, fetch error.
- App: unchecked, checking, reachable, slow, degraded, unavailable.
- Form: pristine, invalid, saving, saved, server error, destructive confirmation.
- Editor: default, active, dragging, keyboard move, order save failure.
- Environment: offline browser, reduced motion, forced colors, 320px reflow.

## Quality scores before implementation

- AI Slop Score: 1/10. Risk comes from the application tile grid.
- Distinctiveness Score: 8/10. The service pulse and direct shared-board editing provide product identity.
- Implementation Readiness Score: 10/10. The hierarchy, tokens, states, and responsive behavior are explicit.
