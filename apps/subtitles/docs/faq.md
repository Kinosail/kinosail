---
title: Subtitles FAQ
description: Answers about providers, privacy, writes, and app boundaries.
section: Project
last_reviewed: 2026-09-27
---

# Subtitles FAQ

## Do I need Kinosail Player?

No. Subtitles writes sidecars for videos in folders you control. A compatible player can use those files.

## Do I need a provider?

Local scanning and coverage work without one. Provider downloads require a configured account/key and available quota. See [provider setup]({{ '/owner-guide/integrations/' | relative_url }}).

## Are my videos uploaded?

No. Providers receive bounded search metadata and, where used, a locally calculated hash. See [privacy]({{ '/reference/architecture-and-privacy/' | relative_url }}).

## Can it replace an existing sidecar?

Only a sidecar that Kinosail wrote and that nobody changed since. Kinosail replaces it when a candidate scores at least ten points higher, and keeps the previous file as a `.kinosail.bak` recovery copy. Kinosail never replaces a sidecar that you or another app created, or a Kinosail file that someone edited. SubSource files stay unchanged under its terms.

## Can I keep fewer subtitle files or playback choices?

Yes. **Playback subtitle choices** can hide extra tracks without changing files. Optional cleanup is off by default and shows the files it would hide before confirmation. It keeps file contents beside each video with a `.hidden` suffix. Remove the suffix to restore a file. Back up your sidecars first and follow [Configure languages and automation]({{ '/owner-guide/playback/' | relative_url }}).

## Does an app backup include my subtitles?

No. Sidecars live in the media tree. Back up media and sidecars separately from application state, external configuration, and keys.

## Where are the production downloads?

Passing builds on `main` publish a signed `ghcr.io/kinosail/kinosail-subtitles:latest` container. Use the [Docker install guide]({{ '/getting-started/install/' | relative_url }}). Numbered GitHub releases are separate from the current container publication flow.

## Do I need to pay to use subtitle search?

No. Every core feature is free on your own Server. [Optional Supporter badges]({{ '/owner-guide/supporter/' | relative_url }}) recognize monthly or one-time support.
