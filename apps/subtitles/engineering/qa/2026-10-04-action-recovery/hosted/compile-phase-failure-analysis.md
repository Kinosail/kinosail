# R06 separate compilation and unchanged public runtime

The actual hosted run `37231208619` at `8c1a6785` reached the combined
90.029-second external deadline. Its safe artifact `11314200684` contains
17 of 33 top-level PASS and 18 of 45 subtest PASS, no failures, and incomplete
remaining cases. Group settlement and source/R16 integrity passed. This is
incomplete public evidence. Accepted architecture metadata is independent.

Before changing the driver, retain controls for these concrete failures:

- Cold compilation can consume the public runtime budget. Compile once in a
  separate 120-second bounded preparation phase. Keep the original public
  test timeout at 80 seconds and external execution deadline at 90 seconds.
- Compilation failure, timeout, missing output, or symlink output must never
  launch tests. Reuse the owned-group controller and require full settlement.
- Source, checkout, tracked-tree, or R16 drift during compilation must block
  runtime admission. Compilation does not authorize changes to selected input.
- Compiler output can contain private text or even plausible test JSON. It
  stays private and is discarded. Only the public runtime's normal Go JSON
  enters the unchanged exact-name/status projection.
- A digest for a warm-up binary does not prove which program was executed.
  Run the exact separately compiled executable through Go's supported
  `test2json` converter. Pin its regular-file shape, size and digest before
  and after execution. A changed executable prevents GREEN.
- Malformed JSON, duplicate terminals, skipped/missing cases and failed
  prerequisites still prevent GREEN. All existing controls remain intact.
- A separately run test binary can inherit the wrong working directory.
  Go's normal runner sets its command directory to the package directory
  (`src/cmd/go/internal/test/test.go`, Go 1.27.1, line 1673). Preparation stays
  in `apps/subtitles`; runtime must use `apps/subtitles/internal/server`.
  A fictional converter assertion protects this compatibility boundary.
- Departed leaders and inherited stdout still require whole-group absence.
  Preserve prior failed sandbox controls; later PID absence cannot repair them.

The chosen preparation command uses the same reviewed overlay:

```text
go test -c -p 1 -overlay=<same generated overlay> \
  -o <private work/public-server.test> ./internal/server
```

The unchanged runtime contract is passed to the exact executable:

```text
go tool test2json -t -p github.com/MikeO7/kinosail-subtitles/internal/server \
  <same work/public-server.test> -test.v=test2json \
  -test.run=<same33 selector> -test.parallel=1 -test.count=1 -test.timeout=80s
```

Go 1.27.1's local `src/cmd/test2json/main.go` documentation confirms this
command for separately compiled single-package test binaries. Reading that
source did not invoke Go. `-test.v=test2json` retains full-fidelity Go JSON.
The projector remains authoritative and receives only the runtime stream.

Both phases keep `GOMAXPROCS=2`, `GOPROXY=off`, `GOTOOLCHAIN=local`, repository
`GOWORK`, and the existing checksum policy. Each phase has separate fixed
receipt fields and the existing three-second leader/four-second group cleanup
bounds. The compiled file stays in the preserved private work directory; no
binary, raw compiler log, raw test output or extra artifact is uploaded.
Only the existing four safe JSON files are exported.

Prepared fictional controls cover compiler failure, absent/symlink output,
compile-time source drift, JSON emitted only during compilation, executable
mutation during runtime, and a compiler leader whose descendant holds stdout.
The existing runtime 90-second descendant assertion is unchanged. The two
long deadline controls retain their real bounds; short control passes do not
stand in for those clocks or actual Go compilation/public tests.

## Executed fictional controls

The test-first `decf3bc3` preparation reproduced seven missing-phase/provenance
failures against the combined driver. Its summary SHA256 is
`5b2dcbef1418e7b008fe54b87486d8f2667c633924c47cfd2a0d9776aa998746`.
The first separate-phase source checkpoint `14ba8371` passed 19 short
fictional controls. That historical receipt does not cover the later directory
correction.

The added package-directory control at `429ff12d` failed before that repair.
Compilation completed, but the fictional converter rejected the runtime
directory before any declared test PASS. This is a driver compatibility
failure, not a Subtitles test failure. Its safe receipt SHA256 is
`f13d45d60660d4bea543281f962810275b400691e469857b77d5e89360a8d0f1`.

At source-reviewed `bc489e10e8e10c0b721fa61b79a6b886f7e22cf4`, all 20 short
fictional controls passed in 12.510 seconds. The summary SHA256 is
`12da11c8354160faa2496bc18cd09c9e22ef39395a443af3fdd3648fc1b45eb1`.
The driver SHA256 is
`09b63d6e5c1ebbe8bee5faea1dc6f6a6173736c978058f7d45dab52e7aa24efb`.
Compilation still runs from the app directory. Only public runtime runs from
the package directory; both phase directories appear in the safe receipt.

The frozen evidence is under `evidence/compile-phase-bc489e10/`. Its manifest
pins the source helpers, controls, scope, overlay and allowlisted JSON receipts.
All disposable fixtures, private binaries and earlier artifacts remain in
their original `.verification` directories. The preserved long controls were
not run: runtime inherited-stdout deadline 90 seconds and compilation
inherited-stdout deadline 120 seconds.

These controls use fictional Python compiler/converter processes and declared
Go-shaped events. They validate driver admission, projection, privacy and
short process settlement only. No real Go compiler, Go `test2json`, public
Subtitles test, browser, lint or required CI gate ran locally. The next hosted
run must prove the unchanged 33 top-level and 45 subtest contracts from the
exact compiled binary. The incomplete `8c1a6785` hosted receipt stays unchanged.
