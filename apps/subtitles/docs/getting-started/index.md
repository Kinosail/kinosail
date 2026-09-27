---
title: Get started with Kinosail Subtitles
description: Install the published subtitle container and save your first validated subtitle sidecar.
section: Start here
last_reviewed: 2026-09-27
---

# Get started with Subtitles

You need Docker or Podman and an existing movie or episode folder that the container can read and write. The published container supports 64-bit Linux hosts; macOS uses the container engine's Linux VM.

1. [Install the Server]({{ '/getting-started/install/' | relative_url }}) on localhost.
2. [Create and secure the Owner]({{ '/getting-started/first-setup/' | relative_url }}).
3. [Choose media folders]({{ '/getting-started/add-media/' | relative_url }}) and a language.
4. [Configure one provider]({{ '/owner-guide/integrations/' | relative_url }}).
5. Fetch one wanted subtitle and check the result before relying on automation.

The `latest` image is published from a passing `main` build. A numbered GitHub release is not required to install it.
