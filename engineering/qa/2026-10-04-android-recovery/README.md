# R09 Android catalog retry recovery

Baseline: `5c3df3a06adad59c65551a65e132a92360b57c4b`.
This commit owns R09 from the canonical ranked audit. R13 is a separate follow-up.
R09 is confirmed, implemented and tested. All eight model and three native host
footer journeys pass at `eeec3b7eda18944965803c2efc6b70328337b916`.

## Failure modes before implementation

R09 uses the public `CatalogModel` paging and retry actions and `/api/v1/library`.
A failed later page must keep loaded rows, their stable IDs, and the failed offset.
Manual and delayed retry must append that same page. Potential failures include
shrinking rows through offset zero; suppressing recovery feedback; duplicate rows;
duplicate concurrent retries; retry loops after malformed or denied responses;
stale retries after search, destination, Viewer, Server, or reset changes; stale
loading indicators; and focus or scroll disruption when loaded cards change.
The existing 15-second automatic retry interval and HTTP retry policy must stay bounded.

## Planned verification and retained isolated coverage

Existing native journeys cover initial load failure, but miss manual and delayed
retry after a later-page failure. New checks exercise production models and Compose actions
through a real loopback HTTP stand-in. Only the unavailable JVM platform KeyStore
lookup uses the existing `PhotoScreenTest.FixtureKeyStore`; session encryption,
session storage, HTTP transport, response decoding, and model transitions remain real.

These are isolated runtime and rendered-fixture checks, not populated Go Server
or device E2E proof. The model failure was reproduced twice for each retry path
on the baseline before production edits. Evidence must separate model, host-rendered UI, simulator,
physical device, authentication, remote input, audio, and deployment boundaries.

R09 retains loaded rows on transient and nontransient failures, retries the same
offset, rejects duplicate rows, and suppresses obsolete delayed work. Global
401/403 reconnect feedback remains above the grid and receives no automatic retry.
No production data, device installation, encoder, or deployment is involved.

## Baseline runtime results

`CatalogRecoveryTest` reached production `CatalogModel` over real loopback HTTP
on the unchanged baseline. Two manual runs and two delayed runs each loaded 48
rows, failed offset 48, then requested offset 0 and replaced the rows with 24.
The duplicate-page and simultaneous-tap regressions also failed; denied-page and
superseded-search controls passed. This is R09 public-model failure proof.

The first scheduled RED suite compiled after correcting an unsupported test
collection selector. Footer checks then exposed a missing fixture HomeModel
factory; TV retry checks timed out before the intended boundary. Those five
failures are harness failures, not evidence of R09 UI or R13 reproduction.
R13 production remains unchanged and is tracked in the follow-up evidence document. Baseline XML and stdout are retained outside Git in
`task-2/android-recovery-red/`; the log is `task-2/android-recovery-red.log`.

## Candidate runtime results

Candidate `093bdbfe7da8dc82a62e9f66806ed309e19bdd9a` executed 13 checks in
41 seconds: eight R09 model checks passed, phone and tablet footer journeys
passed, the TV footer timed out, and two R13 ownership regressions failed as intended.
Both manual and delayed retry passed twice with 48 loaded rows retained, 72 rows
after recovery, and offset 48 requested. Duplicate responses, simultaneous taps,
denied pages and obsolete delayed retry controls all passed.

The TV timeout occurred while waiting for the Movies destination, before paging
or footer assertions. It is not a TV footer success or demonstrated production
failure. The fixture now scrolls to the actual Movies button, checks visibility
and focus, and sends a D-pad center key rather than a touch click on a TV surface.
The footer still must receive focus and the actual key event; assertions remain
unchanged. The next run also retains bounded state diagnostics on failure.

Phone and tablet pending, loaded, failed and recovered captures were inspected;
loaded cards remain visible during failure and the new recovery control is
visible without replacing rows. These are synthetic host renders. Candidate XML,
stdout and checksums are in `task-2/android-recovery-green-r09-red-r13/receipt.json`,
SHA-256 `bf0e13ad681eb2c7754eaa008a1cd1b106550c955ae28fcc3ceb8af1faa3ef63`.
The receipt explicitly hashes the R13 test that was compiled while still untracked.
PNG and semantics artifacts are in `task-2/android-recovery-green-r09/`.

## Final GREEN and curated QA

The combined candidate run at `eeec3b7eda18944965803c2efc6b70328337b916`
completed in 31 seconds: 13 executed, 13 passed, zero failures, errors or skips.
R09 accounts for eight model checks and phone/tablet/TV footer journeys; R13
accounts for two focused TV playback retry checks. The TV footer reached the
actual displayed, focused button, sent a D-pad center key, appended offset 48,
retained the original 48 rows and removed recovery feedback after success.

All four TV host renders were inspected: loaded cards, pending later-page status,
failed-page feedback with visibly focused retry, and appended recovery content.
Phone/tablet captures were also inspected across loaded, pending, failed and
recovered states. The final receipt hashes 12 PNGs and 12 semantics files, including
polite failure feedback and the TV retry's `Focused=true` node. Existing initial
empty-state UI was not changed or rerendered by this later-page batch.

The final XML, stdout and render receipt is
`task-2/android-recovery-final-green/receipt.json`, SHA-256
`9ab137daa2478d73add6daa5ebc68b128fc00a52650b88426f4b372bfa07b68e`.
It pins all eight runtime production/test source hashes at the executed revision;
later evidence-only commits do not replace that runtime identity. Full Android
unit suites and physical accessibility/input checks were not run in this bounded
slot. `make -C apps/player verify-changed BASE=5c3df3a06adad59c65551a65e132a92360b57c4b`
passed max-loc and diff-check after committing the app changes; its Android-only
dispatcher does not execute Gradle, so the native test counts above are separate.

The host phone/tablet renders also show low contrast in the existing Movies
heading and Loading more text. Their default text styling and KinoTheme are
unchanged from the merged baseline; this batch does not establish full-screen
contrast or TalkBack compliance. The new failed-page message and retry control
remain visibly readable. Preserve this observation for the later Android UI
batch rather than treating a displayed semantics node as contrast proof.

## Reproduction command and evidence limits

From `apps/player/apps/android`, use the cached JDK 17 and Android SDK:

```sh
JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home \
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools \
KINOSAIL_ANDROID_RECOVERY_EVIDENCE=/tmp/android-recovery-evidence \
./gradlew --offline --no-daemon --max-workers=1 \
  -Dorg.gradle.jvmargs=-Xmx1536m :app:testDebugUnitTest \
  --tests '*CatalogRecoveryTest' --tests '*CatalogRetryFooterTest'
```

Schedule this bounded test run with the integration owner. It runs no assembly,
device install, media encoder, or production write. `CatalogRecoveryTest` calls
public model actions. `CatalogRetryFooterTest` operates native phone, tablet and
TV Compose surfaces, captures synthetic host PNG/semantics when the evidence
variable is set, and checks TV focus and a host D-pad center event.

The baseline receipt is `task-2/android-recovery-red/receipt.json`, SHA-256
`61da3a4a2f2b26300ba88ed1d7e99c1f79da0129f9d1dac3a209001957eba0ca`.
Full traces and generated PNGs remain local outside Git. A host-rendered PNG is
not emulator, physical display, decoder, playback continuity, D-pad hardware,
TalkBack, Wear discovery, or deployment evidence. The synthetic HTTP fixture
also cannot prove current Go Server policy or permission handling. No real user
content or credentials are used.
