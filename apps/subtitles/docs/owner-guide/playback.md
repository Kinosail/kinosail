---
title: Configure language and automation
description: Choose subtitle language, role, and scanning behavior.
section: Own the Server
last_reviewed: 2026-09-15
---

# Configure language and automation

In the subtitle setup plan, select the primary language and the preferred subtitle role (standard dialogue or SDH/captions). Choose the appropriate catalog language tag, such as `en` or `pt-br`. The deployment equivalent for the primary language is `KINOSAIL_SUBTITLE_LANGUAGE`.

Existing embedded text is preferred before requesting a provider download. A language-tagged sidecar covers that language; changing the language does not translate a sidecar.

Choose a safety scan schedule in setup. Filesystem events help detect completed copies, and scheduled scans catch changes they miss. Background provider maintenance runs in bounded cycles with a minimum 15-minute interval. Provider quotas and failures can leave items wanted for a later cycle.

Use a one-item fetch to validate your plan before relying on maintenance. See [subtitle management]({{ '/user-guide/' | relative_url }}) for upgrade and preservation rules.
