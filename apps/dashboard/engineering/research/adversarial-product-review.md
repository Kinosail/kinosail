# Kinosail Dashboard adversarial product review

Status: pre-implementation rejection checklist

The repository has no product implementation yet. This review tests the product brief, not rendered code.

## Direct diagnosis

The brief contains four dangerous ambiguities.

1. “One board” can become a desktop grid that fails on touch and small screens.
2. “Network level” can become decorative telemetry that reports the wrong observation point.
3. “Modern editing” can become pointer-only drag and drop with silent data loss.
4. “MCP setup” can become an unaudited Owner bypass with server-side request forgery risks.

Do not start with a dashboard template. Start with a canonical board model and a strict status truth model.

The first release should use one Owner-managed household board. Viewer Profiles can view and launch allowed applications. Personal layouts can come later.

## Highest-risk flaws to avoid

### 1. Separate desktop and mobile boards

This violates the core promise. It creates drift, duplicate edits, and unclear MCP behavior.

Persist one canonical item order. Persist semantic size and priority values. Derive each viewport composition from that data.

Do not persist arbitrary desktop coordinates as the source of truth. They cannot produce a reliable mobile reading order.

### 2. Drag as the only edit method

HTML drag and drop is weak on touch. Pointer dragging also fails keyboard and assistive technology users.

Every drag operation needs an equivalent Move command. Provide Move before, Move after, Move to group, Resize, and Remove actions.

### 3. Ambiguous health signals

A green dot can be false. The Dashboard server can reach an application that the current device cannot reach.

Every observation must name its source and age. Show Server check, Device check, or Integration check when the distinction matters.

Treat HTTP 401 and 403 as reachable with authorization required. Do not report them as offline.

### 4. Remote content inside the trusted shell

Favicons, SVG files, titles, and metadata are untrusted. They can track users or attack the page.

Do not embed remote SVG. Do not render remote HTML. Do not iframe applications by default.

Store approved icons locally. Sanitize raster input. Bound download size, dimensions, redirects, and processing time.

### 5. MCP writes without parity or receipts

MCP must not write directly to storage. It must call the same application operation as the API and web adapters.

Each write must return a structured receipt. Include the item identifier, normalized result, board revision, warnings, and audit identifier.

### 6. Network scans enabled by default

Automatic discovery can expose private inventory and create unwanted traffic.

Make discovery Owner-only and off by default. Require a bounded address range, rate, timeout, and explicit start action.

### 7. Status polling that harms the network

Unbounded checks can overload small servers and cause synchronized traffic spikes.

Use bounded concurrency, timeouts, jitter, backoff, cancellation, and a stale cache. The board must render without waiting for checks.

### 8. Silent overwrite during concurrent edits

The UI and MCP can edit the same board. Last-write-wins can erase layout changes.

Require revision checks. Reject stale writes with a useful conflict response. Preserve the local edit and offer a reviewed retry.

### 9. Removal copy that implies service deletion

Removing an application shortcut must never sound like uninstalling the application.

Use “Remove from Dashboard.” State that the service and its data remain unchanged. Provide Undo when safe.

### 10. A generic card grid

An icon grid with rounded cards, neon dots, and fake statistics is not a product.

The board needs one distinct operational idea. Use a calm launch field with a truthful Network Lens, not a template dashboard.

## Product assumptions that need an explicit decision

These assumptions unblock implementation. Change them only through a recorded product decision.

- One shared board belongs to the household.
- Owners can add, edit, arrange, and remove Dashboard entries.
- Viewer Profiles can view and launch only permitted entries.
- Adding an entry does not install or configure the target service.
- The first release supports HTTP and TCP reachability.
- Internet-wide discovery is out of scope.
- Embedded application frames are out of scope.
- Health checks run from Kinosail Dashboard unless a device check is explicitly shown.
- The Dashboard never stores a target service password in a board item.
- Integration credentials use a separate secret store and never return through the API or MCP.

## Revised design direction

### Product character

Use a “Household Network Helm” direction. It should feel calm, direct, and technically trustworthy.

