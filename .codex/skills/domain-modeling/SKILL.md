---
name: domain-modeling
description: Define project-specific domain terms or record a durable architectural decision.
---

# Domain modeling

Use this when terminology or a lasting decision is the work, not merely because another task reads domain documentation.

- Resolve overloaded terms against concrete scenarios and the code's behavior.
- Use the canonical term consistently after it is settled.
- Update the relevant `CONTEXT.md` with domain language, relationships, and invariants; exclude implementation details.
- Create files only when there is something durable to record.
- Record an ADR only when a decision is hard to reverse, surprising without context, and the result of a real tradeoff.

Read [CONTEXT-FORMAT.md](CONTEXT-FORMAT.md) when editing a glossary and [ADR-FORMAT.md](ADR-FORMAT.md) when an ADR qualifies. Ask only when ambiguity materially changes the model.
