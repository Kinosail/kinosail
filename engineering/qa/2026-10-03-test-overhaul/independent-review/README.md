# Independent integrated review

`receipt.json` records the independent physical removal/keeper review at exact
source SHA `694cf408ecc021d83af3f6c0d64df058a3bdacd7`, compared with baseline
`876b771a7dd0aef5e65957fcee87add0312535e8`. It includes manual review of every
changed retained Go declaration body. The owner semantic ledgers and runtime
artifacts remain separate evidence. This receipt does not claim that source
presence, fixture tests, or mocked APIs are genuine E2E execution.

The generators use only committed Git blobs and ignore dirty worktree files.
They compile no code, install no dependencies, and launch no browsers/devices.
Python 3.9 or newer and Git are sufficient. From a checkout containing the
reviewed commit and this directory, reproduce the three reports:

```sh
python3 engineering/qa/2026-10-03-test-overhaul/independent-review/inventory-go.py 694cf408ecc021d83af3f6c0d64df058a3bdacd7 --repo . --output /tmp/independent-go-inventory.json
python3 engineering/qa/2026-10-03-test-overhaul/independent-review/inventory-other.py 694cf408ecc021d83af3f6c0d64df058a3bdacd7 --repo . --output /tmp/independent-other-inventory.json
python3 engineering/qa/2026-10-03-test-overhaul/independent-review/retained-go-bodies.py 694cf408ecc021d83af3f6c0d64df058a3bdacd7 --repo . --output /tmp/independent-retained-go-bodies.json
shasum -a 256 /tmp/independent-go-inventory.json /tmp/independent-other-inventory.json /tmp/independent-retained-go-bodies.json
```

Expected SHA-256 values are in `receipt.json`. JSON results are deterministic
for the exact source commit and generator contents. The full reports retain
the ledger/file checksums, declaration presence and proof-reference findings.
The Go inventory deduplicates overlapping owner rows by file and current name;
explicit `renamed_to` fields normalize the eleven honest Subtitles renames.
Sub-audit packets do not increase the global declaration counts.

All 236 approved Go deletions are absent, all retained declarations are present,
and all 65 unresolved `C` candidates remain physically retained. The other
inventory checks 1,116 frontend/native/Android/fixture/tooling ledger rows,
including ten deletions, with no missing retained declarations or unresolved
baseline names. All 3,455 retained baseline Go bodies extract successfully;
twelve changed bodies have a manual review in the receipt. Remaining name
warnings are resolved to actual executable keepers there.

This is a review checkpoint. Required CI, genuine browser journeys and final
merged-SHA artifacts must still be confirmed by the coordinator. Native device
runtime is not inferred from declaration presence. No required check was changed
to make a deletion pass.
