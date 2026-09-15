---
name: diagnosing-bugs
description: Diagnose persistent, intermittent, or performance bugs that need evidence beyond a straightforward fix.
---

# Diagnose bugs

Build the fastest reliable feedback loop available: a focused test, command, HTTP request, browser action, trace replay, benchmark, or small harness. Source inspection and provisional hypotheses may help construct it; label them as unverified.

Confirm the loop observes the user's symptom. Tighten or minimize it when that materially reduces the search space. For intermittent failures, improve the reproduction rate and record it.

Form as many falsifiable hypotheses as the evidence supports. Test the highest-value discriminating observation first and change one variable at a time. Prefer debuggers or targeted instrumentation; tag temporary logs for cleanup. Measure performance before and after a performance fix.

When a suitable public seam exists, turn the reproduction into a failing regression test before fixing it. If no suitable seam exists, explain the architectural limitation without adding a misleading test.

Before completion:

- rerun the original reproduction and affected tests;
- remove temporary instrumentation and artifacts;
- state the verified cause and evidence;
- redact secrets from commands, outputs, and artifacts.

If an inaccessible environment blocks reproduction, continue useful local investigation and state what evidence or access would resolve the remaining uncertainty.
