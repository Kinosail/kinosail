# R03 web progress recovery

The approved audit identified routine web progress saves that ignore failures. This
campaign starts from `2e9ede47`. The audit is historical evidence, not runtime proof.

Before implementation, the failure analysis covers network rejection, HTTP error,
redirected login, hung requests, late responses after newer revisions, profile or
item changes, watched completion followed by pause, automatic next navigation,
audio selection, page hiding/closing, and offline or cast ownership changes.

The Server's web POST returns 204 after the validated operation settles. Repeating
the same session/revision is a no-op once accepted. Older revisions are rejected
only within the same session. Another session can overwrite state, so this change
does not persist or replay an old web session after navigation or tab closure.

`player-progress.spec.ts` is a retained isolated browser regression. It drives the
actual shared progress asset at its HTTP boundary with rendered media fixtures and
controlled responses. Populated Server journeys cannot deterministically delay
old responses, reject a browser connection, or simulate every ownership race. It
is not populated-server E2E proof. Separate populated proof is recorded below.

Recovery retains one bounded position in page memory, scoped to the current Viewer
Profile, item, and playback session. No credentials or URLs enter browser storage.
Final save failure must offer Retry and Continue without saving. Manual navigation
remains available. Page-close keepalive is best effort, not durable recovery.


## Local results

Before code, four isolated Chromium runs failed: HTTP 503 and rejected connections,
each repeated twice. The baseline never showed an unsaved notice. The red log is
`red-browser.log`. A separate page-close regression exposed serial dispatch losing
the latest keepalive while an older request was in flight (`close-red-browser.log`).

The final focused isolated suite passed 25 cases (`review-browser.log`). An earlier
combined run passed 47 progress, seeking, and startup cases (`verified-browser.log`).
The final progress changes add localized copy, audio-queue completion, and safe DOM
diagnostic metadata. Broader startup assertions were unchanged.

Use the E2E directory as the working directory. Existing startup fixtures resolve
CSS from that directory. One root-directory run failed to load that fixture CSS;
it passed when rerun from the correct directory. An early diagnostic assertion
incorrectly inspected Chromium's shortened console preview. It was corrected;
the final diagnostic path uses bounded DOM metadata and makes no console writes.
All failed run artifacts are preserved in `.verification/r03-progress/browser-history`.

Repeat focused browser checks from `apps/player/e2e`:

```sh
KINOSAIL_BROWSER_WORKERS=1 KINOSAIL_E2E_VIDEO=off \
  node node_modules/@playwright/test/cli.js test player-progress.spec.ts \
  --project=chromium --workers=1 --reporter=line
```

Browser lint passed with zero errors and zero warnings (`browser-lint.log`).
The strict populated-results helper passed three contract tests (`ci-helper-green.log`).
The explicit-required-title cases failed before its extension (`ci-helper-red.log`).
The helper still requires both existing settings journeys by default. It rejects
missing, skipped, failed, or duplicate selected journeys.

```sh
node scripts/quality/lint-browser-scripts.mjs
python3 -m unittest scripts/ci/test_populated_settings.py
make max-loc
git diff --check
```

## Server, cache, and logging boundaries

The populated proof runs a native Go Kinosail Server on loopback with generated
12-second media and disposable Owner/MFA state. It does not touch real user data.
The original progress asset is replayed against that unchanged public Server API;
this reproduces the old browser behavior, not a historical deployed binary.
The candidate uses the real Server and allows its service worker.

The final run passed at committed `7956411098dba77931c30cd5966a6e542eae115d`.
Baseline rejection reproduced twice; its two candidate-only repetitions were
explicitly skipped. All four candidate repetitions passed. `native-proof.json`
records the tested source hashes and safe artifact checksums. Selected screenshots
and browser logs are in `native/`. The integration owner reviewed this exact
implementation commit and reported no findings before the final Server run.

The rejected-token public request returns HTTP 400 and leaves stored progress
unchanged. Browser notice recovery first intercepts HTTP 503, then passes Retry
to the real Server. Only HTTP 204 hides the notice. The stored item's seconds and
revision must match the latest pending position. Candidate checks also cover real
pending, empty, failed, and success states, 390/1440/1920 widths, and scoped axe.

The effective Player script URL already includes the bundle's SHA-256 in
`apps/player/internal/server/player.go`. The real Server regression verifies that
hash, the immutable response header, and delivery of the changed progress source.
No locale asset token bump is required.

Changed browser failures expose bounded class, revision, playback session, and
request ID on the status element. Values are validated; URLs, bodies, credentials,
and remote error strings are absent. Browser lint forbids console logging, so this
is a local diagnostic projection rather than persistent browser log history.
Existing Server progress logs correlate the route, outcome, request ID, and
playback session. `server-progress-diagnostics.json` is a safe selected projection;
raw process logs remain private in the disposable fixture directory.

