# Non-key copied-video resume proof

Base: `c9ffb537bfd6be50847ff6af2e3f10b2317eec22`, after delivered PR500.

The preserved public MKV and copied-MP4 cases request 12.5 seconds. Both
return 480 frames instead of the independent 468-frame reference window.
They retain a preceding GOP, advertise 19.999 seconds instead of 19.5,
and fail the existing complete-frame and duration assertions. The encoder
receives `input_seek_ms=12500`. These are not incorrectly submitted requests.

Before changing production, extend the existing hosted public proof with:

1. A complete source-to-public frame map, independent decoded source PTS,
   source/public video and audio packet PTS/DTS, and initialization edit metadata.
2. A small fixed set of offline copied-video mux experiments using the same
   source bytes and pinned hosted FFmpeg. Compare their complete decode against
   the output-side reference seek. Preserve every decoded frame and every failure.
3. Exact source/helper/receipt checksums, source snapshots, worker joins and
   public initialization stability through the existing authenticated Server.

The offline experiments diagnose mux behavior; they cannot admit a production
repair. The manual twelve-case proof remains strict. The six required audio,
four required HEVC, nine timeline, five cache and three source-version cases
retain their assertions. No frame is discarded or reordered by hash matching.
Negative timestamps remain unqualified without an independently verified
presentation/discard map and a real decoder that presents the requested window.

Relevant failures to distinguish: incorrect source origin; preceding-GOP
presentation; missing requested frames; duplicate/interior frames; reordered
PTS/DTS; incomplete AAC or excessive interior/adjacent gaps; wrong AV offset;
missing or unsupported edits; mutable initialization; stale cache reuse; source
mutation; overlapping or unjoined producers; and a parser accepting malformed
or unbounded metadata. An offline pass alone does not establish Server, browser,
Safari, iOS or live-Nox acceptance.

Public generated initialization cannot inject malformed box lengths, duplicate
track metadata or unsupported edit-list versions. Small parser checks will
cover those concrete evidence-integrity gaps before the parser is implemented.
They must reject oversized, truncated, illegal nested layouts, duplicate and invalid-clock
inputs without attempting subprocess or filesystem work.

The diagnostic run retains each packet as a compact numeric JSON row. Missing
demux timestamps remain null; no clock is inferred. Frame correspondence uses
the actual format origin plus 12.5 seconds and the independent 468-frame window.
It records all source and public frames, including the preceding GOP.

The initial five offline variants are the existing HLS mux, negative timestamps,
explicit HLS edits, delayed fragmented MP4 edits, and copied timestamps with
explicit HLS edits. Each copies video and audio. The loop has a 120-second
deadline per source and bounded subprocesses and media reads. A timed-out or
rejected variant remains failed evidence. The manual workflow still runs all
twelve authenticated public cases and preserves only receipts and checksums.

No production scanner, compatibility policy, credentials, source media,
simulator or deployment changes are authorized by this proof itself.

## Next diagnostic admission

Exact `e271b3d5` run `37545412532` retains the two public failures and ten
passing controls. Both failures contain source indices 288 through 767.
The requested window contains indices 300 through 767. No requested frame is
missing, duplicated or unknown. Every offline variant still decodes 480 frames.
Negative-clock variants expose the preceding twelve frames; that fact alone
does not qualify presentation. Explicit HLS and delayed MP4 edits differ.

Before any production change, retain the first complete raw fragment sample
times and an explicit output-zero decoder experiment. Compare media and movie
clocks and track IDs with the initialization edits and independent source PTS.
Test the mux without initial fragment discontinuity, keeping prior variant
counterevidence. A decoder experiment must preserve its whole output and command.
It cannot replace the strict public result or justify dropping frames by hash.

Generated fragments cannot inject malformed headers, unsupported versions,
duplicate track clocks, missing sample duration, overflowing sample counts or
invalid signed composition offsets. Write bounded parser checks for those
evidence-integrity failures before the raw-sample parser. Record unsigned decode
time as stored; never infer a signed or wrapped clock from an unsigned field.

The next fixed five variants retain the original HLS, negative HLS, and delayed
MP4 controls. They replace redundant copied-clock/explicit-discontinuous cases
with negative and automatic-clock HLS edits without initial `frag_discont`.
Keep the initial run immutable. Compact numeric JSON rows retain every sample,
source frame, reference frame and decoder output within the 4 MiB receipt bound.

## Actual public renderer admission

