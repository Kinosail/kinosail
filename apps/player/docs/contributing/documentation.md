---
title: Documentation guide
description: Maintain accurate and accessible task-oriented Kinosail documentation.
section: Project
last_reviewed: 2026-09-15
---

# Documentation guide

Write each page for one reader task. Separate tutorials, operational procedures, reference, and explanation. Start with the outcome, prerequisites, and the directory in which commands run.

## Accuracy

Check ports, configuration names/defaults, file permissions, mount behavior, API routes, and flags against the installed source version. Distinguish source setup from a published signed container. Explain expected output, likely failures, recovery, and the next step. Keep app-specific workflows in their own app's guides.

Use reserved example domains and synthetic media. Never include credentials, private addresses, account data, personal library titles, or unlicensed artwork.

## Structure and images

Use front matter with a unique title, description, section, and honest review date. Keep one H1 and a logical heading order. Add pages to `_data/navigation.yml` and link related tasks. Use the `relative_url` filter for site paths so the site works below a hosting subpath.

Published instructions must work without screenshots. Add a local optimized image only when it clarifies a step, with useful alt text and a caption. Keep unfilled screenshot plans in engineering notes; do not ship empty screenshot placeholders as user instructions.

## Preview and verification

The docs README explains Jekyll preview and deployment settings. Inspect rendered headings, links, code, tables, search, light/dark themes, narrow reflow, keyboard navigation, and focus at the intended hosting base path. A source review does not prove a deployed site works.

Repository gate policy still applies: while `.gates-disabled` exists, do not run disabled suites or describe skipped checks as passing. Record manual observations separately. Site publication, signed app release, and application acceptance are different outcomes.
