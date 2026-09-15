---
name: code-review
description: Review a branch, pull request, commit range, or working tree against repository standards and the requested behavior.
---

# Code review

Review the exact change surface the user named.

## Establish the diff

Resolve a named base to a commit once and use its merge-base with HEAD for branch work. Include staged and unstaged changes for a working-tree review, and inspect relevant untracked files. Do not silently exclude work because it is not committed.

If no base was given, infer the repository's default branch when clear. Ask only when different choices would produce materially different diffs.

## Establish requirements and standards

Use the user's request, linked issue or spec, commit messages, and relevant repository instructions. The conversation is a valid requirements source. If no separate spec exists, continue and state that boundary.

## Review

Evaluate independently:

- **Standards and correctness:** defects, security, data loss, concurrency, performance, maintainability, and documented repository rules.
- **Requested behavior:** missing or partial requirements, incorrect behavior, and unrequested scope.

Use parallel reviewers only when both axes are substantial and delegation is authorized. Verify suspicious behavior with focused tests or code tracing when practical.

Report findings first, ordered by severity within each axis, with file and line references. Distinguish defects from judgment calls. If there are no findings, say so and name remaining test or environment boundaries.