Run Chromium against the disposable authenticated Server's real watch pages.
Do not mock media, metadata, progress, Hls.js, decoder methods or Server responses.
Copy the generated source packets into a browser-compatible reference container;
verify both packet identities before starting the Server. The full reference must
present all 768 source frames. Bind its ordered browser pixel hashes and clocks
to the independent source PTS. Compare the complete resumed browser output with
the independently requested 468-frame window, without trimming either output.

Install the callback before the media element loads. Retain every callback's
clock, presented-frame counter and SHA256 of its complete native-resolution RGBA
pixels. Slow playback to 0.25 only to avoid observation loss. Missing callbacks,
duplicate clocks, skipped counters, ambiguous reference hashes, malformed rows,
decoder errors, unexpected rendition requests, missing tail or an incomplete
reference make presentation unqualified. Keep partial safe evidence on timeout.
Use only synthetic credentials through private subprocess stdin, never receipts,
command arguments, logs or artifacts. Bound each browser journey and receipt.

The existing twelve-case raw proof remains strict. Add separate browser facts;
an observed browser pass cannot erase its raw failures or waive AAC/AV clocks.
Compact complete diagnostic frame-map rows with an explicit schema to make room
inside the existing 4 MiB safe archive bound. Preserve earlier receipts unchanged.
The actual public baseline must run before any production flags or clock change.
Safari, iOS, simulator and Nox remain separate acceptance boundaries.
The public adapter's native movie clock must match independent source PTS within
1 ms for every mapped displayed frame. Pixel identity cannot hide a wrong clock.
Retain buffered ranges, native/reported times and all bounded lifecycle events.

Independent review found that a Python timeout killing only Node could leave
Chromium descendants running. Before hosting, cover timeout and ordinary leader
exit with a retained descendant, and prove an unrelated process survives. Run
the renderer in its own process session. Bound terminate, kill and join checks
for that process group only. Retain timeout evidence and require zero live owned
processes across repeated samples. Never use a global browser/process cleanup.

## Native frame observation repair, before implementation

Exact `61973443` preserves ten passing controls and both raw failures. Both
browser references omit source frame 2 and report one dropped frame. All 480
public RGBA hashes are unknown against those incomplete references. Process
ownership and planned media delivery pass; browser content stays unqualified.

Use the shipped Play button and Chromium's user-gesture autoplay policy. Keep
the application's autoplay wiring intact. Record two paused native-clock samples
before the real click, with at most one preloaded callback and no advancement.
Require all 768 reference frames and zero drops; do not fill missing callbacks.

Construct each native VideoFrame synchronously in its callback, without supplying
a timestamp. Retain its timestamp, visible/coded/display geometry, orientation
and color metadata. Bind the native timestamp to callback mediaTime within 1 ms.
Copy only its native I420 or NV12 format and complete visible rectangle. Strip
stride padding and deinterleave NV12 into Y/U/V bytes without color conversion.
Native-plane identity establishes frame correspondence, not RGB display color
equivalence. Retain color differences explicitly. Unsupported formats stay
unqualified, with complete partial callback evidence and no Canvas fallback.

Before the packer, write isolated checks for concrete gaps generated valid public
media cannot inject: padding/offset errors, U/V reversal, truncated or overlapping
planes, unsafe dimensions/strides and unsupported formats. Fixed hand-authored
bytes must establish equivalence across I420/NV12 and sensitivity to every plane.
Bound each allocation and pending copy, retain original callback ordering, and
close every frame on success or failure. Add pre-implementation admission checks
for wrong native timestamps, metadata and autoplay advancement. Preserve `6197`
and every prior receipt; this remains a diagnostic test change before production.

Exact `a038ad22` still retains both raw failures and ten passing controls. Native
frame copies and timestamp binding work. The browser result stays unqualified:
autoplay advanced, and the custom Play overlay was hidden in native-controls mode.
Partial callback evidence remains intact. No production change is admitted.

Before the next diagnostic, select Chromium's document-user-activation policy
and disable its media-engagement autoplay exemptions. Use the visible shipped
Play overlay, or the shipped player-region Space handler for native controls.
Never force-click a hidden button or call play through evaluate. Record unmuted,
positive-volume, paused clock samples before the gesture. Set the real default
playback rate to 0.25 so MediaSource loading does not reset observation to 1.
Retain the real playback rate in every callback and reject rate changes from
the declared observation speed. Test omitted/wrong rate and muted startup
admission before extending the observer. This is still a diagnostic baseline.

Pinned Playwright inspection uses Chromium Runtime calls with userGesture=true.
Use an explicitly non-gesture CDP read before playback instead. Retain both
native userActivation flags in each pre-gesture sample and require false. Focus
the shipped player region through non-gesture CDP, then send trusted Space key
down/up through CDP Input. No ordinary Playwright evaluate or locator inspection
may precede that gesture. Preserve this as a keyboard-start observation boundary.

