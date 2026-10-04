# R09 Android catalog retry recovery

Baseline: `5c3df3a06adad59c65551a65e132a92360b57c4b`.
This commit owns R09 from the canonical ranked audit. R13 is a separate follow-up.
R09 is confirmed and implementing; scheduled GREEN verification is pending.

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
