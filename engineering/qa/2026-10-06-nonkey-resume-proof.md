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
