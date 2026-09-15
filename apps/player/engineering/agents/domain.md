# Domain Docs

How engineering skills consume this repository’s domain documentation.

## Before exploring

- Read `CONTEXT.md` at the repository root when it exists.
- Read relevant ADRs under `engineering/adr/` when they exist.
- If these files do not exist, proceed silently. Domain-modeling workflows create them lazily when terms or decisions are resolved.

## Layout

This is a single-context repository:

```
/
├── CONTEXT.md
├── engineering/adr/
└── src/
```

## Vocabulary

Use terms defined in `CONTEXT.md` consistently. If a required concept is absent, reconsider whether it reflects repository language or note the gap for domain modeling.

## ADR conflicts

Explicitly identify output that contradicts an existing ADR rather than silently overriding the decision.
