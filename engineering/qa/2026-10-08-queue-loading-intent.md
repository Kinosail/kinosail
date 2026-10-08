# Queue loading intent regression

A source review of PR503 `fdc08dd7` suspected that seek or Stop during a queued
track load can be overwritten by saved progress when real metadata arrives.
The complete queue source blob `b21d38111a94b249afb083b1656dcd0e4f8af7b9` is identical
in that candidate, PR523 `e11212eb`, and published main `63f38a09`.
This is a source suspicion before the first hosted browser result.

This first checkpoint changes tests and their required selection only.
The existing fictional two-track album, all prior tests and production stay intact.
The new journeys save five seconds through the public progress API, delay the
next track's real media request, and queue seek or Stop through the public API.
They require real command consumption and actual browser position before metadata.
After releasing the unchanged media request, they observe the real loadedmetadata
event and require the latest intent to survive. There is no synthetic media or
metadata event. The request delay is explicit fault injection in a real Server.

The first attempt and every retry must remain attributable to their exact source.
Prerequisite failures do not prove the position-overwrite suspicion. Production
changes require a confirmed failure and independent review. Preserve native
intent, preparation, all existing failure controls, scanner/Q12 holds and PR517
ownership. This work performs no native/UI/local build or manual deployment.
## Retained actual negative and exact-position continuation

Test-only headb13d8500fd8f91b72d60593780855b9827835e55, tested merge
5e1f828ec663b1621ae4c8c5adf97d3822a75d39, run37714466718 and artifact11523124686
(SHA-256b64e2a722f5e732b2d739480a7f89a18a87e94cd7ca503152c6e53acced8ec81)
show53 preceding cases passing on first attempt. Both real public Home Assistant
variants reach actual queued-B loading and accepted command responses, then
actual metadata restores saved5 instead of seek2/Stop0, first attempt and retry.

A separate fictional60-second album adds saved35 cases without altering the
original R08 media or tests. These require actual queueSourceChanging=true,
queueProgressReady=false, command delivery for B, actual metadata/duration>35,
clock/target observations and an accepted web progress revision/status with a
public readback. Production remains unchanged for this written-first checkpoint.

The actual platform MediaSession Stop/seekto0 trigger remains UNADMITTED.
Hosted Home Assistant variants cannot establish that trigger. The available
[Chrome input protocol](https://chromedevtools.github.io/devtools-protocol/tot/Input/)
dispatches page keyboard input; it does not prove an OS media action. No handler
invocation, synthetic intent or synthetic metadata event is used to replace it.
[Chrome Media Session guidance](https://developer.chrome.com/blog/media-session)
also distinguishes loading from active platform controls. The media owner must
admit the actual platform path separately; green variant checks cannot release it.

## Isolated successor and written-first Stop readback contract

PR526 and its branch are preserved at bdd7816eeaebcd3d65165beee5dbcb7bb18f8ffc.
The successor starts from current main c452a2e73cebac42d280247fa9684efb630ecc93,
which includes PR517 and PR527. No other production seam is imported.

Before changing the assertion, inspect the exact original source and terminal
[CI run 37719450557](https://github.com/Kinosail/kinosail/actions/runs/37719450557).
Player browser job113124989748 records55 passes and two Stop failures on both
attempts. All preceding action, source, metadata, pause and accepted-write
assertions passed. The final readback at helper line120 received undefined.
The seek cases passed. This is failed acceptance, not a green Stop result.

Both that head and current main use the identical PlaybackState source blob
 a77499b459081cbed2a67ff579dd0494cb615058 in packages/catalog/progress.go.
Its seconds field has the existing JSON tag seconds,omitempty. Zero is omitted.
The assertion repair must reject a missing item, wrong item identity, missing,
null or array progress, present null/string/nonfinite/negative seconds, and
missing seconds for seek. Only Stop after the existing accepted current-track
zero write (HTTP204 and positive revision) may interpret an absent own seconds
field in a valid progress object as zero. Keep the numeric position comparison.
No production API normalization or serialization contract change is proposed.
The first successor commit records these controls before importing the original
queue-only production fix in the next commit.

The separate Subtitles job113124989355 failed
TestHealthcheckUsesConfiguredAuthHost with HTTP421 at healthcheck.go:42.
The shared commandtest healthcheck blob22cbfa45120541187de5df0797dbf7ff4fdbe8bd
is identical in the failed head and current main. Main run37740698722 passed
with Subtitles skipped, so it does not resolve this failure. Fresh selected
Subtitles CI must classify it; auth validation and the test stay unchanged.

The capacity hold forbids local builds, application runs, artifact downloads,
clones, worktrees and cleanup. GitHub source APIs and hosted CI provide this
successor's route. Generated architecture must pass the official generator
against current main plus the proposal. Scanner configuration, ignores,
required protections and original evidence remain unchanged. Actual OS
MediaSession, native HLS and the separately owned native-layout work remain
unadmitted. This proposal does not authorize manual deployment.
