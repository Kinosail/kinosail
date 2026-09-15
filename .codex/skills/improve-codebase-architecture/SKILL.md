---
name: improve-codebase-architecture
description: Find and compare concrete opportunities to simplify module interfaces and improve locality or testability.
---

# Improve codebase architecture

Scope the review to a user-named area or recent code hotspots. Read only relevant domain terms and ADRs.

Look for repeated caller knowledge, shallow pass-throughs, scattered change, leaky interfaces, and behavior that cannot be tested through a useful public seam. Apply the deletion test: a valuable module removes complexity rather than moving it to callers.

Present a small ranked set of candidates. For each, include affected files, observed friction, proposed shape, expected locality or testing benefit, risks, and evidence strength. Use the repository's vocabulary. Do not propose a refactor merely to conform to this skill's terminology.

Use a concise Markdown report by default. Create a self-contained local HTML visual only when diagrams materially improve comparison; use local CSS and inline SVG or Mermaid already available in the project, with no new CDN dependency.

If the user selects a candidate, explore competing interfaces when useful and record only durable domain or architecture decisions. Implementation remains a separate action unless requested.
