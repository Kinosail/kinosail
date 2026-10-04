# R07 dialogue-pairing evidence

Status: confirmed; public Go and isolated renderer checks pass; native preview journey and hosted gate/review remain pending. Baseline revision: `5409186a5`.

The public preview adapter received an installed synthetic four-cue SRT: an edge credit, two adjacent copies of “Hello there”, and later dialogue. Cleanup plus a 750 ms shift returned HTTP 200, four current cues, and two proposed cues. It supplied no comparison rows. The independently expected rows are removed cue 1, merged source cues 2/3 with proposed cue 1, and source cue 4 with proposed cue 2.

The pre-change public regression failed twice for the intended missing-comparison reason. [API red log](evidence/r07-api-red.log) records both runs. [Baseline manifest](evidence/baseline-manifest.json) identifies and hashes the actual Server-emitted inspector HTML, assets, inspect response, cleanup preview, and unrelated-import preview. The browser red check used these preserved assets and failed at both 390 and 1440 px: the baseline rendered four index-zipped rows instead of three comparisons. [Browser red log](evidence/browser-red.log) and its screenshots, traces, and result receipt preserve the defect. This browser run routed actual pre-change Server outputs and aborted media; it is isolated renderer evidence.

Command, from `apps/subtitles/`:

```sh
GOMAXPROCS=2 KINOSAIL_UI_FIXTURE_DIR="$PWD/engineering/qa/2026-10-04-cue-pairing/evidence/baseline-fixtures" go test -p 1 ./internal/server -run '^TestWriteUIStateFixturesSubtitlePairingPreservesDialogueCorrespondenceThroughCleanup$' -count=2
```

The Go repair carries source indices through optional credit removal and merging, before timing changes. Imported files, drafts, independently edited text, and encoding overrides remain explicitly unpaired. The primary test also requires immutable installed data and no recovery-sidecar creation. A second public case reproduced a missing removed credit inside a merged source group twice. Its [red log](evidence/interior-red.log) and [source receipt](evidence/interior-red-context.json) record the partial mapping failure. The narrow cursor repair then passed both public pairing regressions in 6.370 seconds; [green log](evidence/pairing-green.log) and [source receipt](evidence/pairing-green-context.json) preserve the tested working state.

Browser tests inspect actual generated comparison rows at 390 and 1440 px, source/proposed cue numbering, each side’s seek target, unrelated-import uncertainty, overflow, and accessibility. This transport/render fixture is isolated; it is not populated-server or playable-media E2E proof. Required populated CI and independent review remain with the integration owner.

No production, Nox, device, paid provider, or real-user data was used. Local Go/Chromium checks follow the integration owner’s serial schedule.

The later native fixture is prepared separately. It uses the approved fictional 12-second MP4, copied read-only into disposable app-owned `.verification` and checked against the supplied SHA256. [Data manifest](evidence/native-data-manifest.json) and [runner instructions](native-fixture/README.md) record the setup. The runner hosts the actual Server with actual probe and direct media responses; browser tests use no routes. Native execution is pending the serial Go/browser slot.

Existing inspector ordering, edit-lock, draft retry and page-lifecycle checks passed all 18 cases as Node VM logic verification in 431 ms ([log](evidence/logic-green.log)); no browser was launched. Static JS syntax, API JSON parse, diff-check and max-loc passed. Automatic audio synchronization, concurrent external source rewrites, cross-browser and deployment/device behavior are separate unverified boundaries.

At committed `0ab173dd7a7edbf81510d966bd984e0aeb776fd3`, `GOMAXPROCS=2 golangci-lint run --timeout=45s --new-from-rev=origin/main ./internal/server` printed 0 issues but exited 4 after its deadline. This is unverified, not green. [Timeout log](evidence/lint.log) is preserved unchanged. A later cached retry with an explicit 90-second bound completed with exit 0 and 0 issues ([retry log](evidence/lint-retry.log)).

The affected app's committed-change verification passed max-loc, diff-check, shellcheck, Go compilation (10 seconds), focused Go tests (9 seconds), and browser discovery (3 seconds). Its outer 45-second bound expired before the complete gate returned; the separately surviving container stage then failed at the unavailable Podman socket. [Gate log](evidence/verify-changed.log) retains both facts. No container was created, and a process inventory confirmed that no task check remained running. The complete app gate remains unverified locally; the integration owner retains required hosted checks.

The isolated renderer GREEN passed both 390 and 1440 px, 2/2 with zero skips, flaky cases, or failures in 17.6 seconds ([log](evidence/browser-green.log), [results](evidence/browser-green/results-all.json)). It uses the actual Go-emitted GREEN assets and preview responses, including the captured shell theme dependency. Both cleanup screenshots and the imported-text screenshot were inspected: removed credits remain visible, both merged source lines correspond to the proposed line, and unrelated imported text is explicitly unpaired. Full-page screenshots retain the incumbent fixed navigation at the captured scroll position. Media requests were aborted in this isolated run. Actual Server/media proof remains pending in its separately guarded disposable fixture.
