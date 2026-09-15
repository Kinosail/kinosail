---
name: codebase-design
description: Design or improve a module's interface, seam placement, depth, and testability.
---

# Codebase design

A deep module provides substantial behavior through a small, coherent interface. Use this vocabulary when it clarifies a design:

- **Interface:** what callers must know, including invariants and errors.
- **Seam:** where behavior can vary without editing the caller.
- **Adapter:** an implementation placed at a seam.
- **Depth:** capability hidden behind a simpler interface.
- **Locality:** related knowledge and change concentrated in one place.

Prefer existing seams. Introduce a new seam only when real implementations or test stand-ins justify it. Test behavior through the module's public interface.

Use ordinary repository terms such as API, service, component, or boundary when they are more precise for that codebase.

Read [DEEPENING.md](DEEPENING.md) for dependency-specific refactors. Read [DESIGN-IT-TWICE.md](DESIGN-IT-TWICE.md) only when multiple competing interface designs would materially improve the decision.
