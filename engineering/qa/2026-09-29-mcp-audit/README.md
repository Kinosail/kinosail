# MCP audit evidence

Revision: `26c9dd0e0f0e75765fd973854e5eb8c3d1bcd3f7`.
Environment: macOS 27.0 arm64, Go 1.27.1, Node 26.8.2.
All tests use temporary app data, synthetic Profiles/tokens, and loopback services or Unix sockets. No production data or credentials were used.

## Regression evidence

Tests were added before the repairs. The initial runs reproduced successful API failure results, refresh-family survival after replay, stale STDIO Owner authority, request floods, capacity leaks, malformed queries, PKCE mismatch, and generic configuration mutation. Protocol checks also reproduced lost request IDs and undeclared prompt behavior. The final focused runs passed.

Repeat from this revision:

```sh
go test -race ./packages/mcpgateway -count=1
go test ./apps/player/internal/server ./apps/subtitles/internal/server -run 'TestMCP|Test.*MCP' -count=1
(cd packages && golangci-lint run --new-from-rev=origin/main ./mcpgateway/...)
make max-loc
python3 scripts/tooling/test-architecture-explorer.py
git diff --check
```

The gateway suite passed with race detection (3.202 seconds). The Player MCP suite passed (5.062 seconds). The Subtitles MCP suite passed (6.506 seconds). Changed gateway lint reported zero issues. File caps, diff checks, and both generated architecture snapshots passed.

The broader command `go test ./packages/... ./apps/player/internal/server ./apps/subtitles/internal/server -count=1` passed both complete server packages and the shared packages except `owneraccess`. Two unchanged private-management tests failed locally: `TestManagementHTTPEnablePairRevokeAndDisable/API` returned 400, and `TestManagementTunnelBindsPeerRevokesLiveStreamAndPreservesPrivateBoundary` could not open UDP port 51821. An isolated rerun also failed private-management setup. The cause was not established by this MCP audit; GitHub's required Linux suites remain the delivery authority.

Both app `make verify-changed` runs passed their file, diff, compile, and focused-server stages. They stopped at shared-package lint on existing issues outside the changed lines. The later shared-consumer and container stages therefore did not run locally. Changed gateway lint passed separately.

## Official conformance runner

Runner source: [modelcontextprotocol/conformance](https://github.com/modelcontextprotocol/conformance), revision `7169291ec0b68eb370fddcd9947313ab0d5e4156`, package `0.2.0-alpha.11`. The npm default package was older and did not support the selected spec flags. The runner was built from this pinned source with `npm ci --ignore-scripts` and `npm run build`.

The saved [host fixture](conformance-host.go.txt) runs the actual gateway through a loopback HTTP server. It injects a synthetic bearer credential for the runner. Authentication and approval behavior are covered by separate regression tests; this fixture does not prove a production login.

| Scenario | Passed | Failed | Skipped | Interpretation |
| --- | ---: | ---: | ---: | --- |
| tools-list | 4 | 0 | 0 | All passed. |
| dns-rebinding-protection | 2 | 0 | 0 | All passed. |
| http-header-validation | 14 | 0 | 0 | All passed. |
| server-stateless | 24 | 4 | 2 | Four diagnostic tools/capabilities were not testable. |
| caching | 4 | 3 | 1 | Three checks require unsupported prompt/resource lists. |

There are 48 successful applicable checks. Raw `*.checks.json` files preserve all results, including failures and skips. No expected-failure baseline was used. The stateless failures require `test_missing_capability`, `test_streaming_elicitation`, or `test_logging_tool`; Kinosail exposes none of these and uses no client sampling, elicitation, or logging capability. The caching failures call prompt/resource methods that the tools-only server correctly rejects. The wire-schema check passed in each applicable scenario. This is partial conformance evidence, not certification or a complete-suite pass.

To repeat:

1. Check out the recorded implementation revision in a leased worktree.
2. Build the runner at its recorded revision in a separate temporary directory.
3. Copy `conformance-host.go.txt` to `packages/mcpgateway/conformance_local_test.go`.
4. Remove old `/tmp/kinosail-mcp-conformance-{url,done}` markers.
5. Run `go test ./packages/mcpgateway -run '^TestLocalConformanceAudit$' -count=1` in the background.
6. Read the local URL from `/tmp/kinosail-mcp-conformance-url`.
7. Run the following command for each listed scenario from the runner directory.
8. Create `/tmp/kinosail-mcp-conformance-done`, wait for the Go test, and remove the temporary Go file.

```sh
node dist/index.js server --url "$MCP_AUDIT_URL" --scenario "$MCP_AUDIT_SCENARIO" --spec-version 2026-07-28 --output-dir /tmp/kinosail-mcp-conformance-results --timeout 15000
```

The JSON summary records the revision, environment, protocol, and runner revision. Required GitHub checks, container publication, deployed health, live OAuth, and device checks are separate facts. No live deployment or device test was performed for this audit.
