# Reusable prompt: interactive code architecture explorer

Use this prompt from the repository root when rebuilding or expanding the standalone code documentation.

```text
Create or improve a standalone, interactive HTML document that visualizes this repository's code architecture.

Scope
- This is code documentation only.
- Put every deliverable in `engineering/architecture-explorer/`.
- Do not add routes, templates, handlers, runtime behavior, or UI features to the Kinosail application.
- Keep the result usable as a local static document.
- Preserve unrelated working-tree changes.

Discovery and research
- Inspect the repository's package, module, file, symbol, import, and test structure.
- Search the web for proven interactive code-exploration patterns before deciding the design.
- Prefer primary sources and cite the sources in a Markdown research note in `engineering/research/`.
- Study tools such as CodeSee, Sourcegraph, CodeCanvas, CodeTour, CodeQL path views, GitLens, and relevant accessibility guidance.
- Clearly separate observed facts, inferred relationships, and future ideas.

Experience goals
- Make the explorer feel like an explorable game without sacrificing code accuracy.
- Start with a broad architecture map, then reveal package, file, and symbol detail progressively.
- Add guided missions that answer real architecture questions.
- Show a mission title, progress, current stop, next action, and a readable journey or trail.
- Let users resume, skip, restart, and return to the map.
- Make paths and relationships understandable in text as well as visually.
- Link every available file or symbol to an exact source location.
- Use plain language and explain why each stop matters.

Required capabilities
- A package dependency map with pan, zoom, selection, and visible relationship direction.
- A searchable table view with real headings and a reliable fallback when the graph library fails.
- A package inspector showing responsibility, lines, files, symbols, imports, consumers, and test-file counts.
- File disclosure that shows declaration names and source line numbers.
- Keyboard-accessible controls, visible focus, live status updates, and no hover-only actions.
- Responsive layout that works at desktop, tablet, compact mobile, and 320px widths.
- Reduced-motion behavior and a textual relationship fallback.
- A generated snapshot plus a repeatable generator command.

Implementation guidance
- Choose the graph library that gives the most useful detail for the repository size and interaction model. Evaluate Cytoscape.js and D3 explicitly.
- Keep the generated data model independent from the rendering library.
- Prefer stable facts from repository tooling such as `go list -json ./...` and source parsing.
- Label or explain relationship provenance when a relationship is inferred rather than compiler-accurate.
- Avoid making 3D, animation, scoring, timers, or novelty the default experience.
- Avoid rendering an unfiltered whole-repository graph when a focused neighborhood is more useful.
- Do not make the visual graph the only source of truth.

Deliverables
- `engineering/architecture-explorer/index.html`: generated standalone document.
- `docs/architecture-explorer/index.html`: generated GitHub Pages copy of the standalone document.
- `../../../../scripts/tooling/architecture-explorer-template.html`: shared editable HTML, CSS, and JavaScript template.
- `../../../../scripts/tooling/generate-architecture-explorer.py`: shared repeatable snapshot generator.
- `engineering/architecture-explorer/README.md`: usage and refresh instructions.
- `engineering/research/interactive-code-explorer-patterns.md`: cited research and design rationale.
- `engineering/architecture-explorer/REPEATABLE_PROMPT.md`: this prompt.
- `docs/reference/code-atlas.md`: documentation entry point for the published explorer.

Verification
- Regenerate the HTML from the current repository state.
- Test the populated document in a real browser at 1440x900, 1024x768, 390x844, and 320px reflow where applicable.
- Exercise map selection, mission start, mission continuation, search, file disclosure, source links, table fallback, and keyboard interaction.
- Inspect screenshots yourself and make at least one evidence-based critique and revision pass.
- Check console errors, horizontal overflow, focus visibility, and reduced-motion behavior.
- Run focused source checks, UI lint, `make max-loc`, `KINOSAIL_VERIFY_WORKTREE=1 make verify-changed`, and `git diff --check`.
- Report exactly what passed, what was not run, and any remaining verification boundary.
- If this is an implementation task, commit only owned changes, push to `origin/main`, verify the exact remote SHA, and confirm Nox reports that revision healthy.

Finish by giving the user:
- The local path to the generated HTML.
- The local path to the research note.
- A short summary of the interaction model.
- The web sources used.
- Verification results and any remaining limits.
```
