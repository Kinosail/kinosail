# Google and Go-project code-hygiene tools

Research snapshot: 2026-08-23. This review compares the current dirty Kinosail checkout with tools published by the Go project or Google and with directly observable practice in a Google-owned open-source Go repository. It does not treat one public repository as proof of a company-wide Google policy.

## Conclusion

Kinosail is not missing a broad Google lint stack. It already covers formatting and imports, `go vet`, Staticcheck, the race detector, native fuzzing, called-code vulnerability analysis, dependency updates, coverage, and several stricter third-party checks. ([formatter and linter configuration](../../.golangci.yml), [`make check`](../../Makefile), [CI hygiene workflow](../../.github/workflows/hygiene.yml), [Dependabot configuration](../../.github/dependabot.yml))

The useful additions, in priority order, are:

1. Run the Go project's `deadcode` tool periodically and after large refactors.
2. Run `go fix -diff ./...` as a reviewed audit when advancing the Go toolchain; optionally enable the already-available `modernize` linter after its initial backlog is resolved.
3. Before distributing releases broadly, add a pinned dependency-license policy check for the packages that build the server binary.

Do not add Bazel or Google's internal analysis infrastructure. The Go team identifies Tricorder as Google's monorepo/code-review batch analysis pipeline and `nogo` as the analogous Bazel/Blaze analysis driver; neither is a missing analyzer, and Kinosail's existing `golangci-lint` runner already composes the relevant public analyzers without changing build systems. [Go analysis-driver overview](https://go.dev/blog/gofix#the-go-analysis-framework) The `rules_go` project itself says `golangci-lint` is appropriate for smaller codebases and positions `nogo` for large Bazel codebases that benefit from incremental builds and caching. [`nogo` and other linters](https://github.com/bazel-contrib/rules_go/blob/master/go/nogo.rst#relationship-with-other-linters)

## What is already covered

