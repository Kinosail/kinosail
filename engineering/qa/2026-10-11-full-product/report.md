# Full product QA — October 11, 2026

This ongoing audit covers Player, Subtitles, shared packages, installation,
delivery, and Apple and Android clients. It continues the
[October 10 audit](../2026-10-10-complete-audit/report.md).
Supporter and Home Assistant implementations belong to their own repositories.
Their Kinosail integration boundaries remain in scope.

## Run record

- Initial revision: `655b7d6db39028db19cad3a1e8dd84dbd77fb936`.
- Worktree: `codex/full-product-qa-20261011`, leased to this task.
- Private evidence: `/Users/mikeo/Documents/Codex/full-product-qa-20261011/`.
- Data: generated CC0 media, disposable Server state, and synthetic accounts.
- Local web environment: macOS ARM64, pinned Playwright 1.63.0,
  Chromium, Firefox, and WebKit. Browser versions appear in runner attachments.
- Hosted baseline: [deep run 38151255345](https://github.com/Kinosail/kinosail/actions/runs/38151255345).
  It passed after one unchanged rerun of the failed native snapshot job.
  The original native failure remains retained below.

The primary checkout's two unrelated Apple edits remain untouched. Eight
unrelated inactive worktrees contain dirty or unique work and remain preserved.
Another task owns the active artwork worktree.

## Journey inventory

For each domain, check outcomes and persistence; failure, empty, permission,
and recovery states; then input, accessibility, layout, console, and network.
Passing executions are evidence for named contracts, not a completeness score.

| Domain | Required scope | Current status |
| --- | --- | --- |
| Installation and identity | First install, Owner protection, MFA/passkeys, sign-in/out, recovery, sessions | Fresh Chromium, Firefox, and WebKit journeys passed; broader identity review remains |
| Viewer access | Profiles, restrictions, public access, Quick Connect, expired/rejected grants | Hosted baseline passed; additional review remains |
| Library | Every media type, search, filters, paging, details, My List, playlists, collections | Hosted baseline passed; responsive shell checks below passed in two engines |
| Video | Direct First, preparation, seeking, progress, tracks, captions, retry, session cleanup | Caption transport/lifecycle checks below passed; complete decoded-media and device review remains |
| Audio | Queue, next/previous, sleep timer, persisted progress, interruptions | Hosted baseline passed; platform review remains |
| Reading and photos | Supported formats, paging, bookmarks, preferences, zoom, invalid resources | Hosted baseline passed; platform review remains |
| Downloads | Preparation, integrity, pause/resume, ownership, offline use, removal, interrupted state | All 27 ownership executions passed; large-file and hardware boundaries remain |
| Subtitles | Search, inspect, pair, save, cleanup, history, restore, provider failures | Hosted baseline passed; expanded local review remains |
| Administration | Configuration, integrations, security, backups, updates, diagnostics | Responsive administration and session settings passed in two engines; remaining operations pending |
| Apple | iPhone/iPad touch, TV focus/remote, Watch relay, lifecycle, accessibility | Native inventory refreshed; current interaction review pending |
| Android | Phone/tablet/TV/Wear, lifecycle, input, accessibility, supported media | Debug/release builds, lint, 174 unit checks, and real Server/Kotlin compatibility passed; emulator and hardware review remain |
| Quality and security | Go tests/races/coverage, lint, metrics, supply chain, workflow contracts | Tooling passed; Halstead loading repaired; package lint remains failed |
| Containers and release | Both apps and architectures, scan, installer, publication, deployment, TLS | Hosted validation passed; publication and deployment remain separate boundaries |

## Iteration 1 results

- `make tooling-check` passed. The CI contract suite reports 169 cases,
  including five environment-dependent skips.
- `download-pause-ownership.spec.ts` passed 27 executions across Chromium,
  Firefox, and WebKit, without retries or skips. The two previously failing
  cases also passed their first focused Chromium replay.
- Caption lifecycle/recovery passed 21 executions across three engines.
  Three decoded-media cases skipped because this runner deliberately exercises
  isolated HTTP transport. These are not populated-server media proof.
- First-install journeys passed separately on fresh real Servers in Chromium
  and Firefox. Chromium used a virtual passkey; Firefox used real TOTP validation.
  Each journey reached decoded media, sign-out, and another sign-in.
- The exact WebKit first-install journey also passed on Linux ARM64 over
  HTTPS. The disposable container alone trusts the generated Server CA.
  Browser certificate validation stays enabled. The input snapshot's Git tree
  and commit match the recorded source; the container was removed afterward.
- Responsive shell, TMDB instructions, session settings, and six show-action
  widths passed 22 executions across Chromium and Firefox. Show-action cases
  use exported real templates with rendered fixtures. Other cases use the
  real Server. These forms of proof remain distinct.
- `make max-loc` and `git diff --check` passed before the first checkpoint.
- `make packages-check` still fails at existing package lint findings. Its
  subsequent aggregate checks did not run. No rule or threshold changed.

## QA-001 — Halstead cannot load three source configurations (P2; repaired)

The complete baseline command reproduces three errors: the fallback source
is excluded on Windows, the `q47proof` fixture has no selected source without
its build tag, and the standalone HLS command cannot resolve its workspace.
The same failures occur through a new command-level regression before repair.

The authoring gate established the following before production edits:

- Contract: the public quality command must measure every selected production
  source under a valid platform, build tag, and module/workspace context.
- Regression: selecting an excluded platform or disabling the command's
  workspace makes the gate fail without measuring that source.
- Coverage gap: existing metric unit checks do not exercise the Python gate's
  selection. Product E2E cannot reach this source-tool boundary.
- Seam: none. The regression runs the actual command and compiled metric tool
  against a disposable Git workspace with independent Go modules.

The repair selects FreeBSD for `_other.go`, enables the fixture's existing
`q47proof` tag, and loads standalone commands as named Go files with their
workspace. Ordinary module packages retain their isolated module loading.
The difficulty threshold and measured-source inventory remain unchanged.

The new regression passes after repair. Existing Go metric tests and the
complete `scripts/quality/check-halstead.sh` also pass. The regression is wired
into `make tooling-check`.

## QA-002 — WebKit HTTP fixture loses secure authentication (resolved environment)

Two fresh-state WebKit runs return to sign-in after Owner creation. The
bounded network projection shows no retained session cookie. The Server uses
a Secure, HttpOnly session cookie. Chromium and Firefox permit this localhost
development case; this WebKit HTTP environment does not establish it.

The remaining WebKit HTTP shell cases were interrupted after this prerequisite
was identified. The combined batch contains 22 passes, one failed case, and
ten skipped or interrupted cases. They are not reported as a passing batch.
The exact first-install journey passes over trusted HTTPS in the disposable
Linux container. Cookie protections remain intact. The remaining WebKit shell
journeys are being replayed there. Local HTTP proof makes no TLS trust claim.

## Reconciliation and hosted findings

Main advanced to `d69cb7a7` with the artwork repair. Reconciliation commit
`6d97305b` includes that work. The full Halstead command passes again with the
new source inventory.

Hosted tooling then identified stale generated architecture snapshots for
the newly merged production source. Both snapshots were regenerated with the
existing generator. The architecture contract suite passes after regeneration.
No generator, template, or validation rule changed.

The baseline deep run records a native snapshot failure in
`MediaCardRatingTests`: PNG encoding returns no data on hosted Xcode 27.1 beta.
Its Swift Testing contracts pass, but the complete native job is failed.
The unchanged rerun passes, including the complete iPhone orientation suite.
The cause and product impact remain unverified. Retain that first result.

The focused iPad orientation run reproduces all three phone-only contract
failures. One applicable detached-window case passes. A first touch-suite
attempt lacked its required fixture and failed before product interaction.
That setup failure is excluded from product defects. The documented touch
fixture replay passed two cases before loaded playback stalled. A process
sample places the blocked main thread inside Apple's RenderBox and Metal
compiler scheduler. The loaded case failed while querying accessibility;
the remaining run was interrupted after preserving its stack and artifacts.
This is not yet attributed to application code. A simulator restart and
focused replay are required.
The simulator restart replay stalled before XCTest launched. Its Xcode build
stack is preserved, and that task-owned run was interrupted. No product
interaction assertion executed in the restart replay.

## QA-003 — Router contract probe sometimes executes no source (P2; repaired)

`make tooling-check` intermittently reports that Q09 accepts an incompatible
suite. The production router already rejects this combination. Its isolated
shell probe sources `/dev/stdin` from a subprocess pipe on Darwin Bash.
Three of 16 focused repetitions returned success with empty output, without
executing the router's dispatch or rejection paths.

The probe now includes the unchanged router in its shell command string.
It still overrides `exec`, so no application driver runs. All rejection and
dispatch assertions remain intact. The focused suite and 200 repetitions
pass after repair. This is test infrastructure evidence, not product E2E.

## QA-004 — Session settings journey aborts sign-in (P2; repaired)

The trusted-HTTPS WebKit shell batch passed ten cases and failed session
settings. Its trace records an aborted sign-in POST followed by an anonymous
settings request. The test navigates immediately after clicking Sign in.
An unchanged focused repetition passes, confirming intermittent setup.

The journey now waits until it leaves `/login` before opening settings,
matching the existing layout helper. Its settings, accessibility, and
viewport assertions remain unchanged. The focused Chromium and Firefox
replays pass. The trusted-HTTPS WebKit shell batch passes all 11 cases,
including session settings. Earlier HTTP replays exceeded their helper
readiness window before browser interaction; the private helper now allows
120 seconds for setup. The retained CLI regression keeps its 15-second limit.

## QA-005 — Eager translation work delays all process commands (P2; resolved)

An initial Player `version` command took 52.23 seconds. A process sample
places startup work in localized template construction, before command
dispatch. Both disposable browser Servers missed their readiness deadlines.
A new process-level regression reproduces the failure at its 15-second limit.

The process regression's authoring gate established these requirements:

- Contract: the actual built application's `version` command must return the
  expected version within 15 seconds, using disposable empty configuration.
- Failure: eager template construction repeats every message lookup for every
  text fragment and supported language before any command can execute.
- Coverage gap: browser journeys cannot begin before process initialization.
  Existing localization checks do not bound actual process startup time.
- Seam: none. The process regression builds and runs each actual application.

Main advanced to `8e937334` with [PR #551](https://github.com/Kinosail/kinosail/pull/551).
Its simpler repair skips catalog lookups for absent source phrases. A real
Player process using that exact catalog source passes in 9.92 seconds.
The additional precomputation candidate showed no useful advantage against
this current baseline. It was removed, along with its extra isolated tests.
The final change retains main's production repair and adds the process check.

The integration comparison preserves all 3,780 sampled template translations
across 108 languages. Its output SHA-256 remains
`7cd9ca4c56b2baa1e43cc2cccb8773ab474621bf86ab7286cfb89bfeb35fc142`.
Language suites passed 157 Player and 228 Subtitles cases during comparison.
Final-source localization race checks and all 157 Player and 228 Subtitles
language cases pass. Subtitles process startup passes in 9.19 seconds.
The final architecture and source-size checks pass. Browser replays remain.
Timing observations do not establish a universal latency guarantee.
CI contracts, workflow validation, and `make max-loc` pass.

`make cli-startup-test` runs both real-process regressions sequentially. Set
`KINOSAIL_CLI_STARTUP_ARTIFACT_DIR` to choose a new private evidence directory.
Each receipt records source and binary checksums, build and execution commands,
environment, timing, and log checksums. Deep hosted Go jobs run the same check
and retain its JSON and logs on success or failure. If an earlier Go step
fails before the startup check runs, artifact upload is skipped. The first
new deep run exposed that missing guard as a secondary upload failure. Its
original Go failure remains under investigation.

## Visual review checkpoint

Twelve hosted images were inspected: Player phone pagination states, phone
and desktop shelves, fullscreen settings, Subtitles phone review states, and
the desktop dashboard. Their checksums and review limits are recorded in
`visual-review.json` in private evidence. These images include rendered and
isolated fixtures; their journey provenance remains authoritative.

No additional product defect was confirmed from this sample. A suspected
fullscreen Close-button contrast problem belongs to simplified fixture markup;
the real template uses a header and quiet button. Full-page screenshots taken
after scrolling also place fixed navigation within the image. Those positions
do not establish ordinary viewport behavior.

## Expanded verification checkpoint

Required [PR checks at `3ec52268`](https://github.com/Kinosail/kinosail/actions/runs/38160668581)
passed. The [three-browser layout run](https://github.com/Kinosail/kinosail/actions/runs/38160820234)
also passed. It combines rendered keyboard and pending-state fixtures with
disposable real Player and Subtitles Servers. These proofs remain distinct.
Q14 primary, cold, and HTMX browse-return proofs passed at the same revision.
Main then advanced through PR #553. Its verifier regression and the architecture
contracts passed after reconciliation.

An additional local populated Chromium batch passed 42 cases and failed three.
The library API request used a loopback HTTP proxy and sent no session cookie;
its response was 401. All six production-suite cases then passed over trusted
HTTPS in Chromium. They include decoded audio, offline video playback and
seeking, Home Assistant state, receiver focus, and responsive accessibility.
The disposable container alone trusts the CA; certificate checks remain enabled.
Trusted HTTPS
setup correctly returned 409 because this disposable Server disables TLS and
fixes a different sign-in address. That replay needs compatible deployment
configuration. A loading-review case using synthetic media events also failed;
its relationship to actual playback remains under investigation.

Android app and Wear debug APKs, instrumentation APKs, release bundles, lint,
and all 174 unit checks passed. Real production Kotlin decoders and clients
also passed against a disposable Go Server. This JVM contract does not prove
emulator playback, device interaction, signing, or store acceptance.
The phone runner suffered a startup ANR before any test assertions. Its first
crash result and safe lifecycle projection remain retained. Original APKs and
application data were restored. A previous single journey on the TV emulator
omitted the `tv=true` argument and therefore exercised MobileActivity. That
pass does not prove TV activity focus or navigation. The corrected runner passes `tv=true`. All seven TV journeys pass, including
focus, Back, playback controls, and pending, failed, empty, and loaded states.
These journeys use an in-process synthetic API, not a production Go Server.
Both Wear accessibility journeys passed. Original Wear APKs and application
data were restored after the run.

ART compilation allowed phone instrumentation to reach all twelve selected
checks. Seven passed, three navigation checks timed out, and two real-playback
checks skipped because their private Server seed was absent. The failure
screenshot shows a System UI ANR dialog over the app. App navigation remains
unproved until a clean emulator replay. The tablet replay reaches all twelve checks: six pass, four fail, and two
real-playback checks skip for the missing Server seed. Task-installed packages
are removed after the run. These failures remain open; no device acceptance
is claimed.

## QA-006 — Held Restore diagnostic can return implicit success (P2; repaired)

The fresh deep run failed `TestRestoreCausalProbePublicControl` with
“intentional abort did not reject.” The unchanged control then passed 120 local
race repetitions. Its original hosted failure remains retained and its exact
cause remains unverified.

Before repair, a new real TLS control deterministically stops the fixture's
lifecycle after the held witness arms, while keeping the client context live.
It fails because the handler returns without writing a response. Go's HTTP
Server then emits an implicit successful response for an undelivered probe.
This is a diagnostic endpoint defect, not a confirmed production Restore defect.

The authoring gate requires transport rejection on held non-delivery, a settled
witness, unchanged Restore state, and unchanged direct/captured 204 framing.
Ordinary request cancellation cannot force the response race deterministically;
the lifecycle control reaches it through the real owned TLS endpoint.
The handler now records its existing witness and aborts the HTTP exchange with
`http.ErrAbortHandler`. The original control and new regression pass 20 race
repetitions each. All 16 controls in the complete fixture pass with the race
detector. The hosted deep replay passes the Subtitles suite. A later required
run found complexity 11 in the new test, above the limit of 10. The witness
assertions now check each named condition in a loop. Their behavior and the
limit remain unchanged. Focused fixture lint passes.

The runtime campaign initially stopped before compilation because two reviewed
source records were stale. One reflects an earlier erased TypeScript type edit;
the other reflects this diagnostic repair. Their checksum records were refreshed
after reviewing both diffs. Existing exact-input admission rejects the old records
and accepts the refreshed records. No admission rule or threshold changed.

## Additional browser and playback evidence

The current populated layout campaign passes in Chromium, Firefox, and WebKit.
Restore runtime controls and browser header diagnostics pass at `6c1b01f7`.
The Restore inspection-body replay also passes.

All six trusted-HTTPS production journeys pass in Chromium and Firefox.
WebKit passes five, but saved offline video seeking times out. The unchanged
focused replay also fails. The saved file is a valid twelve-second H.264/AAC
MP4, and its small range probe returns the expected bytes. This does not yet
identify the failure in the app, WebKit, or the Linux media pipeline. A further
diagnostic attempt failed HTTPS readiness before browser execution and is
excluded from behavior results.

The canonical Apple playback launch journey passes at `6c1b01f7`. The canonical
startup preparation journey reaches transcoder-setting invalidation, then
remains queued beyond its thirty-second deadline. Its generated codec fixture,
resource samples, commands, and failure are retained. No startup acceptance is
claimed. The decoded scrubbing journey passes in Chromium and WebKit; Firefox
times out during a media action. That Firefox failure remains open. The deep Swift run again fails the existing rating-card PNG encoding
assertion. The image encoder reports a zlib error. The test now compares
canonical rendered RGBA pixels and image dimensions directly, retaining its
visual assertion without requiring PNG compression. The focused iPhone simulator replay passes: one test covers all eight card
variants, with no skipped tests.
This changes test evidence only; the card UI is unchanged.

## Evidence and remaining work

Private command receipts record revision, command, duration, result, log path,
and SHA-256. Local Server receipts also record the exact binary checksum and
generated media checksums. Browser JSON, traces, videos, screenshots, and
synthetic Server logs remain private. No credentials or raw logs are committed.

Remaining work includes the previous unproved browser cases, full-tree lint
and complexity findings, native phone-only test applicability, physical
devices, external receivers and services, long playback, codec/HDR diversity,
assistive technologies, and release/store acceptance. Source, browser,
simulator, physical-device, CI, publication, deployment, health, and TLS
evidence must remain separate. This checkpoint does not certify full QA.

## Iteration 1 delivery

[PR #549](https://github.com/Kinosail/kinosail/pull/549) merged with all required
checks passing. Fetched remote main is
`be9458cfe2e4fb97cb2df9ad4955041c3f8f9388`. A fetched ancestry check confirms
that reviewed task commit `3ee08a1e7424025f5220b992c7d9a05d2ed2ffaa` is included.
The primary checkout still contains unrelated Apple edits. Its cleanup
preconditions are unmet, so those files and the checkout remain untouched.
This task reuses its leased worktree for the next iteration.

The current optional deep run confirms the changed rating-card test passes
on both iOS and tvOS. It also exposes two independent failures: Firefox times
out on a settings navigation, and the tvOS near-end completion journey does
not receive its watched-progress update. Both remain open. Their original
artifacts are retained. The latter is being replayed with its existing native
media fixture. A passing card assertion does not make that native job green.

All three Save and all three Restore R06 diagnostic campaigns now pass.
Q47 primary, recovery, supersession, and contract campaigns report green.
Q09 reports an accepted result. The reconciled layout campaign passes in
all three engines without source drift. These receipts cover their named
contracts and revisions; they do not establish complete QA.

## QA-007 — A second tab interrupts saved video (P2; repair under verification)

Reloading the current Viewer Profile in another tab sends an identity
notification. The offline player handles every notification by pausing its
media, clearing the source, and rebuilding the download view. A notification
that retains the same Viewer Profile therefore interrupts active playback.

A real-Server journey reproduces the defect before repair. The retained
media state is paused at 0.41 seconds of a 12.01-second video, with `ended`
false. The clean baseline fails the assertion that playback keeps advancing.
Earlier attempts contained test callback and menu-helper errors. Those runs
are retained, but they are excluded from the authoritative regression proof.

The repair keeps transfer ownership checks on every notification. It resets
the offline view only when the active Viewer Profile changes. Identity
notification behavior stays intact. Sign-out still clears the player and
shows the Viewer Profile selection message. The public HTTP API is unchanged.
No new error path or diagnostic log is introduced.

Both shell cache versions advance to 56 because both apps serve shared PWA
assets. Registration URLs and existing fixtures use that same version. This
lets an installed worker refresh the changed download script. The existing
asset and localization contracts pass for Player. Both Subtitles registration
and local-playback contracts pass after the shared version update.

The final CI-selected regression fails against the original binary and passes
in Chromium, Firefox, and WebKit without retries. All seven populated Firefox
journeys pass, including offline playback and seeking. The new regression
is selected by the existing populated CI runner with retries disabled.
Retained storage, worker, and ownership checks pass all 54 executions across
Chromium, Firefox, and WebKit. Those retained checks use isolated fixtures;
they are not populated-Server proof.

The broader WebKit batch passes five cases and fails two. The existing offline
seek assertion still fails. The library accessibility review reaches all six
desktop screenshots, then exceeds its test deadline before phone coverage.
Neither failure is waived or described as a passing batch.

Both apps' `make verify-changed` stop at the full shared-package lint stage.
Its 95 findings match the initial baseline after normalizing source locations.
One localization finding moved three lines in an earlier main repair. Full
Player lint also reports 50 findings; none is in a file changed by this task.
These complete-tree gates remain failed. The repository's CI changed-code
lint and module tidy checks pass for packages, Player, and Subtitles. All 169
CI contracts pass, including five documented environment skips. Shell syntax,
shellcheck, and source-size checks pass. Required hosted checks and publication
remain pending.

The focused tvOS completion replay also fails the watched-progress assertion
at the exact movie endpoint. The 24-millisecond-before-end argument passes.
The test retains its original timing and media prerequisites. The task-created
simulator was deleted after collecting the result bundle and receipt. The root
cause remains under investigation; no native repair is claimed.

Main reconciliation now includes the Android collection/EPUB and tvOS navigation
changes through `0d3fd1d6292992b30afbc75adcc86bf06a506a56`. The offline fix's
Go and browser inputs are unchanged. Earlier native build, render, and emulator
receipts describe the preceding native UI. The new native surfaces require
refreshed verification before extending those claims to this revision.

The first hosted attempt for PR 559 fails before media preparation. Its trace
records HTTP 401 from the new test's direct library API request. The regression
now selects a movie and its title through the authenticated Library UI. Its
exact revised Chromium execution passes against the real HTTPS Server. Two
earlier local commands used the wrong selection variable and ran the broader
production suite instead. Both suites pass, but they do not verify this revised
test. Their receipts are excluded from the regression claim.

## QA-008 — Final native watched update remains queued (P2; repair pending)

The exact tvOS endpoint failure reproduces twice before repair. A bounded
diagnostic shows an acknowledged unwatched revision and a newer watched
revision still in the durable local journal. Local progress is retained, but
the Server's watched status remains stale after playback stops.

`ProgressSyncStore.synchronize` returns immediately when another synchronization
owns the request. That owner processes its earlier snapshot and does not drain
the newer request. The pending repair coalesces another requested pass into the
current owner. Existing conflict and journal persistence handling remain intact.

The original completion case passes after repair. The broader TV replay passes
18 selected tests with no skips. It covers actual MP4/HLS endpoint playback,
duplicate completion, stale callbacks, seeking, persistence, profile isolation,
transport validation, and missing titles. The corresponding iPhone selection
passes 12 tests with no skips. These use native simulators and retained synthetic
HTTP fixtures, not the production Go Server or physical devices. Temporary TV
simulators were deleted; the reusable iPhone simulator and its data remain.

## QA-009 — Continue Watching grows when progress appears (P2; repair pending)

The hosted Firefox layout audit records a four-pixel increase in a Continue
Watching card after delayed scripts initialize its progress bar. Later shelves
move by the same amount. The initial and settled measurement frames establish
the change; the audit fails its existing geometry assertion.

The pending repair reserves the progress bar's four-pixel grid row before the
bar is shown. It adds no loading placeholder. Local Firefox currently exits
before opening a page with `Could not find profile folder`. That launch failure
is excluded from product evidence. Verification continues in isolated Linux
and the hosted layout matrix. No layout threshold or assertion changed.

## Native verification refresh

The current Android debug, instrumentation, and release builds pass. App unit
tests and lint also pass. A real-Server/Kotlin contract replay first fails to
compile because its explicit source list omits the newly added `CollectionsApi`.
Adding that production source to the verification tool restores the contract
run: both its retained JVM checks and real HTTP journey pass. This tool repair
is still pending delivery. Current emulator UI journeys remain in progress.
