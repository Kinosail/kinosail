# Domain Docs

How engineering skills consume this repository's domain documentation.

## Before exploring

- Read the affected app's `CONTEXT.md`: `apps/player/CONTEXT.md` or `apps/subtitles/CONTEXT.md`.
- Read relevant ADRs under the affected app's `engineering/adr/`.
- Create a missing file only when a domain-modeling workflow resolves a term or decision.

## Layout

Each app is its own domain context:

```
/
├── apps/player/
│   ├── CONTEXT.md
│   └── engineering/adr/
├── apps/subtitles/
│   ├── CONTEXT.md
│   └── engineering/adr/
└── packages/
```

Shared `packages/` code uses the vocabulary of the app that consumes it. Note a term that both apps need in each app's `CONTEXT.md`.

## Vocabulary

Use terms defined in the app's `CONTEXT.md` consistently. If a required concept is absent, reconsider whether it reflects repository language or note the gap for domain modeling.

## ADR conflicts

Explicitly identify output that contradicts an existing ADR rather than silently overriding the decision.
