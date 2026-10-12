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
| Installation and identity | First install, Owner protection, MFA/passkeys, sign-in/out, recovery, sessions | Fresh install and required MFA journeys passed in three engines; external identity-provider and physical passkey acceptance remain |
| Viewer access | Profiles, restrictions, public access, Quick Connect, expired/rejected grants | Mixed maturity, excluded Library, required Viewer MFA and Show action checks passed three times per engine; physical pairing remains |
| Library | Every media type, search, filters, paging, details, My List, playlists, collections | Canonical paging passed 30 cases per engine; responsive sweep passed Chromium/WebKit with one Firefox protocol timeout |
| Video | Direct First, preparation, seeking, progress, tracks, captions, retry, session cleanup | HLS lifecycle and Show playback repairs delivered; native decoded follow-ups passed; WebKit watched-startup and physical media boundaries remain |
| Audio | Queue, next/previous, sleep timer, persisted progress, interruptions | Decoded queue and media actions checked; physical interruption and receiver acceptance remain |
| Reading and photos | Supported formats, paging, bookmarks, preferences, zoom, invalid resources | Platform and reading evidence is recorded below; Android EPUB accessibility and physical acceptance remain |
| Downloads | Preparation, integrity, pause/resume, ownership, offline use, removal, interrupted state | Ownership and worker controls passed; real 2.16 GB restart, independent hash and disconnected playback passed; quota controls pass three times per engine; hardware remains |
| Subtitles | Search, inspect, pair, save, cleanup, history, restore, provider failures | Hosted cross-browser evidence retained; provider, deployment and production acceptance remain separate |
| Administration | Configuration, integrations, security, backups, updates, diagnostics | Responsive administration reviewed in three engines; one Firefox resize timeout remains; external providers and deployment remain |
| Apple | iPhone/iPad touch, TV focus/remote, Watch relay, lifecycle, accessibility | Hosted iOS 332 and tvOS 313 pass; Watch renders and pending transport checked; actual relay, VoiceOver, larger text and physical acceptance remain |
| Android | Phone/tablet/TV/Wear, lifecycle, input, accessibility, supported media | API23/30/36 emulator and Wear boundaries recorded; original API36 EPUB tests remain intermittent; physical acceptance remains |
| Quality and security | Go tests/races/coverage, lint, metrics, supply chain, workflow contracts | Deep Go coverage passes at `6008fa77`; local lint debt, hosted WebKit startup and current-main cold-suffix failures remain open |
| Containers and release | Both apps and architectures, scan, installer, publication, deployment, TLS | Player `a2d5f2b1` publication, scanned Nox image, health and private-network TLS verified; Subtitles runs `6f41117d`; main `5f169ee2` CI blocks the next rollout |

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

## QA-007 — A second tab interrupts saved video (P2; repaired)

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

The earlier CI-selected regression fails against the original binary and passes
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
shellcheck, and source-size checks pass. Required hosted checks pass at
`003d6af90012016d95d50483c8a64853327b8dc8`. Publication remains separate.

The focused tvOS completion replay also fails the watched-progress assertion
at the exact movie endpoint. The 24-millisecond-before-end argument passes.
The test retains its original timing and media prerequisites. The task-created
simulator was deleted after collecting the result bundle and receipt. The
subsequent diagnosis and pending native repair are recorded in QA-008.

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

The revised Library-driven test passes in Chromium and Firefox. Its WebKit
replays stall before the cross-tab action while starting saved media. A private
probe passes after the saved position finishes restoring. The regression now
waits for decoded data and completed seeking before Play, then requires time
to advance beyond the restored position. Its profile and sign-out assertions
remain unchanged. This stronger prerequisite passes in Chromium, Firefox, and WebKit. Its
unfixed-binary control still fails at the intended cross-tab assertion. Failed
runs remain retained. The hosted checks are being refreshed for this test edit.

## QA-008 — Final native watched update remains queued (P2; repaired)

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
HTTP fixtures, not the production Go Server or physical devices. After removing
the temporary journal diagnostic, the original TV completion test still passes.
Temporary TV simulators were deleted; the reusable iPhone simulator and its data remain.

## QA-009 — Continue Watching grows when progress appears (P2; repaired)

The hosted Firefox layout audit records a four-pixel increase in a Continue
Watching card after delayed scripts initialize its progress bar. Later shelves
move by the same amount. The initial and settled measurement frames establish
the change; the audit fails its existing geometry assertion.

The repair reserves the progress bar's four-pixel grid row before the
bar is shown. It adds no loading placeholder. Local Firefox currently exits
before opening a page with `Could not find profile folder`. That launch failure
is excluded from product evidence. The isolated Linux Firefox audit passes
39 measurements and 11 flows without moved geometry or overflow. Its populated
desktop Home includes Continue Watching. The hosted layout matrix passes in
Chromium, Firefox, and WebKit at `003d6af9`. No threshold or assertion changed.

## Native verification refresh

The current Android debug, instrumentation, and release builds pass. All 161
app unit tests pass without skips; lint also passes. Earlier Watch/Wear evidence
remains tied to its recorded source. A real-Server/Kotlin contract replay fails to
compile because its explicit source list omits the newly added `CollectionsApi`.
Adding that production source to the verification tool restores the contract
run: both its retained JVM checks and real HTTP journey pass. This tool repair
is delivered through PR #561.

The refreshed Android TV run passes nine applicable tests and skips two EPUB
journeys. TV reading is unsupported. The phone and tablet runs each pass nine
tests and fail one EPUB reopen assertion, without skips. Their app packages
and data are restored or task-installed packages are removed after each run.
These emulator journeys use synthetic HTTP fixtures, not the Go Server.

## QA-010 — TV Collections pending rows have different insets (P2; repaired)

The pending row begins 24 pixels above and left of the loaded collection row.
Its width is 48 pixels greater. A geometry assertion added to the existing
native recovery journey fails before the production edit. Matching the TV
list's 12-dp content padding fixes the pending geometry. Phone padding stays
zero. The full TV replay and the cleaned final instrumentation replay each pass nine
applicable tests, with two EPUB skips. Both include the new assertion. Pending,
loaded, empty, and failed captures were inspected. Focus and recovery pass.

The earlier idle-controls failure was a test prerequisite problem. Its
UiAutomation connection enables an accessibility service, and the player
correctly keeps controls visible. The idle journey now requests automation
without that service. A complementary journey verifies controls remain visible
with accessibility enabled. Both pass during decoded TV playback. Production
accessibility behavior is unchanged. Older Android API levels remain unproved.

## QA-011 — EPUB accessibility text is missing after reopening (unverified)

The phone reading journey restores the chapter-2 indicator, but cannot find
its text after reopening. The tablet save-interleaving journey similarly fails
to find chapter-1 text after reopening. Both first renders and intervening
chapter navigation pass. Another reading journey passes on each device.
Both unchanged focused replays pass. The full phone replay also passes all ten
tests with the same assertions and failure-only capture instrumentation. The
final tablet replay again passes nine and fails one. Its failure capture shows
the expected chapter-2 text rendered correctly; automation cannot find that
text in the accessibility tree. This is an accessibility or observer issue,
not blank rendering. The initial failures remain retained. Temporary tree
logging was removed; chapter failures retain a screenshot. No production cause
or repair is claimed yet. A failure-only cache-clearing control also reproduces
the failure: the expected text is absent before and after clearing the
automation accessibility cache. A further full tablet replay passes all ten journeys, so its failure-only
accessibility-focus probe does not execute and cannot establish a cause. The
private probe was removed. These controls preserve the original assertion.

## Complete-tree lint inventory

Shared-package lint retains 95 findings. Player retains 50. The newly refreshed
Subtitles inventory reports 101 findings, including complexity, formatting,
unused code, and other checks. These complete-tree gates remain failed.
Changed-code CI checks pass. No rule, threshold, or required check is bypassed.

## Firefox settings follow-up

The exact settings-discovery journey passes three consecutive Firefox runs
against a real HTTPS Server with editable settings. An earlier private fixture
locked Server name through an environment override. Those three setup failures
never reached the hosted navigation failure and are excluded from product
proof. The hosted malformed-fragment timeout remains unexplained. The focused
success does not turn the original deep job green.

## Playback startup follow-up

The fresh canonical startup journey passes at `d8399829`. Its receipt records
12 measurements, including cold and warm moving frames, preparation adoption,
rejected recipe values, Direct First, playback-exit acknowledgements, and
resource bounds. Invalidation and spanning-fragment decoding assertions pass.
Peak speculative and total FFmpeg workers are both one. Burn-in subtitle
coverage is explicitly unrun because this host FFmpeg lacks the subtitles
filter. This remains one Chromium journey with several internal assertions,
not twelve independent tests. The earlier queued-preparation failure remains
retained; no production cause or repair is claimed from this successful replay.
The binary, generated media, logs, measurements, and checksums were copied to
private evidence, excluding configuration, backups, and cache state.

## Web delivery and native checkpoint

PR #559 merged with a merge commit after required CI `38179788646` passed.
Final layout run `38179788404` passes Chromium, Firefox, and WebKit. Fetched
ancestry confirms reviewed head `d83998297d2a697b9fb4dee7742f2c0589aae0d5`
is included in remote main `8047e689b22d3c2c210bf85594baae5bb9c1f2ae`.
The native branch reconciles that main before publication. Its fixes remain
pending protected delivery. The primary checkout has unrelated Apple edits;
its files and local main were preserved.

## Native delivery and continued failure investigation

PR #561 merged after required CI `38181412353` passed. Fetched ancestry
confirms reviewed head `d0efff30861ec805ee69b94edf2787016743b78d` is included
in main `85b6714db40238af99494fc008006969b8dd976e`. Post-commit Player
`verify-changed` passes its selected native-client checks in 42 seconds. The
earlier web verification stopped at shared lint; complete-tree findings remain.