## Renderer scheduling diagnostic, before implementation

Exact `d98e54d2` completes both actual watch journeys with verified paused,
unactivated, unmuted startup and a trusted Space event. Every native copy binds
to its callback clock; all public native-plane hashes exist in the reference.
Both references retain 767 callbacks and omit source frame 2. Both public
journeys retain 479 callbacks. Their quality counters report one dropped frame.
All ten controls pass and both strict raw 480-versus-468 failures remain.
Receipt `67b3b5e8` is RED for complete browser observation, not a presentation
certificate. Do not infer the sole cause from the identical startup omission.

Test a continuous requestAnimationFrame scheduling pulse in the observer.
It must leave the DOM, media methods, clocks, source bytes and app adapter intact.
Record its callback count, maximum interval and stop state as diagnostic facts.
Bound it by the existing journey deadline; cancel on media end/error and page
close. This tests the scheduling hypothesis described in Chromium's
[rVFC issue](https://issues.chromium.org/issues/40902598); it does not establish
that hypothesis as the cause of this run's dropped frame.

Keep the complete 768-frame reference, zero-drop quality counter, every raw
callback, native timestamp binding and exact-source-clock gates unchanged.
Do not fill, replay, trim, seek to or filter a missing frame. A continued drop
must remain unqualified. The existing actual public RED supplies the test-first
failure; no isolated implementation-mirroring check is needed for this pulse.
No production or simulator change is authorized by a scheduler-only result.

## Optional installation counterfactual, before implementation

Before changing production, test the normal negative-edit mux through the
supported FFmpeg executable configuration in a fresh disposable Server.
Enable it only through a fixed manual HLS choice. Keep the default twelve-case
proof and required audio/HEVC gates unchanged. Reject unknown or foreign modes
before tool setup. Label the receipt as a counterfactual installation.

Match only the owned H264 fixture snapshot, one input seek at 12.5, copied
video/audio, initial segment zero and the exact existing HLS options. Keep
unrelated invocations unchanged. Replace only initial fragment discontinuity
with normal edit-list muxing, and add disabled negative timestamp avoidance in
the output group. Hash the real executable and generated wrapper. Preserve
bounded private original/effective arguments and transformation counts; publish
only their digest and bounded classifications. Close descriptors before exec.

First test selector near misses, duplicate options, escaped output paths,
source mutation, file bounds and symlinked private receipts. Valid public media
cannot inject these trust-boundary faults. Preserve PID/cancellation ownership
through real exec, with public worker/join evidence. Retain every raw negative
frame and strict failure; neither edits nor an output-zero decode may admit it.
Full browser correspondence remains required. Incorrect playlist timing is
expected counterevidence, not grounds to adjust the projection in this test.
Production still needs scoped cache identity, AAC presentation, interrupted
reopen and lazy refill compatible with the published init.

Independent review found that audit verification and parsing used two opens.
Before repairing it, test rejection of an unchecked second path read. Read,
hash and parse one bounded no-follow regular-file descriptor, and revalidate
its identity after reading. Generated public media cannot race this private
artifact boundary. Keep exact transformation-count validation unchanged.
Also test a FIFO without a peer and a held audit lock before repairing either
wait. Use nonblocking opens before regular-file validation. Bound lock acquisition
to 250 ms and preserve the audit on failure. The focused child tests use only
their own disposable paths and processes, with a one-second outer kill/join.

## Normal-speed renderer observation, before implementation

The quarter-speed `d98` and `bd1` watch journeys each omit source frame 2 in
the complete reference, with one quality-counter drop. The bounded animation
pulse did not resolve this. Keep both receipts unqualified and unchanged.
The optional negative-edit installation run is also retained independently.

Observe the shipped watch pages at a real fixed playback rate of 1. Require
the declared rate in both paused pre-gesture samples and every callback.
The rate is a diagnostic input, not a broader admission interval. Preserve
the full 768-frame reference, zero dropped/corrupt frames, native timestamps,
every raw callback, trusted activation, source identity, clocks, network and
owned-process checks. Do not seek, trim, fill, replay or omit any frame.
Keep copied packets, all twelve public cases and strict raw failures unchanged.

Before changing the observer or verifier, require fixed 1 in the retained
integrity fixtures and add a source-contract test for that fixed rate. These
isolated checks cover malformed observation evidence that valid generated
media cannot inject. A successful helper check cannot qualify runtime pixels.
Normal-speed runtime still needs exact-source independent review. A repeated
drop leaves the complete browser oracle blocked; it does not authorize mux,
cache, scanner, client, simulator or production changes.

## Exact installation seek text, before repair

Run `37555634930` failed the one-transformation audit for both non-key cases.
Its raw output and quarter-speed browser rows match the unchanged baseline.
It cannot establish the negative-edit counterfactual. The Server calls
`playback.FFmpegSeconds`, which formats its seek with three decimal places.
The helper mistakenly required `12.5`, rather than the exact `12.500` argv.

Before repairing the matcher, change its independently authored argv fixture
to the source-derived `12.500` form. Require `12.5` and every other near miss
to remain unchanged. Add a contract binding the fixture to the Server's
formatter and call site. Retain the full exact argv comparison and exactly
one audited transformation. Preserve the failed attempt; do not classify its
unmodified output as a mux experiment. Re-run the corrected fixed1 experiment
separately after the normal-speed baseline and exact-source review.

Before changing failed-audit diagnostics, test that validated zero/two-transform
audits retain their hash and bounded invocation/transformation counts while
still failing acceptance. Publish no arguments or private path. Invalid audit
shapes must continue to fail before counts are added. The public fixture cannot
inject a duplicated private transformation audit; this is an isolated evidence
integrity gap. Keep exactly-one acceptance unchanged.

## Corrected mux runtime follow-up, before implementation

Corrected run `37557288711` applies exactly one transform per non-key case.
Both references retain all 768 frames. Both public callback sequences retain
exactly source frames 300 through 767, without preceding, missing or unknown
frames. The public quality counter reports 469 total frames for 468 callbacks.
Keep that mismatch failed; do not infer what the additional frame contained.
The raw negative decoder rows and incorrect playlist durations remain failed.

Record bounded quality checkpoints at existing startup events, the first three
callbacks and the two paused pre-gesture samples. Include native time and row
count. Add at most eight explicitly labeled native-frame snapshots from these
events, using the same lossless packer and resource bounds. Event snapshots are
diagnostics, not rVFC callbacks; never add them to the callback sequence or use
them to waive the counter gate. Preserve timestamp and descriptor evidence.
This actual failed public receipt is the pre-implementation E2E test.

Independent decimal audit finds the maximum MKV source-clock difference is
exactly 1 ms. Binary subtraction exceeds that limit by about 1e-15 seconds for
seven rows. Before changing arithmetic, test exact retained decimal timestamps
at 1 ms and above 1 ms. Compare their decimal values; retain the same limit.
Do not loosen the threshold or the quality-count requirement.

## Direct seek control, before implementation

Run `37558838175` retains all requested source frames 300–767 but fails the
unchanged quality-count guard: 469 decoded ready frames versus 468 callbacks.
Pinned Chromium 153 source increments this counter before display. Startup
queue reset is a hypothesis; event snapshots identify only frames 300–302.

Add a separate copied MP4 reference item with verified saved progress 12.5.
Keep the original full 768-frame Direct reference and HLS journey unchanged.
Observe the extra shipped `direct=1` watch journey serially in the same owned
browser, with the same trusted gesture, fixed rate, deadlines and native packer.
Retain every callback, checkpoint and raw counter. Compare against independently
expected source PTS and the full reference. Neither a matching nor a different
counter total waives an existing assertion or identifies hidden queued frames.

Verify actual Direct media delivery against the immutable copied source bytes.
Bound response count, size and range metadata. Reject failed or foreign media,
malformed ranges, wrong lengths and hashes. Source snapshots must remain equal.
Before implementing this evidence verifier, test these failures in isolation:
the public success journey cannot inject a forged response receipt. Keep all
raw non-key failures and existing scanner, packet and quality guards unchanged.
Run media and browser work only on the existing bounded hosted workflow.

## Paired playlist and marked AAC proof, before implementation

Run `37561397308` proves that ordinary Direct seeking also has unequal decoded
and callback counts. Keep both equality failures and all callbacks unchanged.
MKV's identical media acquired a different final advertised cut. The initial
physical manifest and worker state were not retained; do not certify that cut.

Add a closed manual `negative-edit-paced` option. It applies the existing
exact-argv transform plus fixed input `-readrate 2` only to the matched initial
12.500-second copied H264 invocation. All other commands remain unchanged.
Retain real exec, PID/cancellation ownership, audit and source bounds. Pacing
is a disposable diagnostic input, never a production option or repair.

Bracket the first public variant GET with bounded physical-manifest and worker
snapshots. Retain validated playlist bytes, cuts, ENDLIST, target duration and
cache generation identity. Reject symlinks, special files, oversized or foreign
URI data. Mark changed or racing snapshots unqualified. After three stable
zero-worker/ENDLIST samples, repeat the same public GET and check every delivered
init/fragment hash against the first measurement. Do not reconstruct missing
state or adjust either playlist. TARGETDURATION must cover rounded EXTINF cuts
as specified in RFC 8216 section 4.3.3.1.

Generate a separate non-key fixture with AAC frequency changes every four
seconds; preserve the regular controls and copied video/audio identities.
Decode source and delivered audio independently. Compare dense half-second
windows at 50 ms intervals around source 16/20/24/28, plus start and tail windows,
using the existing 10 Hz content limit. Retain sample counts and every window.
This is offline content/timing evidence; browser/native audible proof and
per-track priming certification remain separate. Sample floating-point centers
by rounded sample index; test this important previously unsupported input first.

Before implementing the new integrity helper, isolate failures that successful
public media cannot inject: invalid/foreign playlist bytes, malformed target
duration, symlink/FIFO/oversized/racing snapshots, missing audio windows, silence
and shifted content. Preserve old scanner, negative-frame and packet assertions.

## Paired oracle repair, before implementation

Independent review of `541a7a46` found four admission gaps. First add failing
checks for conflicting or repeated scalar headers, unsupported generated EOF
placement, absent or nonnumeric frequency facts, changed joined generations,
invalid final targets and responses returning after the stage deadline.
Retain initial target validity even when false. The bounded generated-playlist
subset requires terminal ENDLIST; RFC 8216 permits other ENDLIST placement,
which this diagnostic deliberately does not support.

Bind final physical identity and generation to the exact joined snapshot.
Require final physical and public EOF, valid target duration and matching cuts.
Propagate remaining HTTP time and check the deadline after every response.
Reject invalid timeout values before network effects. Independently check the
marked source frequencies against its four-second analytic fixture pattern;
two equally wrong or missing observations must not qualify the AAC proof.
These integrity failures cannot be injected by the valid hosted fixture.

## Closed compositor tail diagnostic, before implementation

The existing complete callback receipts do not close the post-END tail.
WICG defines `presentedFrames` as compositor submissions; pinned Chromium 153
counts these separately from decoded ready frames. Add a separately named
diagnostic. Retain every legacy equality result, raw row and case failure.
This diagnostic cannot certify physical screen scanout or surplus decode identity.

Keep rVFC registration and the existing rAF pulse through END. Settle for at
least 500 ms and eight animation frames, with 250 ms quiet time. Restart quiet
time on callbacks, copy completion, quality changes or lifecycle changes.
A separate two-second timer must fail even when rAF stops. Require settled
samples before and after one final native copy, zero pending copies and equal
terminal count, PTS, hash and quality facts. Preserve all existing copy bounds.

Retain one aligned timing row per callback, including callback, presentation
and expected-display times. Require visibility and one attached video generation
through observation and settle. Permit initial loading, then reject post-gesture
source replacement, loadstart, emptied, navigation or counter reset. Compare
source strings privately; retain only bounded identity facts.

Before implementation, test late callback/copy facts, changed terminal counters
or generation, missing timing rows and timeout. Explicitly test that a separately
qualified diagnostic leaves legacy equality false. The full Direct reference
must still pass unchanged and settle. Public native identity, source clock,
trusted startup, fixed rate, delivery, process and source guards remain required.
EOF duration/cuts, signed edit/preroll, AAC, refill and cache proof stay separate.

## Negative-origin AAC decoder budget, before implementation

Run `37565697792` retained all AAC packets through 19.4995/19.499833 seconds.
Its ordinary PCM decode stopped near 19.22/19.20 seconds. Both public containers
have format start time -0.5. The difference closely matches normalization of that
negative origin against the output `-t` limit. This is a hypothesis, not missing
media proof. Preserve the original failed terminal window and command.

Add a separately labeled decoder-budget control. Extend only its output time
bound by the independently measured negative format origin, limited to one
second. Keep sample byte, process, content and source-window bounds unchanged.
Retain decoded frame PTS/sample counts and complete AAC packet payload hashes,
including skip/discard facts. Require a unique contiguous source-packet match
through actual source EOF. No packet or frame is trimmed to obtain a match.
Before implementation, test missing/invalid clocks, hashes, skip fields and
oversized probes, plus ambiguous or incomplete source-tail correspondence.

The previous indented receipt uses 3,825,784 bytes of the fixed 4 MiB artifact
budget. Lossless compact JSON retains every value in 2,526,496 bytes. Write new
receipts compactly to fit added timing and AAC rows. Preserve historical bytes
and all receipt, response, native-copy, manifest and archive bounds.
