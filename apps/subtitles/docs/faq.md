---
title: Subtitles FAQ
description: Answers about providers, privacy, writes, and app boundaries.
section: Project
last_reviewed: 2026-09-15
---

# Subtitles FAQ

## Do I need Kinosail Player?

No. Subtitles writes sidecars for videos in folders you control. A compatible player can use those files.

## Do I need a provider?

Local scanning and coverage work without one. Provider downloads require a configured account/key and available quota. See [provider setup]({{ '/owner-guide/integrations/' | relative_url }}).

## Are my videos uploaded?

No. Providers receive bounded search metadata and, where used, a locally calculated hash. See [privacy]({{ '/reference/architecture-and-privacy/' | relative_url }}).

## Can it replace an existing sidecar?

Yes, under conservative maintenance rules. Managed files need a meaningful score improvement; unknown sidecars require an exact OpenSubtitles hash match and retain a `.kinosail.bak` original. SubSource files stay unchanged under its terms.

## Does an app backup include my subtitles?

No. Sidecars live in the media tree. Back up media and sidecars separately from application state, external configuration, and keys.

## Where are the production downloads?

As of September 15, 2026, this monorepo has no published GitHub releases. Use the documented source setup and evaluate the app's release checklist before production deployment. A source build is not a signed-release acceptance result.
