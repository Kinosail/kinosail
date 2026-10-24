# Native download engine implementation

The Player server seals a prepared revision before clients transfer it. Native
Player uses authenticated HTTPS ranges and OS background scheduling. Originals
remain byte-for-byte copies; converted downloads preserve all audio/subtitle
tracks by default, with explicit selection available for a single video.

## Implemented

- Version 1 manifests contain the full SHA-256 and 8 MiB block hashes, bounded
  to 128 GiB. Source size/mtime and selected tracks contribute to revision IDs.
  Replacing a source creates a new job; existing prepared files remain available
  until explicitly removed. Equal-size replacements with restored timestamps are
  not detectable by the source fingerprint; sealed bytes remain independently hashed.
- iOS background URLSession keeps up to 12 extents of 32 MiB queued, so
  about 384 MiB continues while the app is suspended. The session still opens
  at most two connections to the Server. Android uses
  user-initiated jobs on API 34+ and a foreground WorkManager fallback. Android
  transfers one extent at a time and reuses OkHttp connections.
- Each verified block is flushed before the durable journal records it. Resume
  rechecks local blocks and retains matching data. Final installation requires a
  full hash and a file-only decoder probe. An integrity error never becomes Complete.
- Preparation polling survives JavaScript suspension. Pause, removal, sign-out,
  playback priority, storage quotas, and Wi-Fi restrictions reach the native queue.
- Authenticated server/profile identity preserves the existing storage scope when
  credentials rotate on the same server URL. Account changes clear downloads.
- Compatible remuxes eligible SDR H.264 video; other sources use H.264 conversion.
  Audio becomes AAC. Styled/bitmap subtitles use Matroska; ordinary text uses MP4.
  Original copies retain embedded tracks; external sidecars require a converted
  quality. Android codec/subtitle support still depends on its playback engine.
- Check trip readiness schedules local hash and decoder checks without a server
  request. Legacy native downloads require an explicit resume to enter verification.

## Recovery and feedback on iOS

- A network loss, a stopped Server, or a busy Server never pauses a download.
  Kinosail retries with backoff of at most about 40 seconds in the foreground
  and at least 5 minutes in the background. Only sign-in, storage, and
  integrity failures need the person to act.
- The background transfer service can resume a dropped request part way. The
  app accepts a resumed response when its final range covers the requested
  extent. Every 8 MiB block is still checked against the manifest hash.
- When the network returns, the Server answers again, or the app opens,
  downloads that wait for a retry start at once.
- Each row and the Downloads summary say why a download waits: no network,
  Wi-Fi only on a cellular connection, a Server that does not answer, a retry
  countdown, or a slow connection. The summary offers "Use cellular data for
  downloads" when Wi-Fi only blocks a download.
- When the app is not in front, a local notification says that a title is
  ready to watch offline or that a download needs attention. Tapping it opens
  Downloads. Kinosail asks for notification permission after the first download
  is added.
- Retry, stop, and completion events log at `warning`, `error`, and `info`
  under the `downloads` category. They contain no titles, URLs, or tokens.

## Verification boundary

Source review and formatting were performed. `.gates-disabled` remains present:
no tests, native builds, populated UI renders, benchmarks, deployment validation,
or physical-device checks were run for this implementation. Regression tests are
included for manifest validation, ownership, selected tracks, cache changes,
preparation pause, stale snapshots, and session ordering. This is not evidence of
release readiness, background reliability on devices, or best-in-class speed.

After the user explicitly enables gates, run the affected Player/package checks,
native Jest suite, iOS/Android builds, and physical-device lifecycle checks. Run
`go test ./packages/downloads -run '^$' -bench BenchmarkSealedDownloadHEAD -benchmem`
for response startup overhead; it is not a network throughput benchmark.

## Experiments that still need evidence

Retain the measurement plan in [best-in-class-downloads.md](../../../../engineering/research/best-in-class-downloads.md).
Compare verified goodput, first byte, retransferred bytes, startup, stalls, battery,
and thermal behavior on the same files and devices, including lossy Wi-Fi,
network transitions, suspension, process death, corruption, low storage, and
server-unavailable trip checks. User force-stop/force-quit remains OS-controlled.

HTTP/3 must be measured with HTTP/2 fallback and negotiated protocol evidence.
Native HLS/CMAF packages, HLS Live Activities, Media3 preloading, and partial-file
playback remain experimental follow-up work; no second package path or partial
Ready offline claim is introduced before those measurements. The disabled gates
prevent running the required evaluations in this task.