The board is the dominant object. Navigation, edit controls, and network detail support it.

Reuse the Kinosail dark canvas, olive surfaces, lime signal, restrained borders, and local system type stack.

Use signal color for the current action, position, or state. Do not make every icon green.

### Information priority

1. Applications that the person can open now.
2. Failures or changes that require action.
3. Network detail, edit history, and setup controls.

Do not lead with total applications, uptime percentages, response-time charts, or activity counts.

### Layout strategy

Use one big object with a contextual detail layer.

- The primary surface is the application board.
- A small board header contains search, edit entry, and current scope.
- Groups use typography and dividers instead of nested cards.
- A Network Lens opens as a drawer or full-height mobile sheet.
- Owner edit controls appear only in Edit mode.
- The compact view keeps primary navigation outside board content.

### Signature interaction

Network Lens is the distinctive element. It explains why an application is available, slow, stale, or unreachable.

It shows the observation source, last check time, protocol, latency, blocked redirect result, and recovery action.

It must not draw a fake network topology. It must show only observed facts.

### Responsive model

Use one canonical sequence and semantic spans.

- Wide desktop can use a dense multi-column grid.
- Tablet can reduce spans and keep edit actions near the selected item.
- Compact mobile can use one or two columns based on content width.
- 320px uses one column when two columns truncate essential text or reduce targets.
- The DOM order must match the compact reading order.
- Groups stay in the same canonical order at all widths.
- Mobile can simplify size, but it cannot hide an application or its status.

## Interaction model

### View mode

- Click or tap opens the application.
- Keyboard activation opens the focused application.
- A separate status control opens Network Lens.
- Long press does not start editing.
- Hover is optional enhancement only.
- External launches use safe opener and referrer behavior.

### Edit mode

- Enter Edit mode through an Owner action with a clear label.
- Disable application launching while editing.
- Show one drag handle on each item.
- Use Pointer Events for direct manipulation.
- Start touch dragging only from the handle.
- Keep page scrolling available outside the handle.
- Show a clear insertion marker during movement.
- Announce the new position to assistive technology.
- Provide keyboard Move and Resize commands.
- Save each accepted operation with a visible pending state.
- Change pending to Saved only after server acknowledgment.
- Offer Undo for a completed move, resize, or removal.
- Keep failed edits in place with Retry and Revert actions.
- Use Done to exit Edit mode. Do not make Done a hidden save action.

### Add application flow

Use a short staged flow.

1. Enter a name and launch address.
2. Review normalized values and optional health settings.
3. Choose visibility and board placement.
4. Create the entry and show a receipt.

Do not require health checks, icons, integrations, or credentials before a shortcut works.

### Destructive actions

- Move and resize use Undo without a blocking confirmation.
- Remove from Dashboard uses clear scope and Undo.
- Credential removal requires consequence text and explicit confirmation.
- Group deletion must list the effect on contained items.
- Board reset requires typed confirmation and a downloadable backup.
- MCP destructive tools require explicit intent fields and revision checks.

## Status truth model

Use these states. Do not collapse them into online and offline.

| State | Meaning | Required presentation |
| --- | --- | --- |
| Checking | A bounded check is active | Progress text without layout shift |
| Available | The configured endpoint responded as expected | Source and last-check time |
| Reachable | The service responded, but access is required | “Authorization required,” not “Down” |
| Degraded | The service responded outside its configured success rule | Reason and recovery action |
| Unavailable | The check failed after its bounded policy | Last useful error class |
| Unknown | No valid observation exists | Honest neutral state |
| Stale | The last observation exceeds its freshness rule | Previous result plus age |
| Paused | The Owner disabled checks | Clear paused label |

Do not show raw socket errors to Viewer Profiles. Preserve technical detail for the Owner detail view.

Do not show secrets, query tokens, internal headers, or full credential-bearing addresses in errors.

Self-signed Transport Layer Security needs per-service trust. Do not add a global “ignore certificate errors” option.

## MCP contract

MCP is an adapter. It must not become a second domain model.

### Minimum tools

