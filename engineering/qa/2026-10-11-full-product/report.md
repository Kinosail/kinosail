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
| Quality and security | Go tests/races/coverage, lint, metrics, supply chain, workflow contracts | Deep Go race/coverage passes at `6fe7a7b9` (89.5%); local lint debt and hosted WebKit playback failures remain open; HLS reconciliation needs fresh hosted checks |
| Containers and release | Both apps and architectures, scan, installer, publication, deployment, TLS | Both apps at `47f250c4`: hosted promotion, separate scanned Nox ARM images, healthy containers and private-network TLS; authenticated and physical acceptance remain |

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


## Watched callback setup and initial native diagnostics

The fixture now delivers its initial queued playing callbacks after decoded startup.
It still holds the later resume notifications for the accepted-watched departure race.
All form, persisted-state, late-progress and decoded-motion assertions remain required.
No deadline, retry count or flaky-test policy changes.

Private prototypes pass 15 WebKit cases, five Chromium cases and five Firefox cases.
The exact retained source then passes all five cases in each browser without retries.
An isolated binary removes only the accepted-watched save guard.
Both the original and adjusted scheduling controls reject that binary in every trial.
They observe late native ended callbacks restoring the wrong watched state.
The served script hash proves the negative control is active.
This preserves detection of the original product regression.

Initial setup failures now retain at most 96 native media events.
Each event contains only its allowlisted type, numeric state and callback count.
Native decoder time is recorded separately from the Player timeline time.
A deliberate second-play denial retains the original resume failure and nine events.
The diagnostic includes no source URL, body, credential or remote error text.
Owned containers are removed and host trust stays unchanged.
These are controlled setup checks; they do not reproduce the hosted seek stall.

The hosted `df8a2259` complete Player Go race run passes with 89.7% coverage.
The unchanged 89% gate remains required. Its profile SHA-256 is
`bcf2d27e17d8e646a78df05d8c43bcaab78bc90a06f3625ee31090b7fc5ea16e`.
That run still rejects the stale snapshot, test lint and old WebKit setup.
The repaired source needs a fresh complete hosted run before those failures clear.

Private exact-source receipts: `watched-native-retained-{webkit,chromium,firefox}`.
Their log SHA-256 values, in that order, are
`bf75cbd93d7478d82d0c9ace22da206b78cdf5810f28a49ba03aac65e9460d2e`,
`37b89dbec5865109f8c0e66f76fa18c0721cdc3308a76eab1e242d80e14845f0` and
`872170357488d503da2da4c2e5208dd7f11063bb23c5cdcf66c0e047e985a880`.
Private `watched-native-retained-denial` retains the controlled failure;
its log SHA-256 is `e5074d20ddcf51455c1bf927364b2313cda27de94adbc386e34e56eaa278469f`.

The complete `df8a2259` WebKit artifact has two failed watched cases and one
flaky ordinary form case. One first-start snapshot reports media decode error 3
and no buffered frames. Other starts remain seeking with no decoded frames.
The ordinary flow has no queued-playing hook. These observations keep a broader
native playback boundary open beyond the callback setup adjustment.
The bounded native journal now covers the ordinary flow too.
Its failure snapshot preserves the original exception without changing the flow.

The final shared journal source passes all five watched cases in each browser.
The ordinary denial control retains its rejected play attempt and 44 native events,
all with zero queued notifications. Existing canplay autoplay can still advance
media during that control; the failure remains the deliberately rejected own call.
Its original error is preserved. No source URL or remote error text is retained.
Private `watched-native-retained-final-*` and `watched-native-ordinary-denial`
receipts record exact source, commands, results, checksums and container cleanup.


## Hosted checkpoint at `6fe7a7b9` and HLS reconciliation

