---
name: tdd
description: Build or fix behavior test-first when the user requests TDD, red-green-refactor, or integration tests.
---

# Test-driven development

Work in thin behavior slices through a public interface:

1. Select an existing public test seam from the request and code. Ask only when a new interface would materially change behavior or scope.
2. Write one test that fails for the intended reason.
3. Make the smallest production change that passes it.
4. Refactor when it improves the design without changing behavior.
5. Repeat, then run the affected suite.

Tests must observe caller-visible behavior and use independently derived expectations. Avoid private-method tests, tautological assertions, and mocks of internal collaborators. Use a real local stand-in where practical.

Read [tests.md](tests.md) for examples or [mocking.md](mocking.md) when choosing a test double. Read `CONTEXT.md` or nearby ADRs only when their vocabulary or decisions affect the test.
