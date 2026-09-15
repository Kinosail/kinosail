---
name: to-spec
description: Turn the current conversation into a concise implementation spec without another interview.
---

# To spec

Synthesize the request, settled decisions, relevant code context, and existing constraints. Do not re-ask answered questions. If a material gap remains, mark it explicitly rather than inventing an answer.

Include, in proportion to the work:

- the user problem and desired behavior;
- concrete acceptance criteria or user stories;
- implementation and contract decisions;
- test boundaries and relevant prior art;
- out-of-scope items and unresolved questions.

Prefer existing public seams. Describe new interfaces only when required. Avoid volatile file paths and code snippets unless they encode a durable decision.

Publish to the configured tracker only when the user requested publication and the destination is known. Otherwise produce the spec in the requested local form.