[Deep run 38247019149](https://github.com/Kinosail/kinosail/actions/runs/38247019149)
passes complete Player Go race tests with 89.5% reported coverage.
The profile SHA-256 is
`0b26b1d5ac87b9ff3b7e1a3c7fad5f2de38af773651cfb67fff84d1d898b72aa`.
Chromium passes 862 cases with 353 selected skips. Firefox passes 858 with 357 skips.
Neither browser records a flaky case. Subtitles passes all 99 cases in each engine.
The hosted Apple artifacts contain 332 iOS passes and 313 tvOS passes.
Each Apple platform has one skip and no failed test.

WebKit passes 854 cases with 359 skips, one failed case and one flaky case.
The flaky watched-resume journal records initial decoded motion, a frame-counter
reset on resume, repeated source reloads, and native media decode error 3.
A failed unwatched case remains seeking near the buffered tail without decoded frames.
The ordinary form case passes this run. Earlier ordinary failures remain retained.
The bounded journal now also accompanies failures after initial startup.
No assertion, timeout, retry policy or product playback script changed for that diagnostic.
The original tooling job fails inside `go list`; its captured subprocess error hides the cause.
The same source passes required PR tooling. The targeted hosted tooling retry also passes.
The deep run still fails because the retained WebKit failures remain unresolved.

Main advanced to `51a5fcc2` through PR #581 with another producer-completion repair.
Its nonblocking preparation and successful-completion-only readmission are preserved.
The existing public HTTP source-change control fails against that implementation:
a fragment published after its source changes returns 200 instead of 404.
The synthetic completion fixture first adopts main's explicit completion cause.
The reconciled path then rechecks the captured source and policy under the manager lock
before serving a fragment that arrived during a producer wait.
That red control passes with race detection after the check.
Canceled producer, canceled request, published output and replacement controls also pass.
These controlled HTTP checks do not replace fresh real-codec and hosted verification.

The incoming `refill-completion` real-codec case replaces the duplicate `lookback-join` case.
It holds the third completed encoder for three seconds and proves the request joins it.
Initial encoder settlement, retained metadata hashes and per-recovery outcomes remain recorded.
The safe producer-join log correlation from main remains required.
Earlier `lookback-join` artifacts remain reproducible at their recorded revision.

Reconciled focused Go race checks pass 102 named tests and subcases, without skips.
The private log SHA-256 is
`dff09681a671014df6ff087275d3178f89db0c1a9c08cf0b9cd95af07fa6b2ea`.
New-code Player lint and complete repository tooling pass.
The 19-case real-codec matrix passes 18 cases, including held producer completion.
Its success case records a connection error during warm restart.
That exact case passes a focused follow-up with an identical binary and source patch.
Warm reuse and both corrupt and missing certificate regeneration pass the follow-up.
The original failed matrix remains retained; it is not reported as a single passing run.
These macOS codec results use FFmpeg 9.0.2. Hosted FFmpeg 8.1.3 remains a separate gate.

A private observation overlay adds known request and session identifiers to the existing HTTP controls.
The source-changed publication emits one WARN with both identifiers, mode and cancellation state.
Successful publication emits no preparation warning. The synthetic private query and source path remain absent.
This observes handler logs; it does not establish deployed logging behavior.
Its private log SHA-256 is
`02229f3be5f5c4c43ed8aeb9891fad896e1996845246864f383bb1c495508e12`.

## Native decoded-frame counter reset in the browser setup helper

The reconciled source reproduces three WebKit watched-journey failures without retries.
Chromium and Firefox each pass all five journeys on that same source.
The new late-failure journal records native frame counters resetting after pause and seek.
The helper retains its pre-reset count and can miss advancing decoded media until the clip ends.
One failed tail start resets from 43 frames to one and then reaches the real ended state.
An earlier start similarly crosses the clip end before the held-navigation proof begins.

The helper now tracks the lowest frame count observed during each play attempt.
It still requires three newly decoded frames, advancing time, unpaused media and no play rejection.
The corrected source passes 15 WebKit watched journeys, five Chromium journeys and five Firefox journeys.
All runs use real Servers, trusted HTTPS, moving media and zero retries.
The four existing checkpoint setup controls also pass in all three engines.
Those controls retain delayed acknowledgements, late abort, rejected start and stored near-end resume checks.

An isolated compiled binary then removes only the accepted-watched save guard.
All three WebKit negative controls detect the original overwrite: watched becomes true after Mark unwatched.
Each fails at the stored-state assertion, with no motion timeout.
The actually served asset contains the negative marker and lacks the original guard.
Its SHA-256 is `2b8a15bf808575f79c86395d2a7e2ca3ed1a19eea70a73fe7b9fd1fd19f52231`.
This verifies that the corrected setup still detects the product regression.
The positive WebKit result SHA-256 is
`8fdff3bb211b186048c44c057de6d2773754e037ce68c6c97583f8e3fcb9ba25`.
The negative result SHA-256 is
`6d96eb4f125c1e526020e45789a14449251e38d74e225716fd599ab998478cad`.
All disposable browser containers are removed. Host trust is unchanged.

Earlier native decode errors and seeking stalls remain separate findings.
The helper repair does not establish their complete cause or physical-device behavior.
A fresh hosted deep run is still required on the committed reconciled source.

The next deep run, `38255568088` at `0cb41055`, passes the complete Player Go suite.
It flags cognitive complexity in the reconciled segment request method.
The local diff filter had missed that function-level diagnostic at an unchanged declaration.
Fragment preparation now has a separate helper with the same missing-file and error handling.
Its diagnostics use the inherited segment context and retain request and session correlation.
Lint against the broader pre-reconciliation revision passes after this repair.
The focused race checks also pass again. Fresh hosted checks remain required for the repair commit.
The run's browser and native results are still pending at this checkpoint.

## Further EPUB accessibility and device boundaries

The byte-identical frozen Android production APK again reproduces missing chapter text.
After the original assertion fails, a content-change event does not recover the heading.
Clearing and restoring the WebView description also fails to recover it.
A separate control successfully activates touch exploration through UiAutomation.
The original heading remains absent, and the exact service flags and touch state are restored.
This is an emulator accessibility control; actual TalkBack is not running.

Two focus/reload control attempts encounter an Android System UI ANR modal.
Their inspected screenshots and unfocused app windows exclude them from EPUB conclusions.
Four intervening unchanged executions pass and therefore do not execute the controls.
All task-installed packages are removed and the existing AVD is shut down after each run.
No production EPUB repair, current localization-asset proof or physical acceptance is claimed.

Four bounded cold-boot executions then pass the unchanged EPUB assertion.
Each disables snapshot loading and saving while preserving the existing AVD data.
The focus/reload controls never run because the original assertion passes.
These results establish neither snapshot causality nor an EPUB accessibility repair.
Task-installed packages are removed after every execution, and the task emulator is shut down.
The fourth execution's pending, loaded, empty and failed captures are visually inspected.
They show another layout finding: pending tablet lines span the viewport, while loaded text uses a centered reading column.
That skeleton geometry repair is queued for the next native UI change.

Fresh physical discovery still finds no connected iPhone or Watch.
The physical Apple TV preview remains at its system profile chooser.
No profile is selected during this refresh. The existing human handoff remains pending.

## Follow-up hosted and decoder observations

Main advances to `a615e769` with the independently reviewed marker-bounds repair.
Reconciliation preserves both source changes and regenerates the Player architecture page.
The preceding deep run at `0cb41055` reports canonical Player Go coverage of 89.7%.
Its WebKit report has one flaky compact-toolbar case and no unexpected final result.
The first attempt passes the button geometry assertions, then fails during screenshot capture.
Its retry passes; the first failure remains retained and is not accepted as a clean pass.
The original watched-state failures do not recur in that report.
The artifact SHA-256 is `edbde696f982ee241ee47c7f1c49d8b2f33a91b5899d9cae7cf554a676077882`.

A separate native-decoder observation keeps the original loopback server, media and playback predicate.
All six Linux WebKit contexts stall near 0.16 seconds, with no native media error.
Each receives the complete HTTP 200 media body with the exact expected SHA-256.
Both silent and AAC files match their original bytes. Incomplete transfer is excluded for these six observations.
Response-body instrumentation remains a diagnostic timing boundary.
The result SHA-256 is `49cd4fc4e0d9bea9ac1cf197e1ac282383826be89fffba7c4bc573b1df6d9220`.
Earlier GStreamer warnings report temporary-file removal, not file creation failure.
They do not establish the playback-stall cause. No production workaround is introduced.
The owned diagnostic container is removed. Physical Safari behavior remains unproved.

## Resolved subtitle-policy follow-up

The deep run at `c0e38f59` retains 862 Chromium, 856 WebKit and 858 Firefox passing browser executions.
Those browser reports have no unexpected or flaky results. Player Go coverage is 89.5%.
The complete run fails the separate startup boundary and a tvOS endpoint-restore prerequisite.
The tvOS failure occurs before its native scrub assertion; no product cause is established.

Two hosted startup executions fail the fragment-success assertion after subtitle burn-in invalidation.
The pinned Linux ARM64 reproduction fails at that same assertion.
The wait-time policy check compares the URL recipe with a policy captured after source resolution.
The URL recipe lacks the external subtitle path included in the captured policy.
A valid subtitle-burned continuation can therefore return 404 after successful publication.

The check now resolves the source recipe before comparing policies.
Source inspection stays outside the HLS manager lock. The existing complete policy check remains under that lock.
The unchanged startup journey passes with this candidate, including continuation decoding and retained initialization bytes.
Its Linux runner copies an attested cross-built binary instead of rebuilding inside the browser image.
The Mac replay passes while explicitly skipping burn-in because its FFmpeg lacks the subtitles filter.
It does not establish the hosted failure's acceptance.

The candidate diff SHA-256 is `b2b704f2eec757bd32485ffb3fc5f52428d5acbaa21d1cee440bae1a9101b2c5`.
The passing candidate binary SHA-256 is `76dde0cd2396ce0fc63645725123c645b8474b9d54b4e673bfb3bf812fa0f8a8`.
Main's shared copied-generation plans and native progress repair are then reconciled.
The generated Player architecture page is rebuilt from the combined source.
On that reconciled source, 42 focused HLS tests and 60 subcases pass with race detection and no skips.
The stale-source and canceled-producer rejection controls remain intact.
The broader pre-reconciliation lint check, complete tooling suite, workflow contracts and architecture contracts pass.
The pinned actionlint fork passes; the Homebrew upstream validator lacks the repository's native parallel-step syntax.

The controlled log observation repeats the five existing HTTP outcomes with synthetic request and session correlation.
The source-changed publication emits one WARN. Successful publication emits no preparation warning.
The synthetic private query and source path remain absent from the diagnostic projection.
These are controlled handler logs, not deployed Server or browser log proof.
The exact commands, patches, binaries and private logs remain in `linux-startup-continuation-*` and `hls-ae91-*` evidence.
Fresh hosted checks are still required for the final committed revision.

The original startup journey also passes on the fully reconciled source with the integrated repair.
Its production-source snapshot is base `c0e38f59` plus patch SHA-256 `e5f51acb995e23f0faa93475e2dbc62080fa82891d7fb5946096984dc0cedcff`.
The reconciled Linux binary SHA-256 is `dbfe66a5d0366a863cfdc13e2dc4c7d6a0cafff769d3c4e1978ab07f8e161a30`.
The owned container is removed. This remains Linux ARM64 proof; exact-head hosted AMD64 checks follow the commit.


## QA-026 — Layout recording ends before slow first paint

The hosted WebKit layout run at `fd70beb4` stops after its first completed case.
The library renders, but the recorder times out waiting for a fresh baseline frame.
Chromium and Firefox layout runs pass. The hosted artifact SHA-256 is
`94a2c9ccc1a22a25228165d52815b4bc875d77b159588bb9cc051dfde313b216`.
A diagnostic Linux ARM64 replay passes 39 cases and 11 flows without reproducing that timeout.
Its largest baseline is 6184ms. The hosted failure has no captured baseline timestamp,
so this replay does not establish its exact cause.

The recorder stops its animation callback eight seconds after navigation.
A new isolated CLI regression serves a real critical stylesheet after 8500ms.
The normal-render control passes. Three slow-render executions fail at the original fresh-frame wait.
A fourth red execution uses the final fixture and fails at the same wait.
No browser clock, frame callback or geometry predicate is mocked.
The fixture is a local HTTP peer, not populated Go Server E2E proof.

Recording now remains active until the settled measurement is collected, then stops.
The initial and settled geometry checks and existing automation bounds remain unchanged.
The regression requires a fresh frame after each viewport's baseline and a later settled frame.
Both slow-case baselines must exceed 8000ms.
The final fixture identifies its two documents rather than assuming one CSS request per document.
Firefox requests each stylesheet twice; the earlier exact request-count failures remain retained.
An earlier Chromium fixture also counted an icon request as a document; that fixture error is corrected.

The final normal and slow cases pass in macOS Chromium and WebKit with zero retries.
Linux Firefox passes both cases and all five retained authentication controls with zero retries.
Its two slow baselines are 9304ms and 9270ms, followed by fresh and settled frames.
The final test SHA-256 is `1c47919d7433c3f381b9dcb9c3b77545d95bba630dfd79ecba2f91a9f1cbbafd`.
The repaired CLI SHA-256 is `09076f6796f4f3be319be6d92d5d7fd3e7aa419ae09dbe58a8b7267d4fe3ca41`.
The macOS run log SHA-256 is `808b1d1ec14db929e4ff3aa7ce1fc924c8dd907d7686927a551022830499bef8`.
The Linux Firefox run log SHA-256 is `005a88415226a1266f26c7fd548d8c86f50b2fdb22accff306eff42290488634`.
The owned Linux container is removed, and host certificate trust is unchanged.

macOS Firefox cannot open its profile folder, including an execution outside the sandbox
and a separate task-owned profile-location control. Those failed launches remain retained.
An initial Chromium authentication control exceeds its subprocess bound; its outside-sandbox replay passes.
The whole-tree TypeScript keyword policy still rejects five `unknown` annotations in the unchanged
`test-instance-watched-startup.spec.ts`. The new test has no keyword-policy violation.
The syntax and source-cap checks pass. Private `layout-sampling-*` receipts retain each attempt.

## Current hosted and simulator boundaries

The `fd70beb4` deep artifacts report canonical Go coverage of 89.7% for Player and 91.1% for Subtitles.
Subtitles passes 99 browser executions in each of Chromium, Firefox and WebKit, with zero skips or flakes.
Player Firefox has no unexpected or flaky result across its retained reports; platform skips remain.
Player WebKit retains one flaky native-startup case. Its first attempt is still seeking at 0.1 seconds
with zero decoded frames and no native error; the retry passes. This is not a clean startup result.
The failed media-state SHA-256 is `6c495f78f0a8138e5596fd9caa2fbc1386166ffee6bd0dcfb11e4f39cc0b4192`.
No timeout, decoder predicate or watched-state assertion is weakened.

Hosted iOS starts zero tests because its runner hangs before connecting.
Hosted tvOS passes 319 tests, skips one and fails none.
The retained native artifact SHA-256 is `d8c34aef603834374f7bbec0e38c85c9a4366fa0c24fe527decbf5a097f2bfc9`.
A separate local iPhone simulator suite passes 329 tests, skips four and fails none.
Three skipped cases require the external decoded-media fixture; the remaining case is platform-specific.
The local run preserves the exact native source tree and restores the simulator's shutdown state.
Its summary SHA-256 is `21a2b1aca02656a8cc0bf2e902cc0d5f56fc1e3e23eac42cbd268ee62c2ad5c5`.
Local simulator proof does not replace the failed hosted runner or physical-device proof.
Fresh discovery still finds no connected iPhone, iPad or Watch. Two Apple TVs remain connected.
The existing physical-TV profile handoff remains pending; discovery selects no profile or app.


The repaired recorder also passes the populated Linux ARM64 WebKit audit: 39 cases and 11 flows.
Every initial frame follows its baseline. The owned container is removed, with host trust unchanged.
The run log SHA-256 is `bf08c96652d999acac0318bd543f31b6dc66cefac7f272c3c17f2f3be361e947`.
The source archive and patch retain the exact recorder; the container adaptation only selects
an existing trusted-CA Firefox launcher. WebKit predicates and native clocks remain unchanged.
The report-only additions made during execution do not change the archived app or recorder sources.

Required CI run `38322824963` passes for PR #579 head `fd70beb4`.
Its tested merge is `66df3b153a084ff44172d442c1ecc4535fc1db3f`.
A fetched ancestry check confirms that the reviewed head is included in that merge.
The startup browser case passes with zero skips, unexpected results or flakes.
Its burned-subtitle invalidation and original-init continuation assertions execute successfully.
The startup artifact SHA-256 is `046cb2beef9dfab95145751019dd2a21336f82bb8399b1d2ed4d97aa05ba8097`.
This is PR-merge startup proof, not remote main ancestry or production deployment proof.
Fresh hosted checks must verify the subsequent recorder commit before protected integration.


## Delayed-CSS fixture correction after hosted Chromium

Required CI for `659046b8` fails only the new slow-render sampling fixture.
Both attempts produce fresh baseline and later settled frames in both viewports.
The CLI then rejects BODY layout shifts of 0.005146 and 0.002829.
The delayed fixture changes the default body margin, heading size and card height.
That is a real fixture shift, so the layout enforcement correctly returns exit 1.
The failed browser artifact SHA-256 is `f3893ea96b6a3a20840eb8c35d129f28ce206361e9e49d5fe3c9f24f0fabf640`.
Later receipt checks cannot complete because this earlier browser step fails; startup is not reached.
This result does not reproduce the HLS subtitle-policy failure.

The fixture now reserves identical real geometry inline.
Its delayed stylesheet exposes the body only after delivery, without changing those dimensions.
CLI enforcement, zero-exit, fresh-frame, settled-frame and 8000ms baseline assertions remain unchanged.
The final fixture still fails against the original recorder at its fresh-frame wait.
The repaired working CLI is restored after that red control, with its exact hash verified.
The normal and slow controls then pass in Linux Chromium, Linux Firefox and macOS WebKit.
Each final engine run has two passes, zero skips, zero unexpected results and zero flakes.
Both owned Linux containers are removed, with host trust unchanged.
The final test SHA-256 is `944dd0ba82a9abb840393afc0071a9c5798e0d275b4961ce6f89c2bdfd4d2c65`.
The Linux Chromium run log SHA-256 is `3b6a2c0289836ea4eba0963e53f353b8cbd250c6dc4e18d2baf4f0022b817a90`.
The Linux Firefox run log SHA-256 is `71784f44ae3fe93782579aba24927c55ed043a06bdfcd54bf07158a23e61babd`.
The macOS WebKit run log SHA-256 is `c9e962df082c294eb687bf3b9a6ce35d98e815264c519dd4e7be72c22054a501`.
The original-recorder red-control log SHA-256 is `7f2ddaac8fb3858658060e3abf97e2adbf9cddfd989a732824fc26a8db5252df`.
These remain isolated CLI fixtures. The populated layout workflow at `659046b8` separately passes all three engines.
Its source receipts show the exact head, empty working diffs and no source drift.

The `659046b8` hosted native artifact also completes both platforms.
iOS passes 332 tests and skips one. tvOS passes 319 tests and skips one. Neither platform fails.
The artifact SHA-256 is `012bb10c30cbbfefd99567eb9fe10cf7a2d3aefb25b339ff72094ee985a185a6`.
This clears the preceding hosted iOS startup boundary for this run; its earlier failure remains retained.
These contracts do not include the separate pending Apple reader UI branch or physical-device acceptance.


The hosted layout receipts for `659046b8` retain every selected case without source drift.
Chromium passes 95 Player cases and 111 Subtitles cases, with 11 and 26 flows respectively.
Firefox and WebKit each pass 40 Player cases and 46 Subtitles cases, with the same flow counts.
Their fixture suites retain platform-specific skips separately from these populated measurements.
The Chromium artifact SHA-256 is `bb258677879920ef47959ac4130652797e610eb5feb106d70ca4a253f73ba4b3`.
The Firefox artifact SHA-256 is `7c80bef31c682b85d38337a3f1c9eb56f581cb5efe5ccb432b6b5ecb9f0602d2`.
The WebKit artifact SHA-256 is `bc4f7b25bc31990a16d2d9bf23740dd9174226a441e7bb8fc0f2274d5c688f51`.
All three record CLI SHA-256 `09076f6796f4f3be319be6d92d5d7fd3e7aa419ae09dbe58a8b7267d4fe3ca41`.
These hosted source receipts precede the fixture-only correction at `b0de9685`.

## QA-024 — Native verifier skips catalog inputs and reuses stale success

The native stage selects only changes under `apps/native`. It can skip commits
that change the Makefile, generator, translations, Android source or catalogs,
or the shared language list. Its cache also omits those dependencies. A previous
native-build success can therefore hide stale generated copy.

The failure model and retained CLI regression precede the verifier repair.
Sixteen source-selection and cache subcases fail before the change, with zero
fixture errors. An unrelated documentation control passes. These isolated
checks execute the actual verifier, Git and content hasher. Compiler, module and
source-cap commands are explicit stand-ins. Product E2E does not invoke this
local selection and cache boundary.

The verifier now selects a catalog check for copy-only changes. Native and
Makefile changes retain the native-build stage. Both stages hash the Makefile,
generator, translation data, native source and catalogs, relevant Android source
and catalogs, and shared language list. Unrelated documentation still does not
start native verification. The regression is part of `make tooling-check`.

Independent controls use the actual Makefile, generator and translation data.
The old verifier incorrectly accepts a stale Spanish catalog through both a
native-cache hit and a copy-only commit. The repaired verifier rejects both.
Catalog hashes remain unchanged and no compiler runs during rejection.
Regenerating catalogs restores success and permits both native compiler calls.
Those controls use a compiler stand-in; the first control's incorrect expected
stage after regeneration is corrected before the passing replay.

The retained CLI suite, shellcheck and complete local tooling command pass.
A separate actual `make -C apps/player client-check` passes iOS and tvOS Xcode
compilation after the real freshness check. This proves compilation, not app
interaction or physical-device behavior. A unittest discovery attempt selects
no hyphenated files and is excluded; the root tooling command invokes the files
explicitly. Private `native-verification-*` receipts retain commands, revisions,
source hashes, controls and logs.


The three locally skipped decoded-media journeys are then executed with the exact CI fixture recipe.
Cold and prepared native playback, seek/pause/resume, video buffering and audio buffering all pass.
This focused execution passes three tests with zero skips or failures on the unchanged native source tree.
It supplements the earlier 329-pass run; it is a separate execution with an added media fixture.
The unsupported Picture in Picture control remains a local platform skip.
The fictional media is 288629 bytes with SHA-256
`fca8542e89dcbfc618e21fdbe98261caacc72070ab92866ac604783ba4c37ff6`.
The test-summary SHA-256 is `38093ff64f22d364446b235b34d20d6996528682b902b0afc7fafd59769ae60a`.
The private run log SHA-256 is `9878657f4f0330a8a58654acbe244a32af43a22bdaa30e4740db6fbad05e7854`.
Both task-created fixture links are removed only after their hashes are rechecked.
The iPhone simulator's original shutdown state is verified after execution.
The hosted iOS connection failure and physical-device boundaries remain open.

## QA-025 — EPUB pending geometry and enlarged reading

The tablet pending column starts at pixel 40; its loaded chapter starts at pixel 592.
The native regression records both public UI bounds before the production repair.
It fails the column assertion on the original source.
The reader now centers pending lines within the default chapter's 68ch column and 16dp page padding.
Pending line height and spacing use the selected, scaled reader font.
The reading page and publisher styles remain unchanged.

The first phone and tablet matrix also exposes horizontal overflow with system font scale 2.0.
A new native viewport assertion fails before the document repair.
Its artifact records WebView text zoom at 200% and horizontal overflow.
Reader CSS already includes the system font scale, so the default WebView zoom applies it again.
The document now fixes WebView text zoom at 100% while retaining scaled CSS text.
Light-page system insets keep the dark toolbar background so white status icons remain readable.

The initial width measurement rounds each zero-glyph advance before multiplying by 68.
The correction uses fractional glyph advances with linear scaling and subpixel positioning.
The native geometry assertion keeps its original two-density-pixel tolerance.
The [Android implementation](https://android.googlesource.com/platform/frameworks/base/+/master/graphics/java/android/graphics/Paint.java)
confirms that `measureText` rounds up; `getTextWidths` preserves advances.

The confirmation journeys pass on normal phone and on phone and tablet at system font scale 2.0 with a light page.
Their loaded chapters have no horizontal overflow, and pending column edges match the loaded content.
The tablet's normal confirmation is interrupted by a visible System UI ANR dialog.
That execution is excluded from EPUB and accessibility conclusions.
Its failure remains retained. The quiet normal-tablet follow-up passes with both column edges exactly aligned.
Pending and loaded left edges are both pixel 592; the pending right edge is pixel 1968.
WebView text zoom is 100%, and no horizontal overflow is present.
The three earlier confirmation journeys precede this final fractional-width correction.

Pending, loaded, empty and failed captures are inspected in both appearances and device classes.
The native fixture retains blocked-script and escaped-resource checks, chapter navigation, renderer recovery and saved position.
The native checks also verify that success and failure remove the pending placeholder.
These are device-side synthetic HTTP journeys, not populated Go Server or physical TalkBack proof.
The separate intermittent chapter-accessibility finding remains unresolved.
Task-installed packages are removed and original system font settings are restored after every execution.
The task emulator is shut down; unrelated AVDs and data are preserved.

The final tablet artifact SHA-256 is `5f14e7f2776a96a2f9642e215c7b69bff7fbf79761611393f742f6f5572db3de`.
The original geometry failure artifact SHA-256 is `c4f38a78cb224b68ce1b9d819f97f8df74742f17ade9683e2398540144b403c3`.
The original overflow failure artifact SHA-256 is `9a9906a12ee6b95bfb8aa8ff61a6457d6e1e93d97e97d9b9dae52f7986c570f8`.
Private `epub-column-*` and `epub-overflow-*` receipts retain exact APK hashes, source patches, commands and environment.

All four final configurations use the same frozen APK pair.
Normal phone and tablet use a dark page. Font-scale 2.0 phone and tablet use a light page.
Every execution passes the complete chapter journey with text zoom 100% and no horizontal overflow.
Phone pending and loaded left edges are pixel 42. The enlarged tablet left edge is pixel 32.
The Android source tree is `62625979b9ad55fd13a227558ed1646b4e579525`.
Reconciliation preserves that exact tree, so it preserves the tested production and test sources.
The app APK SHA-256 is `2984a5122d4989f7f940fd9ac25b7ae91dc391e231e09c663f3814e7a801fe2c`.
The test APK SHA-256 is `f6d9bda776c17399ddd7ae2a86c7f20f5d4dcce238ca482b9e02218561dbc213`.
The final matrix log SHA-256 is `5e6c916eea637b5aa2a0ad7c5cd06d8f6d58abf626c16d6c71acd14067805d8c`.

A separate cold-boot audit reads each existing AVD's settings and installed packages.
Both font settings equal their original 1.0 value. Both task package pairs are absent.
Process and ADB checks confirm that each task emulator is off.
Two earlier audits saw ADB's brief disconnect delay after process exit; those failures remain retained.
The corrected bounded disconnect observation passes without changing product assertions.
Its log SHA-256 is `859ce8cda45781d2a9c1cfa5fbaf579695e37e0714f6de7d61c6ef072579a3ba`.
Private `epub-final-all-configurations-safe.json` and `epub-emulator-cleanup-verified-safe.json`
retain all four artifact hashes, geometry, exact APK provenance and cleanup observations.


## QA-027 — Apple reader pending column spans the iPad viewport

A test-first native journey holds reader metadata while the real pending UI is rendered.
In both iPad appearances, pending lines span x=20–1012; loaded text spans x=176–856.
The source remains unchanged during those two reproductions. Both edge assertions fail by 156 points.
The retained owner suite reports six tests, with one failed parameterized test and four issues.
An earlier test compile error uses a read-only environment value and is excluded from product evidence.

The pending column now uses the default WebKit serif glyph measure, 68ch and 16-point page padding.
Line size and spacing use the current scaled reader font.
The pending footer uses disabled native controls with the loaded footer's styles and insets.
Its center remains a placeholder. Only real pending work shows these controls.
Chapter markup, resource security policy, loaded navigation and reading persistence are unchanged.

The retained native journey compares rendered SwiftUI pixels with actual WKWebView DOM bounds.
It also verifies placeholder removal after success, an invalid empty response and a failed metadata response.
The test is part of the existing serialized preferences owner to preserve Keychain isolation.
It uses a fictional held HTTP peer, not a populated Go Server or physical-device journey.
Animations are disabled in the host transaction for geometry capture; motion is not claimed as verified.

The phone and iPad confirmation suites each pass six tests with zero failures or skips.
Both appearances match their loaded edges exactly: x=176–856 on the 1032-point iPad window,
and x=16–386 on the 402-point phone window. Neither loaded page has horizontal overflow.
Pending, loaded, empty-response and failed captures are visually inspected on both devices and appearances.
The native source patch SHA-256 is `6af2d4b95e0e9c052047c7ec07cc7e552dc25e09ee92cebb7f3ac7be0f3100d9`.
The iPad run log SHA-256 is `39db5a60303dfe6f5f32800d2de5f6a2262f742b51d773b58f7da8cc91a999ca`.
The phone run log SHA-256 is `0213890503688c6f9bb4c84ef9068c283d05a6fe35e4305e56bc91a3ef67260e`.
Original simulator shutdown states are verified after execution, and the previous Keychain session is restored.
This checkpoint covers the default 20px preference. Other reader sizes, Dynamic Type,
publisher fonts, physical VoiceOver and physical reading remain verification boundaries.
Private `apple-reader-*` receipts retain exact source, commands, xcresults, geometry and screenshot hashes.

### QA-027 — saved font confirmation

The expanded journey requests 20px and 32px text in both appearances.
Before the fix, pending lines retain 20px while held metadata waits, despite confirmed saved preferences.
Both 32px reproductions fail before metadata is released.
The reader now publishes validated preferences independently during its concurrent metadata and position work.
It retains the generation and cancellation guards before publishing those preferences.

The final phone and iPad owner suites each pass six tests, with no failures or skips.
Each suite executes all four font and appearance combinations.
The 32px iPad pending and loaded edges both span x=16–1016; the phone spans x=16–386.
The 20px edges retain the earlier matching measurements. All eight loaded pages have zero horizontal overflow.
Pending, loaded, invalid-empty and failed captures are visually inspected at both font sizes on both simulators.
This remains an isolated native HTTP-peer journey. Publisher fonts, Dynamic Type and physical reading remain open.
The final native patch SHA-256 is `3ee7431f20742ab0d4480812cc0b43c51ce9eea109841b4a9b876edb95901784`.
The iPad log SHA-256 is `6fce866ae3fc70c541cedfbc0a3930fa554f2838a38c26d516582703af4fc2b9`.
The phone log SHA-256 is `f5df5522973c9321cf7cbb7c6d5b098acd87b162111bbe4e5ddd34b45ce422fe`.

## QA-028 — canceled catalog consumers keep the reader pending

Held reader metadata returns a failure while its shared preference request remains pending.
The reader stays in its loading state because catalog consumers directly await the producer task.
Canceling a consumer does not interrupt that await. Async child cleanup therefore delays the screen's failure state.
Four font and appearance controls reproduce this failure after metadata is released, with exactly one preference response held.
The HTTP transport already supports cancellation; the catalog consumer wait is the affected boundary.
The red native patch SHA-256 is `8a2bf2321705c58867d5504a38e43210e196b70593798a3c4b3e25e37fb800d9`.
Its log SHA-256 is `8361a087c4af187cee019e942e7df72ae11771bbe217995d90cf1251de02deae`.

A catalog request now resumes canceled consumers independently through actor-owned continuations.
The shared producer retains ownership of validation and persistence.
The reader journey requires failure UI before releasing the preference response.
A second consumer must still receive the saved preference, and a subsequent cached read must retain it.
Both device suites pass all four combinations without changing timeouts or removing failure assertions.

The full iOS suite passes 333 tests, with zero failures and one platform skip.
Its exact tested native patch is the same `3ee7431f` patch above.
Its private log SHA-256 is `a361d152820013ce15d32ff03f9b382dd17d73b3724542b54e3db39a047b3cc8`.
The run uses the hosted decoded-media fixture recipe; both task-owned fixture links are verified and removed afterward.
Debug catalog logs contain 123 received and five canceled outcomes across 128 validated records.
Four correlated producer IDs have both outcomes. Every message matches the bounded operation and random-ID schema.
No remote URL, response body, account identifier or credential is added to these diagnostics.

The initial full tvOS run exposes an obsolete synchronization assumption in the retained navigation-cancellation test.
That test reads the cache immediately after the canceled caller completes, before independent persistence is guaranteed.
The corrected test first requires cancellation while the response is held, then waits for the observable cached value.
This also protects persistence when every consumer leaves, a case the reader's surviving-consumer journey does not cover.
The initial tvOS failure is retained; its three skips include absent decoded-media prerequisites.
Confirmation uses those prerequisites and the corrected observable synchronization.

### Further hosted audit and environment boundaries

Required CI run `38330723203` and layout run `38330722937` pass at `6012dba24dd27c3a39ec0adf375bb3877298ae57`.
PR #579 is merged with merge commit `7131f22a317f97d7819fce5c60b1a766eb61c3db`.
A fresh fetch and ancestry check prove that task commit is included in remote main.
The deep run `38330990339` fails and remains evidence, despite required checks passing.
Its WebKit watched-state startup fails once, then passes on retry: seeking=true, seconds=0.1, decodedFrames=0 and errorCode=3.
This is an additional retained failure, not proof of the earlier Linux decoder stall's root cause.
Its iOS contracts pass 331 tests, fail one extensionless offline AAC decode assertion, and skip one platform case.
The full local iOS confirmation passes that assertion; this does not erase the hosted failure or explain its cause.
Hosted tvOS passes 319 tests with one platform skip.
The WebKit artifact SHA-256 is `8607c7456e35fde3591c4485df8b2cf1a73ee9ac43426c3861988c39ef6a3c6f`.
The native artifact SHA-256 is `718e4f83fd22f926931ac693790fe8dcdfb5fd20c8f03d4e366083553a3049e2`.

A newly installed, task-owned copy of the pinned macOS Firefox runtime still fails with the same profile-folder error.
Global browser runtimes remain unchanged. macOS Firefox is not verified by the passing Linux Firefox controls.
Image publication, deployed revision, physical devices, VoiceOver and the remaining product journeys remain separate evidence boundaries.

QA-028 confirmation passes the full tvOS suite: 319 passed, zero failed and one platform skip.
All decoded-media prerequisites are present. Both task-owned media links are verified and removed afterward.
The native source remains unchanged throughout that run; only this report changes while validation runs.
The final native patch SHA-256 is `e0d7002c262937e71e89b4fe898dd708fc2830294bec03feecbbda9328863f55`.
The tvOS log SHA-256 is `2c29725906b7afb1cab12d0a5d39d4984ffc8a790461e75ee9423f06b64f8130`.
The focused iOS catalog-revalidation suite also passes after the test synchronization change.
Its receipt retains the selected test count and exact source; no production code changes after the full iOS run.
The tvOS diagnostic stream contains 87 received and five canceled records, all at Debug level and with valid bounded shapes.
Original shutdown states for the reused phone, iPad and TV simulators are verified after execution.

The hosted AAC failure log reports its 20-second deadline at phase `asset`.
That default phase cannot distinguish a delayed worker from blocked asset construction.
The root cause remains unconfirmed, and the deadline is unchanged.
The existing manual `AAC` probe option is dispatched against current main for a focused hosted comparison after the full suites.

## QA-029 — native reader counters bypass the interface locale

A rendered Spanish control translates Appearance to Apariencia while the reader still shows Chapter 1 of 1.
The native chapter label is assembled as a runtime English string and has no generated catalog entry.
The owning suite selects seven tests: six pass and the localization test fails its two counter assertions.
The earlier function filter selects zero tests and is excluded from product proof.
The failing source patch SHA-256 is `df6dcbbe4ab4a0f863673bcce0f005b965769d00e28d9475e893cc8ce6de5364`.
Its private log SHA-256 is `8e5fe824772bc392104ec380ce579aa23f1b57fa9c60cfc6d42bfbec6e492d0d`.

The native footer now uses localized Text labels for chapter, page and PDF document modes.
Explicit format-key annotations retain the interpolated keys in the existing catalog generator.
Three messages are added to all 108 source catalogs, preserving their existing formatting and fallback policy.
The 17 translated locales receive translated labels. Generated Apple, Android and Wear catalogs remain synchronized.
The three existing Go catalog compatibility checks pass, and native catalog freshness passes.
The first Go command uses an incorrect relative script path and is excluded; the corrected receipt selects three passing tests.

The retained native regression checks the footer after validated metadata presents the reader.
It uses English, Spanish and Japanese with page 2 of 3, so unequal arguments test positional formatting.
A clearly separated Appearance harness control verifies the selected catalog through rendered pixels and OCR.
The native footer capture does not wait for unrelated document decoding.
Its proof class is metadata-ready native footer; it does not claim that a document finished loading.
The phone owner suite passes seven tests with zero skips, including all nine format and language combinations.
Its source patch SHA-256 is `21e01fcebd5436bbf4a5d0c7b16b2dec41ce7d20eb27fdab82bc199e138b57d1`.
Its private log SHA-256 is `1121db3b230d099aadd275b6a482640ef538ab0b940067868b89f053625c20d0`.
The existing four font and appearance cases still compare pending, loaded and terminal states.
Other translated locales, right-to-left rendering, physical VoiceOver and physical reading remain separate acceptance boundaries.

## QA-030 — native comic navigation finishes late in simulator controls

The expanded format audit retains repeated first-comic readiness failures on phone and iPad.
The original 10-second readiness deadline is unchanged in those retained diagnostic executions.
The iPad observation records metadata at 0.054 seconds and the complete image transfer at 1.382 seconds.
Its WebView still reports loading at 12.293 seconds after window setup begins.
A failure capture completes later, when the WebView is loaded; those observations are distinct.
The first PDF fixture uses an invalid asset route and is correctly rejected.
That fixture is corrected to the required file route before any PDF acceptance claim.

Bounded Debug phases now identify secure setup, resource processing, scheme delivery and navigation completion.
They contain a random reader ID, byte count and monotonic timing, without URLs, titles, bodies or credentials.
The diagnostic iPad owner suite passes all seven tests and all nine loaded-format controls.
Its 66 records match the bounded schema and use Debug level exclusively.
The first image conversion and transfer take about 0.029 seconds; WebKit completion then takes 7.975 seconds.
Later comic WebKit completions take 2.746 and 2.565 seconds.
The source patch SHA-256 is `74bcb25d9a476253fcd1be27c73974f4e294c355426fe45c75165729238b7304`.
The private log SHA-256 is `e5494fee58e034071965a39cd05e258d6905fd8623876cb96ec72a41438f0346`.
The Debug stream SHA-256 is `58f0adcc7da00fcca87e2c6ddfb9c93b1fc16b5dd0e1537fdd28e8aad36ea955`.
The host has ten logical CPUs and measured one-minute load above 23, with five-minute load above 50.
This is a measured environment factor, not a proven root cause.
The passing control and added diagnostics do not repair or erase the earlier readiness failures.
The localization regression now asserts its own native-footer interface; full-document readiness remains open QA work.
Private diagnostic sources, screenshots, timings and xcresults retain every earlier control.

## QA-031 — additional hosted recovery and browser failures

Main CI run `38335277859` passes and publishes both selected containers at `7131f22a317f97d7819fce5c60b1a766eb61c3db`.
The separate manual deep run `38336225457` fails and remains open evidence.
Its Chromium browser reports have no unexpected results or flakes, but the prepared-timeline command fails a cold-suffix control.
Eighteen cold-suffix fault cases pass; the lookback case reports certificate_or_initialization_changed after HTTP 200 recovery.
Its source remains unchanged and its owned probes and Server settle. The cause is not yet diagnosed.
The prepared-timeline artifact SHA-256 is `751af5b0c707d3b229999767b84cb0805370f64d4f83a9a0df3d49d9ed48dc70`.
Later missing-receipt checks fail after that command; they are not independent product failures.
WebKit retains an audio-queue readiness flake and a watched-state failure after another submission is canceled.
The latter fails its real-media precondition on the retry as well.
Its artifact SHA-256 is `3f2ae2036b67bca0ff04830298e4b82a0d4215d88d5503ab870e7152fe1ae20a`.

That deep run passes 332 iOS contracts and 319 tvOS contracts, with one platform skip each.
Its separate AAC probe selects both expected tests and passes both without skips.
Those successes do not explain the earlier AAC deadline failure.
The native artifact SHA-256 is `6e5ecd3185cb2baed33f08370910238044ee5f2ed65f572a491cb060be84a860`.
PR #588 then merges with commit `47f250c46f9d3118942804727e140a6bb9f8fb01`.
A fresh fetch and ancestry check prove that native verifier task commit `ac3ff109406ae34da5858f2f2598c3d5ce2796f0` is included.
No Nox deployment of either new main revision is claimed here.

QA-029 final iPad confirmation also passes seven tests, with zero failures or skips.
All nine native-footer format and language combinations match their rendered expectations.
The final phone and iPad native patch SHA-256 is `21e01fcebd5436bbf4a5d0c7b16b2dec41ce7d20eb27fdab82bc199e138b57d1`.
The iPad private log SHA-256 is `d62b99d3cfd2318f77c428663e94e18e82c904e619559a7493ead8f5572ed3be`.
The same owner still executes the four pending/loaded/font/appearance and terminal-state cases.
Both reused simulators return to their original shutdown state.
Generated Android catalog additions change the APK inputs; the earlier frozen-APK matrix remains proof of its recorded revision.
A fresh Android build and focused native confirmation are required before claiming the final APK.

QA-029 fresh Android phone, test and Wear APK builds pass at `c363c936f554506a60f0bae723fd53c83bb5586a`.
The Android source tree is `a1e34d54e0f35b3fbffb0174e7a76d6e8bee44f9`; only 34 generated catalog files differ from the frozen matrix source.
The app APK SHA-256 is `1fa6120f695ded6630c9930f08fc0e141dd82e1830d9a893011df48f47e89f01`.
Its DEX files and all non-catalog, non-signature ZIP entries match the earlier frozen APK exactly.
The test APK is byte-identical to that matrix's test APK.
The fresh phone journey confirms pending x=42–1038, loaded x=42, text zoom 100% and no horizontal overflow.
It passes initial loading, renderer recovery and chapter navigation, then reproduces QA-011 after reopening chapter two.
The missing-accessibility screenshot visibly contains Harbor lights. No System UI ANR is reported.
The full journey fails; it is not counted as a passing final-APK confirmation.
Its instrumentation log SHA-256 is `d8e56d4e04e3466c174be36611565898f3ecbe7a338b2eaf55bec3cda094bb4c`.
Both task-installed packages are removed and the font setting is restored.
A separate cold boot confirms font scale 1.0, both packages absent and emulator shutdown.
The first cleanup audit checks ADB disconnection too early and fails that observation.
A separate audit waits up to ten seconds for ADB to observe shutdown and passes.
Its run log SHA-256 is `d65d0e3108ae2fd76413a46340d8febfccf736d78df8b22b6406d9e083f843a9`.
Neither audit launches the app, changes settings or saves an emulator snapshot.
The reader accessibility failure remains retained.

The hosted layout receipts for `659046b8` retain every selected case without source drift.
Chromium passes 95 Player cases and 111 Subtitles cases, with 11 and 26 flows respectively.
Firefox and WebKit each pass 40 Player cases and 46 Subtitles cases, with the same flow counts.
Their fixture suites retain platform-specific skips separately from these populated measurements.
The Chromium artifact SHA-256 is `bb258677879920ef47959ac4130652797e610eb5feb106d70ca4a253f73ba4b3`.
The Firefox artifact SHA-256 is `7c80bef31c682b85d38337a3f1c9eb56f581cb5efe5ccb432b6b5ecb9f0602d2`.
The WebKit artifact SHA-256 is `bc4f7b25bc31990a16d2d9bf23740dd9174226a441e7bb8fc0f2274d5c688f51`.
All three record CLI SHA-256 `09076f6796f4f3be319be6d92d5d7fd3e7aa419ae09dbe58a8b7267d4fe3ca41`.
These hosted source receipts precede the fixture-only correction at `b0de9685`.

## QA-031 — Cold-resume verification snapshots an unfinished producer

Deep main run `38336225457` fails the prepared-timeline lookback control.
The missing fragment recovers with identical bytes, frames and timestamps.
Only the master playlist changes after the test records its metadata hashes.
The original encoder intentionally finalizes measured bandwidth when it finishes.
Waiting for probe wrappers alone does not wait for that publication.

A two-second delay after real encoder output reproduces this ordering failure.
The red receipt observes an unfinished original producer before the snapshot,
then a completed producer and changed master bandwidth after recovery.
Its receipt SHA-256 is `bd39dd1fde759db013b5b4e48183d4ae0f120ed0f56ad775855816124386818d`.
The retained lookback fault now includes this bounded completion delay.
The test waits for original encoder completion before recording immutable hashes.
It preserves fragment, frame, timestamp, source, metadata and logging checks.
Application publication behavior and recovery deadlines do not change.

The controlled lookback replay passes. The complete local suite passes all
19 cases and 18 recoveries against an authenticated real Server and synthetic media.
Every case verifies settled processes, correlated logs and absence of its token.
The intentional source-change case remains the only changed source.
The complete receipt SHA-256 is `05f43ddb31a579491399b6f530b85376c9092dbcfaebdc19bcd9666ff6f32504`.
The run log SHA-256 is `ca960dfa2d8604a6ef911e5e5c429e6e4468c3772d3e6501d6cc251b113d9c6f`.
This is macOS ARM64 server proof. Hosted Linux, browser and device proof remain separate.

## QA-032 — Missing deployment scanners fail late without a useful message

A real deployment builds its ARM image, then exits because the default PATH
does not contain Trivy or Syft. Nox remains unchanged during this failed attempt.
The host already has actual scanner binaries in the task's private tool folder.

Both CLI failure models and failing regressions precede the repair.
The deployment command now checks scanners before archiving or building an image.
The standalone scan command checks before creating evidence or running either tool.
Both return exit 127 with one bounded warning naming the missing scanner.
They log no host, URL, credential or PATH value.
An already healthy deployment still succeeds without rebuilding or requiring scanners.

The retained CLI controls execute real shell entry points with explicit tool stand-ins.
They prove both missing-tool failures cause no build, upload or deployment side effects.
The standalone controls also prove no scanner runs and no evidence directory appears.
Populated application E2E does not exercise host executable discovery.
Both regressions, shellcheck and the complete local tooling command pass.
The tooling log SHA-256 is `1f17cbdba843657341ab77640fd958fe2afb1da1a7cf9a9a6a15a6e7b59e3a35`.

## Main `47f250c4` publication and Nox verification

PR #579 merges at `7131f22a`; PR #588 merges at `47f250c4`.
Fetched ancestry checks prove both reviewed task commits are in remote main.
Main CI `38340092170` succeeds and promotes both apps for the exact revision
`47f250c46f9d3118942804727e140a6bb9f8fb01`.
Independent registry reads verify both manifest digests and AMD64/ARM64 entries:

- Player: `sha256:c584af225770c5de89319ec72a8eff680a4a41999c73523efd7d0044444e0e99`.
- Subtitles: `sha256:6c469075c2a112caab137f308921cfa698c7d2d15f0b3607ecccbd61f17a4032`.

Using the actual local scanner binaries, both exact-revision Nox deployments pass.
Separate local ARM images pass scans with zero fixable HIGH or CRITICAL findings.
Player's local image is `sha256:0ef2ea36e2cb84b4ad6d0c29b8519f7ec84e8c21ab6a74bf4bfeb5b253662e46`.
Subtitles' local image is `sha256:c659dc3cb39e07a1cbbeb5d3b67ddeffb5b04258d2667282d2256ffd024a9ab5`.
These image IDs are distinct from the published registry manifest digests.
Independent inspection at 04:49 UTC confirms both exact-revision containers
are running and healthy, with zero restarts. Existing rollback images remain retained.
The local save watcher remains unchanged.

Anonymous HTTPS checks require normal CA and hostname validation and TLS 1.3.
They observe root redirect 303, health 200, settings page 404 and settings API 403.
CSP is present; HSTS is absent on this private-network endpoint.
These checks establish transport and anonymous access behavior only.
Authenticated production journeys, public gateway and physical acceptance remain open.
Private `nox-main47-*` and `ghcr-main47-*` receipts retain scan and transport hashes.

PR #589 merges through the four protected checks and all three layout jobs.
Fetched ancestry proves reviewed head `ce55f771a81c8106252f34735587afce92ad57e8`
is included in remote main `3b9fb3c595b83b556ddbf2767d2443227bfbab7f`.
Final-head deep run `38349207874` remains in progress. Its result is separate
from required PR checks, prior local native tests and the deployed main47 snapshot.

## QA-033 — A concurrent preference save strands an opening native reader

A successful preference save cancels an older shared GET. The active reader
receives cancellation without leaving its pending state. A test-first native
journey holds that GET and book metadata, then saves font size 32 through the
real client HTTP interface. The original screen remains pending after metadata
is released. Before-save and failed-after-save screenshots have identical hashes.
The focused red run has eight tests: seven pass and this new journey fails.
Its source patch SHA-256 is `75a77eb81f94c4ec899193472ea920f3916d7c2352021da65c86cba21b7e003f`.
Its run log SHA-256 is `a7fe8043c03b3234a9902c8592b55d83f6976750a58b904e0ec6a7554308a41d`.

Validated preference saves now complete waiting consumers with the accepted
typed value before canceling the obsolete fetch. The existing helper also
handles accepted playback preference saves and resets. Validation and cache
persistence remain in place. Canceled consumers still check cancellation before
returning a value. An already completed request does not emit a replacement log.

The retained journey passes on iPhone and iPad. Both focused suites pass eight
tests without skips. They verify one GET, one save, a loaded 32px document,
a surviving consumer with the saved value, and the accepted cached value after
the obsolete response is released. This isolated native UI fixture covers a
shared-client boundary that populated browser E2E cannot host.

The complete iOS suite for this production repair passes 335 tests and skips one platform
control; the complete tvOS suite passes 319 and skips one. Neither suite fails.
Their source patch SHA-256 is `cb2e47eaeabf0f8206fd9e14c294f38a24c6079abfeb05e6ca2a85666acf5c61`.
The iOS summary SHA-256 is `8b92bc700cd4d6f13e9a9524dfdfbdfc6473c7f6f7ff9ba150e0c0facb6c8ae6`.
The tvOS summary SHA-256 is `88121c14ef930a545566aaf82387d0db19a703fe0e2e7a074010b47f458c6fe5`.
Actual H264/AAC fixtures execute decoded playback controls. Both temporary links
are removed after independent hash checks. All three reused simulators are
confirmed shut down afterward.

The iPad log projection validates 59 records, including one replacement and
nine cancellations. The full iOS projection validates 140 records, including
one replacement and five cancellations. The tvOS projection validates 102 records.
All use Debug level, bounded operation names and random correlation IDs.
No record has an invalid shape, and fixture tokens are absent from the iPad log.
The iPad raw-log SHA-256 is `64d72e36f553a2a9be2cefb707d2dddcb05e202b21ced6f617493b1d72842c08`.

Final-head hosted deep run `38349207874` remains failed on separate contracts.
Its iOS suite passes 333, fails the dark font-32 reader journey, and skips one.
tvOS passes 319 and skips one. The isolated AAC probe passes one test and fails
the valid extensionless AAC case at its unchanged 20-second deadline.
Its artifact SHA-256 is `083b5bd15a7214a0e1c028f98ff0d551bb8f76d4fc50d8e6d8d3fb39a145ebbb`.
The failing reader result lacks the runtime stage files needed to classify its
visible-state timeout. Both hosted failures remain open; local success does not
clear them. General catalog invalidation and physical cross-window acceptance
remain separate from this accepted preference-save repair.

The source cap rejects the expanded reader journey file before commit.
The new preference-save scenario moves into its own file and reuses existing
test helpers. The cap then passes. The focused owner suite again selects and
passes all eight tests, with zero skips. Production source is byte-identical
to both complete native runs; those runs precede only this test-file split.
The split confirmation log SHA-256 is `fc2ca2f972d72e709da5b9e0a023ac027f850f190a90ed749e1a1132a8e86bcf`.

## QA-034 — Nox adapter fixture omits the new scanner prerequisites

PR #590's hosted Player and Subtitles installer jobs fail the shared Nox adapter
fixture. Each fixture expects an unhealthy or exited target to reach its deliberate
image-build failure. It supplies no scanner executables, so the correct new
preflight rejects before the existing build assertion. The verification-plan test
is not the failing Makefile step. Its separate ignored-fixture control is excluded
from this diagnosis, and its exploratory diagnostic edits are removed before commit.

Both adapter cases reproduce locally at exact head `840a2a55`, before fixture edits.
Their red log SHA-256 values are
`f5d2b7d05c4ae8a60060af4ca03d3d84f4c92de44bd573f52b51cd178254e62d` and
`01781a7a24b9265b57db092a91ec9fef9de7e5dc9995fd3a62041034e724f7e4`.
The fixture now supplies explicit Trivy and Syft stand-ins. Neither runs because
the fixture deliberately fails its image build. All existing unhealthy-state,
no-rebuild, watcher and adapter assertions remain unchanged.

Both complete app local-pipeline test commands pass afterward. The shared
missing-scanner regression also passes, preserving rejection before side effects.
Focused shellcheck passes. This corrects test prerequisites; actual deployment
scanner discovery, vulnerability policy and image promotion checks do not change.
Fresh hosted checks remain required before integration.

PR #591 merges through all four protected checks. A fetched ancestry check proves
reviewed head `64e9db012ef1eff4f77782415fc73aee3a6c6648` is included in remote
main `a1c7a1dc8396be32ac66851f098f2f23a8018c6a`. Main3b9 CI `38349891258`
separately succeeds and promotes Player; this does not update the Nox47 evidence.

A separate plain native audio control uses the exact 384044-byte fictional WAV
from the hosted queue failure. Its requested 4800-byte tail matches that full file.
Natural playback, three 150ms tail seeks and three two-second tail seeks all pass
in macOS ARM64 and Linux ARM64 WebKit 26.6, using pinned Playwright 1.63.0.
Every case reaches the actual native ended state and 12-second clock.
Server receipts independently verify all returned WAV byte ranges and hashes.
The macOS results SHA-256 is `0a198b0b9ccd1f2dc349de896bbb93cdc81637860141a6be0ec2718b95c9abb1`.
The Linux results SHA-256 is `b2de5a5964af7a386500b0acad63ca0b91fd086d9f20357676f700240a96d7c4`.
Both the owned Server and container are confirmed stopped or removed.
An earlier missing-module setup attempt executes no media cases and is excluded.
These controls contain no Player code, authentication or queue preload. They do
not clear the hosted x64 populated queue failure or identify its root cause.

## QA-035 — Native timeout results lose useful stage evidence

The hosted dark font-32 reader failure has no media attachments. Its stage JSON
and screenshots remain in the simulator, outside the uploaded result bundle.
The failure message therefore cannot distinguish a held request from unfinished
WebKit document loading. The AAC probe also labels its initial state as asset
before its detached worker starts, hiding a possible queue delay.

Reader journeys now retain a fixed allowlist of fictional PNG and JSON files
through Swift Testing attachments. The collector rejects symbolic links,
nonregular files and files larger than 10 MiB. Reader failure context records
the operation stage and bounded request counts. The AAC probe labels queue and
asset entry separately and correlates each final outcome with a random probe ID.
Accepted probes log Info; rejected probes use the warning API and an explicit
warn field. Native OSLog reports those warning records as Error.
Media validation, completion ownership and both existing deadlines remain unchanged.

The focused reader suite passes all eight tests with zero skips. Exporting its
result retains 49 attachments: 31 PNGs and 18 JSON files. Every exported byte hash
matches the original captured files; the largest is 510545 bytes.
The export manifest SHA-256 is
`ce1b52fdbdec13b1ad0934e7663f932711d77e58671c47ea52fe329286b00517`.
The focused AAC suite selects and passes both tests. Its strict log projection
validates seven unique probe records: six rejections and one accepted decode.
All outcomes match their semantic log levels, with zero invalid record shapes.
The raw-log SHA-256 is `d1b82fadefa009576e8f0bd43c7a7ddb0a9f79ee4f870670ed2949b691b64ece`.

A test-owned held response deliberately fails only the dark font-32 journey.
The suite retains seven passes and one expected failure, without changing the
ten-second readiness limit. The failed test exports 25 attachments, including
stage loaded-document, one held reader request and no held preference request.
The stage JSON SHA-256 is `b62229e96cebbe546b765b8080888dc765b44019909ab9df65dec6acad3c4993`.
Xcode's failure-only attachment filter selects none of these regular Swift
Testing attachments. Exporting the failed test by its identifier retains them.
The workflow already uploads the complete result bundle.

A separate native E2E control selects exactly the preference-save test. After
its normal assertions, the fixture adds an unknown JSON file, a linked PNG and
a JSON file of 10485761 bytes. Both guards record issues. The exported result
retains only its three normal artifacts and contains no sentinel payload.
Both temporary source injections are restored byte-for-byte. The reused phone
simulator is shut down after the failure control. These are artifact controls,
not newly reproduced product failures.

The normal reader and AAC source patch SHA-256 is
`23a4da345bceb265496e0b3ad087dc585fdad7da05ddafde7fdb196c5c6f3f86`.
These local results do not clear the earlier hosted reader or AAC timeouts.
Fresh hosted execution must classify them using the retained evidence.

PR #590 separately merges at `5cf9ba061dd52c5d3f811d2cdc85b0d367d948e3`.
Fetched ancestry proves its reviewed head `01d6cf33f90cfa0ba618ff5173d212d9c9497e08`
is included. The hosted x64 cold-suffix suite at `840a2a55` passes all 19 cases
and 18 recoveries, with matching current helper hashes and correlated safe logs.
Its receipt SHA-256 is `da6cc2730c41e4e639a6714d849f0fe9eee51bab15aa8cec3951fb0cd72647c1`.
CodeQL's retried JavaScript upload still reports a missing task ref while a fresh
Git read finds that exact ref and expected head. That CI service boundary remains
open; the other four retried CodeQL languages pass.
