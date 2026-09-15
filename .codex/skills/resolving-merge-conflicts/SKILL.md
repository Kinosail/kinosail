---
name: resolving-merge-conflicts
description: Resolve an in-progress Git merge or rebase conflict while preserving both changes' intent.
---

# Resolve merge conflicts

Inspect the current Git state, the conflicting hunks, and the commits or task sources that explain each side.

Resolve each hunk by intent. Preserve both intents when compatible; otherwise follow the current task and report the tradeoff. Do not add unrelated behavior.

Run checks affected by the resolution. Stage only conflict resolutions and other task-owned files, preserving any pre-existing staged work. Continue the merge or rebase until Git reports it complete.

Honor an explicit user request to abort. Stop on unexplained HEAD, rebase, or worktree movement and report the state.
