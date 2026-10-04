# R04 implementation: retain paginated Show cards

Status: tested locally; integration and required hosted gates remain with the integration owner.
Baseline: `2e9ede47aa56f5177b1e994ffd4b1c62f6c445ce`.
Tested product, regression and CI-hook revision: `30eac3c60bddf8a931deb3cca60511fcb73e5dbc`.

Two independent Chromium runs against a real disposable Go Server reproduced
R04. The API returned a total of 30 Shows, while the page announced completion
with only four Show cards. Mixed pagination retained only two of 30 Shows.
Both phone and desktop failures are preserved in `red-1/` and `red-2/`, with
logs, screenshots, and traces. These are runtime reproductions of the public
Server contract, beyond the historical source-only audit.

The card identity now comes from the bounded local canonical detail link.
Show wrappers use their nested Show detail link. Incoming cards are validated
before any append. Repeated cards are suppressed within and across pages,
including newly introduced media groups. A late or aborted continuation cannot
append after its Library is replaced. Ordinary pagination remains available
without IntersectionObserver, and a failed request exposes keyboard retry.
Safe failure class and bounded request correlation are attached to the existing
status element; no console payload or new telemetry is added.

The pagination responsibility is split into `pwa-library.js` to preserve the
300-line source cap. Go composes it into the existing PWA asset and both apps'
existing public bundles. Effective immutable main bundle tokens advance from
Player 34 to 35 and Subtitles 18 to 19. Existing routes and cache policy remain.

The retained regression starts the real Server with 30 synthetic Show folders
and six synthetic Movie files, then runs Playwright against actual API, HTML,
and embedded script responses. Overlap, malformed fragment, 503, and delayed
response cases intercept only disposable fixture responses and are explicitly
isolated fault checks. They do not establish production incident frequency.
The hosted Player browser gate invokes the runner for each selected browser,
including smoke runs, with one worker, independent artifacts, and a six-minute
bound. Ordinary Go runs keep the browser runner opt-in.

The populated Chromium green run passed all 10 tests: zero skipped, unexpected,
flaky tests or runner errors. The browser report records 23.402 seconds; the Go
wrapper completed in 30.462 seconds. It establishes all 30 unique Show cards at
390px and 1440px, all 36 mixed cards, overlap suppression, rejection of five
malformed identity variants before partial append, keyboard retry, preservation
of newer search results, and ordinary next/previous links without an observer.
The independent card-count and completeness assertions were retained.
`green-report/results-all.json` and `green-report/html-all/index.html` preserve
the detailed report. `green/` contains 16 full-page success screenshots.

The pending, failed, loaded and empty states were visually inspected at 390px,
1440px and 1920px. Cards retain their geometry and status/retry controls remain
usable. Twelve lossless viewport crops in `review-crops/` make the pagination
footer states inspectable; original screenshots are unchanged. Crops use macOS
`sips -c HEIGHT WIDTH --cropOffset OFFSET 0 SOURCE --out TARGET`, with HEIGHT
bounded to 1000px and OFFSET=max(0, original_height-HEIGHT-1). PNG dimensions
were independently checked after creation.

Additional checks passed:

- Source cap, browser lint (zero errors/warnings), shell syntax/shellcheck,
  and browser-project selection/signature contracts.
- Eight Node loading/bundle resolver contracts.
- Three Chromium scroll and Player/Subtitles worker upgrade cases.
- Twenty-five Chromium HTMX pending, error, abort, timeout and stale-result cases.
- Independent source review of the exact product revision found no production
  correctness or security issue. A proposed fixture-group placement concern was
  withdrawn after the reviewer verified the real response topology in the red trace.

Repeat the populated run from `apps/player`:

```sh
GOMAXPROCS=2 KINOSAIL_LIBRARY_BROWSER=1 KINOSAIL_E2E_VIDEO=off \
  KINOSAIL_E2E_OUTPUT_DIR=/absolute/task/evidence/green \
  KINOSAIL_E2E_ARTIFACT_DIR=/absolute/task/evidence/green-report \
  ../../scripts/tooling/with-go-module.sh go test -p 1 ./internal/server \
  -run '^TestLibraryPaginationBrowserJourney$' -count=1 -timeout=6m
```

This macOS run reused the standard Go cache through a scoped, approved command.
It reused the available exact Playwright 1.63.0 dependencies and locally installed
Chrome via the Chromium project. The exact hosted invocation is in
`apps/player/scripts/test-container.sh`. Its browser selector is passed through
`KINOSAIL_BROWSER_PROJECT`; its artifact and output directories are separate
from the surrounding container journeys. The ordinary `test-instance-check`
builds the full populated container/media suite and is reserved for integration.

Readable generated red logs/error contexts have trailing whitespace normalized.
Exact original bytes are preserved in adjacent `.gz` files, verified against the
first checkpoint commit; original traces and screenshots are unchanged. The
manifest records all source and evidence SHA-256 checksums. A directory secret
scan covers the readable evidence; it does not establish a binary-secret guarantee.

Limits: this local proof does not cover deployment, physical devices, real
media decoding, authenticated Viewer filtering, or live household libraries.
Fresh Firefox/WebKit, fresh Subtitles Server pagination, the container-backed
`test-instance-check`, complete affected-app verification, required GitHub
checks, merge and ancestry proof remain with the integration owner until
explicitly recorded. No production data, Nox or native device state was changed.
PiP/miniplayer remains excluded.

MAIN: NO — awaiting integration, required hosted checks, merge and ancestry proof.
