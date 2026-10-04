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
