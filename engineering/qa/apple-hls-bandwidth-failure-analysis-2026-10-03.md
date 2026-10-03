# Apple HLS bandwidth failure analysis — 2026-10-03

Written before the regression test and production change.

## Observed evidence

- Source revision: `876b771a7dd0aef5e65957fcee87add0312535e8`.
- Nox Player revision: `21ba0b8ed2d165996bd18cfc48340ed01c0d740e`.
- Read-only FFprobe of 1917 completed in 208 ms. The source is Matroska, 7,325,147,541 bytes, 7,139.488 seconds, and 8,208,036 bits/s average. Its video is HEVC Main 10, 1920×804 at 23.976 fps, Dolby Vision profile 8.1 with an HDR10 base layer. Default audio is EAC3 Atmos, six channels, 768,000 bits/s.
- Existing SDR H.264 cache uses software `libx264`. Each rendition playlist contains 88 segments, 176.009 seconds, and no `ENDLIST`. Later seek segments also exist.
- The retained master advertises only 20,951 / 23,304 / 27,228 bits/s for 540p / 720p / 1080p. Those values match the first black segment. The second segments are 326,979 / 485,507 / 711,718 bytes for 2.002 seconds: approximately 1.31 / 1.94 / 2.84 Mbit/s.
- Duration-matched historical native-HLS traces reach `playing` in 552 / 358 / 518 ms. Their later stalls are paused. The trace does not identify a physical iPhone, so it cannot prove or disprove the reported iPhone problem.

The confirmed defect is an unfinished master claiming tiny intro-segment bandwidth as the demand for the entire rendition. Its estimate can survive inactivity and later cache reuse. This can mislead native HLS rendition selection. It is not proof of the sole cause of the user's prolonged buffering.

## Failure modes before isolated validation

1. An initial black or silent segment underestimates later bitrate and lets an HLS client choose a rendition its network cannot sustain.
2. A cached unfinished presentation keeps the original underestimate after later segments arrive or after a process restart.
3. A completed presentation loses its measured bitrate if a conservative startup fallback is applied forever.
4. A observed peak above the encoder's planned target must remain advertised; the fallback must not cap real peaks.
5. Repair must keep the cached rendition set, actual codec/resolution, independent-segment declaration, and cache identity. A changed resource governor must not add unavailable renditions.
6. Repair must not start another encoder or invalidate usable media segments.
7. Malformed or missing cached metadata must fail through the existing HLS error boundary rather than publish unsafe paths or invented output.
8. Audio and copied-video HLS must retain their existing policies and measured completed-stream behavior.

## Validation design

Add a public HTTP integration regression first, with a real FFmpeg-generated synthetic MPEG-4 fixture containing a black intro and subsequent moving picture. A real FFmpeg adapter reads at a bounded pace so the first response is an unfinished presentation. Assert startup bandwidth against the playback API's planned target. Replace only the temporary fixture's master bandwidth fields with the observed stale values and add the normal seek-cache marker; then request the public HLS master repeatedly. Assert corrected fields, preserved rendition metadata, and unchanged media-segment identity. Finish the encoder and assert completed bandwidth describes the measured output.

This is synthetic HTTP integration evidence, not live playback, Safari decoding, or physical-device proof. No Nox state is changed and no library movie bytes leave Nox. Existing format, speed, random-access, audio, and cache regressions remain separate retained evidence.

## Result and repeat recipe

The regression failed before production changes: initial average/peak were 8,080 bits/s against a 493,000 bits/s plan; three retained-cache requests remained at 1 bit/s. After the change, startup average/peak are 493,000 / 542,300 bits/s. Three cached requests preserve rendition metadata, first-fragment identity, and encoder invocation count. The completed master returns measured average/peak of 180,056 / 286,056 bits/s, and the delivered first fragment decodes successfully.

Unfinished playlists use their planned bitrate as a floor while retaining higher measured peaks. New masters carry a bandwidth-policy marker. Old cached video-transcode masters are repaired once from their existing rendition list, without changing fragments or starting encoding. The marker bounds future cache reads and prevents repeated segment scans. Completed publication continues to use measured bandwidth.

Repeat from the repository root:

```sh
cd apps/player
GOCACHE=/tmp/kinosail-apple-go-cache GOMAXPROCS=4 go test -p 1 ./internal/server -run '^TestRealBlackIntroHLSBandwidthSurvivesCachedMasterReuse$' -count=1 -v
```

The final shared playback suite, source-file cap, and whitespace checks pass. Retained HLS, seek, cache, cold-probe, and real copied-fragment decode checks pass. The retained scheduler fixture requires three rendition slots: `GOMAXPROCS=4` passes all cases; `GOMAXPROCS=2` or `3` reduces those slots and cannot meet the fixture's existing 540p-to-1080p expectation. The fixture was preserved.

Exact commands, base revision, environment, file hashes, observations, and verification boundaries are recorded in `apple-hls-bandwidth-http-evidence-2026-10-03.json`. Parent-owned CI, PR review, merge, deployment, and physical iPhone verification remain separate.
