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

The five offline variants are the existing HLS mux, negative timestamps,
explicit HLS edits, delayed fragmented MP4 edits, and copied timestamps with
explicit HLS edits. Each copies video and audio. The loop has a 120-second
deadline per source and bounded subprocesses and media reads. A timed-out or
rejected variant remains failed evidence. The manual workflow still runs all
twelve authenticated public cases and preserves only receipts and checksums.

No production scanner, compatibility policy, credentials, source media,
simulator or deployment changes are authorized by this proof itself.
