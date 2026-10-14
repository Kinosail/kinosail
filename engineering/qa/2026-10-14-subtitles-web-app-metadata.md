# Subtitles production web-app metadata

## Public interface and finding

The production Subtitles CLI sets `SubtitleApp: true`.
This routes `/` to the subtitle dashboard instead of the legacy Home page.
The dashboard, subtitle inspector, and production Settings do not declare their public web-app manifest.
The sign-in page already declares it.

The existing `TestHomeIsInstallableAsAWebApp` uses the legacy configuration.
Its passing result does not establish manifest discovery from the production dashboard.
Production Subtitles also has no in-app Install action in its header.
That absence is not a failure of Player's install-help journey.

A document must associate its manifest for the browser to process that metadata.
See Chromium's [web-app concepts](https://chromium.googlesource.com/chromium/src.git/+/main/docs/webapps/concepts.md).
The [DevTools protocol](https://github.com/ChromeDevTools/devtools-protocol/blob/master/json/browser_protocol.json)
exposes the processed manifest through `Page.getAppManifest`.

## Regression before repair

The new journey uses the disposable production Subtitles Server and an isolated Owner account.
It visits Overview, Wanted, Library, History, the populated inspector, Settings, and the sign-in page.
Each document must contain exactly one same-origin link to `/manifest.webmanifest` in its head.
The public response must have the expected content type, identity, scope, start URL, and display mode.
Chromium must also process that manifest without errors.
Firefox and WebKit provide document and HTTP evidence.
The sign-in page is the existing-link control and uses the same isolated context after its cookies are cleared.
The six authenticated pages must retain their application navigation; sign-in must retain its authentication layout.

The journey records bounded results, the manifest checksum, and one screenshot per page.
The existing runner records the revision, command, environment, source pins, and artifact checksums.
No response is mocked and no synthetic document replaces the application.

The existing isolated asset test now uses the production configuration before any production repair.
Its retained coverage gap is icon decoding and worker response headers, which the new metadata journey does not check.
Relevant failures are a missing document association, wrong manifest identity or content type, invalid icon dimensions, and incorrect worker headers.

Local application and browser execution remain held.
Syntax, source-size, and whitespace checks do not establish a failing runtime baseline.
The hosted baseline must fail for the missing association before production code changes.
Browser metadata does not establish successful operating-system installation or physical-device behavior.

## Hosted baseline

Baseline commit `35c13a418a44157c8a9b9cc7cc9b2f3f6b0c40a0` adds the regression before production changes.
Its isolated asset test fails because production Home lacks `rel="manifest"`.
The hosted checkout `655826b768c1bf31209cec42a13788a94035532c` has the same tracked tree as that baseline.

WebKit run `38583458310` reaches all six original page cases.
Each document has zero manifest links, while the public manifest returns HTTP 200 with valid identity and content type.
Artifact `11905540149` has SHA-256 `cc252fe7fcdcdb20fc1ca4f9e17d1a62b0310b98d6d6585c9dcf6efdbe308f3d`.
The archive digest, clean revision, and all nine direct script pins verify.
The existing 46 Subtitles snapshots and seven accessibility audits pass before the new flow fails.

The original Settings control assumed the legacy template was served.
Actual response evidence and `settings_routes.go` show that production selects `subtitleSettingsView` instead.
Its response omits the manifest and the application shell adds navigation.
The corrected fixture requires that navigation and uses sign-in as the existing-link control.
This correction does not remove the failing manifest requirement from Settings.

Firefox times out during an earlier library navigation, before reaching the manifest flow.
Its trace records a 30-second navigation timeout; the later failure screenshot shows the populated Library.
That timeout remains unexplained and is not accepted as manifest-baseline evidence.
Artifact `11903854976` has SHA-256 `5bc70e90c1894b18027e3829708b623ee7e7aa948b09db13af56e87111e0f14b`.

## Repair

The dashboard, inspector, and production Settings templates now declare the existing `/manifest.webmanifest` resource.
The dashboard link belongs to its full-document shell, so partial content responses keep their existing shape.
The sign-in template retains its original link.
No manifest values, navigation controls, or service-worker behavior change.

## Verified repair and delivery

At `3b3e2ecd5cfe17bfa79f68a6a1aae33eaef5d408`, run `38586349263` passes all seven metadata cases in each engine.
Chromium also processes the expected manifest without errors.
The surrounding layout checks pass with zero overflow, terminal skeletons, identified layout shifts, or accessibility violations.
Chromium records 97 Player and 111 Subtitles measurements. Firefox and WebKit each record 42 and 46.
All 21 new page screenshots were inspected. The 33 accessibility audits report zero violations.

| Engine | Artifact | Archive SHA-256 |
| --- | --- | --- |
| Chromium | `11907410054` | `971bad5030568d4f1d561ffb4c3665277c363a5daa3a5d95c8bc721a49cd7724` |
| Firefox | `11905144734` | `a9ceab33fe63ee5266a56b9cf58f09a33365ba32a3133e2c9066c7ea067f4825` |
| WebKit | `11905499085` | `3fec902c0e87627673361bc67f4bda52c9beb027ae249474be784db05154642e` |

Each archive's clean revision and all nine direct script pins verify.
The retained fixture suites pass separately: 35 passed and two skipped in Chromium and WebKit, and 37 passed in Firefox.
Those isolated fixtures are not described as populated-server proof.

Required CI `38586349662` passes. The Subtitles quick Go suite and all 38 selected Chromium smoke tests pass.
The hosted PR checkout `31c372bb06cabaff45464476adfacf2575f4ad8c` has the same tracked tree as the task commit.
The local `verify-changed` invocation was plan-only under the runtime hold.

PR #602 merges with commit `9457698061136b33fb3eca0f1c9858ae60a4b66a`.
A fetched ancestry check proves that the reviewed task commit is included in remote main.
Exact-main run `38591712153` and all nine publication jobs pass.
Registry inspection verifies both apps, both architectures, revision labels, tested candidates, and commit/main/latest tags.
The Player index digest is `sha256:554ea175ae27f3c52580103345ed14b1b90bb34685c567d5e4ef5a05ff776dfb`.
The Subtitles index digest is `sha256:321029851b6a06c9712c194d411f302f77a1bc7993cf84d14a8b375888bef8f9`.
These are published registry artifacts; no local image execution or deployment is claimed.
The earlier Firefox timeout remains unexplained. OS installation, deployment, and physical-device acceptance remain separate.
