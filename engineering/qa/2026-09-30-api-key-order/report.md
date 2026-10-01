# API-key ordering verification

Source commit: `1c86e79365329844fe88c3a78631ef01c032841c`. Baseline: `76739af6cb9581e797766e93854e7fe885a568f5`.

The first PR CI run failed when its clock crossed from September into October. Both apps' shared API-key formatter sorted human-readable dates as text. September sorted before October, so an older key appeared before the newest key. The same defect affected different years, day numbers and times within one displayed day.

The formatter now compares the existing creation timestamps. It retains stable ID order for identical timestamps and keeps every displayed field unchanged. Both Player and Subtitles delegate their settings and API-key views to this shared public operation. No inputs, authorization rules, persistence, transport fields, dependencies or production test seams changed.

## Evidence

The existing formatter test now uses a fixed month boundary. Four added cases cover month, year, day and same-day creation order. They were written before the production repair. All four cases and the existing fixed-month assertion failed on the original owner, then passed after the comparator changed. The original source and test, commands and failure log are retained.

- Focused formatter regressions passed.
- All shared package tests passed: 60 test-bearing packages reported success.
- Both apps' existing API-key server tests passed.
- `make max-loc`, diff checks and root `make tooling-check` passed.
- The generated Code Atlas snapshots changed only three line-count values in each app, reflecting two added production lines.
- The final changed-code package lint passed with zero findings. It uses GitHub's `--new-from-rev origin/main` boundary.

## Local gate limits

Both post-commit `verify-changed` attempts passed source-file caps and diff checks, then failed the full shared-package lint on 112 findings outside the changed files. No report entry names `api_keys.go` or `api_keys_test.go`. The findings are retained, and the full local gate is not claimed as passing. Hosted CI uses changed-code lint and remains pending before publication.

An overlapping local lint attempt hit the shared linter lock. After all local checks finished, the final serial changed-code lint passed. Its failed attempt is retained separately.

No new browser layout, loading, empty-state or recovery behavior changed. These are shared domain and existing server-test results. Physical-device and Nox evidence remain separate.

## Reconciliation with current main

Main advanced through PR381 to `6ce161669ef9730dfa897123c2040375317561bf`, then through PR394 to `f1e7139ca5b38b7db57280adc2c02059b9d3ef34`. It independently includes the timestamp comparator and comprehensive shared date-order tests. The merge retains that implementation and removes this branch's duplicate test table. Both API-key files and Code Atlas snapshots match captured main byte-for-byte. The original source and failure results above remain historical evidence. The final PR production change is the Android photo overlay.

Reconciled shared package tests, main's formatter regression, the latest catalog tests, both changed-app gates and root tooling passed. The first local attempt could not link because the disk was full; it did not execute tests. After verified private backup and reset of the task-owned Apple simulator, serial package tests passed. The simulator recovery archive is private and ignored by Git.
