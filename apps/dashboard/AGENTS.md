# Dashboard agent guidance

Use the repository-root `engineering/agents/issue-tracker.md`, `triage-labels.md`, and `domain.md` when their workflows apply. Keep agent guidance and decisions under `engineering/`; reserve `docs/` for published user documentation.

- Use Kinosail Dashboard and Viewer Profile consistently.
- Read `CONTEXT.md` when domain vocabulary or behavior is changing.
- Every user-visible capability must be available through `/api/v1`; presentation assets alone are not capabilities.
- Never proxy application content through Dashboard. Persist application state in embedded SQLite.
- The supported installation has one Dashboard container.
- Run `make test-instance-check` for populated browser evidence when a change affects rendered behavior.
