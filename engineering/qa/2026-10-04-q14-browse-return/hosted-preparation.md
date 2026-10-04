# Q14 portable hosted diagnosis preparation

This is a source-only checkpoint. No new Node, collection, Go, browser, lint or
media command ran locally. The preserved pnpm collection timeout executed zero
journeys; Q14 has no accepted public runtime RED and no production repair.

The opt-in driver is `apps/player/scripts/campaign-q14-public.py`. From a clean
Ubuntu 24.04 checkout, run `python3 apps/player/scripts/campaign-q14-public.py`.
Root owns the manual `layout-stability.yml` selection and workflow integration.
Ordinary PR layout behavior and all protected required gates remain mandatory.

Prerequisites are setup-go using `go.work` (Go 1.27), Node 26 on PATH, pnpm
11.22.0 for frozen-lock dependency installation, installed Playwright 1.63.0 and
its Chromium/headless shell. The driver invokes the fixed installed Node CLI
directly, avoiding package-manager resolution during collection and journeys.
It records executable/dependency/browser descriptor and headless-shell byte
hashes. Its browser-cache convention is Linux's default `~/.cache/ms-playwright`
or an explicit `PLAYWRIGHT_BROWSERS_PATH`; Windows and macOS are unverified.

Reuse the committed, previously generated 53,073-byte portrait AVC fixture:
`engineering/qa/2026-09-30-android-playback-overlay/fixtures/Native portrait contrast 01a0f2ab.mp4`.
The exact required SHA-256 is
`507ff669647ce0eda1f8965341a52fbb9a7bc9fe3a4b0fea4efa3933777a50c2`.
Its recorded source is `generated-fixtures.json` in that existing QA directory.
No fixture is copied into this QA directory, encoded, downloaded or regenerated.
The Go owner copies bytes into its disposable temp catalog. FFmpeg/FFprobe remain
unavailable in the fixture config. Primary journeys assert browse, Go Player
navigation, Back URL, extent, focus and scroll; they do not assert media decoding.

The fixed sequence is:

1. Direct Node all-nine collection: three preserved spec files, `--list`, one
   worker, zero retries, Chromium. Command deadline 15 seconds; at most five more
   seconds settle only this driver's process group. Require exactly the original
   nine unique file/title pairs, no executed cases, no global error, and exit zero.
2. Two separate actual Go `TestBrowseReturnBrowserJourney` primary invocations.
   Each selects precisely the original 390px and 1440px visible Back journeys,
   one headless worker, zero retries. `go test -timeout=70s` is the **test binary**
   timer after compilation. Independently, the driver stops the entire command
   group at 70 seconds and allows five seconds for scoped shutdown, bounding
   compilation plus runtime plus shutdown to 75 seconds per repeat.

The sum of these command/shutdown ceilings is 170 seconds. Source/tool hashing,
prerequisite setup and receipt writing are additional work and are not included
in that number. Root's dependency warmup is separately bounded; `go mod download`
does not compile the test package. A cold compile that exhausts the command
deadline is an incomplete prerequisite, never a product RED. Stop after missing
dependencies, incomplete collection/run, explicit journey prerequisites, global
errors or unsettled owned processes. Complete acceptance failures remain
nonzero and require root's boundary review; the driver cannot declare a bug.

The Go harness adds only a proof opt-in reporter/trace selection. The original
nine journey bodies and assertions are byte-preserved since the independently
reviewed collection repair at `437496f1`. The custom reporter publishes bounded
known fixture states, served browse-asset digest, status, retry, duration and
allowlisted failure labels/source locations. It omits error messages, stacks,
URLs/origins, tokens, HTML and arbitrary attachments. State query values are
restricted to fictional fixture values; generated item routes remain opaque.
Existing screenshots/error-context artifacts may remain under the separate
private fictional output tree; the driver does not delete them or publish them.
Raw stdout/stderr stays in memory, contributes only byte count/SHA-256 and is
never written or uploaded. Normal E2E reporters are unaffected.

Upload only these four JSON files:

- `.verification/campaign-proof/Q14/receipt.json`
- `.verification/campaign-proof/Q14/results.json`
- `.verification/campaign-proof/Q14/source-manifest.json`
- `.verification/campaign-proof/Q14/artifact-manifest.json`

The artifact manifest hashes the other three receipts, avoiding self-reference.
The source manifest pins the executed revision and current selected source,
module, workflow, fixture, driver and reporter bytes. Its canonical serialization
is lexically sorted UTF-8 `path<TAB>lowercase SHA-256<LF>`, including the final LF.
Fresh proof paths are required; existing evidence is never overwritten or cleaned.

To deliver Q14 onto root's already integrated Q09 candidate, cherry-pick these
coherent commits in order: `de2f03fa5`, `437496f1`, then this hosted driver commit.
Exclude `f7528afba` (unrelated follow-up preparation). Root's separate route commit
`0fe843fe` follows the driver. Historical Q09 and Q14 manifests stay historical;
the hosted manifest records the exact reconciled head instead of reusing old
hashes as if they were current.

Q09 needs its actual Go owner `download_pause_browser_test.go`, default eight-case
`download-pause.spec.ts`/`download-pause-ownership.spec.ts` and shared
`download-pause-fixture.ts`, composed DownloadsUI/offline modules and both consumer
cache tokens. The additional phone hit-target spec is a separate opt-in one-case
selection, never a replacement for the eight default cases. It generates exact
16,777,247-byte non-playable content; it has no external media dependency.
Original source hashes and 250 artifacts remain frozen at `1b68c55b`; root owns
candidate `061e7a` and its mandatory launcher, so no root Q09 file is changed here.
The original Go owner uses pnpm, making root's reviewed launcher/provenance delta
a new boundary rather than byte-identical replay of the historical Go input.

Remaining gates: independent source review of this driver delta, actual hosted
collection and both complete public repeats, review of intended failure boundary,
then explicitly authorized implementation and normal exact-head protected checks.
Cold Back, HTMX history, Shows, search and real BFCache admission are only collected
in this focused job. Their runtime coverage remains unrun and separately gated.
