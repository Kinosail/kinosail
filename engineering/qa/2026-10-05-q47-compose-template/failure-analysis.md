# Q47 Compose template recovery: test-first source checkpoint

This is preparation, not runtime proof. The current helper's source has no
deadline or abort signal for template fetch or response.text(). A successful
request that never completes can leave Preparing file visible indefinitely.
The canonical audit calls this a feature opportunity. No incident, real account,
entered user path, installed service, or public deployment is claimed.

## Source boundary and preserved authority

The read-only baseline is Kinosail/kinosail main
a57dfc29de798d15f1c6094b5039f8e20f361062. GitHub GET, RAM source preparation, and
the six authorized unreferenced source blobs are the only new operations.
No local files, objects, refs, dependencies, builds, collection, browser, UI,
Compose commands, cleanup, or deployment are run.

Selected authoritative Git blobs at that revision:

- apps/player/docs/assets/js/platform-install.js: e778290caacb567cc67384fda07a4f2b40272833
- apps/player/docs/getting-started/platforms.md: 80695914c8ccf3993a4a2b9df8fdd7af5e154877
- apps/player/packaging/platform-compose.yaml: 4a5fbc2f894130790943da9f003cbf583c119c19
- apps/subtitles/packaging/platform-compose.yaml: 8680c29d2e00a9eab3f0040191edb088ce30f66c
- apps/player/packaging/platform-compose-both.yaml: 90937e3dc3320943b9d08642d78303ee13c952db
- engineering/documentation/build.py: c082c9cad2fd630c4c381c15878e8f7217d905bd
- engineering/documentation/test-install-builder.cjs: 9c0664f9b6ac2648cb162b591ef7824b682e38af
- scripts/tooling/test-platform-install-kits.py: 6a9adaa557b2964e0bb08630399e2679ffe4bdb7
- engineering/documentation/e2e-install-builder.cjs: dbacf5fcda661acca92abace00c4207e2eb1ba93

The generated site copies the three packaging templates byte for byte. The new
fixture reads those published copies and serves the generated Jekyll HTML,
helper, CSS, scripts, fonts, and other public assets directly. It does not replace
HTML, patch the helper, intercept browser fetch, substitute JSDOM, or mock a
Playwright route. Only fixed template transport routes receive controlled faults.

The eight existing test-install-builder.cjs controls remain byte-identical.
They protect generation, privacy/copy, app switching, both-app isolation,
validation, altered templates, Blob revocation, and ignored obsolete responses.
They use JSDOM, mocked fetch, and mocked Blob URLs. The existing named E2E script
also uses JSDOM and mocked fetch on real built files, then Compose config.
These are isolated transformations/Compose validation, not native HTTP proof.
The new tests supplement them and do not authorize changing their assertions.

Repository AGENTS.md, Player AGENTS.md, and Player DESIGN.md were read at the
baseline. Impeccable is not available in this session. No product presentation
change or responsive visual acceptance is claimed at this source checkpoint.

## Failure analysis before implementation

1. Delayed headers never settle fetch. A fetch-only timeout must cancel the peer
   and expose recovery without changing inputs.
2. Headers arrive but the body stalls. The same attempt deadline must include
   response.text(), not reset when headers arrive.
3. Loss or a failed HTTP response settles quickly. The user still needs Retry,
   original links, and retained input; a new attempt must use a new transport.
4. Reset/input change or app change supersedes a held request. Old transport must
   close; a late result must not publish or clear the newer file.
5. A deadline, error, or edit can race with a resolving body. Native cancellation
   alone cannot prove that a fetch implementation ignoring AbortSignal cannot
   later publish. The existing obsolete-response isolated control remains;
   proposed append-only fake-clock/race controls require separate content review.
6. Retry could reuse an aborted signal, leave the action disabled, retain a stale
   timer, lose an entered port, or revoke the new Blob. Native recovered file and
   clipboard equality guard the full generated output.
7. Validation could change while adding recovery. Invalid paths/ports must still
   fail before a template request or Blob. App selection intentionally resets its
   default ports; Retry and ordinary input edits retain entered values.
8. Diagnostics could leak a media path, generated YAML, cookie, token, URL, or
   exception text. The proof emits fixed enums, bounded counts, equality flags,
   and digests only. Unknown observations are rejected before serialization.
