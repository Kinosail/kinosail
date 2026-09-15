# Kinosail Code Atlas

This folder contains a generated, standalone explorer for the Go package graph. It is code documentation only; it is not part of the Kinosail application.

Open [`index.html`](./index.html) in a browser. The explorer supports:

- Map, Table, and Journey views.
- Four guided missions that move from package boundaries to source files.
- Search, dependency highlighting, pan and zoom, and keyboard node selection.
- Package metrics, incoming and outgoing relationships, file structure, symbols, and source links.
- A table fallback when the graph library cannot load.

The Journey view makes an architecture question concrete. Select a mission, move through its stops, and use the selected package inspector as the evidence panel. The [research notes](../research/interactive-code-explorer-patterns.md) explain the interaction patterns behind this design.

Use [`REPEATABLE_PROMPT.md`](./REPEATABLE_PROMPT.md) to rebuild or expand this documentation in a future run.

## Refresh the snapshot

Run this command from the repository root after package changes. It refreshes both the local engineering copy and the GitHub Pages copy:

```sh
python3 scripts/tooling/generate-architecture-explorer.py player
```

The shared generator reads the Player and `packages` modules from the root template. It records production files, tests, imports, lines, types, functions, and declaration lines. The graph shows only first-party package dependencies; standard-library and third-party imports remain visible in the inspector.

The document loads D3 7.9.0 from jsDelivr with Subresource Integrity for force layout and zoom. The table and inspector remain available if the graph library cannot load.

The published copy lives at [`docs/architecture-explorer/index.html`](../../docs/architecture-explorer/index.html) and is linked from the docs Reference section.