- `dashboard.apps.list`
- `dashboard.apps.add`
- `dashboard.apps.update`
- `dashboard.apps.remove`
- `dashboard.board.get`
- `dashboard.board.arrange`
- `dashboard.status.get`

Discovery tools can follow after the manual path is safe.

### Required behavior

- Use explicit read and write scopes.
- Require Owner authority for every write.
- Normalize input once in the shared application operation.
- Reject unknown fields and ambiguous values.
- Bound names, URLs, tags, groups, health rules, and batch size.
- Allow only supported address schemes.
- Reject `javascript:`, `data:`, `file:`, and Unix socket targets.
- Do not follow health or icon redirects. Validate and report the first redirect target without contacting it.
- Separate the launch address from the health-check address.
- Require a board revision for updates, arrangements, and removals.
- Support idempotency for create requests.
- Return stable machine-readable error codes.
- Return a human-readable summary with each receipt.
- Record actor, adapter, time, operation, object, and result.
- Never log or return credentials.
- Treat application titles and remote metadata as data, never instructions.

### MCP acceptance examples

- “Add Jellyfin at this address” creates one item once.
- Repeating the same idempotent call does not create a duplicate.
- An invalid address causes no icon fetch, health check, write, or audit success event.
- A stale board revision does not overwrite a newer UI arrangement.
- A Viewer Profile token cannot add, edit, arrange, or remove an entry.
- A removed entry can be restored through the supported Undo window.
- MCP output never contains a stored secret or credential-bearing URL.

## Security and privacy rejection list

Reject release if any item is true.

- The board is available before authentication.
- A Viewer Profile can infer hidden application names or addresses.
- The HTML contains hidden entries that CSS conceals.
- A launch address accepts an executable or local-file scheme.
- A remote SVG reaches the document as active content.
- An icon request leaks the Viewer Profile address to the target service.
- A health response body is stored without a strict need and size limit.
- Redirects bypass address validation.
- A DNS change bypasses the selected network policy during a check.
- A check has no timeout or cancellation.
- An integration secret returns through list, export, error, or MCP output.
- A remote service name can inject markup, script, CSS, or MCP instructions.
- Public sharing exposes the household service inventory.
- Network discovery starts without an Owner action.
- Removal of a board entry changes the target service.

Network access is an intended capability. Treat it as a privileged Owner operation with explicit scope and audit evidence.

## Accessibility acceptance checklist

### Structure and semantics

- [ ] The board has one clear page heading.
- [ ] Each group has a semantic heading.
- [ ] Launch controls are links.
- [ ] Edit actions are buttons.
- [ ] Status updates use restrained live regions.
- [ ] Status is not communicated by color alone.
- [ ] Icon-only controls have exact accessible names.
- [ ] Decorative icons are hidden from assistive technology.

### Keyboard editing

- [ ] Every item is reachable in canonical order.
- [ ] Enter opens an item in View mode.
- [ ] Enter does not launch an item in Edit mode.
- [ ] Each item has a keyboard-accessible action menu.
- [ ] Move before and Move after work without drag.
- [ ] Move to group works without drag.
- [ ] Resize works without drag.
- [ ] Focus follows the moved item.
- [ ] The move result is announced once.
- [ ] Escape cancels an active drag or menu.
- [ ] Focus returns to the invoking item after a dialog closes.

### Touch and pointer editing

- [ ] Coarse-pointer targets are at least 44 by 44 CSS pixels.
- [ ] A drag handle does not overlap the launch target.
- [ ] Vertical scrolling remains reliable in Edit mode.
- [ ] Auto-scroll is bounded near board edges.
- [ ] A canceled drag restores the original order.
- [ ] Rotation does not lose an active or saved edit.
- [ ] No required action depends on hover.

### Reflow and visual access