9. Collection, Jekyll output, helper delivery, clipboard capability, or fixture
   startup can fail. Such failures are prerequisites or unclassified; they are
   not accepted product REDs. Timeouts and missing cases cannot certify GREEN.

Current same-origin acceptance validates the initial URL and expected template
pathname. Fetch follows redirects; the final response origin is not separately
checked. The template size check occurs after response.text(), so it is not a
streaming allocation bound. These are separate source boundaries, not proven
incidents. This Q47 checkpoint does not relax them, claim them fixed, or expand
account/security scope. Any additional security repair needs its own proof and
scope decision.

## New owned source package

- apps/player/e2e/compose-template-fixture.go
- apps/player/e2e/compose-template-recovery.journey.ts
- apps/player/e2e/compose-template-recovery.config.ts
- apps/player/e2e/compose-template-proof-reporter.ts
- apps/player/e2e/compose-template-recovery.observation.ts
- engineering/qa/2026-10-05-q47-compose-template/failure-analysis.md

The .journey.ts suffix prevents normal Player .spec.ts discovery from changing.
The standalone fixture uses the q47proof build tag, so ordinary product package
tests/coverage do not discover a new executable package or run its environment
guard. The explicit proof build must request that tag. No existing test is skipped.
The unchanged check-ts-types.mjs gate rejects explicit AnyKeyword/UnknownKeyword.
New validators use named recursive JSON values and concrete projected output
types; no forbidden keyword type is introduced. Its AST execution remains pending.
Root owns integration, driver, result admission, workflows, required gates, and
publication. Unreferenced blobs are not commits or a runnable candidate branch.

The stdlib Go peer opens the supplied generated site beneath RUNNER_TEMP through
os.Root. Template reads are bounded to 16 KiB and use controlled relative names.
The loopback listener is fixed; no public listener or original source deployment
is used. Controller input has a custom header, 512-byte maximum, exact JSON
fields, EOF check, and fixed operation/mode/app enums. Browser witnesses never
return request values or filesystem paths. No user data is created or removed.

Fixed fixture modes are valid, headers, body, loss, late_headers, late_body,
and http_error. Each hold expires after 25 seconds, beyond the asserted
20-second deadline. Body mode flushes 32 authentic template bytes with the
original full Content-Length. Headers mode sends no response bytes before
release. Loss aborts the actual native connection; http_error returns 503.
Loss/http_error remain faults until recover to avoid treating a transparent
browser GET retry as user recovery. Start rejects active prior handlers.
The peer records actual request cancellation, completion, expiry, presence-only
privacy checks, and the authoritative template hash.

The server has bounded HTTP headers and timeouts, a 240-second emergency
lifetime, and five-second graceful shutdown followed by Close. Root's driver
must impose the shorter per-command process-group bound and settle every owned
browser/server process. The emergency lifetime does not extend the proof budget.

## Fixed test selection and evidence

primary, two cases:
- Q47 headers deadline phone Player
- Q47 body deadline desktop Both

recovery, two cases:
- Q47 lost request recovers with retained inputs
- Q47 failed HTTP recovers with retained inputs

supersession, two cases:
- Q47 input change discards late headers
- Q47 app change discards late body

contracts, four cases:
- Q47 Player preserves Compose download and copy
- Q47 Subtitles preserves Compose download and copy
- Q47 Both preserves Compose download and copy
- Q47 rejects invalid inputs before template requests

Primary uses the trusted Save/Generate click's browser performance.now timestamp.
A capture listener installs an absolute 20-second timer before the product handler;
no request, witness, attachment, or RPC await restarts that budget. Mutation
observation records actual first enabled and first settled Retry/error times after
observed Preparing/disabled state. The absolute snapshot must occur at 20,000 to
20,200 ms; later wake-up, missing click, or missing pending state is ineligible.
The remaining-budget RPC wait never adds another 20.2 seconds. The 200 ms is
observation allowance only. Both first enabled and first settled recovery must be
observed at <=20,000 ms; recovery first seen during grace cannot pass that contract.
Timer/RPC scheduling is measured, not inferred from the final enabled snapshot.
All 58 original expectation expressions, labels, conditions, and loops remain;
three additional hard timing guards supplement them. Hard failure still stops
the case. The afterEach ledger records attempted/completed/pass-or-null/count for
each fixed ID. No reached-only snapshot is fabricated for later stages.
On unchanged production, missing Retry may establish only the original deadline
failure after delivery, template, pending UI, held peer, and clock prerequisites.
Cancellation, Retry, fallback, Blob/copy, and focus stay null/unattempted if the
deadline assertion stops first. A known failed case is not recovered proof.
After that boundary, the original tests require retained app/path/ports, peer
cancellation, a new request, and exact native Blob/clipboard bytes. Phone uses
pointer Retry; desktop uses native Tab focus and Enter. Primary repeats twice.

