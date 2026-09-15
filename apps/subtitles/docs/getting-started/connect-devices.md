---
title: Access Subtitles from another device
description: Reach the administration UI on a trusted network.
section: Start here
last_reviewed: 2026-09-15
---

# Access Subtitles from another device

Complete localhost Owner setup first. On another device, use the host's reachable HTTPS address and configured port 38128; `localhost` on a phone identifies the phone itself.

Keep certificate trust and accepted hostnames consistent with the chosen origin. A generated local certificate must be trusted appropriately, or configure trusted HTTPS. Changing the passkey origin can require enrolling passkeys again.

Subtitles is an administration and automation app. Use Kinosail Player or another media player to view the resulting sidecars. You do not need Jellyfin compatibility, a public listener, or a router forward for providers to find subtitles.

For remote administration, use a private connection. See [access security]({{ '/owner-guide/security/' | relative_url }}) and [troubleshooting]({{ '/troubleshooting/sign-in-and-access/' | relative_url }}).
