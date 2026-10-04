# R06 focused hosted public protocol

This recipe pins the public protocol sources reviewed at
`ac27d7af98624bf333091382aa9a66e9e5a917be`. It selects exactly 26 R06
operation tests and seven incumbent controls, with 45 known named subtests.
The previous complete run remains 31 top-level PASS, two early prerequisite
FAIL, and 45 subtest PASS. The startup prerequisite correction is source-clear
but has not executed. This recipe does not establish runtime acceptance.

## Failure analysis before driver implementation

- A hosted checkout may have different production or test bytes. Verify all
  53 selected source hashes before launch. Record the actual checkout revision
  separately from the reviewed protocol snapshot. A mismatch must stop work.
- Local R16 preparation must never become remote protocol assertions by
  accident. Replace the maintenance test with the exact clean baseline
  (`21659fdc35a9edaa0169d7addebf6f92bc61fca8e40ea364f00c0880ae892bee`)
  through a generated absolute-path Go overlay. Exclude the unpublished
  dashboard revision test. Verify physical originals remain unchanged.
- A clean hosted checkout has the committed maintenance file and no dashboard
  preparation file. Report absence explicitly. Do not claim the local dirty
  originals were present or tested remotely.
- Cold dependencies or compilation can prevent the tests from starting.
  Download dependencies in a separately bounded existing workflow step.
  Keep the actual Go command offline and retain its 80-second timeout and
  90-second external process-group bound. Missing tests and timeouts are
  incomplete results, even when every observed case passes.
- Audio or authentication fixture failures can invalidate later controls.
  Stop the owned process group after a recognized failed prerequisite. Export
  only an allowlisted failure phase, never the raw output that identified it.
- A count alone can hide missing, duplicated, skipped, or unexpected tests.
  Require all 33 exact top-level names and 45 exact subtest names once, all
  passing, plus the package PASS and process exit zero. Any inconsistency
  prevents aggregate GREEN.
- Go JSON output can contain credentials, paths, provider text, and bodies.
  Consume it privately in memory. Export only exact allowlisted test names,
  terminal statuses, finite elapsed values, fixed failure-phase labels, source
  hashes, counts, and fixed execution facts. Do not save or print raw logs.
- Unbounded output can exhaust the driver. Bound individual input lines and
  total captured bytes. Treat overflow or malformed/unexpected events as
  incomplete harness evidence. Do not silently convert them into PASS.
- An interrupted runner can leave descendants active. Put the Go command in
  its own process group; terminate only that group and wait for the owned
  process. No foreign-process termination or cleanup is part of this recipe.
- Artifact reuse can mix old and new evidence. Require a fresh empty output
  directory. Upload only the four explicitly named JSON artifacts, never its
  work directory or a recursive raw-log glob.

## Portable inputs and commands

Use Python 3, Go 1.27, `libarchive-tools` (`bsdtar`), `/bin/sh`, `dd`, `sleep`,
and `/usr/bin/false` on the existing Ubuntu runner. Audio uses disposable
fictional PCM at the real child-process seam; no encoder, playback, provider
account, browser, simulator, device, or deployment is required.

From the repository root, after the existing setup-go step:

```sh
timeout 120s go -C apps/subtitles mod download
python3 apps/subtitles/scripts/campaign-r06-public.py \
  --output .verification/campaign-proof/R06
```

The dependency download is preparation, not test acceptance. A cold compile
inside the driver may still time out; retain that result without weakening
the bound. Root owns the manual-only opt-in job in the existing
`layout-stability.yml`. Its ordinary PR behavior and all normal required
CI gates remain independent and unchanged.

For source-only local validation, use `--static-check` with a separate fresh
output directory. This mode must never invoke Go or run a public test.

The generated Go command runs from `apps/subtitles`:

```text
go test -p 1 -parallel 1 -overlay=<generated portable overlay> \
  ./internal/server -run <protocol-scope.json runPattern> \
  -count=1 -timeout=80s -json
```

The command uses `GOMAXPROCS=2`, `GOPROXY=off`, `GOSUMDB=off`,
`GOTOOLCHAIN=local`, and the repository's `go.work`. The exact selector and
53 source hashes are published in `protocol-scope.json`. The tracked
`subtitle-maintenance-baseline.go.txt` is the clean compilation fixture;
neither dirty R16 original is staged or copied into the published recipe.

## Safe artifacts and remaining gates

Upload only these exact paths, with `if: always()` and the repository's pinned
`actions/upload-artifact` action:

```text
.verification/campaign-proof/R06/receipt.json
.verification/campaign-proof/R06/results.json
.verification/campaign-proof/R06/source-manifest.json
.verification/campaign-proof/R06/artifact-manifest.json
```

The artifact manifest binds the other three files by size and SHA-256.
The portable runtime overlay stays in the preserved work directory and is
not uploaded. The receipt exposes only its digest and relative mapping.
No fixture data, private logs, environment dump, source blob, or secret is
exported. Known test names are fixed by this reviewed selector.

Independent source review must assess the driver, fixture, selector, output
allowlist, process settlement, and actual versus reviewed source provenance
before execution. A complete focused Go PASS covers only this public protocol
boundary. Browser durable recovery, actual playback/transport, summary
preservation, lint, full required gates, integration, merge, and fetched
ancestry remain separate requirements.

The driver has a written failure analysis before implementation, AST and
source/fixture static validation, and a mode that never invokes Go. Its new
timeout, interruption, process-group, malformed-output, unknown-name, and
safe-export controls have not executed. No separate test-first dynamic harness
controls were prepared or run for those paths. That validation gap must remain
explicit during source review and hosted evidence assessment. The 26 public
operation tests were written before product implementation; their admission,
durability, Owner/CSRF, audio, restart, expiry, and child/Wait assertions remain
unchanged in the portable selector.
