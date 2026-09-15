# Interactive code explorer patterns

- **Research date:** 2026-08-29
- **Scope:** A standalone, documentation-only evolution of Kinosail Code Atlas. This report does not propose app routes or runtime integration.
- **Question:** How can Code Atlas feel like an explorable game while remaining useful, precise, accessible, and maintainable?

## Executive recommendation

Build a layered explorer, not one giant animated graph:

1. **Atlas view:** a stable package and folder overview.
2. **District view:** one selected package, its files, symbols, and immediate neighbors.
3. **Trail view:** a source-to-target path through imports, calls, HTTP routes, storage, or media flow.
4. **Tour view:** an authored, resumable sequence of focused stops with plain-language explanations.
5. **History view:** optional change heatmaps and a time slider for how a path or package evolved.

This is an inference from the strongest patterns below: CodeSee reduces clutter with collapsed folders and “show connected” filtering; Sourcegraph makes precise symbol navigation the core action; CodeQL presents a path as ordered steps; CodeTour stores replayable, line-aware tours; and GitLens keeps history and search in the same workbench ([CodeSee map exploration](https://docs.codesee.io/docs/explore-your-map), [Sourcegraph Code Navigation](https://sourcegraph.com/docs/code-navigation), [CodeQL path queries](https://codeql.github.com/docs/writing-codeql-queries/creating-path-queries/), [Microsoft CodeTour](https://github.com/microsoft/codetour), [GitLens features](https://help.gitkraken.com/gitlens/gitlens-features/)).

The result should feel like a game because it gives the user a map, destinations, trails, discoveries, and progress. It should not require game knowledge, scores, timers, or a 3D camera to communicate basic code facts.

## Proven interaction patterns

### 1. Start broad, then reveal only the relevant neighborhood

CodeSee begins with most folders collapsed. Its file browser can hide selected items, show only connected files, search the tree, and open code from a node. That is a practical progressive-disclosure model for a large repository ([CodeSee map exploration](https://docs.codesee.io/docs/explore-your-map), [CodeSee map best practices](https://docs.codesee.io/docs/first-map-best-practices)).

**Apply to Code Atlas:** keep the current package map as the landing view, then make every selection offer “focus here,” “show callers,” “show dependencies,” and “open one level deeper.” Preserve the user’s prior camera and collapse state when returning from a detail view.

**Do not copy:** a permanently hidden graph. Every filter needs a visible scope indicator and a one-click “show all” or “reset view” action.

### 2. Make navigation semantic, not merely spatial

Sourcegraph’s primary loop is symbol-level: hover for a signature and documentation, go to definition, find references, and find implementations. It supports both search-based navigation and compiler-accurate precise navigation ([Sourcegraph Code Navigation](https://sourcegraph.com/docs/code-navigation), [Sourcegraph feature details](https://sourcegraph.com/docs/code-navigation/features)). SCIP is a language-agnostic indexing protocol intended to power these navigation actions ([SCIP repository](https://github.com/scip-code/scip)).

**Apply to Code Atlas:** a click should answer “what is this?” and “where can I go next?” The inspector should expose definition, incoming references, outgoing references, implementations, owning package, source location, and evidence type. A selected symbol should highlight the smallest useful connected subgraph, not every transitive dependency.

**Important data rule:** label relationships by provenance and confidence. Distinguish parsed import, resolved call, route registration, storage access, and inferred relationship. Never present a heuristic edge as compiler-accurate.

### 3. Treat a path as a first-class object

CodeQL path queries model a source, a sink, and the steps between them. In VS Code, users expand a result, inspect each step, and click a step to jump to its source location ([CodeQL path queries](https://codeql.github.com/docs/writing-codeql-queries/creating-path-queries/), [GitHub data-flow exploration](https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/scan-from-vs-code/explore-data-flow)).

**Apply to Code Atlas:** add a “trace” action with two endpoints and an edge-type selector. Show the trail as both an animated map highlight and a numbered, keyboard-accessible list. Selecting step 4 should focus its node, show the exact file and line, and explain why the edge exists.

**Do not copy:** a path that implies runtime truth when it is only static possibility. CodeQL documents that global flow can be expensive, large, and less precise, and that some behavior is unavailable until runtime ([CodeQL data-flow limits](https://codeql.github.com/docs/writing-codeql-queries/about-data-flow-analysis/)). Use “possible path,” “observed path,” or “indexed path” labels where appropriate.

### 4. Use authored tours as replayable missions

Microsoft CodeTour stores tours as repository files. A step can target a directory, file, line, selection, or explanatory content; tours support titles, reordering, previous/next navigation, keyboard shortcuts, and markers that reveal relevant tours from code locations ([CodeTour repository](https://github.com/microsoft/codetour)). CodeSee also treats a tour as a step-by-step walkthrough of a map ([CodeSee tour guidance](https://docs.codesee.io/docs/first-map-best-practices)).

**Apply to Code Atlas:** define a small JSON or Markdown tour format outside the application. Each stop should contain:

- a human title, such as “The request enters the server boundary”;
- a stable target, such as package, file, symbol, route, or edge;
- a short explanation and optional “why this matters” note;
- an optional task, such as “find the next caller”;
- previous, next, skip, restart, and exit controls; and
- a fallback target when a line moves after regeneration.

Use missions for real questions: “How does a movie request become direct media?”, “Where does owner configuration enter?”, or “What depends on the library scanner?” A mission should teach the explorer’s controls while answering a useful architecture question.

**Game-like layer:** give the user a trail name, progress steps, discovered landmarks, and a “continue where I stopped” state. Avoid points for clicking nodes. Understanding, not activity, is the success condition.

### 5. Add history without replacing the current map

GitLens combines a commit graph, search, visual file history, heatmaps, and deep links. Its visual file history maps contributors over time and uses bubble size and bars to show change magnitude; its graph supports filtering, search, markers, and context menus ([GitLens features](https://help.gitkraken.com/gitlens/gitlens-features/), [GitLens home view](https://help.gitkraken.com/gitlens/home-view/)).

**Apply to Code Atlas:** make “current architecture” the default. Add a lightweight history mode that can answer “when did this package become central?” and “what changed around this trail?” Use a time slider or selected commit only after the user asks for history. Keep current and historical edges visually distinct.

### 6. Choose the rendering library by interaction needs

D3 provides low-level force simulation, link and collision forces, deterministic random sources, and rendering through SVG or Canvas ([D3 force simulation](https://d3js.org/d3-force/simulation), [D3 force module](https://d3js.org/d3-force)). It is a good fit for a custom scene, camera transitions, and bespoke visual storytelling.

Cytoscape.js is a graph-focused library with compound nodes, multiple layouts, selectors, events, viewport controls, and extensible layout algorithms ([Cytoscape.js documentation](https://js.cytoscape.org/)). It is the stronger single foundation if “most detail” means graph semantics, compound package/file/symbol structure, selection, filtering, and layout control. D3 remains useful for a separate custom minimap, timeline, or mission animation.

**Recommendation:** use Cytoscape.js for the graph core if the explorer grows beyond package-level nodes. Keep D3 only where its low-level scene control is materially valuable. Make the graph data model library-neutral so the static document can change renderers without regenerating the analysis snapshot.

## Accessibility and orientation requirements

The graph is supplementary content. W3C guidance for complex images calls for a short description plus a detailed textual representation; the example uses prose and a structured table so the relationships do not depend on visual layout ([W3C complex images](https://www.w3.org/WAI/tutorials/images/complex/)). W3C’s keyboard criterion requires all functionality to have a keyboard equivalent, and the ARIA grid pattern documents efficient arrow-key navigation for dense interactive data ([WCAG keyboard](https://www.w3.org/WAI/WCAG22/Understanding/keyboard.html), [ARIA grid pattern](https://www.w3.org/WAI/ARIA/apg/patterns/grid/)).

Code Atlas should therefore provide:

- a semantic package/file/symbol outline beside the canvas;
- a relationship table with real headings and links;
- a focusable graph surface with a clear selected-node name and relationship summary;
- arrow-key movement or next/previous relationship controls, plus Enter to inspect and Escape to go back;
- a live status line for selection, filters, tour step, and trace results;
- visible labels and patterns in addition to color;
- no hover-only action and no requirement to drag a node;
- reduced-motion behavior that disables camera flights and edge animation;
- a “jump to selected node” action for low-vision users; and
- a persistent “where am I?” breadcrumb showing scope, depth, and active filters.

This fallback is not a second-rate mode. It is also the fastest way to compare exact relationships, copy source locations, and search long symbol names.

## What not to copy

### Do not make 3D the default

Code Park is a useful research reference: it represented classes as rooms, offered bird’s-eye and first-person views, and reported that participants found it engaging and helpful for code understanding ([Code Park paper](https://arxiv.org/abs/1708.02174)). The same paper notes that its camera transitions were at least 1.5 seconds long. That is acceptable for an optional orientation scene, but too slow for repeated definition/reference navigation.

Use a 2D graph and text inspector as the default. Offer a “walk the architecture” scene only for tours, with a skip button, direct jump, reduced-motion mode, and a map view that never loses the user.

### Do not render the whole repository at every detail level

Global data-flow analysis can become large and slow, and Sourcegraph notes that search-based navigation can produce false positives or false negatives while precise navigation needs indexing ([CodeQL data-flow limits](https://codeql.github.com/docs/writing-codeql-queries/about-data-flow-analysis/), [Sourcegraph architecture](https://sourcegraph.com/docs/admin/architecture)).

Precompute stable facts, lazy-load deeper relationships, cap noisy reference lists, and show why a relationship is present. A missing relationship should say “not indexed” or “not resolved,” not silently imply “no relationship.”

### Do not let novelty replace a useful question

CodeSee recommends one topic per map and removing irrelevant items before explaining a flow ([CodeSee map best practices](https://docs.codesee.io/docs/first-map-best-practices)). Follow that principle. Every mission, visual mode, and animation should answer a concrete code question. If a user cannot state the question, return to search, outline, or the package map.

### Do not make the graph the only source of truth

The visual layer should link to exact source locations and preserve a plain-text report of the selected neighborhood or trail. A screenshot, animation, or AI summary can orient the reader; it cannot replace source evidence.

## Suggested Code Atlas roadmap

| Phase | Capability | Evidence of success |
|---|---|---|
| 1 | Collapse/expand hierarchy, focus neighborhood, stable breadcrumbs, semantic outline | A new user can find a package and return to the overview without losing context |
| 2 | Symbol nodes and definition/reference trails with edge provenance | A user can trace a real Kinosail request from entry point to implementation and open every step |
| 3 | External tour files with titles, checkpoints, resume, skip, and keyboard controls | A tour answers one architecture question in under ten deliberate stops |
| 4 | History mode, change heatmap, and shareable deep links | A user can share an exact node, trail, or tour stop and reproduce the same view |
| 5 | Optional Cytoscape.js graph core and custom D3 mission scene | Large graphs remain responsive while the game-like layer remains optional |

## Sources

- [CodeSee: Explore Your Map](https://docs.codesee.io/docs/explore-your-map)
- [CodeSee: First Codebase Map Best Practices](https://docs.codesee.io/docs/first-map-best-practices)
- [Sourcegraph: Code Navigation](https://sourcegraph.com/docs/code-navigation)
- [Sourcegraph: Code Navigation Features](https://sourcegraph.com/docs/code-navigation/features)
- [Sourcegraph: Architecture](https://sourcegraph.com/docs/admin/architecture)
- [SCIP Code Intelligence Protocol](https://github.com/scip-code/scip)
- [GitHub CodeQL: Creating Path Queries](https://codeql.github.com/docs/writing-codeql-queries/creating-path-queries/)
- [GitHub Docs: Exploring Data Flow with Path Queries](https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/scan-from-vs-code/explore-data-flow)
- [CodeQL: About Data Flow Analysis](https://codeql.github.com/docs/writing-codeql-queries/about-data-flow-analysis/)
- [GitLens: Core Features](https://help.gitkraken.com/gitlens/gitlens-features/)
- [GitLens: Commit Graph Is Home](https://help.gitkraken.com/gitlens/home-view/)
- [D3: Force Simulations](https://d3js.org/d3-force/simulation)
- [D3: d3-force](https://d3js.org/d3-force)
- [Cytoscape.js](https://js.cytoscape.org/)
- [Microsoft CodeTour](https://github.com/microsoft/codetour)
- [Code Park: A New 3D Code Visualization Tool](https://arxiv.org/abs/1708.02174)
- [W3C WAI: Complex Images](https://www.w3.org/WAI/tutorials/images/complex/)
- [W3C WCAG 2.2: Keyboard](https://www.w3.org/WAI/WCAG22/Understanding/keyboard.html)
- [W3C ARIA APG: Grid Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/grid/)
