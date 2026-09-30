# MCP Events verification

Date: September 29, 2026.
Source revision: `906a30208d071fdc4dece2c5d8e4caf0705b44ee`.
Original baseline: `517fdf6b25047845adefd7bd9489214b82f0fc24`.
Reconciled main: `2245b84bf6ac074d23a8474ef687995b9ba4fa8a`.
Environment: macOS ARM64, Go `1.27.1`, dedicated leased worktree.

## Repeat the HTTP contract journey

From the repository root:

```sh
git show 906a30208d071fdc4dece2c5d8e4caf0705b44ee:packages/mcpgateway/events_test.go
cd packages
go test -race -count=1 ./mcpgateway ./liveevents
```

The fixture uses a Viewer, one built-in OAuth client and grant, and serialized
connection state. Requests exercise the production `/mcp` router. A real local
TLS callback checks HMAC signatures, verification challenges, event IDs, and the
absence of bearer credentials. A test transport routes `receiver.example.test`
to that local callback. This verifies the protocol without claiming public DNS
or an external ChatGPT receiver was exercised.

The journeys cover discovery; subscribe, refresh, and unsubscribe; restart;
Profile and resource filtering; revocation and expiry; signing-key rotation;
rejected input; callback verification failures; durable-save failure; stable
retry bodies; suspension and recovery; an in-flight failure during refresh;
quiet worker cleanup; external OAuth identity; and private-safe diagnostics.
Persisted authority conflicts must prevent subscription loading.

## Results

| Command | Result |
| --- | --- |
| `go test -race -count=1 ./mcpgateway ./liveevents` in `packages` | Passed after reconciliation: gateway 14.171s, liveevents 1.171s |
| `go test ./internal/server -count=1` in Player | Passed: 260.616s, before reconciliation |
| `go test ./internal/server -count=1` in Subtitles | Passed: 231.626s, before reconciliation |
| `golangci-lint run --allow-serial-runners --new-from-rev=origin/main ./mcpgateway/... ./liveevents/...` in `packages` | Passed after reconciliation: zero issues |
| `make max-loc` and `git diff --check` | Passed |
| `make -C apps/player verify-changed` | App cap, diff, compile, and focused checks passed; shared-package full lint blocked by 112 existing issues |
| `make -C apps/subtitles verify-changed` | App cap, diff, compile, and focused checks passed; the same shared-package lint blocked completion |
| Changed-code Subtitles server lint | Passed after reconciliation: zero issues |

The full lint failures concern unchanged code, including identity, Owner access,
public gateway, and existing MCP Profile checks. No gate marker or protection was
changed. Required GitHub checks run complete affected quick suites and populated
Chromium journeys for both apps. Their result belongs to the published PR commit.

New regressions first failed for absent Events discovery, quiet worker cleanup,
unknown event errors, missing Hide notices, refresh versus older failures,
missing safe diagnostics, and conflicting persisted authority. Each passed after
its fix. The Hide regression also proves a rejected stale digest emits no notice.

An initial independent race run timed out during fixture callback verification
with a one-second HTTPS timeout. The fixture now permits five seconds; subsequent
fresh race runs and the independent review passed without race reports.

## Evidence boundaries

- No live ChatGPT plugin callback or user reaction was tested.
- No durable event replay, poll, push, or control notifications are provided.
- External OAuth provider revocation is detected on the next authenticated request;
  local policy and access-token expiry are checked before delivery.
- No production deployment, physical device, or manual browser proof is claimed.
- The primary checkout's unrelated dirty work was preserved.

The feature and optional-extension audit is in
[the research note](../../research/mcp-events-and-extensions-2026-09-29.md).
