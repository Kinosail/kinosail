# Local validation receipts

The semantic audit covers every baseline declaration in the owner ledgers.
Inventory-only rows are not retention verdicts.
Per-declaration ledgers identify exact baseline bodies, production callers,
retained proofs, failure boundaries and limits of existing E2E coverage.
See `audit.md` for consolidated counts, retained exceptions and reproduction.

## Reproduce populated browser proof

Use an isolated checkout and a fresh state/output name for every run. Existing
Playwright dependencies, Chrome and FFmpeg are required. No container engine is
used by this route. Build serially with adequate free disk space:

```sh
GOCACHE=/tmp/kinosail-testing-go-cache python3 scripts/ci/e2e-artifact.py --output .verification/test-overhaul/build-player -- go -C apps/player build -o ../../.verification/test-overhaul/build-player/kinosail ./cmd/kinosail
python3 scripts/ci/e2e-artifact.py --output .verification/test-overhaul/player-preservation-NEW -- python3 engineering/qa/2026-10-03-test-overhaul/run-populated-local-e2e.py "$PWD" player player-state-NEW ui-happy-paths.spec.ts title-jump-first-paint.spec.ts --grep 'Dark is the default|populated mobile library'
python3 scripts/ci/e2e-artifact.py --output .verification/test-overhaul/player-happy-NEW -- python3 engineering/qa/2026-10-03-test-overhaul/run-populated-local-e2e.py "$PWD" player player-happy-state-NEW --setup-in-browser happy-path.spec.ts
GOCACHE=/tmp/kinosail-testing-go-cache python3 scripts/ci/e2e-artifact.py --output .verification/test-overhaul/build-subtitles -- go -C apps/subtitles build -o ../../.verification/test-overhaul/build-subtitles/kinosail ./cmd/kinosail
KINOSAIL_UI_FIXTURE_DIR="$PWD/.verification/test-overhaul/ui-fixtures-subtitles" GOCACHE=/tmp/kinosail-testing-go-cache go -C apps/subtitles test ./internal/server -run '^TestWriteUIStateFixturesSubtitleInspector$' -count=1
python3 scripts/ci/e2e-artifact.py --output .verification/test-overhaul/subtitles-preservation-NEW -- python3 engineering/qa/2026-10-03-test-overhaul/run-populated-local-e2e.py "$PWD" subtitles subtitles-state-NEW subtitle-dashboard.spec.ts polish-shell.spec.ts subtitle-inspector-layout.spec.ts --grep 'review starts with sync|administration uses a plain canvas|inspector shell and review states'
```

The runner creates synthetic media and an MFA Owner through the real HTTP API;
`--setup-in-browser` leaves fresh installation onboarding to the browser journey.
It requires a successful build receipt for the same clean source revision and
matches the exact binary checksum before launch. Choose a fresh build output
directory when rebuilding, then place its receipt/binary under `build-player`
or `build-subtitles` for the runner.
It records exact FFmpeg commands, source and binary hashes, tool versions, fixture
hashes, a copied runner/config, server logs, results and screenshots. The outer
recorder rejects source changes during a run and hashes every evidence file.
Check `SHA256SUMS` from inside its artifact directory. Reproduction on another
toolchain can change media byte hashes; commands and actual inputs remain recorded.

The Server uses supported loopback HTTP. Browser certificate-error bypasses are
disabled. These runs make no TLS claim and no physical Safari/device claim.
The 14 inspector layout cases render exported templates with response fixtures;
the subtitle review and administration journeys exercise the real Subtitles CLI,
database, media scanning and API. A template fixture is not populated-server E2E.

## Earlier receipt limitations

`local-player-e2e-complete` passed the real Player happy journey at
`8613c7beb`; `player-theme-firstpaint-initial` passed two populated journeys at
`f0af87c08`. Those earlier browser runs had `ignoreHTTPSErrors=true`,
`--ignore-certificate-errors` and `--allow-insecure-localhost` configured. They
are certificate-verification-bypass runs, not strict browser TLS proof. A separately
CA-verified API/health request does not establish browser trust. Explicit limitation
amendments and revised checksum receipts preserve that distinction.

`subtitles-preservation-http-initial` passed two real Server journeys and 14
template fixture cases at `c635a3cf9` with all browser bypasses disabled. It records
the clean source revision and the generated effective browser configuration.

The initial missing-media/missing-direct-MP4 receipts are setup failures. They are
preserved as failed evidence and are not product-regression or passing-suite claims.
