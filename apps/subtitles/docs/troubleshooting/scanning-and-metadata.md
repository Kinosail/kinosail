---
title: Fix scanning and matching
description: Check paths and video identity before retrying providers.
section: Fix a problem
last_reviewed: 2026-09-15
---

# Fix scanning and matching

Confirm that the video exists inside the container's media mapping, not just on the host. Check selected relative library folders, read permissions, and whether copying has finished. Run a scan and inspect the reported error/status.

For missing provider results, check movie year, episode identifiers, release information, chosen language, configured credentials, and account quota. A provider outage or lack of a suitable release is different from a scanner failure.

An existing sidecar or embedded text track can already satisfy coverage. A protected unknown sidecar will not be replaced merely because another candidate exists. Review [maintenance rules]({{ '/user-guide/' | relative_url }}).

See [media layout]({{ '/getting-started/add-media/' | relative_url }}) and [provider setup]({{ '/owner-guide/integrations/' | relative_url }}).