The fresh canonical decoded scrub journey passes three browser executions at
`d0efff30`. It verifies previews, caching, seeking, responsive geometry, and
control accessibility in Chromium, Firefox, and WebKit. The previous Firefox
failure remains retained without a confirmed cause. Private retained artifacts
include the generated video, binary, measurements, logs, and checksums.

Additional failure-only EPUB focus controls produce eight passes and two
failures in one tablet run, nine passes and one failure in another, and ten
passes in a third. In each reproduced failure, clearing the cache and attempting
accessibility focus do not reveal chapter text. Rendered chapter content remains
visible. The original assertion always fails; diagnostics never convert that
result into a pass. Probe APKs are frozen and checksummed separately from
delivery builds. Repository production and test sources contain no probe.

The exact older WebKit offline-seek journey fails again at its seek assertion.
A subsequent failure-only state read also times out. A plain blob-video control
is being moved before the original seek so it can run while the page responds.
An earlier selection used the wrong title and executed no matching test; it is
excluded from behavior evidence.

Deep run `38181434746` continues at reviewed head `d0efff30`. Its CodeQL Java
upload fails after the merged PR branch is automatically deleted. The annotation
identifies a missing ref, not a compiler or source-analysis finding. The exact
reviewed ref was restored. Failed jobs must be replayed after the run finishes.
The earlier connected Apple TV entry was not filtered by hardware reality.
A later inspection confirms that entry is simulated. Its installed-app inventory
is simulator evidence. No physical interaction or current-revision hardware
acceptance was established. The October 12 refresh finds two connected simulated
TV entries and no connected physical devices.

## QA-012 — Native cast-pitch check omits scroll content (P2; repaired in PR #563)

