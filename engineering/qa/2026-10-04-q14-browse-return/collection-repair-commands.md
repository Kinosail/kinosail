# Q14 pending bounded RED sequence

These commands are prepared source, not executed proof. Start only after an
explicit parent slot grant. Run from the leased checkout root. The runner
preserves unique `.verification/q14-browse-return/<UTC>-<label>` receipts and
uses the standard shared Go cache without changing global configuration.

```sh
python3 engineering/qa/2026-10-04-q14-browse-return/run-bounded.py \
  --cases primary --label primary-red-1 --seconds 75
```

Inspect the complete two-case result before starting the separately preserved
repeat. Stop on a fixture, discovery, collection, navigation prerequisite,
incomplete aggregate, or external-timeout failure. Such failures are not RED.

```sh
python3 engineering/qa/2026-10-04-q14-browse-return/run-bounded.py \
  --cases primary --label primary-red-2 --seconds 75
```

Each process uses Go's 70-second timeout and an external 75-second bound, with
up to five additional seconds solely to stop its task-owned process group.
Initial maximum reservation: 150 seconds of execution plus ten seconds of
possible shutdown. Typical duration is unmeasured. Each run selects exactly
two phone/desktop cases; assertions may fail only after public prerequisites.
Use GOMAXPROCS=2, Go -p1, cached Chromium, one worker, no video, no retries.
The reused fictional clip is checksum-verified before a Go process starts.

After a separate grant, select the following controls individually. Each uses
the same 70/75-second bounds and a unique evidence directory:

```sh
python3 engineering/qa/2026-10-04-q14-browse-return/run-bounded.py --cases cold --label cold-red --seconds 75
python3 engineering/qa/2026-10-04-q14-browse-return/run-bounded.py --cases bfcache --label bfcache-control --seconds 75
python3 engineering/qa/2026-10-04-q14-browse-return/run-bounded.py --cases htmx --label htmx-red --seconds 75
python3 engineering/qa/2026-10-04-q14-browse-return/run-bounded.py --cases shows --label shows-red --seconds 75
python3 engineering/qa/2026-10-04-q14-browse-return/run-bounded.py --cases search --label search-red --seconds 75
```

This list is not permission to run consecutive slots. Cold selects two cases;
BFCache selects one native admission control; HTMX selects one fetched history
case; Shows selects two distinct actions; search selects one changed-URL case.
Root schedules every process. No all-mode runner command is proposed yet.

The receipt pins the exact revision, command, safe selected environment, clip
hash, timestamps, elapsed duration, exit, timeout/interruption, raw report stats,
global errors, and log digest. Preserve raw JSON, captures, traces, and real
Server peer attachments. A nonzero exit alone does not establish intended RED.
Require complete nonzero executed counts, zero skips/flaky/global errors, and
the correct acceptance assertion before changing product behavior.

Cached full Chromium is used only by the BFCache spec with top-level channel
and launch overrides. Require real persisted pageshow, the same document and no
additional root/continuation request. If the browser does not admit that page,
record the prerequisite limit. Do not synthesize a persisted event or describe
rendered fixtures, modified responses, or synthetic lifecycle as native proof.

Record actual browser executable/version/digest and final source manifest with
each granted run. Runtime, source checking, integration, and protected gates
are all pending for the collection repair. Production requires both valid
public RED and the parent's explicit implementation go-ahead.
