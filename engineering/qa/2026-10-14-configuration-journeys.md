# Populated configuration journeys

The campaign still lacks accepted evidence for 11 existing configuration cases. Add a fixed `configuration` selection to the hosted populated-instance runner. Keep the existing assertions unchanged.

The selection runs `layout-audit-configuration.spec.ts` with one Chromium worker, no retries, and one maximum failure. It requires exactly 11 passing results. The cases cover SCIM, OpenID Connect, SAML, update controls, Media Shares, and six widths of Basic/Advanced settings navigation. These include geometry and accessibility checks, settings navigation, and saving synthetic OpenID Connect configuration.

Each run owns a disposable Server, synthetic media, credentials, browser trust, containers, and volumes. Teardown must remove those resources even after failure. Configuration changes stay inside that fixture. Retained traces can contain synthetic credentials and must remain private. The existing login helper disables conditional WebAuthn mediation; these journeys do not verify passkeys or an external identity provider.

The isolated runner contract protects a gap that browser E2E cannot cover: rejecting omitted or extra results and cleaning the fixture when the browser cannot start. Before implementation, the new selection contract failed six assertions against the old workflow and runner. The focused runner suite passes all 11 contracts. The full CI contract suite passes 203 tests with five documented skips. Syntax, ShellCheck, Actionlint, and the source-file cap pass. Hosted application evidence remains pending until the exact revision, selected identities, source hashes, results, screenshots, and cleanup are verified.

This checkpoint changes verification only. It does not change product behavior or clear any of the 152 remaining historical browser cases before acceptance. Local runtime remains on hold.
