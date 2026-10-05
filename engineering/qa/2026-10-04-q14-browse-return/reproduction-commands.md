# Q14 prepared runtime commands

These commands have not run. Start only after the parent grants the serial slot.
Use the leased `codex/qol-q14-browse-return-tests` checkout. Preserve each RED
repeat separately. No production, container, encoder, or device is involved.

From `apps/player`, run the prepared actual Go/browser owner with a unique
absolute output directory and an external process-group deadline:

```sh
GOMAXPROCS=2 PLAYWRIGHT_CHANNEL="" KINOSAIL_BROWSER_PROJECT=chromium \
KINOSAIL_BROWSER_WORKERS=1 KINOSAIL_E2E_VIDEO=off \
KINOSAIL_BROWSE_RETURN_BROWSER=1 KINOSAIL_BROWSE_RETURN_CASES=primary \
KINOSAIL_BROWSE_RETURN_MEDIA="/Users/mikeo/Documents/Codex/2026-10-04/task-2/r03-progress/.verification/r03-progress/20261004T075204Z/media/R03 Example.mp4" \
KINOSAIL_E2E_OUTPUT_DIR="<unique absolute run path>/browser" \
KINOSAIL_E2E_ARTIFACT_DIR="<unique absolute run path>/report" \
../../scripts/tooling/with-go-module.sh go test -v -p 1 ./internal/server \
  -run '^TestBrowseReturnBrowserJourney$' -count=1 -timeout=70s
```

Proposed initial external bound: 75 seconds per primary process. Each primary
run registers exactly two cases (390px and 1440px); run it twice. A Go timeout,
external timeout, missing fixture, navigation/setup error, or incomplete JSON is
not a completed feature RED. Retain its artifacts but stop before implementation
until the harness is corrected and both baseline runs complete.

Separate grants select `cold` (two cases), `bfcache` (one cache-admission
control), `htmx` (one in-document history case), `shows` (two Show journeys), or
`search` (one changed-URL case) with the same environment and distinct paths.
`all` registers nine cases; it is prepared for later integration, not the first
RED slot. The Go runner pins one worker and zero retries. No selected mode has
an intentional browser skip. Without opt-in, ordinary Go runs skip this fixture
and ordinary browser runs mark it unavailable; those are not runtime proof.

The cache-control suite uses cached full Chromium (`channel=chromium`), removes
only Playwright's `--disable-back-forward-cache` argument and requires observed
native cache admission. Other suites use the pinned project's cached browser
with `PLAYWRIGHT_CHANNEL=""`. Record both actual browser executable/version
and its SHA-256, the exact committed source snapshot, package lock hashes, safe
environment selection, fixture digest, exit code, and raw report counts.

After a complete run, preserve raw JSON, screenshots, traces, peer projections
and stdout. Hash every artifact, exclude the manifest itself, and record sorted
`path<TAB>sha256<LF>` source aggregate bytes. Compress bulky text losslessly only
with original/compressed byte counts, digests, and a round-trip comparison.
Stop all owned Go/browser processes before releasing the serial slot. Do not
clean or change another owner's processes or the fictional source clip.

Runtime proof and all consumer/protected gates remain pending. The parent owns
CI hook integration; this checkpoint does not edit existing gates.