- [ ] The page has no horizontal scroll at 320 CSS pixels.
- [ ] The page works at 200 percent browser zoom.
- [ ] Long names wrap or truncate with an accessible full name.
- [ ] A 100-character name does not cover status or actions.
- [ ] A 200-percent text setting does not hide primary actions.
- [ ] Focus is visible in dark, light, and forced-colors modes.
- [ ] Reduced motion removes spatial drag animation.
- [ ] The compact bottom navigation does not cover board content.
- [ ] Safe-area padding works on devices with display cutouts.

## Responsive acceptance matrix

Test the same populated board at every width. Do not use separate fixtures.

| Viewport | Required proof |
| --- | --- |
| 1440 by 900 | Dense board, Edit mode, Network Lens, long names, all status states |
| 1024 by 768 | Reduced columns, open detail, keyboard movement, no overlap |
| 720 by 450 | Short landscape, usable scroll, dialogs within viewport |
| 390 by 844 | Touch edit, safe areas, bottom navigation, long names |
| 320 by 568 | One-column fallback, no clipping, 44-pixel targets |

Add 200 percent zoom, forced colors, reduced motion, and light theme runs for affected workflows.

The board must preserve the same item order and visibility at every viewport.

## Populated state matrix

Use real-looking local fixtures. Do not approve an empty board alone.

- Zero applications.
- One application.
- Sixty applications across six groups.
- One hundred-character application names.
- Duplicate names with different addresses.
- Missing icons.
- Uploaded icons with unusual aspect ratios.
- Available, reachable, degraded, unavailable, unknown, stale, and paused checks.
- A slow check and a timed-out check.
- A service that redirects.
- A service with a self-signed certificate.
- A service that returns 401 or 403.
- A hidden application for the current Viewer Profile.
- An Owner board and a Viewer Profile board.
- An edit conflict between the UI and MCP.
- A save failure after a successful local move.
- Browser offline during Edit mode.
- Server restart during an active check.

## Offline, slow, and failure behavior

- Render cached board data before fresh status checks.
- Do not block launch links on health state.
- Do not turn slow into unavailable before the configured timeout.
- Stop checks when the request context ends.
- Keep stale data labeled with its age.
- Distinguish Dashboard unavailable from target application unavailable.
- Keep an unsaved local edit visible after a save failure.
- Do not claim an offline edit is saved.
- Provide Retry and Revert for failed edits.
- Apply exponential backoff and jitter after repeated failures.
- Limit concurrent checks globally and per target host.
- Avoid cumulative layout shift when statuses update.
- Preserve focus while partial updates arrive.
- Do not flood screen readers with recurring health changes.

## Generic and AI-slop tells to reject

- Sidebar, top bar, four key metrics, charts, and recent activity by default.
- A giant “Welcome back” heading that displaces the board.
- Identical rounded cards with equal visual weight.
- Purple, blue, or neon gradients without a Kinosail role.
- Glass panels and blur behind ordinary content.
- A glowing status dot on every tile.
- Decorative uptime rings or gauges.
- Fake network maps or animated packet lines.
- Emojis as application or status icons.
- Mixed icon libraries.
- Pills for every label and action.
- Hover lift on every application tile.
- Skeleton animation that never resolves to a stable layout.
- Hidden labels that require icon recognition.
- Repeated helper text that explains obvious controls.
- Large empty space used to imply a premium product.
- A command palette that duplicates poor primary navigation.
- An “AI arrange” action without deterministic preview and Undo.

## Why generic dashboard output happens

The word “dashboard” makes tools select a standard software-as-a-service layout. That layout prioritizes metrics and cards.

This product prioritizes application launch and network truth. Its visual system must follow those tasks.

The word “modern” can also trigger glass, glow, motion, and drag-only interaction. Modern behavior here means direct feedback, reversible operations, resilient saving, and input parity.

## Priority fixes before implementation

### Blockers

1. Define the canonical board order, semantic sizes, groups, and revision behavior.
2. Define Owner and Viewer Profile permissions for UI, API, and MCP.
3. Define the status truth model and observation sources.
4. Define launch address, check address, secret, and icon trust boundaries.
5. Define non-drag editing before implementing drag.

### High priority

