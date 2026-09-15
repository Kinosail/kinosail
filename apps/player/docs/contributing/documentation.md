---
title: Documentation guide
description: Maintain accurate, accessible, task-oriented Kinosail documentation and replace planned screenshots.
section: Project
---

# Documentation guide

Kinosail documentation uses tutorials, task guides, reference, and explanation as separate content types. Each page must have one clear purpose.

## Page rules

- Start task pages with the reader goal and prerequisites.
- Use exact interface labels and verified commands.
- Put warnings before the action they qualify.
- Keep reference factual. Link to a task guide for procedures.
- Use descriptive link text. Preserve one logical heading order.
- Never publish credentials, tokens, private hostnames, or personal media details.

## Screenshot placeholders

Use the shared include until the application design is final:

{% raw %}
```liquid
{% include screenshot.html
  title="Library home"
  alt="Future screenshot of a populated library home."
  description="Show desktop navigation and Continue Watching."
%}
```
{% endraw %}

Replace the placeholder with an optimized local image. Keep the caption and useful alternative text. Do not use a screenshot to carry instructions that the page does not state in text.

## Verify a change

Check front matter, internal links, heading order, code blocks, narrow reflow, keyboard navigation, focus, light and dark themes, and the generated GitHub Pages base path.
