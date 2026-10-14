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

Hosted verification of the repair remains pending.
