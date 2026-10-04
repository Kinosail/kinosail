# Android playback and Home contract repair

## Scope and baseline

This batch addresses R01 and R02 from the ranked audit. The baseline is
`2e9ede47aa56f5177b1e994ffd4b1c62f6c445ce`. The audit remains historical evidence.
Android response validation, focused regressions, and disposable fixtures are in scope.

## Failure analysis before implementation

R01 crosses the public `/api/v1/items/{id}/playback` boundary. The current Server
serializes `policy`, while Android rejects that unknown field before a player can
start. A repair can fail by accepting malformed types or unknown policy values;
rejecting a supported policy; requiring a field omitted by earlier Servers; or
weakening duplicate-key, item identity, URI origin, type, subtitle, timeline, and
response-size validation. Supported values are `automatic`, `direct`, and
`compatible`. An omitted policy must retain the existing acceptance behavior.
The field reports Server settings; accepting it does not alter source selection.

R02 crosses `/api/v1/library`. Home requests 36 category rows and 200 history rows.
A repair can fail by retaining the 24-row cap; accepting rows beyond the requested
limit; trusting a mismatched returned limit or offset; accepting more rows than
the total permits; accepting empty nonterminal pages; admitting duplicate item
IDs or unsafe media URLs; exceeding 200 rows or 2 MiB; or weakening Viewer scope.
One rejected page prevents Home's combined refresh and its cache update.

## Retained isolated checks and E2E gaps

`PlaybackApiTest` exercises the public production decoder. Its policy regression
protects real Server schema acceptance and malformed policy rejection. The
existing Android instrumented journey served a small synthetic playback response
without `policy`; it could not detect the current Server mismatch.

`CatalogPageLimitTest` exercises the public production decoder at Home's limits
and proves that oversized, conflicting, duplicate, unknown, and unsafe rows remain
rejected. The existing catalog and instrumented journeys contain too few rows to
reach Home's 36/200 limits. These isolated cases do not prove device rendering,
the encrypted cache, or physical playback.

The disposable loopback journey uses the real Go Server and the production Kotlin
HTTP client and decoders. It uses committed fictional media, an isolated data
directory, and no production account or personal media. It must reproduce each
reported failure twice before production changes.

## Implementation and observed results

R01 is confirmed and tested. Android accepts an optional `policy` string only when
it is `automatic`, `direct`, or `compatible`. Omitted policy remains supported.
Every other playback validation remains in place. The instrumented journey's
synthetic response now includes `policy` so future device runs cover this field.

R02 is confirmed and tested. Catalog row count uses the validated returned limit,
which must equal the requested limit. Home's 36-row category and 200-row history
pages are accepted. Responses beyond smaller requested limits remain rejected.
The 200-row and 2 MiB caps, row identity, Viewer header, offset, total, unique item,
and media path checks remain in place.

| Check | Before repair | After repair |
| --- | --- | --- |
| Real Server playback and validated media fetch, twice | Rejected before media fetch | Passed twice |
| Real Server movies page, 36 rows, twice | Rejected | Passed twice |
| Real Server library page, 200 rows, twice | Rejected | Passed twice |
| Real Server history page, 200 rows, twice | Rejected | Passed twice |
| Focused decoder JUnit | 3 failures in 11 tests | 11 tests passed |
| `make max-loc` | — | Passed |
| `git diff --check` | — | Passed |
| Python fixture syntax compilation | — | Passed |

Both runtime runs used JDK 17, Kotlin 2.4.20, installed Android Platform 37 stubs,
and a Go Server built from the baseline. The fixture hardlinks 200 copies of the
committed fictional MP4. It writes 200 synthetic progress records through the
production Android progress client, then verifies history through the real Server.
No encoder, emulator, physical device, Nox, deployment, or production request ran.

The red and green runs used the baseline Git revision with working-tree test and
repair changes. Receipts identify exact source SHA-256 values. The unchanged red
decoder checksums match the canonical audit. The green source checksums are:

| Source | SHA-256 |
| --- | --- |
| `PlaybackApi.kt` | `553f1bf17c0b314a4a6bc1b0c76e2be62d7e19e6a0538eb2623f1811f76584f6` |
| `CatalogApi.kt` | `0207d862e60b4007a852db143f3a756dc07ca5c6481bece8a598c680218a6803` |

## Evidence and replay

Local evidence is retained outside the source tree at
`/Users/mikeo/Documents/Codex/2026-10-04/task-2/android-contract-red-4` and
`/Users/mikeo/Documents/Codex/2026-10-04/task-2/android-contract-green`.
Each directory contains a receipt, result projection, compile/JUnit/journey logs,
and disposable Server responses. Raw responses and process logs are not committed.

| Artifact | Red SHA-256 | Green SHA-256 |
| --- | --- | --- |
| `receipt.json` | `98a155727ef5731a595d866a9a906f4878b98c86720882297a564af8a8db9fe4` | `cec893ceb9f4961e69a80b758a55477ca896283a0b5a2a0cf6e34e085f891e02` |
| `android-result.json` | `c292d7d11fac13aa80cd7edb52942a1c62b26eaf4219a101c2acfc6128114765` | `87ff5ee26ebd377ac8b1dc4ab35511b51c6d5db81ef44b755eea9e55340b973b` |
| `junit.log` | `cb866c29878281dfe94a553532668e6c28dcf13c1c50c442a6f193298f235aba` | `26f8bfd1b493e51e5c7afb11e0ba2a9f08fecac1eae8ec2e74017f66318623ff` |
| `journey.log` | `43b839f7e9fe78fab27c2bbcb2ddfcb51435cbeb6d1d73b975df99044bda0c40` | `0b627103b65b2528f402524103eff1053765f5e3f082ad8bbc6877239cc1fe1e` |

Build the existing fixture Server from `apps/player`:

```sh
GOMAXPROCS=2 go build -p 1 -o /tmp/android-contract-server ./scripts/native-playback-contract/server.go
```

Run from the repository root with an unused output directory. Supply the installed
JDK and SDK paths. The runner uses pinned, cached Kotlin and dependency jars.

```sh
python3 apps/player/apps/android/qa/server-contract/run.py /tmp/android-contract-output \
  --server-binary /tmp/android-contract-server \
  --java-home /opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home \
  --sdk-root /opt/homebrew/share/android-commandlinetools
```

## Verification limits and delivery state

The real Server journey proves host HTTP contract acceptance and validated media
fetch. It does not prove ExoPlayer startup, phone/TV rendering, codec support,
encrypted Home cache persistence, authenticated Viewer isolation, or Play release.
The loopback fixture runs in open local Owner mode with the matching Viewer header.
SDK stubs provide Android class signatures; they are not an Android runtime.
Existing isolated rejection tests validate failure outcomes without raw credentials
or personal data. This change does not introduce a new network logging path.

The broader existing Gradle decoder suites are queued for the shared build slot.
Root integration owns independent review, GitHub checks, PR, merge, and main ancestry.
No merge or deployment is claimed by this batch evidence.