| Area | Google or Go-project evidence | Kinosail status |
| --- | --- | --- |
| Formatting | Google's canonical Go style requires `gofmt` output and says this is enforced in Google's presubmit. [Google Go Style Guide](https://google.github.io/styleguide/go/guide.html#formatting) The Go project's `goimports` also formats like `gofmt` while adding missing and removing unused imports. [`goimports` documentation](https://pkg.go.dev/golang.org/x/tools/cmd/goimports) | Covered more strictly by the enabled `gofumpt` and `goimports` formatters. [Configuration](../../.golangci.yml) |
| `go vet` | `vet` reports suspicious constructs not caught by the compiler, while explicitly not claiming program correctness. [`cmd/vet`](https://pkg.go.dev/cmd/vet) | Covered by the enabled `govet` linter. A direct `go vet ./...` probe on this checkout also passed. [Configuration](../../.golangci.yml) |
| Staticcheck and `golangci-lint` | Staticcheck is an independently maintained linter, not a Google-published tool. [Upstream Staticcheck repository](https://github.com/dominikh/go-tools) A current Google-owned project, `google/go-github`, runs `golangci-lint` in CI and enables `govet`, `modernize`, and Staticcheck; this demonstrates public-project use, not an internal Google standard. [`go-github` workflow](https://github.com/google/go-github/blob/master/.github/workflows/linter.yml), [`go-github` linter configuration](https://github.com/google/go-github/blob/master/.golangci.yml) | Covered: Kinosail uses `golangci-lint` v2 and enables `govet`, Staticcheck, `unused`, `unparam`, and many additional checks. [Configuration](../../.golangci.yml) |
| Race detection | Go includes a built-in race detector, invoked for tests with `go test -race`; it only detects races in executed paths. [Race-detector documentation](https://go.dev/doc/articles/race_detector) | Covered across all packages by `make test-race` and CI. [Make target](../../Makefile), [CI workflow](../../.github/workflows/hygiene.yml) |
| Fuzzing | Go's standard toolchain has coverage-guided fuzzing and describes it as especially useful for finding security exploits and vulnerabilities. [Go fuzzing documentation](https://go.dev/doc/security/fuzz/) | Covered, but narrowly: CI runs one 30-second scheduled fuzz target at the backup-restore boundary. Add targets only where a parser or other untrusted-input boundary justifies them; there is no missing fuzzing tool. [CI workflow](../../.github/workflows/hygiene.yml) |
| Go vulnerability scanning | The Go security team curates the Go vulnerability database, and `govulncheck` uses call-graph information to surface vulnerabilities that can actually affect the program. [Go vulnerability management](https://go.dev/doc/security/vuln/) | Covered locally and in CI with `govulncheck ./...`. [Make target](../../Makefile), [CI workflow](../../.github/workflows/hygiene.yml) |

## 1. Add periodic `deadcode`

The Go team publishes `golang.org/x/tools/cmd/deadcode` to report unreachable functions and says it runs the tool periodically, especially after refactors. The analysis begins at a real `main` function and follows the whole program; it can also include generated test mains with `-test`. [Go `deadcode` announcement, usage, limitations, and practice](https://go.dev/blog/deadcode)

A read-only probe of the current Linux production module graph found 18 unreachable Kinosail functions:

```text
go install golang.org/x/tools/cmd/deadcode@latest
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  deadcode \
  -filter='github.com/MikeO7/kinosail-player' ./...

cmd/kinosail/main.go:86:6: unreachable func: newHTTPServer
packages/library/organization.go:127:6: unreachable func: OrganizeArtists
packages/library/organization.go:163:6: unreachable func: OrganizePhotos
internal/server/playback_plan.go:61:6: unreachable func: DecidePlayback
internal/server/playback_plan.go:111:6: unreachable func: selectedAudio
internal/server/playback_plan.go:123:6: unreachable func: selectedSubtitle
internal/server/playback_plan.go:135:6: unreachable func: includes
internal/server/playback_plan.go:145:6: unreachable func: lower
internal/server/playback_plan.go:147:6: unreachable func: minimumPositive
internal/server/playback_plan.go:157:6: unreachable func: playbackReason
packages/wireguard/wireguard.go:108:25: unreachable func: Manager.Config
```

Treat this as an audit result, not an instruction to delete all 18 immediately: the checkout contains extensive uncommitted work, and exported functions may be intentional future or library API. Repeating the module analysis with `-test` reported no unreachable functions, which shows that tests currently reach them but does not make them reachable from the shipped server.

Recommendation: pin a reviewed `deadcode` version and add a manual or scheduled target scoped to `./...`. Start as a report rather than a per-commit hard gate so intentional exported API can be classified without blocking unrelated work.

## 2. Audit with `go fix -diff` at Go upgrades

Go 1.26 rewrote `go fix` around analysis-based modernizers. The Go team recommends running it whenever a project updates to a newer Go toolchain and provides `go fix -diff ./...` as the non-mutating preview. The tool selects fixes based on the module's effective Go version and asks users to review the result. [Go team's `go fix` guidance](https://go.dev/blog/gofix)

Kinosail declares Go 1.27. [Module definition](../../go.mod) A read-only `go fix -diff ./...` probe proposed current-idiom changes including `strings.CutPrefix`/`CutSuffix`, `slices.Contains`, `maps.Copy`, and `strings.SplitSeq`. The installed `golangci-lint` also supports `modernize`; running only that linter reported 18 findings on the current checkout. The upstream analyzer describes these as behavior-preserving simplifications but still requires normal human review because combined fixes can cause build issues or discard valuable comments. [`modernize` analyzer documentation](https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize)

Recommendation: add `go fix -diff ./...` to the Go-version upgrade checklist, not the normal hard CI gate. If the team prefers continuous enforcement, clear the current findings in a dedicated reviewed change and then enable `modernize` in `.golangci.yml`.

## 3. Consider a dependency-license gate

Google publishes `go-licenses`, whose `check` command analyzes a built package's dependencies and fails on forbidden or unknown license classifications; it can include test-only dependencies and explicitly warns when non-Go code prevents complete dependency inspection. [`go-licenses` check documentation](https://github.com/google/go-licenses#checking-for-forbidden-licenses), [`go-licenses` limitations](https://github.com/google/go-licenses#warnings-and-errors)

This read-only probe passed for the current server's third-party Go dependency graph when Kinosail's own proprietary packages were excluded:

```text
go run github.com/google/go-licenses/v2@latest check \
  --ignore github.com/MikeO7/kinosail-player ./cmd/kinosail
```

It warned that assembly in `github.com/coder/websocket` and `golang.org/x/sys/unix` cannot be inspected for further dependencies, matching the tool's documented boundary. Pin the tool version if this becomes CI policy. An alternative for a cross-ecosystem and container-oriented policy is Google's OSV-Scanner, which can recursively scan `go.mod` and other package files, scan container images, and check dependency licenses through deps.dev; it would complement rather than replace the lower-noise Go call analysis already provided by `govulncheck`. [OSV-Scanner source, container, and license capabilities](https://github.com/google/osv-scanner#scanning-a-source-directory)

## Suggested order

1. Add a periodic/reporting-only `deadcode` target and classify the 18 current results.
2. Add `go fix -diff ./...` to Go toolchain upgrade work and review the current modernization diff separately.
3. Decide the allowed dependency-license policy, then add a pinned check before public release.
4. Keep the existing formatter, `golangci-lint`, race, fuzz, and `govulncheck` setup; do not introduce Bazel, `nogo`, or a second general-purpose linter runner.
