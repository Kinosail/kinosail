# Player agent guidance

Use the repository-root `engineering/agents/issue-tracker.md`, `triage-labels.md`, and `domain.md` when their workflows apply. Keep agent guidance and decisions under `engineering/`; reserve `docs/` for published user documentation.

- Use Kinosail, Kinosail Server, Owner, and Viewer Profile consistently.
- Read `CONTEXT.md` when domain vocabulary or behavior is changing.
- Every user-visible capability must be available through `/api/v1`; presentation assets alone are not capabilities.
- The supported self-hosted installation contains one Kinosail Server container, plus a restricted public HTTPS gateway container when public viewing is enabled.
- Preserve Direct First playback. Buffering alone does not authorize transcoding without compatibility or decode evidence.
- For visible work, read `DESIGN.md`, inspect the affected populated flow, and use the local UI skill.
- Use `make verify-changed` before publication. Use `make test-instance-check` for populated browser evidence when the change affects rendered behavior.
- The local watcher deploys remote main to Nox. Prove the remote commit, image revision, container health, and TLS separately.
