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

The current command uses `GOMAXPROCS=2`, `GOPROXY=off`,
`GOTOOLCHAIN=local`, and the repository's `go.work`. It does not override
`GOSUMDB`; normal checksum verification remains active. Historical local
receipts retain their original environment and are never rewritten.
The exact selector and
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

The initial driver has a written failure analysis before implementation, AST
and source/fixture static validation, and a mode that never invokes Go. Source
review found dispatch, provenance, output-validation, and process-settlement
gaps before any hosted execution. Thirteen fictional public-driver controls
were prepared at `d8f5d08ce52a8a61f6ea3664182298d947b9a3df` before correcting
those paths. They are source text under `driver-controls/`. Root executed
12 controls at `f723fa6f`: ten passed, while the normal-success and tracked-drift
controls failed at the driver's process boundary. The 90-second descendant
control was not run in that attempt. These are driver failures, not product
failures. The failed receipts and small fixtures remain preserved under
`.verification/r06-fake-admission-f723/` in the campaign workspace.
They use a small disposable Git fixture and a fictional Go event executable;
their results cannot establish product GREEN. The real Go program is never
invoked by that fixture. The 26 public
operation tests were written before product implementation; their admission,
durability, Owner/CSRF, audio, restart, expiry, and child/Wait assertions remain
unchanged in the portable selector.

The repaired driver supplies the exact default artifact output expected by
root's router. Public execution requires its checkout revision to equal the
valid `GITHUB_SHA`; source-only mode can run outside GitHub. Before/after
revision and whole tracked-tree digests detect drift, excluding only the exact
physical R16 maintenance overlay file, which has separate original hashes.
The Go tool, driver, and two provenance/projection helpers have byte digests.
The 53 protocol pins are selected inputs, not every compiled dependency.
The transient compiled test executable's digest is not captured; the receipt
states that limitation explicitly.

Duplicate package terminals, missing or malformed used fields, and unknown
named-test actions prevent GREEN. Failed tests export only fixed phase hints
and an allowlisted test filename/line when available. Unknown assertions stay
`unclassified-inconclusive`; discarded private output cannot prove their exact
cause. The current process repair uses nonblocking reads and preserves a normal
exit through a natural wait within the remaining 90-second execution bound.
It then proves the complete owned group absent. A deadline still stops that
group when a departed leader leaves a descendant holding stdout. Settlement
allows up to three seconds to reap the leader and four seconds to confirm the
group has stopped; those cleanup bounds are recorded separately. Permission
failures export a fixed failure class and incomplete status, with source and
R16 integrity fields retained. Fourteen fictional controls now include
independent group-absence checks before control cleanup and injected signal
permission failure. The strengthened controls precede this repair; their
runtime result at `ddd85da04ced8b03ee97761a3a1e0d228925dbc4` is 13 PASS and
one FAIL in 97.449 seconds. The departed-leader control reaches its intended
90.003-second timeout and reaps the leader with exit zero. Its owned-group
settlement check receives `PermissionError`, so the unchanged group-absence
assertion fails. This default-sandbox run does not establish complete driver
acceptance. Later absence of both owned PIDs does not replace that assertion.
The original receipt stores the fixed class and phase; errno and the specific
signal call were not captured. Preserve this platform boundary for hosted
evaluation without relaxing typed acceptance.

The safe four-file projections for all 14 cases, original control summary,
separate later PID observation, and SHA manifest are preserved under
`evidence/driver-controls-ddd85da0/`. The manifest pins all 58 supporting JSON
files. The summary SHA is
`0e2ebd0d3c601adfefc7d63062dd6d6b37d63f6140ff64f9702ced5994653977`.
`integration-plan-ddd85da0.json` lists 25 owned commits in order from current
main `ee567e9b9d6b4dc4211055e1f2ef968c5ce63209`, with no changed-main path
overlap and no physical R16 preparation changes. Root integrates its route
separately once and adds this evidence-only checkpoint afterward. No actual
Go has run under this driver. The previous actual public result remains
31 top-level PASS, two early prerequisite FAIL, and 45 subtest PASS.
