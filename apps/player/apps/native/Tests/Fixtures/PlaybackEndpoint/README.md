# Synthetic playback endpoint fixture

These test-only files contain a generated color pattern and a 440 Hz tone.
They contain no catalog media, account state or network credentials.
The synchronized `Tests` group belongs to the XCTest targets, not the app targets.

| Bundled file | Bytes | SHA-256 |
| --- | ---: | --- |
| `endpoint-source.mp4` | 288629 | `fca8542e89dcbfc618e21fdbe98261caacc72070ab92866ac604783ba4c37ff6` |
| `endpoint-index.m3u8` | 147 | `6e1aec2f21470e8ac0c5a994d7221f8f28aac34da1155ecf6cff3142f8b37be7` |
| `endpoint-segment-00.bin` | 313020 | `353c2f395d892f8efcd08be20a9577bfa31679a550a2fb01921626594c1b7a0d` |

The MP4 contains twelve seconds of H.264 video and AAC audio.
The HLS fixture contains the same media in one MPEG-TS segment.
Its playlist declares a twelve-second VOD segment and `EXT-X-ENDLIST`.
The loopback server exposes the bundled `.bin` bytes as `segment-00.ts`
with the `video/mp2t` content type. The packaged extension avoids treating
the transport stream as a TypeScript source file. Playlist bytes are unchanged.

`PlaybackEndpointMedia` locates these unique names in the XCTest bundle.
Missing files, unexpected sizes or different hashes fail a prerequisite.
The endpoint suites do not silently skip because a host path is absent.

## Reproduce the checks

Use an existing, owned tvOS simulator with the repository's current Xcode.
Run from `apps/player/apps/native`. Preserve earlier apps, results and logs.
Use a fresh output directory and check available disk space before testing.

```sh
xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-tvOS \
  -configuration Debug \
  -destination "platform=tvOS Simulator,id=${QA_SIMULATOR_ID}" \
  -derivedDataPath .build/tvos-simulator -jobs 4 \
  -parallel-testing-enabled NO \
  -maximum-concurrent-test-simulator-destinations 1 \
  -collect-test-diagnostics never \
  CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- \
  -resultBundlePath "${QA_EVIDENCE_DIR}/endpoint.xcresult" \
  -only-testing:Kinosail-tvOSTests/PlaybackEndpointJourneys test
```

The common serialized parent contains thirteen parameter cases:
eight cold/prepared, attached/unattached HLS endpoint cases and five completion
safety cases. Confirm thirteen actual cases, zero skips and successful exit.
The common parent prevents these two suites from competing for their window.
Keep whole-target execution serial as other native suites also own UI windows.
Do not infer serialization across unrelated suites from `@MainActor` alone.

Startup stays bounded at twelve seconds. Terminal and saved-progress checks
each retain their original three-second bounds. The tests measure the native
item duration, source mapping, decoded interior prerequisite, EOF, callback
ownership, pause state and positive saved-progress evidence.

The baseline missed EOF during restored startup. Native EOF occurred before
public startup returned, while the observer was registered after restoration.
The tests also protect duplicate completion and a scrub that supersedes the
restored endpoint. Explicit notification posts test ownership only; they do
not replace natural EOF evidence. These are isolated fictional fixtures,
not real-server, physical-device or caption coverage.

## Synthetic provenance

The original bytes were generated with this command:

```sh
ffmpeg -nostdin -v error -n \
  -f lavfi -i testsrc2=s=320x180:r=12:d=12 \
  -f lavfi -i sine=frequency=440:sample_rate=48000:duration=12 \
  -c:v libx264 -threads 1 -preset ultrafast -crf 38 -pix_fmt yuv420p \
  -c:a aac -b:a 48k -movflags +faststart -shortest source.mp4
```

The HLS bytes were packaged without re-encoding:

```sh
ffmpeg -n -hide_banner -loglevel error -i source.mp4 \
  -map 0:v:0 -map 0:a:0 -c copy -f hls -hls_time 2 -hls_list_size 0 \
  -hls_playlist_type vod -hls_segment_type mpegts \
  -hls_segment_filename 'segment-%02d.ts' index.m3u8
```

Generate into a new directory if replacing the fixture. Encoder versions may
produce different bytes. Requalify the native duration, endpoint behavior and
hashes before replacing the checked-in files. Do not overwrite test evidence.
