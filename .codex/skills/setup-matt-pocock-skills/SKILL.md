---
name: setup-matt-pocock-skills
description: Configure a repository's issue tracker, triage labels, and domain-document locations for the bundled engineering skills.
---

# Configure engineering skills

Use this setup only when the user requests it or the repository lacks configuration needed by a chosen workflow.

Inspect existing Git remotes, instruction files, tracker conventions, `CONTEXT.md` or `CONTEXT-MAP.md`, ADR locations, and `engineering/agents/`. Infer settled choices. Ask together only about choices that remain material, such as an unknown tracker or genuinely ambiguous monorepo context.

Update the existing `AGENTS.md` or `CLAUDE.md`; do not create a competing instruction file. Keep its routing block concise and preserve surrounding user content.

Write only the needed files under `engineering/agents/`:

- `issue-tracker.md`
- `triage-labels.md` when triage is installed
- `domain.md`

Use the matching GitHub, GitLab, or local tracker template in this folder as a starting point. For another tracker, record its actual read, write, relationship, and authorization workflow.

Show one concrete draft before writing when unresolved choices remain. Finish by listing the configured consumers and files.
