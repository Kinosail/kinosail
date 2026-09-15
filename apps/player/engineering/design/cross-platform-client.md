# Cross-platform client design review

## Design direction

The client is a household media screen for touch devices and television remotes. The primary action is resume or open. Media context is second. Account and theme controls are third.

The composition uses a cinematic asymmetric hero and horizontal library shelves. Setup uses one focused connection task. Errors explain recovery without hiding a server conversion. Desktop, tablet, compact, and 320 px layouts change density without changing task order.

The interface reuses Kinosail's dark cinema surface, lime signal, editorial titles, local imagery, and direct-media language. It avoids generic dashboards, equal card grids, decorative glass, purple gradients, and multiple competing actions.

## Token decisions

The client reuses the semantic background, surface, raised, text, muted, line, signal, signal-ink, focus, and danger roles from `DESIGN.md`. It uses the 4 px and 8 px spacing rhythm. Controls use a 10 px radius. Media uses a 16 px radius. Motion is limited to image loading, press feedback, and television focus scale.

Applicable states include default, pressed, focus, disabled, busy, loading, populated, empty, connection error, malformed response, unsupported direct playback, and recovery. Dark is the default. Light and system preferences persist.

## Layout strategy

The hero reserves visual space for the selected backdrop. Library shelves virtualize off-screen media. Touch controls have a minimum 44 px target. Television controls use 56 px targets and visible focus borders. Compact layouts reduce the brand lockup and optional account text before controls wrap.

## Implementation

Expo Router supplies three routes: home, details, and playback. One strict server client validates all boundary data. Secure storage keeps the server URL and bearer token on native systems. Native playback controls provide seek, audio, subtitle, and picture-in-picture capabilities when the device supports them.

The first critique found a fabricated progress percentage, eager shelf rendering, and crowded 320 px account content. The revision replaced the percentage with an explicit state badge, virtualized shelves, and removed optional viewer text below 360 px.

## Anti-slop check

- AI Slop Score: 1 of 10. The only residual risk is repeated poster geometry.
- Distinctiveness Score: 9 of 10. The media hierarchy, Direct First language, brand mark, and cinema composition identify Kinosail.
- Implementation Readiness Score: 9 of 10. Shared states, tokens, routes, tests, and native generation are present.

The populated browser gate covers 1440 px, 1024 px, 390 px, and 320 px. It also covers keyboard focus, reduced motion, forced colors, theme persistence, details, and direct playback recovery.

## Remaining trade-offs

Browser rendering cannot prove remote focus, platform media controls, codecs, interruptions, or store behavior. Native compilation proves integration, not physical playback. Release acceptance must use signed builds and representative devices.
