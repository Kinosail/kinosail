# Core resilience implementation — 2026-09-12

Tracks the 23 findings in the preceding core-functionality resilience audit. The 23 finding IDs have implementation coverage below. This note records source changes and build evidence; runtime correctness and performance qualification remain outstanding.

## Preserved contracts

Direct First playback, origin-confined credentials, profile isolation, bounded input, durable data before verified metadata, and block plus whole-file integrity remain required. Network failures do not authorize transcoding. `.gates-disabled` is preserved; test and quality suites are not run.

## Native download and progress batch

- F01: claim admission before actor suspension; validate generation after authorization, identity, preferences and preparation.
- F02: temporary HTTP outages allow the saved offline session; authentication denials and invalid responses still fail.
- F03: isolate invalid journals without disabling healthy profiles. Metadata limits and path validation remain.
- F04: durable deletion intent survives interruption; payloads disappear before journals and the marker. Removal also cleans orphan files for an owned key.
- F07: cancellable hashing runs on a serial worker separate from delegate/control work. Successful integrity and decode evidence is reused only within the process while device/inode/size/nanosecond modification and change timestamps match. Relaunch, replacement or mutation requires a full recheck. The final probe-to-publication boundary also rechecks file identity. Timestamps are app-container metadata, declared under Apple's C617.1 privacy reason.
- F08: pause and playback suspend OS-owned extents instead of discarding their partial data. Failed connections retry one verification block per extent. Two transfer slots remain bounded; physical background wake scheduling and the best extent window still need measurement.
- F09: preparation failure count and elapsed polling deadline are durable; valid preparing responses are distinguished from failures. Retry-After is a minimum with bounded jitter. Exhaustion is recoverable through Resume.
- F10: current Wi-Fi/quota policy reaches active engine work. Smart completion intent is separately persisted per profile. Requested replacement episodes must be ready before watched-file removal.
- F11: a season holds one admission and reuses identity/preferences. Track lookups and preparation stay bounded; already saved selections are skipped on retry, and partial completion is reported.
- F20: a missing/deleted title retains its pending progress but no longer blocks unrelated updates. Authentication, transport and disk failures still stop synchronization.

Evidence: ordinary iOS and tvOS simulator builds succeeded. Focused Swift regression sources cover offline fallback classification, retry bounds, malformed retry headers, corrupt-journal isolation, interrupted deletion, file-identity invalidation and missing-title progress. These tests were not executed. No new physical-device, background-transfer, energy, throughput or latency evidence is claimed.

## Server recovery and concurrency batch

- F05: live admission enforces the same 10,000-job bound as restoration before persistence; queue saturation returns retryable 503 and Retry-After. Stored-cache capacity requires removal and returns 409.
- F12: queued/active work has an attempt-specific cancellation context. Removal cancels reservations, FFmpeg and bounded copy/hash reads; publication checks attempt identity. Old sidecars are removed before a new attempt can reuse the ID.
- F13: startup reads bounded metadata and exposes restored ready jobs as preparing until one background worker verifies their bytes. Requested titles are preferred between files. Unverified or corrupt files are unavailable for delivery.
- F14: original copy produces block/whole digests in the copy pass and synchronizes before publication. Missing-manifest repair is shared per revision, cancellable when unused or removed, and bounded to two disk readers and 32 pending revisions. Kernel-copy tradeoffs still need benchmarking; fewer passes are not a measured speed claim.
- F18: candidate Viewer state is captured under progress/list locks; filtering, sorting, grouping and pagination run after unlocking.
- F21: three provider searches share a 15-second deadline and execute concurrently. Results retain the original provider order for stable ties.
- F22: OpenSubtitles login shares an in-flight result and releases its mutex during HTTP. Waiters can cancel. Transient failures use the existing provider retry policy instead of an additional one-hour login ban.
- F23: Dashboard validates the complete bounded DNS set before dialing numeric addresses. A staggered alternate family and at most two concurrent dials share the probe deadline. Losing sockets close; destination checks and redirect refusal remain.

Evidence: ordinary Go builds succeeded for downloads/catalog, Dashboard, and Subtitles. Added regression sources cover admission without side effects, removed/re-added preparation, copy/manifest parity, canceled copy, asynchronous integrity restoration, provider concurrency, cancellable login waits, transient login recovery, alternate-address dialing and losing-socket cleanup. Suites remain unexecuted while gates are disabled.

## Independent review and progress storage

The native correction pass excludes suspended tasks from occupied transfer slots, preserves authorized restored transfers, clears verified flags when the payload disappeared, persists catalog deletion before engine deletion, and re-verifies replacement files before smart deletion. Empty transient HTTP bodies remain retryable and cancellation completes pending background events. Independent source review found no further concrete blocker in that correction pass. The corrected iOS simulator production build succeeded.

