# Q03 Android search feedback: test-first plan

Canonical item: Q03 / AND-Q04. Reliability R09/R13 are the preceding batch.
This document records preparation only; Q03 production changes are not made.
Baseline: frozen recovery candidate `47bcc64c8d737b1249498ea402e41788d781507b`,
with origin/main `50fd82f329c651258245451a55b8fff8dfa9adec` reconciled.

Use the existing bounded explicit Search/IME request. Add a labeled Clear search
control and a polite result announcement bound to the submitted query and the
Server total, rather than the unsent input. Keep phone/tablet/TV native themes,
D-pad/IME interaction, and current Viewer restrictions.

Failure modes to cover before production edits:

- A draft edit must not send a request or relabel older results as a new query.
- Success and zero matches need the correct submitted query and total.
- Clear must clear the field and submitted query through one explicit action.
- A newly submitted query's loading/failure must not announce an older total.
- A slow old response must not replace newer submitted results.
- Clear/submit must not transfer TV focus to a card unexpectedly.
- Empty, long Unicode, control characters and byte bounds retain validation.
- No persisted search history or media/request telemetry is introduced.

Planned public proof uses the existing real SessionStore/HTTP Android recovery
fixture and actual phone/tablet/TV Compose field, IME and Clear actions. Capture
bounded synthetic native host PNG/semantics for entered, loading, results, zero
and cleared states. An actual hardware D-pad, TalkBack, process restoration and
current populated Go Server result ranking remain separately named boundaries.
The fixture validates request decoding/auth headers but cannot prove the Server
search algorithm. Retain existing catalog parser and generation checks.

Failure reproduction and GREEN results are pending separately scheduled bounded
Gradle runs. Eight active native journeys now cover phone/tablet/TV submitted
results, draft edits, independent phone/TV Clear paths, pending/failed queries,
zero matches with a quoted Unicode query, and an older held HTTP response after
a newer successful query. The fixture keeps default R09 data unchanged and adds
query-specific totals/failures/gates with valid JSON serialization.

Implementation design remains narrow: retain a nullable submitted-result query
only after a successful current-generation response; a newly submitted query
clears the older result summary until its own response succeeds. Keep the count
node stable during paging and draft edits when the submitted query/total has not
changed, so later-page pending work does not repeatedly announce the same total.
Use the existing validated search action
for Clear, keep the Clear control in the composition so TV focus remains on it,
and show a bounded submitted query independently of draft edits. Existing Server
query ranking, API validation and authorization need no production changes.

`SearchFeedbackJourneyTest` uses a displayed, focused Search destination and
actual TV center-key input, then the native field/IME and Clear controls. It has
not yet compiled or run. The earlier smaller inactive draft remains outside the
application source set as preparation history and is not runtime evidence.
Runtime availability and benefit remain unverified until baseline and candidate
native journeys execute.

The synthetic HTTP stand-in adds query-specific total, failure and gate
controls for those journeys. Keep its responses valid JSON for Unicode and quoted
queries, and release held responses during fixture disposal. A zero-result
response must contain no items and total zero; a pending old query must be held
while a newer query succeeds, then released to prove the stale response cannot
relabel or replace results. Do not infer this from a fixed delay or a call count.

Check the native polite live-region semantics of the successful count and its
absence during pending/failed work. TalkBack speech remains a physical-accessibility
boundary. Keep first-page loading placeholders and the empty/failure copy alongside
the new count; it must not reduce the area available to search results below the
native phone navigation or TV remote controls.

From `apps/player/apps/android`, request the serial build slot before running:

```sh
JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home \
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools \
KINOSAIL_ANDROID_SEARCH_EVIDENCE=/tmp/android-search-evidence \
./gradlew --offline --no-daemon --max-workers=1 \
  -Dorg.gradle.jvmargs=-Xmx1536m :app:testDebugUnitTest \
  --tests '*SearchFeedbackJourneyTest' --tests '*CatalogRecoveryTest'
```

The eight R09 model controls accompany the eight Q03 native journeys to check
that new fixture controls do not alter recovery behavior. Capture and reconcile
nonzero XML counts and separate intended missing-control/summary assertions from
harness failures. No production behavior, device install, encoder or external
sharing occurs in this baseline run.
