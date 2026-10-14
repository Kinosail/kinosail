# Admit the existing beta page audit

The campaign still lacks accepted populated evidence for 54 existing beta page cases.
They cover six widths across library, utility, settings, onboarding, media, and created pages.

The hosted runner adds one fixed selection for `layout-audit-beta.spec.ts`.
It uses one Chromium worker, no retries, and stops after the first failed case.
Acceptance requires exactly 54 passing results and complete owned-fixture cleanup.
The default admission selection and existing job time limits stay the same.

Runner failures include an omitted selection, failed browser launch, skipped cases, and missing or extra results.
The retained command-stand-in contract covers these failures before any real browser can observe them.
The test-first check failed six assertions before runner admission was implemented.
These contracts are isolated runner checks, not populated-browser proof.

The existing page tests remain unchanged for the first hosted run.
Potential prerequisite failures need qualification before any product repair:
strict fixture TLS in fresh browser contexts, populated media routes, and SCIM section visibility.
The test also records accessibility rules, control inventories, and full-page images.
Full-page images alone do not prove viewport clearance for sticky controls.

Hosted execution and artifact inspection are pending.
No local application runtime ran during the workstation resource hold.