F19: playback progress commits one bounded SQLite record instead of copying and encoding every Viewer entry for every checkpoint. One transaction commits aggregate counters and the record; memory is updated only after durable commit. Schema 1 upgrades atomically. Whole-document saves, imports and exports retain their JSON contract. Existing schema 2 records are validated before migration cleanup; decomposition rejects duplicate decoded keys and malformed UTF-8. Independent review prompted both boundary corrections.

Evidence: ordinary documentdb/catalog production builds succeeded. Regression sources cover individual commit failure, invalid input without effects, aggregate-limit rollback, whole-document replacement, schema 1 upgrade/reopen, ambiguous migration rejection and validation before retired-document cleanup. These tests remain unexecuted.

## Browser downloads and playback recovery

- F06: each page-owned transfer has a cancellable owner. Remove broadcasts cancellation before acquiring the job lock, cancels active bodies/backoff/queued admission, and waits for writer closure. Old playback-source removal cannot erase a newer transfer. Closing the page aborts owned work; verified progress remains resumable.
- F15: a dedicated OPFS worker holds one synchronous access handle across the transfer and flushes each block before publishing its metadata. Unsupported synchronous access uses IndexedDB for new files; legacy OPFS resumes retain their durable stream fallback. Two shared Web Lock slots bound concurrent transfers across tabs, and waiters acquire whichever slot opens first. One block is prefetched while the current block is committed and hashed. Capacity admission reserves outstanding bytes. IndexedDB connections are reused, version changes close them, and writes request strict durability. Admission atomically retains concurrent progress. Player and Subtitles share the bounded range reader; Subtitles no longer reconstructs every offline chunk for each range and now matches the profile-scoped media URL.
- F16: a fully hashed download is labeled “Saved and verified. Play to check compatibility.” A loaded local media frame records compatibility evidence for the current browser and transfer generation. The state update is transactional with progress, so it cannot overwrite a newer checkpoint. Active transfer text states that the page must remain open; interrupted records expose Resume. This does not promise OS-owned background transfer in browsers.
- F17: transient playback failures retry the same representation up to three times with exponential delay and jitter. Direct network failures do not select transcoding. Browser retries retain position and pending actions through pause/resume, cancel stale source timers/probes, and bound/cancel reachability reads. Native retries preserve coordinator and AVKit intent, fence seek publication by player/generation, and retain upstream HTTP/validation errors across the local media gateway. A stable playback interval restores the retry budget. Exhaustion leaves an explicit recovery message.

Independent source review found and prompted fixes for cached-connection closure, stale progress snapshots, missing shared bundle dependencies, queued-slot delays, stale native seeks, old-title playback intent, offline autoplay after pause, and lost upstream error classification. Focused regression sources cover these boundaries where a deterministic fixture is available, including real OPFS/IndexedDB multi-block resume/repair, cancellation, queue admission, malformed input and playback retry budgets. The regression sources were not executed.

## Research basis and verification boundary

Research preceded implementation. The changes apply the bounded-recovery principles from [Metastable Failures in the Wild](https://www.usenix.org/system/files/osdi22-huang-lexiang.pdf), latency/critical-section guidance from [The Tail at Scale](https://research.google/pubs/the-tail-at-scale/), [AWS retry/backoff guidance](https://d1.awsstatic.com/builderslibrary/pdfs/timeouts-retries-and-backoff-with-jitter.pdf), [Apple background-download guidance](https://developer.apple.com/documentation/foundation/downloading-files-in-the-background), [HTTP semantics](https://www.rfc-editor.org/rfc/rfc9110.html), the [File System Standard](https://fs.spec.whatwg.org/), [SQLite WAL durability](https://www.sqlite.org/wal.html), and [Happy Eyeballs v2](https://datatracker.ietf.org/doc/rfc8305/). These are design inputs, not measurements of Kinosail.

Ordinary Go builds of Player, Subtitles, Dashboard and affected shared packages, plus iOS and tvOS simulator production builds, are the compile evidence. Player and Subtitles builds and both native simulator builds also succeeded after the final recovery corrections and shared-reader integration. `.gates-disabled` remains intact: no unit/integration/browser/race/benchmark/lint/container/release suite was run. Newly authored test source is not passing-test evidence.

Outstanding qualification: populated cross-browser behavior, network-loss/crash/quota scenarios, physical iPhone/iPad/tvOS controls and background lifecycle, before/after latency/throughput/energy measurements, installed builds, deployed revisions, container health and TLS. The source improvements reduce known repeated work and bound known failure amplification; no numeric speedup or “best in class” certification is claimed. Further PGO, transport, codec, journal batching and shared-artifact changes require representative profiling rather than speculative replacement.
