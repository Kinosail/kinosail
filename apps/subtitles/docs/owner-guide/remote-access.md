---
title: Private access to Subtitles
description: Keep subtitle administration on a trusted connection.
section: Own the Server
last_reviewed: 2026-09-15
---

# Private access to Subtitles

Provider search/download traffic is outbound. It does not require opening the Subtitles web port to the internet.

Use the local browser, a private SSH forward, or the Server's configured private-management connection for remote administration. Finish Owner setup and strong authentication before enabling other devices. Do not expose the administration listener through a router forward.

Public media viewing belongs to Kinosail Player's separate restricted HTTPS boundary. Subtitles does not need that workflow to write sidecars. See [device access]({{ '/getting-started/connect-devices/' | relative_url }}) and [security]({{ '/owner-guide/security/' | relative_url }}).
