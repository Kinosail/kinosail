# Subtitles production web-app metadata

## Public interface and finding

The production Subtitles CLI sets `SubtitleApp: true`.
This routes `/` to the subtitle dashboard instead of the legacy Home page.
The dashboard and subtitle inspector do not declare their public web-app manifest.
Settings already declares it.

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
It visits Overview, Wanted, Library, History, the populated inspector, and Settings.
Each document must contain exactly one same-origin link to `/manifest.webmanifest` in its head.
The public response must have the expected content type, identity, scope, start URL, and display mode.
Chromium must also process that manifest without errors.
Firefox and WebKit provide document and HTTP evidence.
Settings is the existing-link control and must retain its separate page layout.

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
