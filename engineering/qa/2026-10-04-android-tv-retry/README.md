# R13 Android TV playback retry: test-first evidence

Baseline: `5c3df3a06adad59c65551a65e132a92360b57c4b`.
R13 / AND-B03 is a separate commit after R09. Public TV retry ownership failed
twice before the one-line production fix. Scheduled GREEN verification is pending.

## Failure modes before implementation

R13 uses the actual TV playback `Try again` action. A retry must retain TV mode,
the persistent TV remote identifier, and item/target matching. Possible failures
include switching to phone ownership; stopping remote status publication; using
a different TV ID; admitting a command for the wrong item or target; duplicate
remote loops; and accidentally treating phone playback as TV playback. The same
button path is used for TV audio and video. Physical audio and Wear discovery need
separate proof.

## Fixture and proof boundaries

Use actual TV Compose Try again focus plus a D-pad center key event, production
PlaybackModel, real encrypted SessionStore, HTTP transport, response decoders and
ExoPlayer construction. Only the JVM-unavailable AndroidKeyStore lookup uses the
existing PhotoScreenTest platform fixture. Synthetic media intentionally returns
404; this is retry ownership and status proof, not successful decoding/continuity.
Verify saved TV ID, status PUT, phone ownership exclusion and item/target rejection.
No device, encoder, real content, Viewer settings or production changes occur.

Two initial baseline checks timed out before playback response reproduction.
Those are harness failures and not evidence of R13. The repaired fixture uses the
public Compose idle check before polling real model state, and emits bounded
path/state diagnostics. It must reproduce twice before the one-line TV retry fix.
Full baseline logs/XML are retained in task-2/android-recovery-red/ and excluded
from Git. Physical TV, audio service, Wear discovery and deployment need separate
proof; a host key event is not a physical remote.

## Confirmed RED before production edits

With production PlaybackScreen still identical to baseline `5c3df3a06`, candidate
`093bdbfe7da8dc82a62e9f66806ed309e19bdd9a` compiled the new test and executed
two independent TV journeys. Each focused the actual `Try again` button, sent
a D-pad center key, made two playback HTTP requests and constructed the player.
Both failed the intended assertion: `currentPhone()` must be null for TV retry,
but phone ownership was present. This confirms the public UI/model defect.
The later remote-ID and command rejection assertions were not reached in RED.

The combined run executed 13 checks in 41 seconds: ten passed, these two failed
as intended, and the separate R09 TV navigation check timed out before paging.
The receipt distinguishes all three outcomes. It pins the exact production and
test hashes and records the R13 test as untracked compiled source:
`task-2/android-recovery-green-r09-red-r13/receipt.json`, SHA-256
`bf0e13ad681eb2c7754eaa008a1cd1b106550c955ae28fcc3ceb8af1faa3ef63`.

## Repair and repeatable verification

The TV button now passes its existing `tv` argument into `PlaybackModel.start`,
matching initial playback. Phone behavior and model security checks are unchanged.
The retained regression requires persistent TV ID publication, exclusion from
phone ownership and rejection of mismatched item or target commands.

From `apps/player/apps/android`, schedule this bounded run with the integration owner:

```sh
JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home \
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools \
./gradlew --offline --no-daemon --max-workers=1 \
  -Dorg.gradle.jvmargs=-Xmx1536m :app:testDebugUnitTest \
  --tests '*TvPlaybackRetryTest'
```

No assembly, installation, device deployment, real data deletion or encoding is
part of this run. Initial fixture timeouts remain separately classified and do
not count toward the two confirmed failures.