Run the repeatable native proof from repository root:

```sh
GOMAXPROCS=2 python3 scripts/testing/test-player-progress-local.py
```

The script preserves receipts, checksums, synthetic data, screenshots, and raw logs
under `.verification/r03-progress/<timestamp>`. Do not share raw traces or state.
Each receipt records the revision, diff hash, commands, environment, outcome, and
source checksums. Baseline and candidate each repeat their applicable cases twice.

## CI integration and remaining checks

The integration owner adds `test-instance-progress.spec.ts` to the prepared-Owner
command called by `apps/player/scripts/test-container.sh`; its focused
`test-browser-journeys.sh` helper keeps the existing container script within the
300-line source cap. Four explicit required titles
retain both existing settings journeys and require both progress journeys below.
The separate R04 Go library-pagination invocation remains unchanged:

- `real Server rejects invalid progress without changing stored state and web reports the rejection`
- `populated player retries the latest progress through the real Server and renders accessible states`

Both tests are tagged `@smoke`. The helper prepares credentials through the existing
CI path; the local proof is a separate disposable native fixture.

Committed affected-app `verify-changed`, protected CI, reconciliation, PR merge,
and ancestry proof are integration-owner obligations. Review the root's CI hook
integration separately from the reviewed implementation commit.
No container, TLS deployment, physical device, physical TV, Safari, Firefox, or
native-client playback proof is claimed. The shared template/asset also serves
Subtitles; its affected checks remain required at integration.

The committed readable red logs normalize trailing horizontal whitespace only.
Their deterministic `.log.gz` companions preserve the byte-exact originals from
`6ec0b5c944ad4651e358e9302875c605c615785b`. The previous diff-check claim covered
only the implementation owner’s working-tree diff; it did not check the committed
QA delta. The integration owner verifies the complete main-to-candidate diff.

The first protected browser run failed five isolated smoke cases before their
assertions: the hard-coded fixture route did not match the hosted Server base URL,
so navigation reached the real Server and its content security policy correctly
blocked inline fixture scripts. The isolated spec now pins its own synthetic
origin. Server CSP and native populated tests are unchanged. Historical source
hashes remain pinned to `795641`; integration hashes record each explicit source override.

The corrected isolated spec passed all 25 cases in 12.14 seconds under an
external hosted-style base URL, with zero skips, unexpected or flaky results.
`ci-origin-green.json` pins the test source and environment; its log and exact
compressed original are retained. This remains isolated browser proof.

## New-track progress drain

A separate isolated HTTP regression reproduced a second ordering failure on the
frozen original sender and queue, excluding the R08 metadata/loading changes.
The old watched save settled, the actual audio source changed to `/media/next`,
and a pause at three seconds occurred before the play continuation was released.
Only the old watched request was dispatched; the pending next-track save remained
in memory. One intended assertion failed, with zero skips or global errors.
`drain-frozen-red.json` pins the exact unchanged sender and controlled run.

The minimal correction continues the sender loop when the continuation creates
a newly owned pending position. Its standalone smoke regression requires both
HTTP dispatches and the correct item, seconds and watched payloads. The corrected candidate passed all 27 focused cases; final protected Server
checks remain required before merge.
The isolated media-promise fault is separate from actual Server/media proof.

Independent review identified a superseding page-close flight race in the first
drain candidate. A test-only assignment initially returned the held Promise and
timed out; that receipt is preserved as inconclusive. After correcting only the
synchronization expression, the browser regression reached the intended HTTP
assertion and observed a duplicate next-track revision. The final guard also
requires the sender to own the current flight before draining. It preserves the
closing 403 policy notice and does not repeat the latest position.

At committed `d72451da`, the existing 25 progress cases and both queue ordering
cases passed together: 27 expected, zero skips, failures, flaky results or global
errors (`queue-drain-green.json`). The hosted-style external base URL remains
independent of both isolated synthetic origins; production CSP is unchanged.
Historical original populated proof remains explicitly pinned to `795641`; the
new isolated sender proof does not replace final protected populated checks.

## Final guarded-source Server proof

The integration owner reran the native Go Server fixture at clean committed
`b4024370`. Both baseline rejected-write repetitions and all four candidate
repetitions passed. Only the two baseline candidate-only cases were skipped;
the candidate had zero skips. The current delivered content-hash bundle, latest
stored seconds/revision, rejected-token no-side-effect behavior, responsive
notice states and scoped accessibility assertions are exercised again.
`fresh-native-proof.json` and `fresh-native/` preserve safe source/artifact hashes
and selected screenshots; raw credentials, process logs and state remain local.

Post-commit Player/Subtitles checks stop at 94 identical baseline lint findings
in 56 unchanged cited Go sources. Focused shared Go template tests pass. The
required local container attempt cannot connect to the existing Podman socket.
Final protected populated checks still gate merge; no local test was disabled.
