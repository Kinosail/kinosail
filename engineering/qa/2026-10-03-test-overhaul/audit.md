# Testing overhaul audit

Baseline: `876b771a7dd0aef5e65957fcee87add0312535e8`.
Review PR: https://github.com/Kinosail/kinosail/pull/448.
Each owner inspected candidate assertions, production callers, test history,
remaining keepers and the concrete failure that could escape existing E2E.
Independent reviewers checked removals and changed retained assertion bodies.
Only version-controlled tests and their proven unused helpers were removed.

## Scope and decisions

| Baseline scope | Audited declarations or runtime cases | Removed | Baseline retained |
| --- | ---: | ---: | ---: |
| Player Go | 837 | 63 | 774 |
| Subtitles Go | 954 | 73 | 881 |
| Shared Go packages | 1,900 | 102 | 1,798 |
| Player browser fixtures | 233 | 3 | 230 |
| Subtitles browser fixtures | 74 | 3 | 71 |
| Shared frontend Node | 131 | 1 | 130 |
| Android | 158 | 1 | 157 |
| Swift isolated and native UI journeys | 327 | 0 | 327 |
| Tooling | 193 | 2 | 191 |
| **Total** | **4,807** | **248** | **4,559** |

Go totals include Test, Benchmark and Fuzz declarations. Browser counts include
expanded runtime cases; these are not interchangeable suite-size measurements.
Overlapping owner sub-audits are counted once. New tests from concurrent main
changes are preserved separately and do not inflate the baseline audit counts.
R means an important independent failure boundary. D means an implemented removal.
F retains an existing weak assertion where deleting it would leave a coverage
gap. C retains source pending a demonstrated consolidation or genuine keeper.
No C row counts as a removal. The ledgers explain the gaps rather than claiming
that fixture execution establishes full product equivalence.

## Why isolated exceptions remain

- Identity and authorization tests inject signer, persistence and clock failures;
  check actor/target isolation, credential revocation and fail-closed permissions.
  Successful browser sign-in cannot demonstrate those failure boundaries.
- Subtitles tests protect preserved dialogue, spoken URLs, malicious provider
  results, rollback and write failures. Browser layout fixtures cannot prove
  subtitle data preservation or filesystem fault handling.
- Player tests protect media path traversal, cached HLS repair, cancellation,
  seek state, malformed playback traces, public-listener restrictions and
  WebSocket leadership. The ordinary happy journey misses those injected faults.
- The Supporter tier-catalog copy test detects aliasing that mutates later reads.
  The custom-client test preserves a supplied timeout; it makes no timing claim.
- Native tests guard platform state, cancellation and preparation transitions
  unavailable to Chromium. Existing fullscreen and startup regressions remain.
- Release, installer, security and deployment-tooling contracts protect failure
  and ownership paths that media/browser journeys do not reach.

Existing assertion repairs have a prior written failure analysis and retained
red/green controls. This task adds no post-implementation Go unit declarations.
Root `AGENTS.md` contains the user's three requested rules and distinguishes
response fixtures from journeys using the real populated Server.

## Removal coverage and limits

Per-declaration ledgers name the executable surviving keeper and any lost detail.
Most removals replay an existing stronger isolated boundary; a fixture keeper
remains a fixture. Genuine Server journeys cover fresh installation, MFA sign-in,
library discovery, playback/resume, administration and subtitle review.
Two Subtitles journeys additionally exercise twenty ordered language choices
and the twenty-row Supporter gallery. Exact alternative URL literals and every
standalone SVG serialization detail are not claimed as browser assertions.
The two Player settings searches must execute on fresh prepared-Owner state;
a smoke tag alone previously selected them but allowed them to skip.
Their required result check rejects skips, failed attempts and missing tests.

## Reproduce the audit

Run from the checkout named in the review receipt. Python and Git suffice:

```sh
python3 engineering/qa/2026-10-03-test-overhaul/controls/verify-go-audit-integrity.py --root . --output /tmp/go-audit.json
python3 engineering/qa/2026-10-03-test-overhaul/controls/verify-subtitles-dispositions.py --root .
python3 engineering/qa/2026-10-03-test-overhaul/verify-player-audit.py --checkout . --output /tmp/player-audit.json
python3 engineering/qa/2026-10-03-test-overhaul/verify_shared_ledger.py --require-review-receipts --require-upstream-additions
```

`independent-review/README.md` reproduces physical presence and retained-body
comparisons at a pinned commit. `tooling-reproduction.md` records the exact
tooling commands and logs. Source hash verification does not execute tests.

## Reproduce E2E evidence

CI wraps the actual browser and production-container commands using
`scripts/ci/e2e-artifact.py`. Each private uploaded archive contains the exact
checkout SHA, source status, command, selected environment, tool versions,
container ID/revision, result JSON, logs, available screenshots/traces and
`SHA256SUMS`. Download and extract a fresh archive, then run:

```sh
shasum -a 256 -c SHA256SUMS
```

The final PR records the exact run and artifact links. Every checksum member
must exist, including Playwright's hidden `.last-run.json` files. Earlier run
37150700625 omitted those hidden members; its receipt explicitly marks that
archive incomplete. The uploader repair includes hidden files only inside
disposable synthetic evidence directories. The prepared settings run has its
own HTML, JSON and result directory so it cannot erase the first report.

`local-validation.md` and `populated-http-receipts.json` provide a native Go
Server/FFmpeg reproduction route without containers. Local historical proof is
pinned to its original clean SHA; it is not fresh-head verification. Supported
loopback HTTP uses no browser certificate bypass. Separate container system
checks exercise HTTPS transport; no physical Safari/device or deployment proof
is inferred. Final heavy local builds and `make verify-changed` were unavailable
under the disk-space and local-container restrictions. Required CI gates remain
enabled and must pass before merge.
