---
name: app-qa
description: Reproduce real web or native app defects with isolated data and evidence, then add behavior-focused regression tests. Use for a full product audit or a PR/change audit.
---

# App QA

Read [README.md](README.md) for this repository's small configuration section. Confirm the running build's revision and environment before testing. Use a local or staging instance with isolated data for any write, delete, purchase, email, or other consequential flow. Do not infer success from navigation or an HTTP 200: assert the resulting user-visible state and, where relevant, persisted or server state.

## Choose scope

- **Full audit:** map the main journeys from navigation, product requirements, and existing tests. Prioritize core paths and cover each journey's success, failure, and recovery. Record every untested journey.
- **Change audit:** read the PR or diff and its requirements; map changed files and API contracts to affected journeys. Test the change, adjacent journeys, shared navigation/state, and likely regressions. Confirm the running build contains the change.

Plan once in three passes: (1) action and expected outcome, (2) adversarial inputs, roles, permissions, empty and failed states, (3) keyboard, accessibility, viewport/device, console and network. Deduplicate into a journey checklist. Test relevant browser/device combinations, including a phone layout for web and platform focus or gestures for native. Do not treat a skipped check as a pass.

## Explore and capture

For web, use the repository's pinned Playwright and local browser first. Take before/action/after observations; inspect console errors and failed requests; run automated accessibility checks where available and test keyboard navigation directly. Use screenshots and traces or video for failures. A browser automation tool may help exploration, but do not require a paid service or install a whole upstream skill collection.

For native, use an installed compatible device tool or the project's simulator and test tooling. Record app build, OS, device, connection, and whether interactions were simulated or physical. A build alone is not device QA.

For each suspected bug, reproduce from a known state at least twice. Record exact steps, expected and actual behavior, environment, severity, and evidence paths in [report template](references/report-template.md). Mark intermittent or unconfirmed observations **unverified**. Keep evidence free of credentials, tokens, private media, and personal data. Do not change production data to create a repro.

## Convert findings to regression proof

Choose confirmed, high-value defects. Before production fixes, write a test at the strongest observable boundary. Check existing coverage first. The assertion must fail for the defect itself on the unfixed build and pass after a fix; if a red run is impossible, document why. Never weaken or remove the intended assertion to make a failing test pass. Reuse existing fixtures, isolate and reset data, and avoid test-only production seams. Run every new test and the relevant existing suite. Save a run artifact with revision, command, environment, data setup, result, and evidence location.

Fix only straightforward defects with a clear expected outcome. Put larger behavior or product decisions in the report. Report source, browser, simulator, physical device, CI, and deployment proof separately. State coverage gaps and blockers; never claim the app is fully tested.

## Upstream techniques adapted here

- [Vercel agent-browser dogfood](https://github.com/vercel-labs/agent-browser/blob/main/skill-data/dogfood/SKILL.md): systematic journey exploration and repro-first screenshots/video. Its CLI and Claude tool permissions are optional.
- [Browserbase ui-test](https://github.com/browserbase/skills/blob/main/skills/ui-test/SKILL.md): diff mapping, adversarial planning, before/after assertions, accessibility, and responsive checks. Its paid remote browser and mandatory subagent workflow are not required.
- [Playwright Test Agents](https://playwright.dev/docs/test-agents): planner and generator concepts fit the existing fixtures. Do not auto-run `init-agents` or accept healer changes without proving the original assertion still detects the bug.
- [Callstack agent-device dogfood](https://github.com/callstack/agent-device/blob/main/skills/dogfood/SKILL.md): native snapshot/explore/repro loop when a trusted compatible binary is available. Use existing simulator and native tests otherwise.