Deep CI passes 332 iOS tests and skips one platform-inapplicable case. tvOS
passes 311, fails the cast-placeholder pitch test, and skips one. The same
cast check fails locally twice. Its 1792-by-1042-pixel diagnostic contains the
hero but no cast cards. The current shelf uses a UIKit-backed ScrollView;
[ImageRenderer only exports content SwiftUI rasterizes directly](https://developer.apple.com/documentation/swiftui/imagerenderer).
The test also retains a four-card expectation from before the pending shelf
was expanded to six in `3a22cb06`. This is a stale test harness, not a proven
missing-card defect in the app.

The repair hosts the current view in a native window and captures its UIKit
hierarchy. It asserts all six current cards, their 230-pixel widths, and
248-pixel pitch. All four detail tests pass. A temporary negative control
restoring 40-point spacing fails the pitch check; production spacing was
restored afterward. No UI behavior changes. Private command, source-diff,
xcresult, and cleanup receipts retain both results.

The independent plain-video service-worker control reproduces the WebKit seek
failure with no Player UI scripts. Its blob-video counterpart seeks the same
saved bytes successfully. A bounded single-chunk Blob response candidate
does not repair the original assertion and was removed. No offline playback
repair is claimed from these controls.

A further EPUB control finds a visible WebView accessibility node but its focus
action returns false; the expected text remains absent. Earlier label-based
focus attempts had no matching Book chapter label in the tree. The current
view is visible, attached, window-focused, and has an accessibility provider.
The provider reports a child. The native accessibility issue remains open.

Main-push run `38181332045` succeeds for web merge `8047e689`, including both
apps' AMD64 and ARM64 production-image builds, scans, publication, and
production-tag advancement. Native-only main `85b6714d` passes its selected CI
run `38181968929`. Publication at the older web merge is not exact-current-main
Nox deployment proof. No physical interaction or production deployment is
claimed. The CodeQL replay runs after restoring the deleted deep-run ref.


## QA-013 — Shortcut sign-in checks a prompt before navigation (P2; repaired locally)

At `4621d625`, the additional populated HTTPS UI batch passes all 29 checks in
Chromium and Firefox. WebKit passes 28 and fails the second shortcut case during
sign-in. The retained screenshot shows the optional passkey offer. The helper
checks whether its Not now link is visible before sign-in navigation finishes.
It then waits for Home while leaving that offer open.

The helper now waits until navigation leaves `/login` before inspecting the
optional offer. This matches the existing populated-instance helper. Both
shortcut cases pass three consecutive WebKit repetitions after the change.
No product authentication or shortcut behavior changes. The final complete WebKit batch also passes all 29 checks. Player
`make verify-changed` passes after commit `9cb3dab4`. Required CI run `38185815124` fails its earlier public HLS regeneration step,
before Chromium starts. The shortcut repair remains in open PR #564.

These five files cover password reveal, repeated navigation, keyboard shortcuts,
and Quick Connect UI. QR scan cases substitute camera streams and decoder
responses; some also substitute confirmation responses. They prove isolated
browser behavior, not physical-camera scanning or a complete public pairing
flow. Other cases use the disposable Go Server. The private
`navigation-additional-evidence.json` retains checksums for source attestations,
commands, JSON results, failure screenshots, and cleanup receipts.

Additional browse-return runs select ordinary, cold, Home, safety, and watch
navigation against the unmodified disposable Go Server. All selected Home,
safety, and navigation runs pass in Chromium, Firefox, and WebKit. The combined
ordinary/cold batch passes eight cases in each browser. Its ninth BFCache case
fails: Chromium does not observe persisted pageshow; the other projects cannot
use that file's Chromium-only launch configuration. These are prerequisite
failures and do not prove native BFCache admission. No assertion was relaxed.
The fixture uses fictional media and unavailable FFmpeg/FFprobe paths; it is
not production codec or TLS acceptance.

## Further offline-seek controls

A standalone local browser control contains no Kinosail application code,
authentication, storage, or external target. It serves the fictional MP4 over
HTTP, or from a service worker with bytes, Blob, or stream response bodies.
The worker cases stop the origin before navigation and playback. All twelve
cases pass playback and a four-second seek across Chromium, Firefox, and WebKit.
This rules out an unconditional service-worker seek failure for this clip and
configuration. It does not establish why the original saved-download case fails;
that case uses a different prepared MP4 and the application worker.

The exact script, clip digest, bounded request/response observations, results,
and container cleanup are retained in `minimal-sw-seek-control-v2`. The isolated
control is not a replacement for the failing public download journey. No offline
production change is retained from these experiments.

The earlier deep run `38181434746` finishes with its original Swift cast-test
failure. Its CodeQL Java replay passes after restoration of the exact source ref.
The repaired cast test passes the complete tvOS suite in run `38183570454`.
PR #563 merges reviewed commit `4621d625` into main `e20532f5`. Fetched ancestry
confirms inclusion. The separate iOS comparison and WebKit failures remain open.


## Updated platform boundaries at `4621d625`

The exact prepared offline MP4 is 289,666 bytes with SHA-256
`8edf2095459752d3e3cc2491d985b2fdc6c61fed01afbc25ef341b85c1a61c5c`.
Before the failed Linux WebKit seek it has readyState 4, no media error, and
buffered and seekable ranges covering its complete 12.011-second duration.
No new media request appears during the failed seek. The same exact clip fails
in the standalone Linux WebKit control over ordinary HTTP and all three worker
body variants. This reproduces without Kinosail or IndexedDB.

The prepared clip contains three mov_text subtitle tracks. Private remuxes keep
the same encoded audio and video. Removing subtitles or retaining one subtitle
track passes both HTTP and worker-stream seeks. A complete remux retaining all
three subtitle tracks still fails both. These controls narrow the Linux media
failure; they do not justify dropping supported subtitle tracks from downloads.
The exact unchanged prepared clip passes all four controls in macOS WebKit,
including worker playback after the origin stops. This is browser-engine proof,
not physical Safari or the complete populated app journey. Relevant scripts,
clip hashes, results, observations, and cleanup receipts are checksummed in
`offline-seek-control-evidence.json`. No production offline change is retained.

The EPUB provider diagnostic passes its first two full tablet runs and fails
one case in its third. At failure, one WebView is visible and attached. Clearing
the system accessibility cache and sending a content-change event do not restore
the expected text. The provider's text search returns zero matches, but
[Chromium implements that search as an empty result](https://chromium.googlesource.com/chromium/src/+/69dee7ad12a0b795f60dcf3c7fc42a4a132983fb/content/public/android/java/src/org/chromium/content/browser/accessibility/WebContentsAccessibilityImpl.java).
That search is therefore not evidence that the provider lost its document.
The root cause remains unconfirmed; no reader production change is retained.

Deep run `38183570454` passes all 312 applicable tvOS tests and skips one
platform case. The cast geometry repair passes in that complete suite. iOS
passes 331, fails the first large poster comparison in MediaCardRatingTests,
and skips one platform case. Five fresh local launches of that comparison pass;
this does not resolve its hosted failure. The paired-image diagnostic passes the full local iOS suite: 329 passed,
zero failed, four skipped. Local paired images match, but this does not resolve
the hosted Xcode/runtime boundary. The hosted WebKit settings batch also fails two
late-ended-notification fixture cases; ordinary watched controls and the other
batches pass. These remaining failures prevent a complete QA claim.


The five selected watched-startup cases pass unchanged in the disposable local
Linux ARM64 WebKit replay. This includes both late-ended-notification cases that
failed on hosted Linux AMD64. The initial hosted failures remain unresolved;
ordinary navigation and the held native-notification fixture are distinct proofs.

A static Android API analysis now includes instrumentation sources. It identifies
unguarded `getUiAutomation(int)` calls in the new parity harness despite minSdk
23. That method requires API 24; disabling accessibility through its flag requires
API 31. The same expanded analysis finds older unguarded accessibility scanner
and renderer-control calls. These checks need explicit platform prerequisites
or supported older-API paths. The additional API 23 and API 30 runtime results are recorded below.


## QA-014 — Native QA uses APIs newer than supported devices (P2; merged PR #566)

Expanded NewApi lint finds eleven Android instrumentation violations. It also
finds nineteen separate JVM-test violations. After repair, instrumentation has
zero findings; the expanded scan still fails on those JVM-test findings.
Standard lint and both debug APK builds pass.

The parity journey uses the older UiAutomation API on older systems. It keeps
semantic and screenshot evidence there. Renderer termination requires API 29;
pixel capture requires API 26; accessibility scanners require API 34. Testing
with UiAutomation accessibility disabled requires API 31. These prerequisites
are explicit and do not change supported-device product behavior.

The API 23 catalog journey passes pending, failed, empty, and loaded states.
The complete stock API 23 run passes eight, fails two EPUB accessibility checks,
and skips three prerequisite cases. Its retained failure screenshot shows the
chapter rendered while its expected accessibility text is absent. API 30 passes
ten and skips three. API 36 phone passes twelve and fails two EPUB reopen checks.
API 36 TV passes nine and skips two EPUB cases. The API 23 and API 30 temporary
AVDs are deleted after receipt-verified cleanup. SDK images and existing AVDs
are preserved. No physical Android or real TalkBack acceptance is claimed.

The iOS rating comparison now retains both images and exact compared RGBA data
when dimensions or pixels differ. Its equality criteria remain unchanged.
The focused comparison passes. A temporary one-byte negative control fails and
retains both images and both raw buffers. The control is removed. The original
hosted pixel mismatch remains unresolved; this change supplies diagnostic proof.

PR #566 has green required checks at `a21534c1`. Deep run `38186878855` at
`7971d07f` passes 332 iOS and 312 tvOS tests, with one platform skip each.
Its first Firefox attempt times out before queue or command operations. The
replay passes 703 root cases and skips 356. Settings passes 63 with one flaky
initial sign-in navigation, so the strict flake gate remains failed. WebKit
passes 701 root cases and skips 358, then fails four settings watched-startup
cases and records one separate flaky case. These browser failures remain open.
Private receipts, source attestations, screenshots, and cleanup are retained.

### Wear module compatibility and capture follow-up

Wear has a separate minimum API 30. Its expanded NewApi scan reports one
unguarded API 34 accessibility-scanner call before repair, and none afterward.
The scanner now runs only on API 34 or newer. Both live-region checks remain
active on API 30. Builds and expanded lint pass. Both instrumentation checks
pass on API 30 and API 36, with actual SDK/ABI receipts.

Whole-display captures initially show Hello or Starting system setup rather
than the app. Those captures are excluded from app rendering proof. Capture
now images the actual tested Compose root. Both unpaired and heart-permission
state images were inspected on each API. The API 36 reusable emulator and SDK
images are preserved. The task-owned API 30 emulator is deleted after shutdown.
This is isolated component evidence, not foreground interaction, paired-phone
relay, heart-sensor, real assistive-technology, or physical Wear acceptance.

Wear checkpoint `a21534c1475c60322c53750537976e1751d4d5ad` passes required
CI `38190740774`, the source cap, diff check, and post-commit verification.
The initial invalid Java path, initial Gradle configuration error, and API 30
ART-precompile failure happen before analysis or instrumentation and are excluded.


## QA-015 — Valid HLS temporary-file replacement rejects the stage (P2; merged PR #567)

Required run `38185815124` returns 404 while regenerating a deleted first HLS
fragment in the audio-transcode case. Its safe warning class is
`generation-assets-or-stage`. An unchanged local AMD64 replay passes nineteen
codec cases and fails one remux refill. ARM64 passes all six. A private
classification build passes twenty AMD64 cases without reproducing the failure.
The original 404 therefore lacks a more specific captured root-cause class.

A focused concurrent-file control confirms a concrete stage-scan defect.
FFmpeg can rename `index.m3u8.tmp` after directory enumeration and before stat.
The monitor treats that retired valid entry as a violation and cancels refill.
The control fails with the original implementation, including a source-overlay
negative control. The real-codec journey cannot deterministically schedule this
filesystem window, which is the retained isolated check's coverage gap.

The repair tolerates disappearance only for recognized temporary names. It
keeps rejecting unknown names, missing final files, nonregular files, excessive
counts, and oversized output. Final media certification, initialization hashes,
committed metadata, and source identity checks remain unchanged. A new single
unknown-file rejection case preserves committed cache data and publishes no media.

Three repetitions of the rename and resource-rejection checks pass. The complete
related copied-recovery test family passes. Changed-code lint passes. The strict
public-route lifecycle then passes all twenty AMD64 and six ARM64 codec cases.
It uses real pinned Jellyfin FFmpeg 8.1.3-1 and synthetic twelve-second media.
It deletes and regenerates segment zero, cold-reopens the cache, joins owned
codec processes, and verifies unchanged source, metadata, initialization, and
retained-fragment identities and bytes. These are direct public-handler tests,
not external TCP/TLS or physical playback acceptance.

A private logging control observes 477 retired-entry events at debug level.
Each has fixed synthetic request/session correlation and the scan outcome.
Default-level runs omit routine events; invalid stages retain their warning.
No credential, cookie, bearer, or raw-URL markers appear in the logging control.
Private `hls-stage-evidence.json` records source, command, codec, binary, log,
result, and container-cleanup checksums. No historical 404 beyond this reproduced
race is silently classified as resolved. Required CI run `38190828542` and all
three layout engines in run `38190828548` pass at `bf220c9e`. PR #567 merges
with merge commit `8307ea47fbc51408548e8e50b996b75ad7a0b561`. A fresh fetch and
ancestry checks prove both the reviewed head and task commit are in remote main.


## QA-016 — Show card repeats a watched episode (P2; repaired and delivered)

The previous show-card journey checks link patterns and alignment. It never
clicks Play or observes moving media. Its expanded real-Server journey marks
episode one watched and episode two unwatched through the public API.
Show Details correctly selects episode two, but the Shows card says Play again.
The journey fails before the production change, at its next-episode assertion.

Catalog paging collapses Shows into representative items. Home organizes those
representatives and computes playback from only one episode. The repair expands
only the selected Shows with Viewer-visible episodes before computing their
card actions. It preserves the selected card set and canonical episode order.
The page, query, and Viewer access policy still select the visible Shows.
The follow-on projection uses stable read-only catalog references and copies
only selected, permitted episodes. Its final candidate replay passes below.
No layout, placeholder, Direct First, or playback policy changes.

The expanded Chromium journey passes after repair. One card click opens episode
two without another native play call. Decoded frames advance from 4 to 13;
media time advances from 0.004769 to 0.356429 seconds. Resume selects episode
two, all-watched Play again selects episode one, and reset Play next selects
episode one. The journey restores prior viewing state. It retains responsive
captures, moving-video evidence, state captures, and exact source attestations.
Existing public Show, Viewer policy, and pagination contract checks pass.
The source cap, diff check, and post-commit Player verification pass at
`2ee56f5eb2c616ac58086cb66f39e99c80372e65`.

The first expanded attempt is excluded from playback proof. An ambiguous Shows
locator fails on Details. Cleanup then omits untouched zero/false progress
fields and masks that failure with HTTP 400. The corrected journey uses the
Details Back link and normalized public progress defaults. The original logs
and safe projections remain retained.

The broader media interaction sweep passes 11 cases each in Chromium and
Firefox at `2ee56f5e`. Each skips 23 dedicated-pagination cases because their
canonical Go fixture is absent from this generic fixture. Those skips are
not product passes. WebKit passes nine and fails the focus and synthetic loading
reviews. Both Show cases pass in all three engines. The dedicated catalog replay
passes below.
The loading review combines decoded initial playback with synthetic media
state events; it does not prove a real bandwidth interruption.


## QA-017 — Layout sign-in navigation retains an unguarded Firefox boundary

The Firefox replay of deep run `38186878855` passes 703 root cases and skips
356. Settings passes 63 with one flaky sign-in navigation, so the strict flake
gate remains failed. The initial login response is HTTP 200; the screenshot
shows the complete sign-in form. The trace only retains its earlier blank
document snapshot. This does not prove a blank product page.

The layout helper still uses raw `page.goto`, unlike the shared populated-instance
helper. A private control throws `TimeoutError` after the actual Firefox login
form completes. The unchanged settings journey fails before credentials. Reusing
the existing guarded navigation helper passes this same control. The guard
requires one successful, prompt, nonredirected navigation and a complete, usable
form before admitting one replacement GET. Explicit response, URL and redirect
checks remain before credential entry. No authentication product behavior changes.

The control is an isolated browser-automation fault with the real HTTPS backend.
It does not reproduce the original browser internals. Real settings repetitions
and existing rejection controls now pass below. Both original attempts, traces,
source-overlay hashes, results and safe projections remain private.


## Final candidate browser and catalog verification at `d2713764`

Three repetitions pass in each engine for Show card playback and states,
settings persistence, asset caching, and existing rejected-login controls.
Chromium and WebKit each pass 18 and skip six Firefox-only controls. Firefox
passes all 24. No failures or flaky cases occur. This checks the reference-based
Show projection and guarded layout login at the committed, clean revision.
Private `auth-show-candidate-evidence.json` records source and report checksums.

The canonical Go pagination fixture passes all 30 browser checks in each engine,
with zero skips or flakes. It covers real catalog paging and isolated navigation
fault controls using the real disposable Go HTTP Server. Its synthetic media
files are empty, authentication is disabled, and it does not prove decoded media
or TLS. The fixture archives the clean candidate once, builds its matching
Linux ARM64 test binary, and removes its owned container. Exact dispatch, data,
commands, receipts and checksums remain in `canonical-library-browser-current`.

Two diagnostic repetitions each of WebKit focus and synthetic loading reviews
pass without reproducing the earlier failures. The focus diagnostic records
connection and active-element categories. The loading diagnostic records bounded
media event/state observations. All original assertions stay unchanged. Neither
root cause is confirmed, and no production repair is claimed. Private overlays
and their original and modified test hashes remain retained.

A further real-Server check for mixed permitted and restricted Show episodes is
prepared. Existing Show handler coverage contains one episode without Viewer
restrictions, while the moving-card regression runs as Owner. The new public-API
journey will verify maturity, Library exclusion, action selection and route
rejection with a disposable Viewer, then restore ratings and remove the Viewer.


PR #566 merges reviewed native candidate `26e1682d` with merge commit
`013d52472504b7d4309ba123b08808af3b8edea8`. Required CI `38193469566` passes.
A fresh fetch and ancestry checks prove inclusion of that head and Wear commit
`a21534c1`. The focused iOS comparison passes on the reconciled source. A complete
iOS suite subsequently passes against its unchanged native tree.

The first mixed-episode Viewer attempt is excluded from policy proof: it stops
at required MFA enrollment. Its cleanup also expects 200 instead of the specified
204 for profile deletion. The corrected journey retains required MFA, completes
real authenticator enrollment, and checks the proper deletion response. Chromium
passes family episode selection, excluded-Library action and route rejection,
and restored next-episode access. All three repetitions pass in Chromium, Firefox and WebKit at `cfc884bf`: 12
applicable checks per engine, zero skips, failures or flakes. This includes the
Viewer policy journey, actual Show card playback and states, and settings
persistence. No production change follows the Viewer check.


The reconciled complete native suites use tree
`c8b05b870b3c22ebc3b4b3bf61d95c2033ba23fc`. iOS passes 329, fails zero and
skips four. tvOS passes 311, fails zero and skips three. Three iOS skips and two
tvOS skips require optional decoded-media files. Focused follow-ups supply those
files and pass all five applicable cases, as recorded below. The other iOS skip
is the inapplicable picture-in-picture callback case.
The other tvOS skip is optional `TVQualitySessionSeed`, which requires a
disposable public Server session. Neither skip is a product pass.

The tvOS result bundle confirms the existing QA All Apps TV simulator. Xcode
leaves it shut down. A preflight error was incorrectly interpreted as app absence;
that receipt is explicitly invalid for absence proof. Post-run app removal and
shutdown both return a state error and change nothing. The shared simulator,
installed app and data are preserved. No successful uninstall or restoration of
a prior app version is claimed. A corrected inspection must distinguish device
state failures before any later cleanup. Raw diagnostics remain private.


## October 12 continuation — responsive, decoded media and protected delivery

PR #564 merges reviewed head `a481eb9d2283f7c117c6b71755fcb97b8ebe525c`
with merge commit `394736cb740fc6873aa6e94b41e22dc797155529`. Required CI
`38194962439` and all three layout jobs in `38194962237` pass. A fresh fetch
proves ancestry of the reviewed head, Show repair and completed Viewer journey.
Its tree equals the merged tree. Main CI `38196658853` passes, including Player AMD64 and ARM64 build,
production-image tests, scans, publication, signing, provenance checks and
production-tag promotion. Published digest values and Nox revision acceptance
remain separate verification work.

A clean `a481eb9d` responsive sweep runs all six `layout-audit-*` spec files
against a disposable populated Go HTTPS Server. Chromium and WebKit each pass
81 cases without skips or flakes. Firefox passes 80 and times out once. The
single sign-on configuration case stalls in its first `page.setViewportSize`,
after successful sign-in and before opening configuration. The trace has no
completed resize call; the failure screenshot shows the loaded home page.
This is retained as an unresolved automation failure, not an identity-provider
failure or a complete responsive pass. All browsers validate the fixture TLS
certificate; only the disposable container and Firefox profiles trust its CA.

The sweep covers six widths from 320 through 1440 pixels, populated media and
library routes, settings, onboarding, created playlists and collections,
configuration persistence, forced colors and dark presentation. The 62 distinct
Chromium 320-pixel renders were inspected in full-page contact sheets and readable
top crops. No additional defect was confirmed in those views. The remainder of
long pages, interactive scrolling, other engine renders and real assistive
technology are separate review boundaries. Private commands, source identities,
reports, screenshots, traces and hashes remain retained.

The decoded native follow-up uses the workflow's generated 12-second H.264/AAC
fixture under two task-owned temporary filenames. iOS passes three applicable
cases and tvOS passes two; none skip. Real AVPlayer decodes, pauses, resumes,
seeks and exposes buffered ranges through authenticated synthetic native loopback.
Thirteen new renders cover iOS pending, loaded and failed video, loaded audio,
and tvOS audio, seek and system video. These were inspected. The system-video
still does not prove interactive AVKit controls, real Go Server playback or
physical hardware. Both owned temporary links are removed after identity/hash
checks; the pre-existing landscape fixture remains untouched.

Xcode changes the active simulator data-container path during installation.
The first follow-up helper restores its prior evidence folder into the old path.
A corrective inspection queries the active container again, preserves all new
renders, and restores the original `Documents/buffer-bar-proof` state there.
Full pre-test data backups remain private. Neither simulator nor its installed
app is removed. Both simulators return to their original shutdown state. No
prior app-version restoration or complete app-data identity claim is made.

The API 36 tablet uses WebView `133.0.6943.137`. A private extra instrumentation
class reproduces missing chapter text with an attached, visible WebView and
enabled accessibility. Disabling screenshot accessibility checks alone still
fails. Comparisons with renderer termination enabled and disabled also pass,
so neither comparison proves a unique cause. Three executions of the two
unchanged tracked EPUB methods pass four and fail two. The latest-position
method passes all three; the longer reader method fails on initial chapter
load once and chapter-two reopen once, then passes. The chapter assertions
remain unchanged. Private diagnostic classes are separately identified by
source/APK hashes and are not production changes. Emulator packages and data
are restored or task-installed packages removed after every run. These failures
and the physical TalkBack boundary remain open.

The opt-in real large-transfer journey's receipt hardcodes Nox and Node 26.
This mislabels local runs. Its environment field now reads the actual process
platform, architecture and Node version. Transfer, hash, restart, offline
playback and seek assertions remain unchanged. A dedicated greater-than-2-GiB
synthetic fixture run follows; tiny mocked chunk checks remain isolated proof.


## Real large offline transfer — October 12

At `d5efeb69399d5c71f71c6eae1b4d71f72798f6f1`, the opt-in Chromium journey
passes against a disposable populated Go HTTPS Server with TLS validation.
The generated file is 2,156,160,861 bytes. Its 12-second H.264/AAC video has an
ISO `free` box; this verifies transfer size, not feature-film playback duration.

The transfer is interrupted at 75,497,472 bytes by closing the browser. The
persistent profile reopens, verifies the original Viewer and private job, and
resumes. The completed OPFS record matches the source size and SHA-256. After
another browser shutdown, an independent Node stream hashes the actual backing
file to `249ab9776988313fdede264d7932891c061cda25a5b34845ed72134f0bc2736b`.
There are 259 range responses, including starts beyond 2 GiB.

A second reopen verifies the same Viewer. The fixture's network connection is
then disconnected. Offline playback decodes 320×180 video, advances and seeks
to seven seconds without a media error. Both retained screenshots were inspected.
The single case passes in 61.40 seconds without retries, skips or flakes. The
full fixture command takes 107.63 seconds. The owned container, synthetic large
file and browser profile are removed; private receipts, screenshots and hashes
remain. Host certificate trust is unchanged.

The prior attempt reaches 72 MiB and fails because the test demands credentials
from an already restored session. The existing session-persistence behavior
redirects ordinary sign-in to the home page. The journey now checks the original
Viewer and authenticated job after both reopens. The first credential sign-in,
restart, range, independent hash and disconnected decode assertions remain.
This is a test-harness repair; no application authentication behavior changes.
The failure model and failing attempt were recorded before the test edit.
`make max-loc` and post-commit Player `make verify-changed` pass.


## Continued diagnostics and Nox acceptance — October 12

The unchanged single sign-on configuration journey runs three focused Firefox
executions at `ece703614a59171883330306183f86b2f6506941`. Two pass and one times
out at its first 1440×900 resize. Private protocol capture pairs request IDs
with session IDs: the failed resize receives no protocol response. Fourteen
resize calls in the passing executions return without protocol errors. An
initial projection paired IDs without their session IDs; its apparent error
response belongs to a different command and is excluded. Six later executions
with read-only pending-resize telemetry pass without triggering the telemetry.
These diagnostic passes do not clear the unchanged journey's failures.

The pinned [Firefox Juggler implementation](https://raw.githubusercontent.com/microsoft/playwright/v1.63.0/browser_patches/firefox/juggler/TargetRegistry.js)
waits for viewport dimensions after setting browser size. Upstream
[issue #43028](https://github.com/microsoft/playwright/issues/43028) describes
resize hangs under other versions and window environments. It does not prove
the same cause here. Test timeouts and assertions remain unchanged.

A failure-only EPUB control runs five fresh API 36 tablet executions. Four pass;
one retains the original chapter assertion failure. The complete bounded
accessibility tree has 32 nodes, with two WebView nodes and no chapter heading
in text or content description. The rendered chapter is visible. Moving the
same WebView into a plain native container without reloading does not expose
the heading. The original parent is restored and the failure remains a failure.
A separate private control that would create a fresh plain WebView passes three
journeys before reaching that failure path, so it provides no comparison result.
Neither private class changes tracked application or test source. Production APK
hashes remain identical. Task-installed emulator packages are removed and the
reusable AVD remains. Physical TalkBack acceptance is unavailable.

Main `394736cb740fc6873aa6e94b41e22dc797155529` publishes Player manifest
`sha256:e2f3799d296dc0c2d9914ec98f8ec6e61e86c4aa603a1a0232973d23985638cb`.
The raw manifest hash matches the registry digest header. All four child
descriptors match the two tested and scanned architecture candidate indexes.
Independent GitHub provenance verification requires that exact source commit,
`refs/heads/main`, the publication workflow and GitHub-hosted runners; it passes.
CI separately verifies the image signature and promotes production tags.

The repository deployment script rebuilds Player for ARM64, scans its archive,
deploys it to Nox and verifies its revision and health. Nox reports that exact
main revision, running and healthy, and its in-container health check passes.
Its local image ID is
`sha256:28fe2e962881de9e92d118ee95a1e4a4eddf7b091cd2dd382961a1801313de0d`.
This image is separate from the published manifest. The saved-source watcher
is preserved and may deploy later primary-checkout saves. Subtitles is unchanged.
No authenticated production journey, external gateway TLS or physical playback
acceptance is claimed from container health.

Trivy 0.75.0 and Syft 1.54.1 run from a task-private directory after official
release checksum verification. The Nox scan records 168 Low, 181 Medium,
133 High, one Critical and 23 Unknown findings. No High or Critical finding
has a fixed package version in the configured distribution, so the existing
release-policy gate passes. This is not a vulnerability-free result.

The Critical scanner entry is `CVE-2026-6653` in
`libxml2 2.12.7+dfsg+really2.9.14-2.1+deb13u3`.
The [Debian security tracker](https://security-tracker.debian.org/tracker/CVE-2026-6653)
marks that exact stable package vulnerable, records it as a minor issue without
a stable security advisory, and lists a fix in unstable. Read-only inspection of the deployed `394736cb` image confirms that FFmpeg,
FFprobe, bsdtar and fpcalc link to this library. This proves runtime presence.
Reachability of the vulnerable function and exploit reproduction remain unproved. No suppression, release-gate
change or unsupported distribution upgrade is applied. Full scan and SBOM
receipts remain private and checksummed.


## QA-018 — WebKit OPFS quota regression lacks persistent storage (test repair)

The remaining offline batch at `43502d17` passes 31 Chromium and 31 Firefox
cases. WebKit passes 30 and fails the orphaned OPFS quota case before reaching
its quota assertions. Three unchanged focused WebKit executions also fail.
The worker advertises the sync writer API, but `getDirectory()` rejects with
`UnknownError`. Plain sixteen-byte controls fail at the same directory step in
both the failing context and a fresh ephemeral context.

Matched controls use pinned WebKit 26.6 on macOS ARM64 and Linux ARM64. On
both systems, an ephemeral context cannot open the directory. A fresh persistent
profile writes, reads and removes sixteen bytes. This isolates a browser test
prerequisite; it does not demonstrate a product OPFS defect.

Only the OPFS quota case now uses a fresh persistent profile. It retains the
configured engine, launch options, Firefox profile CA wrapper, TLS validation
and desktop context settings. The context closes and its owned profile is
removed in `finally`. A bounded attachment records removal. Traces, screenshots
and failure videos remain available; successful temporary videos are deleted.
The IndexedDB case retains its ordinary context. No production source changes.

## QA-019 — Seeded quota test retains the initial download action (test repair)

The first repaired-profile repetitions pass all three OPFS cases in each engine.
One WebKit IndexedDB case instead times out seeking “Download to this device.”
Its screenshot shows “Resume on this device” and the interrupted-download message.
The trace never dispatches that click. The test seeded an interrupted job after
capturing the initial download locator; asynchronous refresh correctly changes
the action label. The same race is possible in the OPFS case.

Both seeded cases now reload the downloads page and use “Resume on this device.”
Their quota values, orphaned bytes, corrupt chunk, manifest hashes, exact
`[16, 16, 1]` range sequence and resulting storage assertions remain unchanged.
Three executions of both cases pass in Chromium, Firefox and WebKit: six passes
per engine, zero skips, failures, flakes or retries. Nine profile receipts confirm
removal. The owned browser containers are removed and host trust is unchanged.
The first committed artifact-cleanup replay exposes a teardown error in all
three engines: calling `video.delete()` after a persistent context closes uses a
closed browser connection. Quota assertions pass, but those executions remain
failed. The fixture now closes pages, handles their finished videos while the
browser is open, then closes the context and removes its profile. The committed replay at `c44bb1e2` passes six quota cases per engine with zero
skips, failures, flakes or retries. A private intentional failure after the
unchanged OPFS assertions, once per engine, retains screenshots, two videos and
a trace, preserves that single failure, and removes its profile without a
teardown error. These expected failures verify artifact handling; they are not
passing product cases. An output audit then finds three tiny blank launch-page
videos remaining after successful WebKit runs. The fixture now removes its
known raw-video directory after closing the browser; retained failure copies
remain outside that directory. Final replays verify both outcomes.

These are retained isolated fault checks with synthetic chunks, mocked manifests,
quota and service-worker state. Real Go sign-in and preparation do not turn the
mocked transfer into populated-server E2E proof. The real 2.16 GB transfer,
independent backing-file hash and disconnected decode remain separate evidence.
Failure models, unchanged red runs, matched controls, exact commands, snapshots,
source hashes, results and artifact checksums remain in the private evidence root.

## Further Android and gateway boundaries — October 12

The private plain-WebView control reaches the original EPUB failure on its fourth
fresh tablet execution. The original reader's heading remains absent after
reparenting. A fresh static WebView exposes its heading and finishes loading in
the same renderer. This rules out a wholly unavailable renderer or accessibility
service in that execution. It does not identify the reader's underlying cause.
A later failure-only control would inspect the original heading after destroying
the plain WebView. Four executions pass without reaching that control, so it
provides no comparison result. Original failures remain open. Production APK
hashes stay identical, temporary packages are removed and the shared AVD remains.

Nox's configured HTTPS site passes normal system CA and hostname validation
with TLS 1.3. Anonymous `/healthz` returns 200; `/` redirects; `/settings`
returns 404 and `/api/v1/settings` returns 403. These responses include CSP and
omit HSTS. A read-only `/login` request returns a password form. The separate
Caddy wrapper proxies by network address, without the repository gateway's Unix
transport or configured proxy-token header. Its DNS resolves only to a private
address from this Mac. WAN forwarding and external Internet reachability are
unproved. The intended LAN/VPN versus restricted public-access role is pending
user clarification; no live gateway or authentication configuration is changed.

The gateway shares no Player application-state mount source. Its process reports
`NoNewPrivs=1` and seccomp filtering, but that does not prove the repository's
process-wide IP-dial restriction. The gateway's separate `/config` destination is
Caddy configuration, not evidence of an application-state share. A diagnostic
initially omitted the configured nonstandard HTTPS port and hit a refused
connection; the corrected request preserves the port and passes. This setup
mistake is excluded from product failures. No private addresses, response bodies,
credentials or raw configuration are published.

Main advanced through other reviewed playback and architecture changes. The
branch reconciles `a95f2cbb` without conflicts as `7f65fbc1`; `make max-loc` and
post-merge Player `make verify-changed` pass. Required GitHub gates will run again
for this reconciled offline test checkpoint before protected delivery.


## Watch disconnected simulator render — October 12

The unchanged Watch target builds successfully for watchOS Simulator at
`6b5ee97d`; its native source tree remains `c8b05b870b3c22ebc3b4b3bf61d95c2033ba23fc`.
The existing QA Watch stays paired with the same QA iPhone. Only the Watch boots;
the iPhone remains shut down. An initial screenshot captures system launch.
At five and fifteen seconds, the Watch renders “Nothing playing,” the instruction
to start a title on iPhone or Apple TV, and “Connecting to paired iPhone…”.
The screenshot fits the device width. The connection activation outcome is not
captured, so this does not establish a terminal failure or a working relay.

The preexisting Watch app and its full data container are backed up before
installation. Cleanup reinstalls the original app, resolves its current data
container and restores its data; every original file hash matches. The Watch
returns to Shutdown and the paired iPhone stays Shutdown. The pair is unchanged.
Simulator is unavailable through the native UI-control inventory and absent at
the selected Xcode GUI path. No Watch gestures, paired-phone playback command,
Health permission, heart-rate sample or physical-device acceptance is claimed.
Apple's [WCSession documentation](https://developer.apple.com/documentation/WatchConnectivity/WCSession)
requires activation before messages; the captured connecting label alone does
not identify activation status or failure cause.


## Offline verification delivery and current deep results — October 12

PR #572 merges the offline verification changes through protected main as
`1560949fa734d0a86ef057ab4cdaa1b19f5c97df`. Required run `38206722040` passes for
reviewed head `c7e9ac536b100ed5cd34fc0db78640373484588e`. A fresh fetch proves that
reviewed head, the real large-transfer fix `d5efeb69`, and final quota checkpoint
`24b9b762` are ancestors of remote main. Primary local main remains untouched
because it contains unrelated native edits. Main CI/publication run `38209590154`
is tracked separately from the passing PR gates.

After reconciliation with the fixture sign-in pacing, both quota cases pass in
Chromium, Firefox and WebKit at `c7e9ac53`: six passes, no skips, failures or flaky
results. No raw successful-run videos remain. This is synthetic quota coverage;
the independent 2,156,160,861-byte transfer retains its own earlier revision,
source/OPFS hashes, restored Viewer and disconnected decoding evidence.

Manual deep run `38206719965` fails. The final Go log identifies HTTP 503 from
real copied-HLS AAC/AC3 offset-12700 requests and the speed suite's resume case.
These requests fail before independent segment decoding begins. No race warning
is found. The collected coverage is 88.6%, but failed Go tests prevent threshold
evaluation; it is incorrect to identify coverage as the reported failure cause.
Production Go and package trees are identical to preceding main `4d8ddd2b`.

All 99 Subtitles cases pass in each browser engine, without skips or flaky
results. Player Firefox's selected suites pass without flaky results. The broad
Player reports also contain fixture-dependent skips; those are not completed
journeys. WebKit's dedicated settings/Player suite reports 61 passes and three
failed late-callback cases. Their attempts include stalled decoded-motion setup
and timeouts after the watched form is accepted. The ordinary watched-form
journey passes. These results do not clear the controlled race failures.

## Subtitles published artifact verification — October 12

Both Subtitles architecture digest artifacts remain available from successful
main run `38181332045` at `8047e689`. Registry reads verify the raw final manifest
hash and all four descriptors against those scanned candidate indexes. The final
manifest is `sha256:6b36075e79217fb568828cac918731dc51a2380e9073725b2a1a2fed14927a51`.
Independent GitHub attestation verification passes for that exact main revision,
main ref, publication workflow and hosted runners. Subtitles and shared package
sources remain unchanged through `c7e9ac53`. This proves that publication;
it does not establish current-main Subtitles promotion or update Nox Subtitles.

## Paired Watch controls and readiness finding — October 12

Private, uniquely namespaced companion packages preserve the original apps and
simulator pair. An early paired render shows “This iPhone” with “Nothing playing,”
which establishes a received empty state. Its stdout instrumentation captures no
reply line, so absence of that line cannot negate the visible reply. Later safe
OSLog controls capture Watch activation, then a busy exchange without a reply
through eighty seconds. Phone-first startup also captures no reply. Screenshots
include black system frames, so those do not establish a terminal app failure.
No actual playback command, Server session, Health permission or physical relay
is verified. Apple's [Watch Connectivity sample](https://developer.apple.com/documentation/watchconnectivity/transferring-data-with-watch-connectivity)
explicitly directs testing its transport on physical iPhone and Apple Watch.

An early cleanup checks Watch absence before rebooting its still-installed
companion. The next preflight detects task-owned packages again. Corrected cleanup
removes the phone companion first, then the Watch package, and proves both absent
together after a delay while both simulators remain booted. Both return to
Shutdown with the original pair preserved. No original package or data is reset.

QA-020: the Watch claims “Nothing playing” before receiving player state, and
also after transport or invalid-reply failures. Actual disconnected simulator
screenshots reproduce the initial contradictory empty/connecting state. A
render contract fails before production edits. A separate private exchange
fixture then executes the original Session parsing/state and RemoteView on
watchOS Simulator: connecting, transport failure, invalid reply and first recovery
failure each fail the render contract; valid empty, playing and recovered states
pass. This is isolated controlled-exchange evidence, not a working paired relay.
A proposed readiness-state repair is being checked privately before publication.

## Deep codec dependency mismatch — October 12

QA-021: the deep Go job installs Noble's FFmpeg 6.1, while supported containers
and hosted browser media controls use Jellyfin FFmpeg 8.1.3. The same compiled
public HTTP tests reproduce all three offset/resume failures with FFmpeg 6.1.1
on Linux ARM64. They pass with pinned Jellyfin FFmpeg 8.1.3. The original hosted
failure uses AMD64 and race instrumentation; that remains a distinct boundary.
The matrix retains original assertions, offset-zero warmup and nonzero offsets.
Its task container is removed. This finding does not justify a runtime fallback
or a claim that all older codec installations work.

The proposed workflow repair installs checksum-verified Jellyfin FFmpeg 8.1.3
before deep Go tests and selects its executable directory. It retains the race
suite, coverage thresholds and all required gates. The workflow contract fails
before this change and passes after it. Complete selected hosted verification
remains required before delivery.

The separate static/moving movie comparison initially puts its file outside the
configured Movies folder. Ten static executions time out before selecting any
movie, and the next invalid comparison is interrupted. These are diagnostic setup
failures, excluded from product evidence. Its task container is removed without
changing host trust. A repaired comparison must verify Library admission before
starting the unchanged watched-state tests.


The complete CI contract suite, `make max-loc`, workflow validation, and the
repository-pinned actionlint v1.17.0 fork pass locally. Its parallel-child and
synchronization contract also passes. Homebrew actionlint 1.7.12 rejects existing
parallel steps and `queue: max`; that unsupported validator result is retained
separately. No workflow syntax or gate is weakened to accommodate it.

The final Chromium artifact from deep run `38206719965` contains 707 broad-suite
passes and 353 fixture-dependent skips. Its selected settings suite passes all
64 cases. Caption recovery (7), Library pagination (30), queue loading (6),
download pause (47), and download hit targets (1) also pass. All these reports
contain zero failures and zero flaky results. The artifact SHA256 is
`54d78b4a4820a4178a608d6a9b74c824a289993d2d0bc59b25c948e3a32f11a8`.
Skips remain outside completed QA.


## Watch readiness repair and render evidence — October 12

The Watch now distinguishes pending connection, a received player state, and an
unavailable iPhone. “Nothing playing” appears only after a valid reply establishes
that empty state. Transport, malformed reply and activation failures show recovery
guidance. Valid later replies restore the player controls. The pending screen has
one connection message and the native progress indicator.

Before these production edits, the original simulator render contract fails for
pending, transport failure, malformed reply and the first recovery failure. Valid
empty, playing and recovered controls pass. The final candidate passes all seven
controlled-exchange contracts on the existing 46mm Watch. Every distinct checked
state image is inspected. The test fixture replaces transport outside production
sources; it does not establish a paired-device reply or control command.

The same seven captured contracts pass on an existing 40mm Watch after correcting
two fixed Vision OCR readings of “iPhone.” Four initial checker failures are
retained. The pixels show correctly wrapped text; no production layout change is
made for OCR. The corrected checker still rejects the original contradictory
empty/connecting screen and the intermediate duplicate connecting copy. A repeated
recovery capture is byte-identical to its already-viewed unavailable capture.
`simctl` reports Dynamic Type control unsupported for this Watch; larger text and
VoiceOver remain unverified.

A separate namespaced build preserves the actual candidate Watch Connectivity
transport. With its paired iPhone shut down, three captured checks remain pending
and contain no empty-state claim. No reply or playback actuation is claimed.
Its simulator build and unsigned watchOS device build pass. Both task companion
packages are removed together, both simulators return to Shutdown, and their
original pair remains. The small Watch also returns to Shutdown with its package
removed, content-size preference unchanged, and the complete pair graph preserved.

Controlled native logs contain initial connection and recovery at `Logger.info`,
and one `Logger.warning` per outage. The unified-log capture represents warning
as message type `Error`; these are separate recorded facts. Operation, bounded
failure class, session UUID and request UUID remain available. Invalid JSON maps
to `invalid_message`; arbitrary error text and the private secret sentinel are
absent. Repeated polling during an outage produces no duplicate warning. These
are native simulator logs, not Server, browser, deployment or physical logs.

`apps/player/apps/native/scripts/check-watch-remote-render.py` retains the visible
state assertions. Run it against a private simulator PNG with `--image`, `--state`
and a fresh `--output` directory. Playing also requires `--title` for a known
synthetic title. The script compiles its local Vision OCR helper when `--ocr` is
absent. Its receipt records capture/checker hashes and per-assertion results;
raw OCR and compiler diagnostics stay in the private output directory. A passing
capture does not prove transport, gestures, playback commands, Health or hardware.

## Direct-media WebKit isolation follow-up — October 12

The corrected source comparison requires authenticated Library admission before
testing. Static FFV1 and moving FFV1 sources each pass all five original watched
journeys through HLS on Linux ARM64. Direct H.264 MP4 fails the decoded-motion
prerequisite in all five cases. An AAC-audio control also fails all five. No
application source, assertion, retry or timeout changes in these comparisons.

Native observation records media time stalled near 0.16 seconds, frame callbacks
stopped near 0.2 seconds, a fully buffered twelve-second source, and no media error.
The same authenticated direct source stalls in a plain video element with Player
scripts absent, both with and without AAC. Each plain control fails its decoded
motion assertion. This narrows the local finding to browser or delivery behavior;
it does not identify the hosted WebKit late-callback failure's root cause or prove
physical Safari behavior. Every task browser container is removed and host trust
is unchanged.

The final native artifacts from deep run `38206719965` report 332 iOS passes and
one skip, plus 313 tvOS passes and one skip. Neither reports failures. These execute the
unchanged pre-repair native source tree `c8b05b870b3c22ebc3b4b3bf61d95c2033ba23fc`;
they are distinct from the new Watch candidate's render evidence.


QA-022: `make -C apps/player native-copy-check` fails against the existing source
revision before the Watch repair. A baseline-source projection proves that missing
“Back to Library,” “Back to library,” “TMDB,” and the existing TMDB disclaimer,
plus the stale “Media” entry, cause the catalog drift. Neither new Watch label
changes the generated key set. The unchanged generator refreshes Apple and
Android/Wear catalogs from existing web translations; the exact check then passes.
No translation text or generator policy is invented. English Watch captures do
not establish acceptance in other locales. Primary checkout catalogs remain
untouched.


PR #576 merges the deep codec dependency repair as
`87539b43ae425208416639eabe7dc1fa4429f1ba`. Required run `38211720643` passes for
reviewed head `523b5863ebe31d933fb4c0cf2898a98af2dff4af`; a fresh fetched ancestry
check proves both the reviewed commit and merge in remote main. Its manual deep
run fails its Player coverage gate at 88.7%, below the unchanged 89% requirement.
The final log contains no failing Go test, race warning or panic. All three former
codec failures pass. Firefox reports one flaky late-callback settings case;
WebKit reports two unexpected failures in that flow. Required protection remains
intact. Previous main run `38209590154` succeeds, including Player publication at
`1560949f`. Current-main publication is tracked separately.

The native build entry point now requires the existing catalog check before iOS
or tvOS compilation. A retained command regression uses the real Makefile,
generator and gate shell with a synthetic source key and compiler stub. Corrected
baseline controls fail for all three stale catalogs; fresh catalogs compile both
targets. After the one-line prerequisite change, all controls pass. Rejection
invokes no compiler and changes no catalog bytes. This is build-contract evidence,
separate from app UI and Swift compilation.

The first setup omitted the gate shell and is excluded. A second setup selected
an untranslated Spanish entry, so its empty Android/Wear files were valid. That
three-platform baseline claim is withdrawn. The corrected fixture uses the
translated “Library” entry and asserts nonempty catalogs before introducing drift.
Exact command receipts, source and log hashes remain in the private evidence
directory under `native-copy-build-guard-*-v3`. The complete CI contract suite
passes after this repair.

## QA-023 — Deep coverage misses consumer execution

The final Player profile marks database opening and configuration paths uncovered,
although Server, CLI and backup consumers call them. The deep collector uses Go's
default per-package instrumentation. It therefore misses execution through tests
in another package. These production functions are used and remain intact.

A disposable two-package module reproduces the accounting gap with a real HTTP
request and filesystem write. The unchanged collector passes the consumer test
but fails coverage at 66.7%. Adding only `-coverpkg=./...` reports 100% for the same
executed production statements. An unexecuted-function control still fails at
37.5%, with that function at 0%. All three runs retain race detection. The first
profile projection ran outside its module and is excluded from complete proof.

Before the collector change, the retained command regression and coverage-policy
contract both fail. After adding module-wide instrumentation, they and the full
CI contract suite pass. The quick command, complete package selection, race mode,
89% app requirement, 85% package requirement and existing test-helper exclusion
remain unchanged. Wider instrumentation can add deep-run overhead; hosted results
must establish its full-module coverage and execution cost. No test or assertion
is removed to obtain coverage. Private `consumer-coverage-*` receipts retain exact
commands, revisions, profiles and source/log hashes.

The first full hosted profile with module-wide instrumentation reports Player at
88.8% and Subtitles at 91.6%, at revision `ea760872`. Player remains below the
unchanged 89% gate. Instrumentation corrects consumer accounting but does not
close the remaining execution gaps. After workflow completion, the full log
confirms no failing Go test, race warning or panic. The 88.8% threshold check is
the failure. Profile archive hashes and block accounting are retained in the
private `deep-ea-go` directory.

## Late native callback failure evidence

The final hosted failure traces select Arrival over HLS, rather than the helper's
preferred Example Movie. WebKit's failure capture shows “Seeking…” after the
near-end seek. Four attempts fail while waiting for the actual late `ended`
notification. Firefox's failed first attempt reaches the later reopened-page
control assertion; its retry passes. Neither cause is resolved by these traces.

Six focused local HLS repetitions pass unchanged assertions on Linux ARM64.
The same six pass with the retained failure-only snapshot. This does not reproduce
the hosted AMD64 failure. Before releasing the held request, the test now reads
native media state through its owned relay. The attachment includes bounded
buffer/seekable ranges, time, frame count, native flags and numeric error code.
It excludes source URLs, credentials, remote text and media identifiers.

A deliberate private assertion failure retains one valid native snapshot, the
original forced error, failure artifacts and successful container cleanup. Host
trust remains unchanged. The first negative selector matches no cases and is
excluded. An earlier interval probe passes six journeys but loses its successful
snapshots after closing the relay; it provides no native-snapshot evidence.
A final type-only cast produces byte-identical JavaScript under TypeScript 6.0.3.
Private `watched-failure-evidence-*` receipts record the controls and hashes.
No playback behavior, retry, timeout or product assertion changes.

The next hosted run, `38219097077` at `ea760872`, again fails both WebKit
unwatched cases. Three failure snapshots show a seek held at 11.9 of 12 seconds,
with readiness 1 and buffered output ending at 3.92 seconds. The remaining
attempt records native decode error code 3 and no decoded frames. The first
three snapshots identify an unbuffered tail while navigation is held; they do
not yet establish why that tail cannot load. Snapshot revision fields are absent
because the hosted environment does not set that variable. Attribution comes
from the exact-revision artifact and workflow metadata. Chromium and Firefox
pass their settings journeys; WebKit also has one flaky album-advance journey.
No failure or flaky case is treated as completed QA.

## Short HEVC preparation through real HTTP and codecs

A retained journey generates an 8.37-second HEVC video with EAC3 audio, then calls
the production Library, playback planning, preparation and HLS routes over real
loopback HTTP. Planning preserves HEVC and converts audio. The journey compares
every delivered video frame hash with the decoded source and requires decoded
audio. It checks warm adoption, cancellation of preparation after playback, and
preparation again. No encoder, response or server operation is mocked.

All three phases pass locally with race detection using macOS ARM64 and FFmpeg
9.0.2. They also pass on Linux ARM64 with the hosted deep run's pinned Jellyfin
FFmpeg 8.1.3 package. Each phase delivers all 201 matching video frames and 394
audio frames. The Linux test is cross-compiled without race instrumentation;
the macOS run supplies the separate race evidence. The disposable Linux
container is removed after collecting evidence.

Private `hevc-public-e2e-first-artifacts` and `linux-hevc-public-e2e` directories
retain source media, frame hashes, exact commands, revision and checksums.
The final Linux receipt also records the test source hash and runtime versions.
This is real API and stream decoding evidence. It does not establish browser,
simulator, physical-device or full-module coverage acceptance.


## Buffered tail and native frame-counter resets (test repair)

The held-navigation fixture now completes a real tail seek and requires the tail
in native buffered ranges before submitting Mark unwatched. Six initial WebKit
attempts still fail. Their precondition snapshots show full buffering, while
final snapshots show actual playback ending at 12 seconds. Two bounded diagnostic
replays establish that the native decoded-frame counter resets from 50 to 1, then
advances to 13. The fixed baseline could never observe the required newer frames.
One diagnostic selector finds no tests and is excluded from execution proof.

The fixture now tracks the lowest observed frame counter during held playback.
It still requires three newer decoded frames, media time advancing over 0.2
seconds, and the actual native pause or ended event. Real form submission,
accepted Server state, zero late progress requests and persisted state remain
required. Product playback code, deadlines, retries and assertion outcomes do
not change.

All five watched-control journeys pass three times per engine on Linux ARM64:
15 Chromium, 15 Firefox and 15 WebKit passes, without retries, skips or flakes.
The final test source hash is
`92029df041f5ed3eaefdf973907fa18f33fd19ec7d9f8d8ef363c478733868e8`.
Private `watched-buffered-tail-full-positive-safe.json` records exact source,
report hashes and cleanup. Each disposable container is removed; host trust is
unchanged. These local results do not close the hosted AMD64 failure, its native
decode-error observation, the flaky album journey, or physical-device acceptance.

PR [#578](https://github.com/Kinosail/kinosail/pull/578) merges after every required
check passes. A fetched ancestry check proves reviewed head `ae711016` is in
remote main `a2d5f2b1`. Watch relay replies, physical Watch controls and VoiceOver
remain separate acceptance boundaries.


The complete tooling check then finds one stale Code Atlas value: the Server
package has 376 test files, while its snapshot still records 375. Regeneration
changes only that count. The Subtitles snapshot remains byte-identical, and the
architecture contract passes. Deep run `38226919051` at `9d4bca82` is cancelled
after its tooling failure; it provides no complete acceptance result. A fresh
exact-revision deep run follows the snapshot checkpoint.

## Deep run at `6008fa77` — coverage passes; one WebKit startup flake remains

[Deep run 38227963067](https://github.com/Kinosail/kinosail/actions/runs/38227963067)
completes at `6008fa77c6a9f7dbf8aee27e25dab224c1d0f1d3`.
Full module coverage reports Player 89.8% and Subtitles 91.6%. Both unchanged
coverage gates pass. Both Go jobs pass, with no race warning or panic in their
collected logs. Passing individual tests are not named in those logs; they do
not independently establish whether the new HEVC test executed or skipped.
The retained local HEVC decoding receipts remain the explicit per-test proof.

Player Chromium passes 862 executions and Firefox 858. WebKit passes 855, with
one flaky case. Primary Player selections also skip 353, 357 and 359 cases,
respectively. Subtitles passes 99 cases in each of the three engines, with no
skips or flakes. The previously persistent WebKit Mark unwatched cases pass
their first attempts. The remaining flaky Mark watched case passes its retry
after timing out during decoded resume setup, before held navigation begins.
The strict hosted flaky-test policy correctly leaves that job failed.

The failed screenshot shows “Playback interrupted.” Its captured HLS requests
return 200. The first decoded start passes; the immediate pause and second start
fail to establish motion. The trace omits media bodies and the fixture captures
no initial-setup media snapshot. A successful HTTP response does not establish
valid media bytes or a working decoder. No root cause is claimed from this trace.

Twelve diagnostic repetitions pass without retries on Linux ARM64. They record
actual native events and numeric state without URLs or remote error text.
One initial selector matches no tests and is excluded. A failure-only capture
then records startup phase, native error code, decoded frames and buffered
ranges. A synthetic resume denial remains failed and produces that attachment.
The same retained capture passes 15 WebKit, five Chromium and five Firefox
watched-control executions after reconciliation with main. Deadlines, retry counts and motion assertions remain
unchanged. These passes do not close the hosted AMD64 interruption.

The native artifact confirms iOS 332 passes with one skip, and tvOS 313 passes
with one skip. Android compilation and both apps' AMD64/ARM64 container jobs
also pass. Native package tests and compilation remain separate from physical
devices, real Watch replies and production media journeys.

Private `deep-6008-go`, `deep-6008-webkit`,
`deep-6008-remaining-browsers` and `deep-6008-native-contracts` retain immutable
archives, parsed reports and checksums. `watched-initial-*` receipts retain the
diagnostic, rejected-start control and final test sources.

## Matched native WebKit decoder controls

The same silent and AAC MP4 bytes run through the same minimal Go HTTP server
and plain video page on macOS ARM64 and Linux ARM64. Neither page includes
Kinosail playback JavaScript, authentication or service workers.
All six macOS attempts advance decoded frames and media time. Linux passes one
silent attempt and fails the other five attempts, including all three AAC runs.
Failed attempts stop near 0.16 seconds with full buffered media and no native
error. Inspected synthetic captures confirm the visible frame difference.

This narrows the direct-MP4 freeze to a browser/media environment boundary.
It does not identify the cause or establish that the separate hosted HLS error
has the same cause. Source, exact media hashes, browser versions, commands and
receipts remain under private `native-decoder-matched-controls`.
The owned server and disposable container are removed. Host trust is unchanged.

## Verified Player deployment; newer main remains blocked

Player main `a2d5f2b1` passes hosted CI and production-image promotion.
The independently downloaded GHCR manifest matches its registry digest,
`sha256:4951917f9683ab36889191f5e89889a1a82e79ded80e4e0e8611b8420efbd1aa`.
Both platform image configurations name the exact source revision. GitHub
attestation verification passes with the expected main source and hosted signer.

The existing deployment script builds, scans and deploys its own ARM image to
Nox. Independent inspection confirms revision `a2d5f2b1`, running and healthy,
with a passing in-container health check. That local image is
`sha256:5ad3b43422754b2fb3cf181e80527e5cf71fa4a176f401ca2b2ef03d2bc613fd`;
it is distinct from the published GHCR image. The scan retains one Critical
finding, CVE-2026-6653, and no fixable High or Critical finding under existing
policy. Runtime linking confirms the installed libxml2 reaches the deployed
media tools; vulnerable-function reachability remains unproven.

Fresh anonymous HTTPS checks require normal CA and hostname validation and
TLS 1.3. Root, health, Settings and settings API return 303, 200, 404 and 403.
Their responses contain CSP and lack HSTS, as previously recorded. A system
curl build cannot request TLS 1.3 and is excluded; the successful Python client
uses default CA trust and hostname checks. This proves private-network transport,
not authenticated production flows or public-gateway acceptance.

Subtitles remains healthy at `6f41117d`. A proposed update stops before building
or changing Nox because main advances to `5f169ee2`. Its main run
[38229443046](https://github.com/Kinosail/kinosail/actions/runs/38229443046)
fails cold-suffix lookback recovery with `fragment_not_restored`; the other 17
fault cases pass. The focused macOS replay and three Linux ARM64 replays pass. Linux uses the
hosted pinned Jellyfin FFmpeg 8.1.3 package and an attested host cross-compiled
Server binary; its runtime compiler copy is explicit. Recovery diagnostics now
record final HTTP status, fragment existence, response size and owned encoder
completion before the existing failure check. Native hosted AMD64 diagnosis and
current-main publication remain open. Reconciliation regenerates the single
conflicting Code Atlas snapshot from both source changes; its contract passes.

Private `main-a2d5-publication`, `nox-main-a2d5-*` and `main-5f-hls-failure`
retain exact revision, manifests, scan evidence, health, transport and failure
receipts. No production data, gateway policy or unrelated checkout is reset.

After reconciliation, the real HEVC and copied-fragment recovery Go journeys
pass with race detection. The full CI contract suite and source limits pass.
The original hosted failures remain retained; no passing replay erases them.


## Cold HLS encoder handoff repair

The retained `lookback-join` case reproduces the missing final fragment.
Its wrapper runs real FFmpeg unchanged, then delays the middle refill's exit.
The original `d69516d9` binary fails three Linux ARM64 runs: segment 15 returns
404, no replacement starts, and the encoder eventually settles.
The repaired binary passes all three runs. Segment 15 returns 200, a fourth
encoder restores it, and decoded video frames and timestamps match.
The source, retained initialization, certificates and playlists remain unchanged.
All disposable servers, probes and containers settle; host trust is unchanged.

Missing-fragment admission now waits for file arrival or the exact producer's
completion, then rechecks source and playback policy before reuse or refill.
A separate HTTP control exposes a changed source being served after publication.
It fails before that recheck and passes after it. Cancellation still settles
without starting a replacement. The existing 30-second request limit remains.
The preparation warning now includes the playback session, mode and cancellation.

The initial delay control exposes expected final master-bandwidth publication.
The retained case waits for initial encoder completion before snapshotting metadata;
it delays only later refills. No metadata assertion is removed.
All 19 real-media cold-suffix cases pass on macOS. The focused Go suite passes
with race detection, real first-fragment regeneration and the named HEVC proof.
Source limits pass after moving a shared test fixture into its existing cache file.
Server logs retain correlated INFO lifecycle and WARN failure events without tokens.
Browser, physical-device, hosted AMD64 and deployment proof remain separate.

The newer deep run retains another failure: Player coverage is 87.9%, below 89%.
Compared with the passing `6008` profile, the two new copied-audio files add
189 uncovered statements. Existing browser/native audio proofs do not contribute
to that Go profile. The gate remains unchanged; coverage repair is still open.

Private receipts: `cold-retained-join-baseline` (three expected failures),
`cold-retained-join-fixed` (three passes), `cold-lookback-final-race`,
`cold-suffix-all-final` and `deep-d695-go-failure`.
- `cold-retained-join-baseline` log SHA-256: `097007aa10a9a1a772e032c743cd622b88574c1be41c64b05a125d206d073ab1`.
- `cold-retained-join-fixed` log SHA-256: `fbeb2cf100afe2f4ae277cabddd6cd719f087d80daf135e290b02ed6fa56ef3e`.
- `cold-lookback-final-race` log SHA-256: `aae06b99ce8a7ac97646a9b7c212654d9586927e22618542580306240668a924`.
- `cold-suffix-all-final` log SHA-256: `048f3ee385ab39d20b6fd0273b8618049b0490fc7dfae76699b700dc1434c5ec`.


## Long-source certification in the deep Go consumer

The real-codec HTTP journey now includes a 74.5-second continuous ALAC source.
It requires matching certified source and AAC clocks, then deletes and restores
both the first and a middle fragment. FFmpeg decodes the middle fragment before
and after refill with original timestamps; every video frame and timestamp matches.
Owned encoders join before cold reopen. Existing source, metadata and retained
fragment checks remain required. This extends the existing public lifecycle proof;
it also exercises warm seeking under Go race instrumentation.

All three codec cases pass with race detection and no skipped case. The complete
local module coverage diagnostic passes at 89.8%: 8,473 of 9,437 statements.
Its profile SHA-256 is
`bb027d59d51302ec1097f537663c07ee358103c017dd02b9e64c8500bd6d8306`.
That complete local diagnostic has no race instrumentation. A separate full local
race attempt is interrupted after 524.87 seconds to inspect runtime stacks.
Those stacks show active asset compression during repeated test-server construction;
the interrupted attempt is excluded from pass evidence. Its receipt is retained.

The workflow now enables the existing strict media journey in deep Go runs.
The wiring contract fails before that change and passes after it. The 89% gate
and complete race command remain unchanged. All CI contracts, workflow lint and
source limits pass. Hosted Linux AMD64, with pinned FFmpeg 8.1.3, must still verify
the complete race profile on the final commit.

The preceding `d69516d9` deep WebKit run retains one first-attempt startup failure;
its retry passes. The diagnostic records resume, seeking, zero decoded frames,
readyState 4 and no media error. All recorded HLS requests return 200. The rendered
player remains on Seeking. This failure remains open; repeated local passes do
not clear it. Private `deep-d695-webkit` retains its trace, screenshot and bounded
native-state snapshot. A private scheduling control investigates whether holding
startup callbacks creates extra resume work before the intended navigation race.

Private `long-copied-clock-*` receipts retain exact source, commands, profiles,
logs and checksums. Browser audio waveform proof and physical-device proof remain
separate from the Go video-frame and lifecycle checks.


## Required-check repair after the HLS push

The hosted tooling check rejects a stale generated Player Code Atlas snapshot.
The hosted new-code lint also rejects complexity in the HTTP timing fixture
and a temporary MP4 write in the long-source comparison. These are retained failures.
The fixture now separates adoption, outcome publication and request settlement.
It preserves all five response outcomes, cancellation and retained-file assertions.
The frame comparison streams owned fragment bytes to FFmpeg through standard input.
It keeps original timestamps and compares every decoded video frame.
No production code or lint setting changes in this repair.

The focused startup and native recovery journeys pass again with race detection.
New-code Player lint passes. The generated Code Atlas snapshot is refreshed.
The complete local tooling check passes after the first snapshot refresh;
the focused architecture contract passes on the final refreshed snapshot.
Private receipts retain the hosted failures and successive local controls.
Final race log SHA-256: `cc2bfa012328453b19d1767f5bfa0e5121a21365956ec2983e572dce9835558b`.
Final new-code lint log SHA-256: `e92606b0bf483111dff0a120c315ea165821348f31365020e2468a0059095c47`.
Complete tooling log SHA-256: `2f2625f866eba8d6ec7fd2f7456627cb1b1dc0ba8b1fb3086a2865e0163a6563`.

The preceding deep browser artifacts confirm Player Chromium 862 passes and
Firefox 858 passes, with 353 and 357 skipped cases respectively.
Subtitles has 99 passes in each browser and no skips or flakes.
The actual native result bundles report iOS 332 passes and one skip,
and tvOS 313 passes and one skip. They do not prove physical-device behavior.
WebKit startup remains open; a successful local repetition does not clear it.
