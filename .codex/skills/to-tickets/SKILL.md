---
name: to-tickets
description: Split an approved plan or spec into dependency-aware, independently verifiable implementation tickets.
---

# To tickets

Use the supplied plan, spec, issue, or conversation. Create the smallest useful set of vertical slices. Each ticket should deliver behavior that can be demonstrated or verified on its own.

For each ticket include:

- title;
- user-visible outcome;
- acceptance criteria;
- blockers, if any;
- parent reference when one exists.

Use expand-migrate-contract tickets only when a wide mechanical change cannot land green as vertical slices. Do not add speculative prefactoring or list irrelevant layers. Avoid volatile file paths unless they are necessary to disambiguate ownership.

If the user already approved the breakdown, do not ask again. Otherwise present it for review only when ticket boundaries materially affect scope or sequencing.

Publish one item per ticket to the configured tracker when authorized. Use native dependency relationships when available and preserve the parent issue.
