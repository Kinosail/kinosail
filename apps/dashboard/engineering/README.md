# Kinosail Dashboard engineering

Start with the [Dashboard README](../README.md) for setup, configuration, lifecycle, and recovery. The [root contribution guide](../../../CONTRIBUTING.md) covers development and Git workflow.

- [Architecture and security](research/architecture-security.md)
- [Product landscape](research/dashboard-landscape.md)
- [Design brief](design/design-brief.md) and [UI contract](../DESIGN.md)
- [Issue workflow](../../../engineering/agents/issue-tracker.md) and [domain vocabulary](../../../engineering/agents/domain.md)
- [Makefile](../Makefile) and [deployment scripts](../scripts/) for app checks and deployment

Dashboard does not currently have separate `docs/`, `adr/`, or release-checklist directories. Use the app README and [cross-app engineering guide](../../../engineering/README.md). Research is dated design context; current source owns runtime behavior. Gates remain disabled while `.gates-disabled` exists.