Supersession covers input change during delayed headers and app change during
delayed body. Native peer cancellation prevents old transport completion.
It does not by itself prove late fulfillment from a transport ignoring abort.
The prepared isolated addition protects that distinct gap but is not staged.

The reporter's exact top-level schema is
{schemaVersion:2,campaign:"Q47",phase,suite,status,collected,cases,errors,runnerErrorCount}.
phase is collection or journey; collection cannot contain executed cases.
Suite is one of the four fixed selections. Status is a Playwright terminal enum.
Case records contain fixed name/status/retry/duration, typed observations and
ledger, fixed outcome/failure/disposition enums, failed/unattempted/incomplete IDs,
totalErrorCount, knownAssertionErrorIDs, unknownErrorCount, assertionErrorsExact.
Reporter printsToStdio is false.
It writes only a mandatory private JSON file, never runner streams or diagnostics.
KINOSAIL_Q47_REPORT_FILE must name q47-proof.json beneath canonical RUNNER_TEMP;
its existing parent must have private permissions. Creation is exclusive with
mode 0600, serialized output is bounded to 256 KiB, and failures set exitCode 2.
Only inline application/json attachments are accepted, at most 64 KiB each.
Unknown/duplicate stages, keys, types, titles, or excessive arrays cause a fixed
error code. No reporter file attachment or arbitrary process JSON is admitted.

Fixed stages are q47-asset, q47-started, q47-pending, q47-deadline, q47-failed,
q47-recovered, q47-stale, q47-rejected, plus terminal q47-assertions ledger.
Asset is {bytes,sha256,sourceMatches}.
Other observations are {state,peer}. State contains action, ten booleans for
pending/output/error/fallback/input retention, nullable Blob/copy equality and
Blob digest/byte count, elapsedMs, and responses. Peer contains fixed mode/app,
templateSHA256, eight bounded counts, and five presence-only booleans.
The synthetic loopback cookie marker is non-authentication fixture data.
Template requests must omit it, authorization, request body, and query.
No entered values are returned.

The helper registers no tests. It wraps only genuine Playwright failed matcher
results with the exact original custom label; acquisition/transport exceptions
and nested wrapper failures remain incomplete, not relabeled product failures.
The safe reporter matches exact fixed ID plus the helper's owned file/line/column
(128:11) and strips only ANSI/optional Error: prefix. It counts every case error
and compares the complete known-ID multiset with completed failed ledger IDs.
No first-substring shortcut or singleton assumption is used. Extra/unknown errors,
retry, interruption, runner errors, malformed data, or missing terminal ledger
produce incomplete/unclassified outcomes. Unknown raw messages/stacks are never
exported. Source callsite and typed matcher shape need actual hosted verification;
a mismatch is a prerequisite block, not permission to relax admission.
The clock ledger contains elapsedMs, trustedClick, clicks, pendingObserved,
firstEnabledMs, firstRecoveryMs, and the bounded deadline sample. Nullable times
remain null when unobserved; dispositions are within-product-deadline,
observation-grace, no-recovery-observed, ineligible, or not-applicable.

The first five unreferenced blobs are historical source preparation, superseded
by later manifests. The previously reviewed five-blob package is also preserved
as a historical source-only boundary; this six-file repair supersedes its timing
and first-substring classification. No prior package has runtime acceptance. They introduced forbidden keyword types and a stdout
projection before the gate/output review arrived. No referenced candidate or
runtime used those bytes. The final package changes only the authorized files.

Root must validate fixed mode-specific stages and actual assertions, zero skips,
retries, flaky cases or global errors, no empty GREEN, source equality, binary and
site bindings, and complete process settlement. Missing prerequisites or unknown
failures remain unclassified. A collection PASS certifies selection only.

