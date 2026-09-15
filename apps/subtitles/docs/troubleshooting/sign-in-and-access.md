---
title: Fix sign-in and access
description: Restore access without discarding application state.
section: Fix a problem
last_reviewed: 2026-09-15
---

# Fix sign-in and access

Use the configured Server origin, including HTTPS and port 38128. Check certificate trust and device time before retrying an authenticator. A passkey is bound to its enrolled origin; changing a hostname may require enrolling it again from an authenticated session.

The first Owner must complete required strong authentication. Use your saved recovery method or another authorized Owner where available. Do not delete the database or recreate volumes to reset access.

For another device, verify LAN reachability and the configured host/certificate names. `localhost` identifies the device making the request. Administration belongs on a trusted local or private management connection.

If deployment-managed settings are read-only, change them in their environment/YAML source. See [configuration]({{ '/reference/configuration/' | relative_url }}) and [security]({{ '/owner-guide/security/' | relative_url }}).
