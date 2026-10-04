# R04 implementation: retain paginated Show cards

Status: implemented; final populated green run is queued with the integration owner.
Baseline: `2e9ede47aa56f5177b1e994ffd4b1c62f6c445ce`.

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

Passed before the final populated run:

- Source cap, browser lint, shell syntax/shellcheck, and diff checks.
- Eight Node loading/bundle resolver contracts.
- Three Chromium scroll and Player/Subtitles worker upgrade cases.
- Twenty-five Chromium HTMX pending, error, abort, timeout and stale-result cases.

Repeat the populated run from `apps/player`:

```sh
GOMAXPROCS=2 KINOSAIL_LIBRARY_BROWSER=1 KINOSAIL_E2E_VIDEO=off \
  KINOSAIL_E2E_OUTPUT_DIR=/absolute/task/evidence/green \
  ../../scripts/tooling/with-go-module.sh go test -p 1 ./internal/server \
  -run '^TestLibraryPaginationBrowserJourney$' -count=1 -timeout=6m
```

The exact hosted invocation is in `apps/player/scripts/test-container.sh`.
Its browser selector is passed through `KINOSAIL_BROWSER_PROJECT`; its artifact
and output directories are separate from the surrounding container journeys.

Limits: this local proof does not cover deployment, physical devices, real
media decoding, authenticated Viewer filtering, or live household libraries.
Fresh Firefox/WebKit, fresh Subtitles Server pagination, the container-backed
`test-instance-check`, complete affected-app verification, independent review,
required GitHub checks, merge and ancestry proof remain with the integration
owner until explicitly recorded. No production data, Nox or native device
state was changed. PiP/miniplayer remains excluded.