Only four final JSON files may be published under
.verification/campaign-proof/Q47/:
receipt.json, results.json, source-manifest.json, artifact-manifest.json.
The root driver must capture runner stdout/stderr only in bounded private RAM
and discard them, never upload them. The reporter's private file is also excluded
from upload until root validates its projection into the fixed results schema. Receipt uses revision/tree/clean/GITHUB_SHA equality, fixed tool/version
fields, terminal command classes, exit/timeout/settlement and byte/digest bounds.
Source manifest uses fixed repository paths and source digests plus generated
guide/helper/CSS/template and compiled-binary digests. Results contains only the
validated reporter projection. Artifact manifest binds the other three JSON
bytes. No screenshot, trace, video, raw URL, path input, YAML, or log upload is
part of this initial source package.

## Dependency and test-first order

1. Independently review the six unreferenced blobs, fixed vectors, and driver
   admission before root creates a candidate commit. Preserve the baseline helper,
   markdown, packaging templates, and all eight isolated controls.
2. Hosted-only prerequisites reuse current locks: Ruby 3.4/Jekyll 4.4.1/Bundler,
   Node 26, docs npm ci --ignore-scripts, pnpm 11.22 frozen Player E2E dependencies,
   Playwright 1.63.0 Chromium, and the repository Go toolchain. No ffmpeg, service
   install, image pull, production account, or GUI is required.
3. Use the actual current build command:
   python3 engineering/documentation/build.py --output "$RUNNER_TEMP/q47-site"
   The directory must be new and outside the checkout, with empty baseurl.
   Keep current npm test, Python documentation checks, check.py, and Compose
   config gates intact; they remain separate required checks.
4. Build only the standalone Go fixture into RUNNER_TEMP under a separate bounded
   prerequisite command, explicitly with -tags=q47proof; capture compiled binary
   and selected source digests. No normal product coverage exclusions are changed.
   Bind generated guide, helper, CSS, and all three published template digests.
5. Proposed direct collection command from repository root:
   KINOSAIL_Q47_COLLECTION_ONLY=1 KINOSAIL_Q47_SUITE=primary node
   apps/player/e2e/node_modules/@playwright/test/cli.js test
   --config=apps/player/e2e/compose-template-recovery.config.ts --list
   Root must join the command lines as arguments, not interpret prose as shell.
   Before collection, root creates a new 0700 private run directory beneath
   RUNNER_TEMP and supplies KINOSAIL_Q47_REPORT_FILE ending in q47-proof.json.
   Collection and runtime use distinct directories/files; existing files cause
   exclusive creation failure. Root reads only that fixed bounded private output.
   Proposed collection bound is 15 seconds plus at most five seconds settlement.
6. Start the compiled fixture with KINOSAIL_Q47_SITE pointing to that actual site.
   Run the same direct CLI without --list and with COLLECTION_ONLY unset.
   Use one headless Chromium worker, zero retries, trace/video/screenshot off,
   unique private output directories, and primary only. Repeat twice.
   Config's 70-second global browser bound starts at Playwright execution.
   Proposed external 75-second bound includes launch/runtime; it does not include
   the separately bounded dependency, Jekyll, or Go-build prerequisites.
   Root must declare each prerequisite bound and a five-second settlement bound.
7. Admit completed primary results and the exact unchanged production/built-site
   source. Only an accepted public RED permits the minimal helper/markdown repair.
   Recovery, supersession, and contracts each get a separate fixed-suite dispatch;
   do not expand deadlines or silently count unselected cases.
8. After the repair, run both primary repeats, the supplementary suites, relevant
   unchanged isolated/Compose controls, static/lint gates, responsive/accessibility
   checks, and normal protected exact-head gates. Root owns merge and ancestry.

## Remaining limits

Everything here is source-only. No Go compilation, TypeScript collection,
Jekyll generation, native peer response, browser timer, cancellation, Retry,
Blob/clipboard integrity, focus, or final artifact admission has executed.
No claim is made about WebKit/Firefox, native clients, full responsive geometry,
OS-saved download bytes, original-link network availability, deployed docs,
streaming allocation bounds, redirects, service startup, or installer behavior.
The browser reads native generated Blob bytes but does not save a download.
Original hrefs are checked against canonical public sources without following
them. Further visual and keyboard coverage must be admitted separately.
R06 has the next serial hosted slot; Q47 runtime dispatch remains pending.
