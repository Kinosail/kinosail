# Apple TV playback investigation

The user reported Raya playback failures and a crash when playing Shrinking.
The physical Apple TV received main `4fee16403237b22018f354372ab981f101820170`, build 3666.
Two physical crash reports from that build show the same UIKit remote gesture exception stack.
Neither report identifies an application callback as the cause. This change does not claim to fix that crash.

## Confirmed stream departure defect

Native HLS requests had no playback session identity. Closing the native gateway sent no departure event.
The server therefore retained anonymous encoder work until its existing idle timeout.
A real Raya browser attempt waited 24,939 ms for admission. Media became ready 101 ms after admission.
The preceding heavy encoder released admission after 49,679 ms. Its title was not established by safe diagnostics.

Each native HLS gateway now sends its own UUID in `X-Playback-Session`.
Stop and replacement send the existing `POST /api/v1/items/{id}/playback-events` API a matching `session-end` event.
Cleanup runs independently of local closure. Failed delivery logs a bounded warning; server idle cleanup remains the fallback.
Crashes and forced termination can still require that fallback.

## Verification

- The isolated native HTTP journey failed on the baseline: six reads, no session identities, and no departure requests.
- The same journey passed after the fix: six reads, three independent identities, and three matching departures.
- Invalid source input sent no departure. Repeated closure sent no duplicate departure.
- A separate HTTP journey rejected cleanup with 503 and then opened another native stream successfully.
- All 35 selected native tests passed on tvOS 27.0 Simulator, build 24J360.
- `TestHLSPhasePublicHTTP/page_departure_releases_admission_for_the_next_movie` passed. It preserves shared viewers and releases admission after the final departure.
- Two simulator remote journeys passed against generated media, including repeated playback and failed-start retries. They did not reproduce the physical crash.
- `make max-loc`, post-commit `make -C apps/player verify-changed`, and `make tooling-check` passed. Both native app builds passed.
- The first CI run found a stale generated architecture snapshot. Regeneration and the full local tooling suite passed before retrying CI.

The native HTTP fixture isolates the upstream server. It verifies the Swift adapter's actual HTTP traffic, not populated-server E2E playback.
The server departure journey uses controlled encoders. It proves admission behavior, not media decoding.
The separate real Raya browser attempt played with advancing time and no media error. It does not prove physical Apple TV playback.

## Evidence and remaining boundaries

`validation.json` records the baseline, exact native commands, source hashes, and checksums for private evidence.
Raw device crashes, authenticated process logs, server targets, and media screenshots remain outside the repository.
Physical tvOS is build 24J361; simulator evidence cannot establish the physical remote touch path.
The exact remote action that triggers the Shrinking crash still needs reproduction on the physical device.
The deployed server remains a separate older local snapshot. This native fix uses its existing playback event API.
