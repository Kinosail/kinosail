---
title: Interactive code atlas
description: Explore Kinosail's package graph, architecture missions, files, and source symbols.
section: Reference
last_reviewed: 2026-08-29
---

# Interactive code atlas

The Code Atlas is a standalone documentation tool for maintainers and contributors. It is separate from the Kinosail application and does not change the Server or web app.

[Open the interactive Code Atlas]({{ '/architecture-explorer/' | relative_url }})

Use the Map view to see first-party package dependencies. Use Table to compare package size and test coverage. Use Journey to follow guided questions through package boundaries, files, and source symbols.

The atlas is generated from the repository's Go package graph and production source files. Its source links point to the snapshot revision used to build the page. Refresh instructions and the reusable generation prompt live in the repository's [`engineering/architecture-explorer/`](https://github.com/Kinosail/kinosail/tree/main/apps/player/engineering/architecture-explorer) directory.

This page provides orientation and context. The atlas itself is a static HTML document so its graph interactions remain independent from the documentation site's Jekyll layout.
