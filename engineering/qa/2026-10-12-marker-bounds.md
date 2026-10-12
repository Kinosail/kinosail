# Playback marker response bounds

The installed tvOS client rejects the playback response for frozen title 23,
London. The normal retry reproduced the startup failure under the original
12-second limit. A separate, unqualified diagnostic reproduction reached
`PlaybackSource.init` and rejected `markers[].end = 304.96799999999996` against
the advertised duration of `304.96`. No playback timing limit will change.

The client accepted HTTP and JSON handling before this model validation failed.
The original attempt has no request-level or numeric correlation. The marker's
creation history and the deployed Server revision remain unknown. No credentials,
response bodies, private session stores, or original media files were inspected
or changed. Diagnostic runs do not count as playback passes.

Before implementation, the response-boundary failure model is:

- A finite marker end exceeds the advertised endpoint by a fraction of a frame.
- A marker lies wholly beyond the endpoint and becomes an empty range.
- Negative, reversed, empty, NaN, or infinite ranges become plausible markers
  when mapped without validating their original bounds.
- Unknown or invalid durations accidentally emit ranges as if time were known.
- Existing valid markers, exact endpoints, and shortened timelines change.
- Normalization mutates saved source metadata or leaks it through diagnostics.

The isolated boundary regression is necessary because populated playback tests
did not contain this 8ms mismatch. It exercises the existing API timeline
projection, preserves valid values, and rejects malformed ranges. It is not
proof of deployed Nox behavior. A focused public HTTP regression will use a real
disposable media file and controlled persisted marker metadata; fixed-release
London testing still requires separately coordinated release adoption.

Source overlap was inspected read-only. The existing QA task's observed changes
cover HLS refill and coverage work, not `packages/playback/timeline.go`. Its work
and an unrelated Xcode project rewrite in this checkout remain outside this patch.

The regression was written before the production change. The isolated check
failed on the original source. The authenticated public HTTP fixture then passed
ordinary and exact-endpoint cases, but failed the observed sub-frame overrun.
After normalization, all three HTTP cases and the timeline matrix passed.
The HTTP fixture also checks the unchanged direct media bytes, preserved saved
marker values, and one safe WARN correlated to the accepted request ID.
It uses real FFmpeg and FFprobe; environments without them report a skip.

Local commands against base `9e8d3bbc8a00cca88ab8a9dc6ec000586597c2ae`:

```sh
go test ./packages/playback ./apps/player/internal/server \
  -run 'TestAPIPlaybackMarkerResponse|TestPlaybackMarkerBoundsPublicRealMedia|TestApplyAPIPlaybackTimeline|TestTimeline' \
  -count=1 -v
golangci-lint run --new-from-rev=9e8d3bbc8a00cca88ab8a9dc6ec000586597c2ae \
  ./packages/playback/... ./apps/player/internal/server/...
```

Both commands passed on macOS with Go 1.27.2 and local real codecs. The generated
MP4 SHA-256 was `1d417ef2edea02a0f1af5bea50ded41dfe9352c4856ab9493f8918a609b0b9b1`.
The broader package check stopped at existing unrelated lint findings. The root
working-tree line check also includes three preserved 306-line runner evidence
copies; they are outside the committed patch and remain unchanged. Required CI
and exact committed-source checks must be reported separately.

The source fix does not establish deployed behavior. Titles 17, 19, and 23 retain
mandatory repair/retest obligations. Title 24 remains untouched. The frozen
ledger remains 1 full pass, 20 partial, 1 failed, and 28 unrun.
