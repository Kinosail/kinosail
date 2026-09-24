# Whole-subsystem test audit

Use this mode when the user asks to audit a whole Kinosail subsystem. Apply
the value bar, retention bar, candidate evidence, and validation in
[SKILL.md](SKILL.md). Keep each edit batch coherent, even if the full campaign
needs several PRs.

## 1. Baseline

Pin a `main` commit. Record test and test-support line counts and the
pass/fail state of every in-scope test file. Keep baseline failures separate;
reproduce them before deciding whether the test or product is wrong.

Done when each in-scope file has a baseline result.

## 2. Inventory

Group tests by production owner boundary, including cases in shared packages,
app adapters, native clients, scripts, and browser journeys. Include any
subsystem-specific QA harness. A test file or scenario belongs to one group.

Done when every in-scope test and QA scenario has an owner group.

## 3. Read-only ledger

Read every assigned test, including parameter tables, then inspect its
production owner, entry points, callers, history, and CI routing. Mark each
test declaration with one decision and a short evidence line:

- `R`: retain; name its independent contract and credible regression.
- `F`: retain the contract but repair a weak or misleading assertion.
- `C`: consolidate; name the stronger owner that receives any unique assertion.
- `D`: delete; name the remaining proof or explain why no contract exists.

Judge assertions and exercised inputs, not test names. Use independent review
only when needed and authorized by repository policy.

Done when every declaration has an evidence-backed mark.

## 4. Layer plan

Find suites that replay a shared helper through mocks while a stronger public
boundary already proves the contract. Name the keeper for each contract, the
assertions to move, files or seams to retire, and any CI routing to update.
Correct ledger decisions when the layer review finds new evidence.

Done when each group has a complete keeper and deletion plan.

## 5. Cutover and preservation

Edit one owner group at a time. Serialize changes to shared harness files.
Remove test-only production seams unlocked by the cutover and update test
inventories or CI routing. Run the keeper and sibling tests after each group.

Compare deleted assertions against the keepers before claiming completion.
Repair any lost contract at its owner boundary. For a doubtful repaired
assertion, make a focused control change that should fail, confirm it does,
and restore the production source exactly.

Done when keepers pass and each reported coverage gap is resolved with evidence.

## 6. Product defects and reconciliation

Treat retained baseline failures as possible product bugs. Fix a verified bug
in a separate commit, with a failing pre-fix control and passing candidate.
Record unrelated defects as follow-ups.

Reconcile current `origin/main` through the repository's PR workflow. Port any
new upstream contract into a keeper before retaining a deletion. Rerun the
affected complete suites after reconciliation.

Report the [SKILL.md](SKILL.md) handoff plus baseline and final line counts,
owner groups and keepers, preservation gaps, product fixes, and remaining
verification boundaries.
