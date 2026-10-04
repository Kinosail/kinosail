# Q03 Android search feedback: test-first plan

Canonical item: Q03 / AND-Q04. Reliability R09/R13 are the preceding batch.
Q03 acceptance RED is confirmed; the narrow production implementation is now
present and scheduled candidate GREEN verification is pending.
Baseline: frozen recovery candidate `47bcc64c8d737b1249498ea402e41788d781507b`,
with origin/main `50fd82f329c651258245451a55b8fff8dfa9adec` reconciled.
R09/R13 subsequently merged in PR #466. The queued Q03 baseline branch is
reconciled with fetched main `d440fb05ede52498b08c9a0b53d2ea929a1b584b`;
Q03 production remains identical to that merged Android tree.

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

## Acceptance RED before production edits

At exact test-first commit `2f2f9acd41ee4926e3d8ff496ae5b381d1fcc7a6`,
the bounded run completed in 28 seconds: 16 executed, eight R09 controls passed,
eight Q03 checks failed, zero errors or skips. All failures were intended missing
feature assertions after native Search/IME and public HTTP/model transitions:
six absent submitted-count summaries and two independent absent Clear controls
on phone and TV. No prerequisite or harness failure occurred. Subsequent
assertions blocked by each first intended failure are not counted as reached proof.

The production Android tree was unchanged from merged main `d440fb05ede` during
RED. The receipt pins six production/test source hashes, two XML files, the log
and ten PNG/semantics artifacts: `task-2/android-search-red/receipt.json`, SHA-256
`8b0673193cef49c5dc212609b32bd0b487ec9d9cf0468c3f8d73ca977dca36f2`.
Phone and TV submitted-search baseline captures were inspected and show no
Clear control or result summary. Query field, Server response totals and rows
were successfully reached; these captures do not prove physical input or ranking.

## Candidate behavior

The model records the query of a successful current-generation result, and Clear
uses the existing validated explicit search action with an empty input. Native
phone/tablet and TV controls show that submitted query and Server total with a
polite live region; unsent edits keep the prior label. Clear stays composed so
TV focus can remain on the actual control. The count uses readable native theme
colors and two bounded display lines. No query history, Server API change,
authentication relaxation, settings mutation or media work is introduced.

The existing translation lookup is retained; new Results/Clear search keys use
its English fallback where absent from generated catalogs. Physical TalkBack,
actual TV remote input, populated Server search ranking, large-text/RTL and other
language renders remain separately named verification boundaries.

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