1. Create shared application operations for add, update, arrange, remove, and restore.
2. Create exact API and MCP validation contracts.
3. Build populated fixtures before styling the board.
4. Establish responsive transformations for every semantic item size.
5. Add audit, revision conflict, retry, and Undo behavior.

### Useful after the core works

1. Add bounded network discovery.
2. Add integration-specific detail adapters.
3. Add saved filters or search commands.
4. Add optional device-side reachability evidence.
5. Add personal Viewer Profile arrangements only with a clear merge model.

## Functional release acceptance

### Board

- [ ] One persisted board drives desktop and mobile.
- [ ] View mode launches every allowed entry.
- [ ] Search finds long, duplicate, and grouped names.
- [ ] Group labels and order remain stable.
- [ ] Missing icons have a restrained local placeholder.
- [ ] Status refresh does not move application targets.
- [ ] Hidden items are absent from the response and document.

### Editing

- [ ] Add, edit, move, resize, group, remove, restore, and Undo work.
- [ ] Pointer, touch, and keyboard paths call the same operations.
- [ ] Edit mode prevents accidental launch.
- [ ] Save state is honest during delay and failure.
- [ ] Stale revisions cannot overwrite newer work.
- [ ] The latest confirmed board survives restart.

### Health

- [ ] HTTP success rules are configurable and bounded.
- [ ] TCP checks use strict address and timeout rules.
- [ ] Authorization responses are not false outages.
- [ ] Redirects and certificate failures have distinct reasons.
- [ ] Cached, stale, paused, and unknown states remain distinct.
- [ ] The board renders before checks finish.
- [ ] Check concurrency remains bounded with sixty entries.

### API and MCP

- [ ] Every visible capability has versioned API coverage.
- [ ] Web and MCP adapters use the same application operations.
- [ ] Missing, malformed, unknown, oversized, conflicting, and stale input has negative tests.
- [ ] Rejected input causes no fetch, probe, file write, storage write, or success audit event.
- [ ] MCP tools return structured receipts and stable error codes.
- [ ] MCP writes are Owner-scoped and audited.
- [ ] Secrets never appear in output or logs.

### Browser evidence

- [ ] The populated viewport matrix passes geometry checks.
- [ ] Accessibility checks pass for View and Edit modes.
- [ ] Keyboard focus remains visible and logical.
- [ ] Touch editing works on an actual coarse-pointer path.
- [ ] Forced colors and reduced motion preserve all meaning.
- [ ] Screenshots include open menus, Network Lens, conflicts, and failures.
- [ ] A reviewer inspects actual screenshots and names at least three weaknesses.
- [ ] Each material weakness becomes a regression test.
- [ ] The revised render passes a second critique.

## Performance acceptance

- The first board response must not wait for remote health checks.
- The first usable render must not require remote icon hosts.
- Direct manipulation must update locally within one animation frame.
- Status updates must not cause visible layout shift.
- Health work must preserve navigation and edit responsiveness.
- The test report must record latency, throughput, and allocation evidence on supported modest hardware.
- A sixty-entry fixture must remain usable while all checks refresh.
- Repeated failures must reduce traffic through backoff.

Do not claim a performance improvement without before-and-after measurements.

## Review scores

These scores apply to the unconstrained brief before this checklist.

- AI Slop Score: 8/10. “Dashboard” and “modern” strongly invite template output.
- Distinctiveness Score: 2/10. The empty repository has no product-specific signature yet.
- Implementation Readiness Score: 3/10. Core data, permission, status, and conflict rules are unresolved.

The “Household Network Helm” direction can target these release scores.

- AI Slop Score: 0–3/10.
- Distinctiveness Score: 8/10 or higher.
- Implementation Readiness Score: 9/10 before visible UI work begins.

## Final anti-slop gate

Do not ship if the interface still resembles a generic admin template.

Do not ship if desktop coordinates define mobile order.

Do not ship if drag is the only move control.

Do not ship if a health color lacks source, meaning, and age.

Do not ship if MCP can bypass Owner permission, validation, revisions, or audit.

Do not ship with any unresolved material finding, missing populated state, or missing responsive browser evidence.
