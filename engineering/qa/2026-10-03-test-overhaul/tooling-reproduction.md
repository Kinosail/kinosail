# Tooling audit reproduction

The audit base is `876b771a7dd0aef5e65957fcee87add0312535e8`. The final
lightweight run exercised committed tooling code at
`be66ecd12aa392ec53e4540fbeab4a9ac8e01b1c`. Only audit receipt/log files changed
while that run wrote its evidence. It ran 38 command groups, all passing. The
CI discovery group contains 49 declarations; metrics contains ten retained Go
declarations and was run separately.

Use `tooling.json` for each declaration's decision, actual important failure,
owner/caller, history, remaining executable proof or gap, and validation command.
The source inventory hashes describe the base, not removed files at the final
revision. Shell tests stub their external commands; none of their passing results
claim a real deployment or product E2E.

The final command was:

```sh
GOCACHE=/tmp/kinosail-tooling-go-cache python3 engineering/qa/2026-10-03-test-overhaul/run-tooling-audit.py final --runtime-workflows /Users/mikeo/Documents/Codex/2026-10-03/task/kinosail/.github/workflows
```

`--runtime-workflows` changed only the existing RuntimeContracts test's workflow
read directory. Its exact app.yml checksum is in `tooling-final.json`. This allowed
the test repair to validate the parent's new evidence upload step while keeping
this audit checkout's own baseline workflow untouched. After integration, omit
the override and validate the workflow in the same checkout.

Python 3.14.7, Node 26.8.2, Go 1.27.1, and macOS 27.0 ARM64 were available. No new
dependencies were installed. The missing ESLint dependency tree was supplied by
an audit-owned symlink at `scripts/quality/node_modules` to the existing tree at
`/Users/mikeo/Documents/GitHub/kinosail/scripts/quality/node_modules`. The runner's
working-status field records that link. The link was removed after the run;
the original tree was not edited. A normal fresh checkout should provision the
quality package's existing frozen dependencies before running its lint tests.

Additional checks observed passing on committed tooling code:

```sh
GOWORK=off GOCACHE=/tmp/kinosail-tooling-go-cache go -C scripts/quality/metrics test ./...
make max-loc
git diff --check
GOCACHE=/tmp/kinosail-tooling-go-cache make -C apps/player verify-changed BASE=876b771a7dd0aef5e65957fcee87add0312535e8
GOCACHE=/tmp/kinosail-tooling-go-cache make -C apps/subtitles verify-changed BASE=876b771a7dd0aef5e65957fcee87add0312535e8
```

An earlier verify-changed invocation used a stale inherited `origin/main` from
the source clone and selected hundreds of unrelated upstream paths. Its broad
Player suite stopped at a sandbox-prohibited TCP listen; the Subtitles run was
interrupted. The pinned base checks above selected the two intended tooling
paths in each app and passed. No product failure was inferred from that invalid
comparison or its environment limitation.

To verify log checksums without running tests again:

```sh
python3 - <<'PY'
import hashlib, json
from pathlib import Path
for phase in ('baseline', 'final'):
    receipt = json.loads(Path(f'engineering/qa/2026-10-03-test-overhaul/tooling-{phase}.json').read_text())
    for result in receipt['results']:
        actual = hashlib.sha256(Path(result['log']).read_bytes()).hexdigest()
        if actual != result['log_sha256']:
            raise SystemExit(f"Checksum mismatch: {result['log']}")
    print(phase, len(receipt['results']), 'log checksums verified')
PY
```

Container/platform cross-builds, real installer packages, browser project
selection, and populated journeys belong to the parent's full required checks.
The ledger explicitly retains their protections and records local run limits.
